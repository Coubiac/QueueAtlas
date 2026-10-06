package file

import (
	"context"
	"errors"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

const (
	DefaultRotationGrace = 30 * time.Second
	MinRotationGrace     = MinPollInterval
	MaxRotationGrace     = 24 * time.Hour
)

func resolveRotationGrace(grace time.Duration) (time.Duration, error) {
	if grace == 0 {
		grace = DefaultRotationGrace
	}
	if grace < MinRotationGrace || grace > MaxRotationGrace {
		return 0, errors.New("rotation grace must be between 10ms and 24h")
	}
	return grace, nil
}

// EOF observations only accrue grace for retained files at a complete boundary.
// A partial physical line, including an oversized one, stays protected.
func (g *openedGeneration) observeEOF(current *openedGeneration, now time.Time) {
	if g == current || !g.atCompleteBoundary() {
		g.eofSince = time.Time{}
		return
	}
	g.eofOffset = 0
	if g.ingestor != nil {
		g.eofOffset = g.ingestor.lines.offset
	}
	if g.eofSince.IsZero() {
		g.eofSince = now
	}
}

func (g *openedGeneration) atCompleteBoundary() bool {
	return g.ingestor == nil || g.ingestor.pending == nil && g.ingestor.lines.offset == g.ingestor.position.Offset
}

// retireExpired rechecks the size after waiting, so an intervening append or
// shrink does not discard unread bytes. Stat and close are not atomic: writes
// after this final observation/grace, including during the Sink commit, can still
// be missed. Registered generations acknowledge retirement before close without
// changing their checkpoint. Unregistered empty files have no state to retire.
// A Sink error stops without closing/removing the candidate; the scheduler's
// cleanup owns closing all remaining descriptors. Current is protected
// even if its path is missing. A closed file is removed before any close error
// returns, so cleanup does not attempt a second close.
func retireExpired(ctx context.Context, opened *[]*openedGeneration, current *openedGeneration, grace time.Duration, now time.Time, sink source.Sink) error {
	for i := 0; i < len(*opened); {
		g := (*opened)[i]
		if g == current || g.eofSince.IsZero() || now.Sub(g.eofSince) < grace {
			i++
			continue
		}
		if !g.atCompleteBoundary() {
			g.eofSince = time.Time{}
			i++
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		id, err := Inspect(g.file)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if id.Size != g.eofOffset {
			g.eofSince = time.Time{}
			i++
			continue
		}
		if g.ingestor != nil {
			if err := sink.Commit(ctx, source.Batch{
				Source: g.ingestor.identity,
				FollowTransitions: []source.FollowTransition{{
					OriginID: g.ingestor.position.OriginID, From: source.FollowFollowing, To: source.FollowRetired,
				}},
			}); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		err = g.file.Close()
		*opened = append((*opened)[:i], (*opened)[i+1:]...)
		if err != nil {
			return err
		}
	}
	return nil
}
