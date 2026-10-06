package file

import (
	"context"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

type FollowOriginsStatus string

const (
	FollowOriginsComplete     FollowOriginsStatus = "complete"
	FollowOriginsAbsent       FollowOriginsStatus = "absent"
	FollowOriginsLimit        FollowOriginsStatus = "limit_reached"
	FollowOriginsInvalidState FollowOriginsStatus = "invalid_state"
	FollowOriginsUnknown      FollowOriginsStatus = "unknown_state"
	FollowOriginsCapacity     FollowOriginsStatus = "capacity_exceeded"
)

type FollowOrigins struct {
	Status   FollowOriginsStatus
	States   []source.OriginState // owned candidates, only on Complete
	Examined int                  // all scanned states, including retired/unknown/invalid
}

// LoadFollowOrigins prepares persisted following candidates from a complete,
// bounded LoadPathOrigins scan. Retired states are excluded; any invalid or
// unknown follow state prevents automatic planning. At most MaxOpenGenerations
// following states are returned. Invalid takes precedence over unknown, which
// takes precedence over capacity. An incomplete scan reports Limit only, with
// no candidates. Errors/cancellation discard the entire result.
//
// IDs/dates/offsets do not choose a current or older generation. Checkpoints
// (including nil/zero) and fingerprints remain unverified metadata. Complete
// proves only the stored follow-state set, not file presence or safe resumption.
// Source state writes must be serialized during scan and application; the reader
// guarantees source filtering. No file is opened and no state is written.
func LoadFollowOrigins(ctx context.Context, sourceID, path string, reader source.PathStateReader, limit int) (FollowOrigins, error) {
	loaded, err := LoadPathOrigins(ctx, sourceID, path, reader, limit)
	if err != nil {
		return FollowOrigins{}, err
	}
	result := FollowOrigins{Examined: loaded.Examined}
	if loaded.Status == PathOriginsLimit {
		result.Status = FollowOriginsLimit
		return result, nil
	}
	var candidates []source.OriginState
	following := 0
	invalid, unknown := false, false
	for _, state := range loaded.States {
		if err := ctx.Err(); err != nil {
			return FollowOrigins{}, err
		}
		switch state.FollowState {
		case source.FollowFollowing:
			following++
			if len(candidates) < MaxOpenGenerations {
				candidates = append(candidates, state)
			}
		case source.FollowRetired:
		case source.FollowUnknown:
			unknown = true
		default:
			invalid = true
		}
	}
	if err := ctx.Err(); err != nil {
		return FollowOrigins{}, err
	}
	switch {
	case invalid:
		result.Status = FollowOriginsInvalidState
	case unknown:
		result.Status = FollowOriginsUnknown
	case following > MaxOpenGenerations:
		result.Status = FollowOriginsCapacity
	case following == 0:
		result.Status = FollowOriginsAbsent
	default:
		result.Status, result.States = FollowOriginsComplete, candidates
	}
	return result, nil
}
