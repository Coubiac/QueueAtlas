package file

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

var ErrInvalidFollowCurrent = errors.New("valid current file decision is required")

// ErrCurrentMissing refuses startup without inventing a current generation.
// The opened set stays owned by its caller, which can close it or observe again.
// The diagnostic contains no path or log content.
var ErrCurrentMissing = errors.New("configured current log is missing at startup")

// FollowOpened resumes a verified set with a known or new current file. Before
// transfer, it validates the source identity and reobserves the configured path;
// any rejection leaves set ownership intact. A stale physical identity or origin
// decision returns ErrPathChanged. New requires one retained file, checks its
// size/anchor and opens the current file with matching physical identity before
// transfer. Capacity is checked before opening. A Missing decision is reobserved
// and refused with ErrCurrentMissing, preserving ownership; if the path returns,
// ErrPathChanged requires a fresh decision. No retry/wait or current choice is
// made for Missing. A capacity decision cannot be adopted here.
//
// After validation, the set becomes empty and the scheduler exclusively owns all
// descriptors, closing each once on retirement or return. Close on the emptied
// set is harmless. Existing ingestors resume without registration/acquisition;
// polling, grace, serial commits and checkpoint checks use the normal scheduler.
// An explicitly prepared zero replay rechecks its retained prefix before transfer
// and at scheduler polls until its first positive acknowledgement.
// A new current is prepared/acquired after transfer, before consuming any line;
// preparation errors close both files without inventing retirement. Empty new
// files remain unregistered until content arrives, using normal polling.
// LastPathStatus resets only on transfer; rejected attempts preserve it.
// Run and FollowOpened share this object's execution guard. The caller must
// serialize set access and source state writes across other objects. Validation
// is an observation, not a filesystem lock. Run uses this same application core
// while holding its own guard, after PrepareFollowResume.
func (s *FileSource) FollowOpened(ctx context.Context, set *OpenedFollowSet, current FollowCurrent, sink source.Sink) error {
	return s.followOpened(ctx, set, current, sink, waitForPoll, time.Now)
}

func (s *FileSource) followOpened(ctx context.Context, set *OpenedFollowSet, current FollowCurrent, sink source.Sink, wait func(context.Context, time.Duration) error, now func() time.Time) error {
	return s.followOpenedWithOpener(ctx, set, current, sink, wait, now, OpenLog)
}

func (s *FileSource) followOpenedWithOpener(ctx context.Context, set *OpenedFollowSet, current FollowCurrent, sink source.Sink, wait func(context.Context, time.Duration) error, now func() time.Time, open func(context.Context, string) (*os.File, Identity, error)) error {
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
	return s.applyOpened(ctx, set, current, sink, wait, now, open)
}

// The caller holds the FileSource execution guard throughout application.
func (s *FileSource) applyOpened(ctx context.Context, set *OpenedFollowSet, current FollowCurrent, sink source.Sink, wait func(context.Context, time.Duration) error, now func() time.Time, open func(context.Context, string) (*os.File, Identity, error)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	newCurrent := current.Status == FollowCurrentNew
	missingCurrent := current.Status == FollowCurrentMissing
	switch current.Status {
	case FollowCurrentKnown:
		if current.OriginID == "" {
			return ErrInvalidFollowCurrent
		}
	case FollowCurrentNew:
		if current.OriginID != "" {
			return ErrInvalidFollowCurrent
		}
	case FollowCurrentMissing:
		if current.OriginID != "" || current.Current.Device != "" || current.Current.Inode != "" || current.Current.Size != 0 || current.Current.info != nil {
			return ErrInvalidFollowCurrent
		}
	default:
		return ErrInvalidFollowCurrent
	}
	if newCurrent && set.Len() >= MaxOpenGenerations {
		return ErrRotationCapacity
	}
	observed, err := set.ObserveCurrent(ctx, s.config.Path)
	if err != nil {
		return err
	}
	if observed.Status != current.Status || observed.OriginID != current.OriginID || !missingCurrent && !observed.Current.SameFile(current.Current) {
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
	if missingCurrent {
		return ErrCurrentMissing
	}
	opened := set.opened
	if err := checkZeroReplayPrefixes(ctx, opened); err != nil {
		return err
	}
	if newCurrent {
		if err := checkOpenedSizes(ctx, opened); err != nil {
			return err
		}
		if err := checkOpenedAnchors(ctx, opened); err != nil {
			return err
		}
		f, id, err := open(ctx, s.config.Path)
		if err != nil {
			return err
		}
		if !id.SameFile(observed.Current) {
			return errors.Join(ErrPathChanged, f.Close())
		}
		if err := ctx.Err(); err != nil {
			return errors.Join(err, f.Close())
		}
		active = &openedGeneration{file: f, identity: id}
		opened = append(opened, active)
	}
	set.opened = nil
	s.setPathStatus("")
	if newCurrent {
		return s.prepareNewAndFollow(ctx, opened, active, sink, wait, now)
	}
	return s.followGenerations(ctx, opened, active, sink, wait, now)
}

// Owns both files during startup. Ownership passes to the scheduler only after
// preparation (or an empty-file decision); each failure closes the entire set.
func (s *FileSource) prepareNewAndFollow(ctx context.Context, opened []*openedGeneration, active *openedGeneration, sink source.Sink, wait func(context.Context, time.Duration) error, now func() time.Time) (err error) {
	handed := false
	defer func() {
		if !handed {
			for _, generation := range opened {
				err = errors.Join(err, generation.file.Close())
			}
		}
	}()
	active.ingestor, _, err = s.prepareGeneration(ctx, active.file, sink)
	if err != nil {
		return err
	}
	handed = true
	return s.followGenerations(ctx, opened, active, sink, wait, now)
}
