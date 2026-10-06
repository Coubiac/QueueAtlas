package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

var ErrInvalidRecoveryOrigin = errors.New("classified current recovery origin is required")

// openedRecovery owns only a temporary proof descriptor, never an ingestor.
// Its state is copied from a completed, serialized LoadRecoveryOrigin scan.
type openedRecovery struct {
	file     *os.File
	identity Identity
	origin   RecoveryOrigin
}

// Close relinquishes ownership before closing, so an error cannot cause a retry.
func (r *openedRecovery) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	f := r.file
	r.file = nil
	return f.Close()
}

// prepareRecoveryCurrent proves only the configured current, never archives.
// Positive checkpoints require Match; explicit lifecycle recovery can verify a
// canonical zero with nonempty physical/prefix evidence, without authorizing
// replay. No Seek, line consumption or durable mutation occurs. On success the
// caller owns Close; every failure closes and joins cleanup errors.
func prepareRecoveryCurrent(ctx context.Context, path string, selected RecoveryOrigin) (*openedRecovery, error) {
	return prepareRecoveryCurrentWith(ctx, path, selected, OpenLog, ObservePath)
}

func prepareRecoveryCurrentWith(ctx context.Context, path string, selected RecoveryOrigin, open func(context.Context, string) (*os.File, Identity, error), observe func(context.Context, *os.File, string) (PathObservation, error)) (result *openedRecovery, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(path) || selected.State == nil || selected.State.Origin.ID == "" || selected.State.Origin.Path != path || selected.Examined < 1 || selected.Examined > MaxPathOrigins {
		return nil, ErrInvalidRecoveryOrigin
	}
	if selected.Status != RecoveryOriginUnknown && selected.Status != RecoveryOriginFollowing || selected.Status == RecoveryOriginUnknown && selected.State.FollowState != source.FollowUnknown || selected.Status == RecoveryOriginFollowing && selected.State.FollowState != source.FollowFollowing {
		return nil, ErrInvalidRecoveryOrigin
	}
	state := *selected.State
	if state.Checkpoint != nil {
		position := *state.Checkpoint
		state.Checkpoint = &position
	}
	selected.State = &state
	f, id, err := open(ctx, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = errors.Join(ErrCurrentMissing, err)
		}
		return nil, err
	}
	owned := &openedRecovery{file: f, identity: id, origin: selected}
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, owned.Close())
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	check, err := VerifyCandidateWithPolicy(f, state, ResumePolicy{AllowZeroCheckpoint: true})
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if check.Status != ResumeMatch && check.Status != ResumeRestartZero {
		status := SelectionInsufficient
		if check.Status == ResumeDifferent {
			status = SelectionDifferent
		}
		return nil, &ResumeDecisionError{Status: status}
	}
	current, err := observe(ctx, f, path)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if current.Status == PathMissing {
		return nil, ErrCurrentMissing
	}
	if current.Status != PathSame || !current.Current.SameFile(id) {
		return nil, ErrPathChanged
	}
	keep = true
	return owned, nil
}
