package file

import (
	"context"
	"errors"

	"github.com/Coubiac/mailtrace/internal/source"
)

type RecoveryOriginStatus string

const (
	RecoveryOriginUnknown      RecoveryOriginStatus = "unknown"
	RecoveryOriginFollowing    RecoveryOriginStatus = "following"
	RecoveryOriginAbsent       RecoveryOriginStatus = "absent"
	RecoveryOriginConflict     RecoveryOriginStatus = "conflict"
	RecoveryOriginInvalidState RecoveryOriginStatus = "invalid_state"
	RecoveryOriginLimit        RecoveryOriginStatus = "limit_reached"
)

type RecoveryOrigin struct {
	Status   RecoveryOriginStatus
	State    *source.OriginState // owned metadata only for Unknown/Following; not file proof
	Examined int                 // includes retired states
}

// LoadRecoveryOrigin classifies a complete bounded scan for an explicitly named
// origin at one source's exact stored path. Exactly one non-retired origin must
// exist and match originID: Unknown is eligible for verification, Following for
// an idempotent verification. Invalid states, concurrent origins or a retired
// target forbid recovery. An incomplete scan exposes no candidate.
//
// Metadata, including nil/zero checkpoints, remains unverified. No journal is
// opened and no state is written. The reader guarantees the source namespace;
// serialize source writes through subsequent verification and application.
func LoadRecoveryOrigin(ctx context.Context, sourceID, path, originID string, reader source.PathStateReader, limit int) (RecoveryOrigin, error) {
	if err := ctx.Err(); err != nil {
		return RecoveryOrigin{}, err
	}
	if originID == "" {
		return RecoveryOrigin{}, errors.New("explicit recovery origin ID is required")
	}
	loaded, err := LoadPathOrigins(ctx, sourceID, path, reader, limit)
	if err != nil {
		return RecoveryOrigin{}, err
	}
	result := RecoveryOrigin{Examined: loaded.Examined}
	if loaded.Status == PathOriginsLimit {
		result.Status = RecoveryOriginLimit
		return result, nil
	}
	var candidate source.OriginState
	count := 0
	invalid, retiredTarget := false, false
	for _, state := range loaded.States {
		if err := ctx.Err(); err != nil {
			return RecoveryOrigin{}, err
		}
		switch state.FollowState {
		case source.FollowUnknown, source.FollowFollowing:
			count++
			candidate = state
		case source.FollowRetired:
			retiredTarget = retiredTarget || state.Origin.ID == originID
		default:
			invalid = true
		}
	}
	if err := ctx.Err(); err != nil {
		return RecoveryOrigin{}, err
	}
	switch {
	case invalid:
		result.Status = RecoveryOriginInvalidState
	case retiredTarget || count > 1 || count == 1 && candidate.Origin.ID != originID:
		result.Status = RecoveryOriginConflict
	case count == 0:
		result.Status = RecoveryOriginAbsent
	default:
		result.Status = RecoveryOriginUnknown
		if candidate.FollowState == source.FollowFollowing {
			result.Status = RecoveryOriginFollowing
		}
		result.State = &candidate
	}
	return result, nil
}
