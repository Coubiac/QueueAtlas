package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

type FollowCurrentStatus string

const (
	FollowCurrentKnown    FollowCurrentStatus = "known"
	FollowCurrentMissing  FollowCurrentStatus = "missing"
	FollowCurrentNew      FollowCurrentStatus = "new_generation"
	FollowCurrentCapacity FollowCurrentStatus = "capacity_exceeded"
)

var ErrInvalidOpenedFollowSet = errors.New("invalid opened following set")

type FollowCurrent struct {
	Status   FollowCurrentStatus
	OriginID string   // populated only for Known; no ordering implies the current file
	Current  Identity // metadata snapshot for Known/New; recheck before application
}

// ObserveCurrent identifies the configured absolute path among this set's live
// descriptors by SameFile. Missing never invents a current origin. A distinct
// regular file yields New only with a spare descriptor slot; otherwise Capacity
// carries no usable identity. Invalid/closed sets and filesystem errors return
// no result. The caller keeps ownership, including on failure.
//
// Only metadata is inspected: no open/read/seek/close, normalization, transition
// or scheduler transfer. All descriptors are checked, so an unusable second
// descriptor cannot be silently ignored. Checks are not atomic or proof of
// checkpoint continuity. Serialize set use and source state writes until
// application, and recheck the path when opening/adopting the current file.
func (s *OpenedFollowSet) ObserveCurrent(ctx context.Context, path string) (FollowCurrent, error) {
	return s.observeCurrent(ctx, path, ObservePath)
}

func (s *OpenedFollowSet) observeCurrent(ctx context.Context, path string, observe func(context.Context, *os.File, string) (PathObservation, error)) (FollowCurrent, error) {
	if err := ctx.Err(); err != nil {
		return FollowCurrent{}, err
	}
	if !filepath.IsAbs(path) {
		return FollowCurrent{}, errors.New("absolute configured log path is required")
	}
	if s.Len() < 1 || s.Len() > MaxOpenGenerations {
		return FollowCurrent{}, ErrInvalidOpenedFollowSet
	}
	ids := make(map[string]bool, s.Len())
	for _, g := range s.opened {
		if g == nil || g.file == nil || g.ingestor == nil || g.ingestor.position.OriginID == "" || ids[g.ingestor.position.OriginID] {
			return FollowCurrent{}, ErrInvalidOpenedFollowSet
		}
		ids[g.ingestor.position.OriginID] = true
	}
	snapshots := make([]Identity, 0, s.Len())
	for _, g := range s.opened {
		if err := ctx.Err(); err != nil {
			return FollowCurrent{}, err
		}
		id, err := Inspect(g.file)
		if err != nil {
			return FollowCurrent{}, err
		}
		for _, previous := range snapshots {
			if id.SameFile(previous) {
				return FollowCurrent{}, ErrInvalidOpenedFollowSet
			}
		}
		snapshots = append(snapshots, id)
	}
	observation, err := observe(ctx, s.opened[0].file, path)
	if err != nil {
		return FollowCurrent{}, err
	}
	if err := ctx.Err(); err != nil {
		return FollowCurrent{}, err
	}
	if observation.Status == PathMissing {
		return FollowCurrent{Status: FollowCurrentMissing}, nil
	}
	for i, id := range snapshots {
		if id.SameFile(observation.Current) {
			return FollowCurrent{Status: FollowCurrentKnown, OriginID: s.opened[i].ingestor.position.OriginID, Current: observation.Current}, nil
		}
	}
	if s.Len() == MaxOpenGenerations {
		return FollowCurrent{Status: FollowCurrentCapacity}, nil
	}
	return FollowCurrent{Status: FollowCurrentNew, Current: observation.Current}, nil
}
