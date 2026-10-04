package file

import (
	"context"
	"errors"

	"github.com/Coubiac/mailtrace/internal/source"
)

// RecoveryDecisionError is a fixed lifecycle classification, not log content.
type RecoveryDecisionError struct{ Status RecoveryOriginStatus }

func (e *RecoveryDecisionError) Error() string {
	return "file recovery decision required: " + string(e.Status)
}

// RecoverUnknownCurrent explicitly authorizes lifecycle recovery of originID,
// after a complete bounded scan and fresh proof of the configured current.
// Exactly one non-retired origin must match. Unknown commits only its transition
// to Following; an already Following retry verifies again without committing.
// Retired/concurrent/incomplete/unverifiable states refuse without fallback.
//
// No records, checkpoints, provenance or LastPathStatus are changed. Verification
// of canonical zero authorizes only lifecycle; Run remains strict unless replay
// is separately enabled. Every descriptor closes before this object's shared
// guard releases, joining cleanup errors. Sink errors (including EOF/lost ACK)
// stop without retry or compensation; a later invocation reloads committed state.
// The reader and Sink must describe the same authoritative storage. Serialize
// source writes across objects through scan/proof/application. Proofs and the
// filesystem/Commit are bounded observations, not an atomic snapshot or lock.
func (s *FileSource) RecoverUnknownCurrent(ctx context.Context, originID string, sink source.Sink) error {
	return s.recoverUnknownCurrent(ctx, originID, sink, prepareRecoveryCurrent)
}

func (s *FileSource) recoverUnknownCurrent(ctx context.Context, originID string, sink source.Sink, prepare func(context.Context, string, RecoveryOrigin) (*openedRecovery, error)) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if originID == "" {
		return ErrInvalidRecoveryOrigin
	}
	if sink == nil {
		return errors.New("sink is required")
	}
	reader, ok := s.reader.(source.PathStateReader)
	if !ok {
		return ErrPathStateReaderRequired
	}
	if !s.running.TryLock() {
		return ErrSourceRunning
	}
	defer s.running.Unlock()
	selected, err := LoadRecoveryOrigin(ctx, s.ID(), s.config.Path, originID, reader, s.config.ResumeLimits.Origins)
	if err != nil {
		return err
	}
	if selected.Status != RecoveryOriginUnknown && selected.Status != RecoveryOriginFollowing {
		return &RecoveryDecisionError{Status: selected.Status}
	}
	owned, err := prepare(ctx, s.config.Path, selected)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, owned.Close()) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if selected.Status == RecoveryOriginFollowing {
		return nil
	}
	if err := sink.Commit(ctx, source.Batch{
		Source: s.config.Identity,
		FollowTransitions: []source.FollowTransition{{
			OriginID: originID, From: source.FollowUnknown, To: source.FollowFollowing,
		}},
	}); err != nil {
		return err
	}
	return ctx.Err()
}
