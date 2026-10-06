package importfile

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/source/file"
)

const MaxImportFiles = 1000

type Input struct {
	RunID int64
	Path  string
	Gzip  bool // explicit encoding, never inferred from names or headers
}

type Config struct {
	Identity    source.Identity
	Inputs      []Input // explicit order, no sorting or directory enumeration
	MaxFiles    int     // positive, at most MaxImportFiles
	MaxDuration time.Duration
	Limits      GzipLimits // positive content bound; gzip also compressed/ratio
	TempDir     string
}

var ErrImportSourceRunning = errors.New("import source is already running")

// ImportSource applies an explicit bounded list in order, with one shared
// deadline and at most one private copy open. It owns its Attempt objects and
// their ambiguous batches, but neither the Sink nor the StateReader. The caller
// serializes writes to this source across objects and protects filesystem parents.
// The object must not be copied after use; dependencies must honor context.
type ImportSource struct {
	identity source.Identity
	attempts []*Attempt
	duration time.Duration
	running  sync.Mutex
}

var _ source.Source = (*ImportSource)(nil)

// New validates the entire list before any IO, copies configuration, resolves
// paths and requires distinct explicit run IDs within the list. Preserve those
// IDs across retries and restarts. The
// caller chooses positive byte/ratio/duration limits. File count is hard capped;
// content is bounded per file, hence also by count across this finite list.
func New(cfg Config, state AttemptStateReader, normalize file.Normalize) (*ImportSource, error) {
	if cfg.MaxFiles < 1 || cfg.MaxFiles > MaxImportFiles || len(cfg.Inputs) < 1 || len(cfg.Inputs) > cfg.MaxFiles || cfg.MaxDuration <= 0 {
		return nil, errors.New("bounded import file count and positive global duration are required")
	}
	s := &ImportSource{identity: cfg.Identity, duration: cfg.MaxDuration, attempts: make([]*Attempt, 0, len(cfg.Inputs))}
	seen := make(map[int64]struct{}, len(cfg.Inputs))
	for _, input := range cfg.Inputs {
		if _, duplicate := seen[input.RunID]; duplicate {
			return nil, errors.New("import run IDs must be distinct")
		}
		seen[input.RunID] = struct{}{}
		a, err := NewAttempt(AttemptConfig{Identity: cfg.Identity, RunID: input.RunID, Path: input.Path,
			Prepare: PrepareOptions{Gzip: input.Gzip, Limits: cfg.Limits, TempDir: cfg.TempDir}}, state, normalize)
		if err != nil {
			return nil, err
		}
		s.attempts = append(s.attempts, a)
	}
	return s, nil
}

func (s *ImportSource) ID() string { return s.identity.ID }

// Run uses a single deadline for the complete ordered list, bounded further by
// the caller's context. It stops at the first error, without starting later
// inputs. Earlier complete attempts remain durable. Retry this object/list:
// completed attempts do not reopen inputs, and the interrupted attempt retries
// its exact ambiguous batch first. A new Run receives a new global time budget.
// A fresh object after process loss uses durable source/run/path state. There is
// no retry inside one Run, implicit ordering, cross-source dedup or correlation.
func (s *ImportSource) Run(ctx context.Context, sink source.Sink) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sink == nil {
		return errors.New("sink is required")
	}
	if !s.running.TryLock() {
		return ErrImportSourceRunning
	}
	defer s.running.Unlock()
	ctx, cancel := context.WithTimeout(ctx, s.duration)
	defer cancel()
	for _, a := range s.attempts {
		if err := a.Run(ctx, sink); err != nil {
			return err
		}
	}
	return nil
}
