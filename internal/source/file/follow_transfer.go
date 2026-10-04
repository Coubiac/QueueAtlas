package file

import (
	"context"
	"errors"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

var ErrInvalidFollowCurrent = errors.New("known current file decision is required")

// FollowOpened resumes a verified set with a known current origin. Before
// transfer, it validates the source identity and reobserves the configured path;
// any rejection leaves set ownership intact. A stale physical identity or origin
// decision returns ErrPathChanged. Missing/new/capacity startup decisions need a
// separate policy and cannot be adopted here.
//
// After validation, the set becomes empty and the scheduler exclusively owns all
// descriptors, closing each once on retirement or return. Close on the emptied
// set is harmless. Existing ingestors resume without registration/acquisition;
// polling, grace, serial commits and checkpoint checks use the normal scheduler.
// LastPathStatus resets only on transfer; rejected attempts preserve it.
// Run and FollowOpened share this object's execution guard. The caller must
// serialize set access and source state writes across other objects. Validation
// is an observation, not a filesystem lock. Run does not invoke this entry yet.
func (s *FileSource) FollowOpened(ctx context.Context, set *OpenedFollowSet, current FollowCurrent, sink source.Sink) error {
	return s.followOpened(ctx, set, current, sink, waitForPoll, time.Now)
}

func (s *FileSource) followOpened(ctx context.Context, set *OpenedFollowSet, current FollowCurrent, sink source.Sink, wait func(context.Context, time.Duration) error, now func() time.Time) error {
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
	if current.Status != FollowCurrentKnown || current.OriginID == "" {
		return ErrInvalidFollowCurrent
	}
	observed, err := set.ObserveCurrent(ctx, s.config.Path)
	if err != nil {
		return err
	}
	if observed.Status != FollowCurrentKnown || observed.OriginID != current.OriginID || !observed.Current.SameFile(current.Current) {
		return ErrPathChanged
	}
	var active *openedGeneration
	for _, generation := range set.opened {
		if generation.ingestor.identity != s.config.Identity {
			return ErrInvalidOpenedFollowSet
		}
		if generation.ingestor.position.OriginID == current.OriginID {
			active = generation
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	opened := set.opened
	set.opened = nil
	s.setPathStatus("")
	return s.followGenerations(ctx, opened, active, sink, wait, now)
}
