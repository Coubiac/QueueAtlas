package file

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

const MaxFollowLocationEntries = MaxOpenGenerations * MaxRotationEntries

var ErrInvalidFollowOrigins = errors.New("invalid complete following origin set")

type FollowLocation struct {
	State source.OriginState
	Path  string // selected path, to reopen and reverify before use
}

type FollowLocations struct {
	Status    SelectionStatus
	Locations []FollowLocation // owned states, only on SelectionUnique for the entire set
	Examined  int              // sum of directory entries across scans, including repeated/skipped entries
}

// LocateFollowOrigins resolves a complete set of 1..MaxOpenGenerations following
// states in the configured path's directory, including the current filename.
// The caller supplies one source's LoadFollowOrigins result and serializes state
// writes until its application. Invalid/mixed-path/duplicate origins fail before
// any filesystem access. Metadata is copied without altering origin/checkpoint.
//
// entryLimit (1..MaxFollowLocationEntries) bounds summed examined entries. Each
// SelectRotation call gets at most MaxRotationEntries and the remaining budget.
// Scans are sequential and stop at the first non-unique decision; it is returned
// with no usable partial locations. Two origins resolving to one path are
// ambiguous. Unique requires all candidates found at distinct paths. No ordering
// selects which file is current; a new current generation also needs capacity.
//
// Strict verification and exclusions of SelectRotation apply (zero/nil checkpoints
// are not silently replayed). Temporary descriptors close before return; no
// ingestion/state write or durable open occurs. Scans and bounded verification
// are not an atomic snapshot. Reopen and reverify every returned path before use.
func LocateFollowOrigins(ctx context.Context, configuredPath string, origins FollowOrigins, entryLimit int) (FollowLocations, error) {
	return locateFollowOrigins(ctx, configuredPath, origins, entryLimit, SelectRotation)
}

func locateFollowOrigins(ctx context.Context, configuredPath string, origins FollowOrigins, entryLimit int, search func(context.Context, string, source.OriginState, int) (RotationSelection, error)) (FollowLocations, error) {
	if err := ctx.Err(); err != nil {
		return FollowLocations{}, err
	}
	if configuredPath == "" || entryLimit < 1 || entryLimit > MaxFollowLocationEntries {
		return FollowLocations{}, errors.New("configured path and bounded follow location entry limit are required")
	}
	if origins.Status != FollowOriginsComplete || len(origins.States) < 1 || len(origins.States) > MaxOpenGenerations {
		return FollowLocations{}, ErrInvalidFollowOrigins
	}
	states := make([]source.OriginState, 0, len(origins.States))
	ids := make(map[string]bool, len(origins.States))
	for _, state := range origins.States {
		if err := ctx.Err(); err != nil {
			return FollowLocations{}, err
		}
		if state.FollowState != source.FollowFollowing || state.Origin.ID == "" || state.Origin.Path != configuredPath || ids[state.Origin.ID] {
			return FollowLocations{}, ErrInvalidFollowOrigins
		}
		ids[state.Origin.ID] = true
		if state.Checkpoint != nil {
			p := *state.Checkpoint
			state.Checkpoint = &p
		}
		states = append(states, state)
	}
	path, err := filepath.Abs(configuredPath)
	if err != nil {
		return FollowLocations{}, err
	}
	directory := filepath.Dir(path)
	result := FollowLocations{}
	locations := make([]FollowLocation, 0, len(states))
	paths := make(map[string]bool, len(states))
	for _, state := range states {
		if err := ctx.Err(); err != nil {
			return FollowLocations{}, err
		}
		remaining := entryLimit - result.Examined
		if remaining == 0 {
			result.Status = SelectionLimit
			return result, nil
		}
		selected, err := search(ctx, directory, state, min(remaining, MaxRotationEntries))
		if err != nil {
			return FollowLocations{}, err
		}
		if err := ctx.Err(); err != nil {
			return FollowLocations{}, err
		}
		result.Examined += selected.Examined
		if selected.Status != SelectionUnique {
			result.Status = selected.Status
			return result, nil
		}
		if paths[selected.Path] {
			result.Status = SelectionAmbiguous
			return result, nil
		}
		paths[selected.Path] = true
		locations = append(locations, FollowLocation{State: state, Path: selected.Path})
	}
	result.Status, result.Locations = SelectionUnique, locations
	return result, nil
}
