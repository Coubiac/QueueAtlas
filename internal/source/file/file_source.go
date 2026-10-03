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
	Identity     source.Identity
	Path         string
	PollInterval time.Duration // zero selects DefaultPollInterval
	ResumePolicy ResumePolicy
}

var ErrSourceRunning = errors.New("file source is already running")

// ResumeDecisionError contains a fixed diagnostic status, never log content.
type ResumeDecisionError struct{ Status SelectionStatus }

func (e *ResumeDecisionError) Error() string {
	return "file resume decision required: " + string(e.Status)
}

// FileSource orchestrates startup and follows the chosen descriptor. It observes
// path changes but does not switch generations or detect live truncation.
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
	path, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, err
	}
	cfg.Path, cfg.PollInterval = path, interval
	return &FileSource{config: cfg, reader: reader, normalize: normalize}, nil
}

func (s *FileSource) ID() string { return s.config.Identity.ID }

// Run reopens and reconsiders an initially empty file after each cancellable
// wait. Once a generation is usable, it follows that descriptor until error or
// cancellation, then closes it. Unusable decisions and all Sink errors stop Run;
// there is no automatic retry of failed commits or opening errors.
// Path observation errors also stop Run. A missing or replaced regular path
// keeps the descriptor in use; a replacement is not opened in this version.
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
	defer func() { err = errors.Join(err, f.Close()) }()
	start, err := EnsureGenerationWithPolicy(ctx, f, s.config.Identity, s.reader, sink, s.config.ResumePolicy)
	if err != nil {
		return false, err
	}
	if start.WaitingForContent {
		return true, nil
	}
	if start.State == nil {
		return false, &ResumeDecisionError{Status: start.Selection}
	}
	ingestor, err := NewIngestor(ctx, f, s.config.Identity, *start.State, s.normalize)
	if err != nil {
		return false, err
	}
	return false, s.followPath(ctx, f, ingestor, sink, func(ctx context.Context) error {
		return waitForPoll(ctx, s.config.PollInterval)
	})
}
