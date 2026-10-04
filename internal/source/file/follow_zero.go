package file

import (
	"context"
	"errors"
	"io/fs"

	"github.com/Coubiac/mailtrace/internal/source"
)

// Called only after a complete lifecycle scan found exactly one following state
// with a present zero checkpoint and an explicitly enabled replay policy.
func prepareZeroFollowResume(ctx context.Context, identity source.Identity, path string, state source.OriginState, normalize Normalize, observe func(context.Context, *OpenedFollowSet, string) (FollowCurrent, error)) (result FollowResume, err error) {
	f, id, err := OpenLog(ctx, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = errors.Join(ErrCurrentMissing, err)
		}
		return FollowResume{}, err
	}
	generation := &openedGeneration{file: f, identity: id}
	owned := &OpenedFollowSet{opened: []*openedGeneration{generation}}
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, owned.Close())
		}
	}()
	if err := ctx.Err(); err != nil {
		return FollowResume{}, err
	}
	check, err := VerifyCandidateWithPolicy(f, state, ResumePolicy{AllowZeroCheckpoint: true})
	if err != nil {
		return FollowResume{}, err
	}
	if check.Status != ResumeRestartZero {
		status := SelectionInsufficient
		if check.Status == ResumeDifferent {
			status = SelectionDifferent
		}
		return FollowResume{}, &ResumeDecisionError{Status: status}
	}
	prefix, err := ParsePrefixFingerprint(state.Origin.Fingerprint)
	if err != nil {
		return FollowResume{}, err
	}
	generation.zeroReplayPrefix = &prefix
	generation.ingestor, err = NewIngestor(ctx, f, identity, state, normalize)
	if err != nil {
		return FollowResume{}, err
	}
	current, err := observe(ctx, owned, path)
	if err != nil {
		return FollowResume{}, err
	}
	if err := ctx.Err(); err != nil {
		return FollowResume{}, err
	}
	if current.Status == FollowCurrentMissing {
		return FollowResume{}, ErrCurrentMissing
	}
	if current.Status != FollowCurrentKnown || current.OriginID != state.Origin.ID || !current.Current.SameFile(id) {
		return FollowResume{}, ErrPathChanged
	}
	keep = true
	return FollowResume{Status: FollowResumeReady, Opened: owned, Current: current}, nil
}

// A zero replay has no acknowledged anchor. Keep its validated prefix evidence
// until a positive checkpoint is acknowledged. ReadAt preserves reader offsets;
// mismatches remain a replay refusal, not a completed archive search gap.
func checkZeroReplayPrefixes(ctx context.Context, opened []*openedGeneration) error {
	for _, generation := range opened {
		if err := ctx.Err(); err != nil {
			return err
		}
		if generation.zeroReplayPrefix == nil || generation.ingestor == nil || generation.ingestor.Position().Offset != 0 {
			continue
		}
		match, err := generation.zeroReplayPrefix.Matches(generation.file)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !match {
			return &ResumeDecisionError{Status: SelectionDifferent}
		}
	}
	return nil
}
