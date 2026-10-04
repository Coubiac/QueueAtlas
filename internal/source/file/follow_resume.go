package file

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Coubiac/mailtrace/internal/source"
)

var ErrUnknownFollowState = errors.New("file following resume requires known lifecycle state")

type FollowResumeLimits struct {
	Origins int // 1..MaxPathOrigins, including retired and unknown states
	Entries int // 1..MaxFollowLocationEntries, shared across directory scans
}

type FollowResumeStatus string

const (
	FollowResumeAbsent FollowResumeStatus = "absent"
	FollowResumeReady  FollowResumeStatus = "ready"
)

type FollowResume struct {
	Status  FollowResumeStatus
	Opened  *OpenedFollowSet // caller-owned, only on Ready; must close or transfer
	Current FollowCurrent    // Known/New only on Ready; reobserve before transfer
}

// PrepareFollowResume composes bounded lifecycle loading, whole-set location,
// strict reopening and current observation. Limits, source, absolute configured
// path and dependencies are validated before reading state. Absent means a
// complete scan found no following states (empty or all retired); it opens no
// journal and does not itself start a fresh generation.
//
// Unknown/invalid lifecycle, capacity, limits and non-unique locations stop with
// fixed diagnostics, never an implicit fallback or partial usable result.
// Unavailable verified generations (completed location scan Absent/Different)
// yield ErrFollowResumeGap, retaining the precise ResumeDecisionError underneath.
// Insufficient/ambiguous/limited scans and filesystem errors do not claim a gap.
// Missing current stops with ErrCurrentMissing; a distinct third file stops with
// ErrRotationCapacity before opening it. After reopening, every error/cancellation
// closes the entire set and returns a zero result, joining cleanup errors.
//
// Ready transfers ownership of the whole verified set to the caller; no lines,
// normalization, Sink commit, lifecycle transition or scheduler transfer occurs.
// The caller must serialize source state writes through application and close
// Opened or pass it to FollowOpened. Files/pages are not an atomic snapshot and
// Ready does not make its current observation permanent. Run uses this preparation
// with its configured budgets, then applies the result under its execution guard.
func PrepareFollowResume(ctx context.Context, identity source.Identity, path string, reader source.PathStateReader, normalize Normalize, limits FollowResumeLimits) (FollowResume, error) {
	return prepareFollowResume(ctx, identity, path, reader, normalize, limits, func(ctx context.Context, opened *OpenedFollowSet, path string) (FollowCurrent, error) {
		return opened.ObserveCurrent(ctx, path)
	})
}

func prepareFollowResume(ctx context.Context, identity source.Identity, path string, reader source.PathStateReader, normalize Normalize, limits FollowResumeLimits, observe func(context.Context, *OpenedFollowSet, string) (FollowCurrent, error)) (result FollowResume, err error) {
	if err := ctx.Err(); err != nil {
		return FollowResume{}, err
	}
	if identity.ID == "" || identity.Kind != "file" || identity.Name == "" || !filepath.IsAbs(path) || reader == nil || normalize == nil || limits.Origins < 1 || limits.Origins > MaxPathOrigins || limits.Entries < 1 || limits.Entries > MaxFollowLocationEntries {
		return FollowResume{}, errors.New("file identity, absolute configured path, dependencies and bounded resume limits are required")
	}
	origins, err := LoadFollowOrigins(ctx, identity.ID, path, reader, limits.Origins)
	if err != nil {
		return FollowResume{}, err
	}
	switch origins.Status {
	case FollowOriginsAbsent:
		return FollowResume{Status: FollowResumeAbsent}, nil
	case FollowOriginsUnknown:
		return FollowResume{}, ErrUnknownFollowState
	case FollowOriginsInvalidState:
		return FollowResume{}, ErrInvalidFollowState
	case FollowOriginsCapacity:
		return FollowResume{}, ErrRotationCapacity
	case FollowOriginsLimit:
		return FollowResume{}, &ResumeDecisionError{Status: SelectionLimit}
	case FollowOriginsComplete:
	default:
		return FollowResume{}, ErrInvalidFollowOrigins
	}
	locations, err := LocateFollowOrigins(ctx, path, origins, limits.Entries)
	if err != nil {
		return FollowResume{}, err
	}
	if locations.Status != SelectionUnique {
		return FollowResume{}, followLocationError(locations.Status)
	}
	opened, err := OpenFollowLocations(ctx, identity, locations, normalize)
	if err != nil {
		return FollowResume{}, err
	}
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, opened.Close())
		}
	}()
	current, err := observe(ctx, opened, path)
	if err != nil {
		return FollowResume{}, err
	}
	if err := ctx.Err(); err != nil {
		return FollowResume{}, err
	}
	switch current.Status {
	case FollowCurrentMissing:
		return FollowResume{}, ErrCurrentMissing
	case FollowCurrentCapacity:
		return FollowResume{}, ErrRotationCapacity
	case FollowCurrentKnown, FollowCurrentNew:
	default:
		return FollowResume{}, ErrInvalidFollowCurrent
	}
	keep = true
	return FollowResume{Status: FollowResumeReady, Opened: opened, Current: current}, nil
}
