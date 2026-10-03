package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

type Config struct {
	Identity      source.Identity
	Path          string
	PollInterval  time.Duration // zero selects DefaultPollInterval
	RotationGrace time.Duration // zero selects DefaultRotationGrace
	ResumePolicy  ResumePolicy
}

var ErrSourceRunning = errors.New("file source is already running")

// ResumeDecisionError contains a fixed diagnostic status, never log content.
type ResumeDecisionError struct{ Status SelectionStatus }

func (e *ResumeDecisionError) Error() string {
	return "file resume decision required: " + string(e.Status)
}

// FileSource orchestrates startup and switches to an observed regular replacement.
// It follows both opened generations, including late writes to a retained file.
// Retained files expire after stable EOF and a grace period. Live truncation
// detection is not implemented yet.
// The caller must serialize state writes for its source ID across all objects;
// Run guards only this object.
// The object must not be copied after use. Dependencies must support context.
type FileSource struct {
	config     Config
	reader     source.StateReader
	normalize  Normalize
	running    sync.Mutex
	pathMu     sync.RWMutex
	pathStatus PathStatus
}

var _ source.Source = (*FileSource)(nil)

// New validates configuration without opening the path or writing state.
// Relative paths are resolved once, so a later working directory change cannot
// redirect the source. Configuration is copied and cannot be mutated via cfg.
func New(cfg Config, reader source.StateReader, normalize Normalize) (*FileSource, error) {
	if cfg.Identity.ID == "" || cfg.Identity.Kind != "file" || cfg.Identity.Name == "" || cfg.Path == "" || reader == nil || normalize == nil {
		return nil, errors.New("file source identity, path, state reader and normalizer are required")
	}
	interval, err := resolvePollInterval(cfg.PollInterval)
	if err != nil {
		return nil, err
	}
	grace, err := resolveRotationGrace(cfg.RotationGrace)
	if err != nil {
		return nil, err
	}
	path, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, err
	}
	cfg.Path, cfg.PollInterval = path, interval
	cfg.RotationGrace = grace
	return &FileSource{config: cfg, reader: reader, normalize: normalize}, nil
}

func (s *FileSource) ID() string { return s.config.Identity.ID }

// Run reopens and reconsiders an initially empty file after each cancellable
// wait. Once a generation is usable, it follows all opened generations, retaining
// an old one on replacement. All descriptors close on error/cancellation.
// Unusable decisions and all Sink errors stop Run;
// there is no automatic retry of failed commits or opening errors.
// Path observation errors also stop Run. A missing path keeps its descriptor in
// use. At most MaxOpenGenerations are retained; exceeding this capacity stops Run.
// A later Run starts from committed state; in-memory pending data is not retained.
func (s *FileSource) Run(ctx context.Context, sink source.Sink) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sink == nil {
		return errors.New("sink is required")
	}
	if !s.running.TryLock() {
		return ErrSourceRunning
	}
	defer s.running.Unlock()
	s.setPathStatus("")
	for {
		f, _, err := OpenLog(ctx, s.config.Path)
		if err != nil {
			return err
		}
		waiting, err := s.runOpened(ctx, f, sink)
		if err != nil || !waiting {
			return err
		}
		if err := waitForPoll(ctx, s.config.PollInterval); err != nil {
			return err
		}
	}
}

func (s *FileSource) runOpened(ctx context.Context, f *os.File, sink source.Sink) (waiting bool, err error) {
	ingestor, waiting, err := s.prepareGeneration(ctx, f, sink)
	if err != nil || waiting {
		return waiting, errors.Join(err, f.Close())
	}
	return false, s.followPath(ctx, f, ingestor, sink, func(ctx context.Context) error {
		return waitForPoll(ctx, s.config.PollInterval)
	})
}

func (s *FileSource) prepareGeneration(ctx context.Context, f *os.File, sink source.Sink) (*Ingestor, bool, error) {
	start, err := EnsureGenerationWithPolicy(ctx, f, s.config.Identity, s.reader, sink, s.config.ResumePolicy)
	if err != nil {
		return nil, false, err
	}
	if start.WaitingForContent {
		return nil, true, nil
	}
	if start.State == nil {
		return nil, false, &ResumeDecisionError{Status: start.Selection}
	}
	ingestor, err := NewIngestor(ctx, f, s.config.Identity, *start.State, s.normalize)
	return ingestor, false, err
}
