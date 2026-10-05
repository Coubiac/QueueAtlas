package importfile

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/source/file"
)

type AttemptStateReader interface {
	source.ImportStateReader
	CheckpointReader
}

type AttemptConfig struct {
	Identity source.Identity
	RunID    int64 // explicit globally unique ID, stable across retries/restarts
	Path     string
	Prepare  PrepareOptions
}

var ErrAttemptRunning = errors.New("import attempt is already running")
var ErrAttemptFailed = errors.New("import attempt is durably failed")
var ErrImportDeadlineRequired = errors.New("import requires a context deadline")

// Attempt owns the lifecycle of one explicit file import, not multiple inputs.
// The caller protects input/database/temp parents, serializes this source's writes
// across objects and supplies a context deadline. The object must not be copied
// after use. Dependencies must honor context; regular-file syscalls can block.
type Attempt struct {
	config    AttemptConfig
	state     AttemptStateReader
	normalize file.Normalize
	running   sync.Mutex
	pending   *source.Batch
	failure   error
}

var _ source.Source = (*Attempt)(nil)

// NewAttempt validates dependencies and resolves paths once without opening an
// input or writing state. Gzip is explicit; no filename inference or sorting.
func NewAttempt(cfg AttemptConfig, state AttemptStateReader, normalize file.Normalize) (*Attempt, error) {
	if cfg.Identity.ID == "" || cfg.Identity.Kind != "import" || cfg.Identity.Name == "" || cfg.RunID <= 0 ||
		cfg.Path == "" || state == nil || normalize == nil || cfg.Prepare.Limits.ContentBytes < 1 ||
		cfg.Prepare.Gzip && (cfg.Prepare.Limits.CompressedBytes < 1 || cfg.Prepare.Limits.MaxRatio < 1) {
		return nil, errors.New("import identity, explicit run, path, state, normalizer and positive limits are required")
	}
	path, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, err
	}
	cfg.Path = path
	if cfg.Prepare.TempDir != "" {
		cfg.Prepare.TempDir, err = filepath.Abs(cfg.Prepare.TempDir)
		if err != nil {
			return nil, err
		}
	}
	return &Attempt{config: cfg, state: state, normalize: normalize}, nil
}

func (a *Attempt) ID() string { return a.config.Identity.ID }

// Run creates/loads a source-scoped attempt, prepares the whole regular input,
// proves/acknowledges its content binding and applies finite records. It closes
// and removes its private copy on every exit, joining cleanup errors. A durable
// complete run returns success without reopening input; the same ID identifies
// that attempt, not a new request to inspect a changed path. Failed runs cannot
// revive. Reimport requires a new explicit ID and can reuse a proven checkpoint.
//
// A Sink error stops Run. The exact ambiguous batch stays on this object across
// runs and is retried before state lookup or input opening. After ACK, fresh
// durable state and a new validated copy govern continuation. After process loss
// only committed state survives; the original content must validate again to
// resume running attempts. No automatic retry, no compensation for commit errors.
// Preparation failure of an unprepared attempt is traced failed, with its own
// retained batch if that write fails. Context interruption and prepared resume
// failures stay running. A context deadline is required before any state write.
func (a *Attempt) Run(ctx context.Context, sink source.Sink) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sink == nil {
		return errors.New("sink is required")
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrImportDeadlineRequired
	}
	if !a.running.TryLock() {
		return ErrAttemptRunning
	}
	defer a.running.Unlock()
	if err := a.commitPending(ctx, sink); err != nil {
		return err
	}
	run, found, err := a.state.ImportRun(ctx, a.ID(), a.config.RunID)
	if err != nil {
		return err
	}
	if found {
		if run.ID != a.config.RunID || run.SourceID != a.ID() || run.Path != a.config.Path {
			return ErrImportResume
		}
		switch run.Status {
		case source.ImportComplete:
			return nil
		case source.ImportFailed:
			return errors.Join(ErrAttemptFailed, a.failure)
		case source.ImportRunning:
		default:
			return ErrImportResume
		}
	} else {
		run = source.ImportRun{ID: a.config.RunID, SourceID: a.ID(), Path: a.config.Path,
			Status: source.ImportRunning, CreatedAt: time.Now().UTC()}
		a.pending = &source.Batch{Source: a.config.Identity, ImportChange: &source.ImportChange{Target: run}}
		if err := a.commitPending(ctx, sink); err != nil {
			return err
		}
	}
	prepared, err := PrepareRegular(ctx, a.config.Path, a.config.Prepare)
	if err != nil {
		if run.Content != nil || ctx.Err() != nil {
			return err // no terminal compensation for an unavailable resume input
		}
		a.failure = err
		target := run
		stamp := time.Now().UTC()
		if stamp.Before(run.CreatedAt) {
			stamp = run.CreatedAt
		}
		target.Status, target.CompletedAt = source.ImportFailed, &stamp
		a.pending = &source.Batch{Source: a.config.Identity, ImportChange: &source.ImportChange{Before: &run, Target: target}}
		return errors.Join(err, a.commitPending(ctx, sink))
	}
	defer func() { err = errors.Join(err, prepared.Close()) }()
	binding, err := PrepareBinding(ctx, prepared, a.config.Identity, run, a.state, a.normalize)
	if err != nil {
		return err
	}
	ingestor, err := binding.Commit(ctx, sink)
	if err != nil {
		a.pending = binding.pending
		return err
	}
	for {
		err := ingestor.CommitNext(ctx, sink)
		if ingestor.pending != nil {
			a.pending = ingestor.pending
			if a.pending.ImportChange.Target.Status == source.ImportFailed {
				a.failure = ErrImportPartial
			}
			return err
		}
		switch ingestor.run.Status {
		case source.ImportComplete:
			return nil // state proves this EOF was acknowledged, not a Sink error
		case source.ImportFailed:
			a.failure = ErrImportPartial
			return ErrImportPartial
		}
		if err != nil {
			return err
		}
	}
}

func (a *Attempt) commitPending(ctx context.Context, sink source.Sink) error {
	if a.pending == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := sink.Commit(ctx, *a.pending); err != nil {
		return err
	}
	a.pending = nil
	return nil
}
