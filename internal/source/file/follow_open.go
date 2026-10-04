package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/Coubiac/mailtrace/internal/source"
)

var ErrInvalidFollowLocations = errors.New("invalid complete following location set")

// OpenedFollowSet owns all reopened descriptors. It must not be copied or used
// concurrently. Close is idempotent, including after a close error, and changes
// no persisted follow state. The zero value and a nil receiver can be closed.
// Scheduler ownership transfer and choosing the current file are separate work.
type OpenedFollowSet struct {
	opened []*openedGeneration
}

func (s *OpenedFollowSet) Len() int {
	if s == nil {
		return 0
	}
	return len(s.opened)
}

func (s *OpenedFollowSet) Close() error {
	if s == nil {
		return nil
	}
	opened := s.opened
	s.opened = nil
	var err error
	for _, generation := range opened {
		err = errors.Join(err, generation.file.Close())
	}
	return err
}

// OpenFollowLocations reopens a wholly unique set of 1..MaxOpenGenerations
// following locations. All IDs, paths and states are validated and copied before
// opening. Absolute selected paths prevent working-directory redirection. The
// caller guarantees one source namespace and serializes state writes through
// application; origin state does not contain its source ID.
//
// OpenLog checks the path/descriptor. Strict VerifyCandidate then NewIngestor
// recheck identity, bounded prefix/anchor and LF boundary, and seek to the saved
// positive checkpoint. Nil/zero checkpoints never silently enable replay. No
// line is consumed or normalized and no state is written. A duplicate physical
// file is ambiguous even when supplied as two distinct paths. Every failure or
// cancellation closes all acquired descriptors and returns no partial set.
//
// Success transfers descriptor ownership to the returned set until Close.
// Checks are bounded observations, not locks or an atomic snapshot of all files.
func OpenFollowLocations(ctx context.Context, identity source.Identity, locations FollowLocations, normalize Normalize) (*OpenedFollowSet, error) {
	return openFollowLocations(ctx, identity, locations, normalize, OpenLog)
}

func openFollowLocations(ctx context.Context, identity source.Identity, locations FollowLocations, normalize Normalize, open func(context.Context, string) (*os.File, Identity, error)) (result *OpenedFollowSet, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if identity.ID == "" || identity.Kind != "file" || identity.Name == "" || normalize == nil {
		return nil, errors.New("file source identity and normalizer are required")
	}
	if locations.Status != SelectionUnique || len(locations.Locations) < 1 || len(locations.Locations) > MaxOpenGenerations {
		return nil, ErrInvalidFollowLocations
	}
	prepared := make([]FollowLocation, 0, len(locations.Locations))
	ids, paths := make(map[string]bool), make(map[string]bool)
	originPath := locations.Locations[0].State.Origin.Path
	for _, location := range locations.Locations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		state := location.State
		path := filepath.Clean(location.Path)
		if state.FollowState != source.FollowFollowing || state.Origin.ID == "" || state.Origin.Path == "" || state.Origin.Path != originPath || !filepath.IsAbs(location.Path) || ids[state.Origin.ID] || paths[path] {
			return nil, ErrInvalidFollowLocations
		}
		ids[state.Origin.ID], paths[path] = true, true
		if state.Checkpoint != nil {
			p := *state.Checkpoint
			state.Checkpoint = &p
		}
		prepared = append(prepared, FollowLocation{State: state, Path: path})
	}
	owned := &OpenedFollowSet{}
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, owned.Close())
		}
	}()
	for _, location := range prepared {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, id, err := open(ctx, location.Path)
		if err != nil {
			return nil, err
		}
		generation := &openedGeneration{file: f, identity: id}
		owned.opened = append(owned.opened, generation)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, previous := range owned.opened[:len(owned.opened)-1] {
			if id.SameFile(previous.identity) {
				return nil, &ResumeDecisionError{Status: SelectionAmbiguous}
			}
		}
		check, err := VerifyCandidate(f, location.State)
		if err != nil {
			return nil, err
		}
		if check.Status != ResumeMatch {
			status := SelectionInsufficient
			if check.Status == ResumeDifferent {
				status = SelectionDifferent
			}
			return nil, &ResumeDecisionError{Status: status}
		}
		generation.ingestor, err = NewIngestor(ctx, f, identity, location.State, normalize)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	keep = true
	return owned, nil
}
