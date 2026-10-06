//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func resumePreparationFixture(t *testing.T, kind string) (*FileSource, *sqlite.Store) {
	t.Helper()
	path, locations := locatedOpenFixture(t)
	if kind != "two known" && kind != "capacity" {
		locations.Locations = locations.Locations[:1]
	}
	if kind == "one known" {
		locations.Locations[0] = FollowLocation{State: followingCurrentState(t, path), Path: path}
	}
	s, store := rotationSource(t, path)
	for _, location := range locations.Locations {
		state := location.State
		state.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
		if kind == "nil checkpoint" {
			if err := store.Commit(context.Background(), source.Batch{Source: s.config.Identity, Origins: []source.Origin{state.Origin}, FollowTransitions: []source.FollowTransition{{OriginID: state.Origin.ID, From: source.FollowUnknown, To: source.FollowFollowing}}}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if kind == "zero checkpoint" {
			f, _, err := OpenLog(context.Background(), location.Path)
			if err != nil {
				t.Fatal(err)
			}
			anchor, err := CaptureAnchor(f, 0)
			if err = errors.Join(err, f.Close()); err != nil {
				t.Fatal(err)
			}
			state.Checkpoint.Offset, state.Checkpoint.AnchorHash = 0, anchor.String()
		}
		seedAcquisition(t, s, store, state)
	}
	return s, store
}

func followingCurrentState(t *testing.T, path string) source.OriginState {
	t.Helper()
	f, _, err := OpenLog(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	state := ingestState(t, f, 4)
	state.Origin.ID, state.Checkpoint.OriginID, state.FollowState = "gen-2", "gen-2", source.FollowFollowing
	return state
}

func resumeStoredPage(t *testing.T, s *FileSource, store *sqlite.Store) source.OriginPage {
	t.Helper()
	page, err := store.FileOriginsByPath(context.Background(), source.OriginPathQuery{SourceID: s.ID(), Path: s.config.Path, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func TestPrepareFollowResumeReadyOwnsWholeSetWithoutConsumption(t *testing.T) {
	for _, kind := range []string{"one known", "two known", "new", "empty new"} {
		t.Run(kind, func(t *testing.T) {
			s, store := resumePreparationFixture(t, kind)
			if kind == "empty new" {
				if err := os.Truncate(s.config.Path, 0); err != nil {
					t.Fatal(err)
				}
			}
			before := resumeStoredPage(t, s, store)
			for _, state := range before.States {
				path := s.config.Path + ".1"
				if state.Origin.ID == "gen-2" {
					path = s.config.Path
				}
				appendNewTestLine(t, path, "late\n")
			}
			normalized := 0
			result, err := PrepareFollowResume(context.Background(), s.config.Identity, s.config.Path, store, func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }, FollowResumeLimits{Origins: 2, Entries: 4})
			if err != nil || result.Status != FollowResumeReady || result.Opened.Len() != len(before.States) || normalized != 0 {
				t.Fatal("prepared result", result, err)
			}
			defer result.Opened.Close()
			want := FollowCurrentNew
			if kind == "one known" || kind == "two known" {
				want = FollowCurrentKnown
			}
			if result.Current.Status != want || want == FollowCurrentKnown && result.Current.OriginID != "gen-2" || want == FollowCurrentNew && result.Current.OriginID != "" {
				t.Fatal("prepared current", result.Current)
			}
			generations := append([]*openedGeneration(nil), result.Opened.opened...)
			for _, g := range generations {
				assertDescriptorPosition(t, g.file, 4)
				if g.ingestor.Position().Offset != 4 || g.ingestor.lines.offset != 4 || g.ingestor.pending != nil || !g.eofSince.IsZero() {
					t.Fatal("prepared set consumed content")
				}
			}
			if !reflect.DeepEqual(before, resumeStoredPage(t, s, store)) {
				t.Fatal("preparation wrote state")
			}
			currentFDs, oldFDs := 0, 1
			if kind == "one known" {
				currentFDs, oldFDs = 1, 0
			}
			if kind == "two known" {
				currentFDs = 1
			}
			if fileDescriptorCount(t, s.config.Path) != currentFDs || fileDescriptorCount(t, s.config.Path+".1") != oldFDs {
				t.Fatal("preparation opened a new current or lost a following file")
			}
			if err := result.Opened.Close(); err != nil || result.Opened.Close() != nil {
				t.Fatal("prepared owner cleanup", err)
			}
			for _, g := range generations {
				if !errors.Is(g.file.Close(), fs.ErrClosed) {
					t.Fatal("prepared descriptor leaked")
				}
			}
			if fileDescriptorCount(t, s.config.Path) != 0 || fileDescriptorCount(t, s.config.Path+".1") != 0 {
				t.Fatal("prepared files leaked")
			}
		})
	}
}

func TestPrepareFollowResumeBlockedFilesystemClosesWholeSet(t *testing.T) {
	for _, kind := range []string{"missing", "capacity", "location limit", "ambiguous", "changed", "absent", "nil checkpoint", "zero checkpoint"} {
		t.Run(kind, func(t *testing.T) {
			s, store := resumePreparationFixture(t, kind)
			limits := FollowResumeLimits{Origins: 2, Entries: 4}
			var want error
			var status SelectionStatus
			switch kind {
			case "nil checkpoint", "zero checkpoint":
				status = SelectionInsufficient
			case "missing":
				if err := os.Remove(s.config.Path); err != nil {
					t.Fatal(err)
				}
				want = ErrCurrentMissing
			case "capacity":
				if err := rotateTo(s.config.Path, ".2", "third\n"); err != nil {
					t.Fatal(err)
				}
				want = ErrRotationCapacity
				limits.Entries = 6
			case "location limit":
				limits.Entries = 1
				status = SelectionLimit
			case "ambiguous":
				if err := os.Link(s.config.Path+".1", s.config.Path+".alias"); err != nil {
					t.Fatal(err)
				}
				status = SelectionAmbiguous
			case "changed":
				if err := os.WriteFile(s.config.Path+".1", []byte("bad\n"), 0600); err != nil {
					t.Fatal(err)
				}
				status = SelectionDifferent
			case "absent":
				if err := os.Remove(s.config.Path + ".1"); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(s.config.Path); err != nil {
					t.Fatal(err)
				}
				status = SelectionAbsent
			}
			before := resumeStoredPage(t, s, store)
			result, err := PrepareFollowResume(context.Background(), s.config.Identity, s.config.Path, store, testNormalizer, limits)
			if result != (FollowResume{}) || err == nil {
				t.Fatal("blocked preparation returned a partial result", result, err)
			}
			if status != "" {
				var decision *ResumeDecisionError
				if !errors.As(err, &decision) || decision.Status != status {
					t.Fatal("location diagnostic", err, status)
				}
			} else if !errors.Is(err, want) {
				t.Fatal("current diagnostic", err, want)
			}
			if !reflect.DeepEqual(before, resumeStoredPage(t, s, store)) {
				t.Fatal("blocked preparation wrote state")
			}
			for _, path := range []string{s.config.Path, s.config.Path + ".1", s.config.Path + ".2", s.config.Path + ".alias"} {
				if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if fileDescriptorCount(t, path) != 0 {
					t.Fatal("blocked preparation leaked file", path)
				}
			}
		})
	}
}

func TestPrepareFollowResumeObservationFailureAndCancellationCloseBoth(t *testing.T) {
	for _, kind := range []string{"error", "cancel", "invalid observation", "cleanup error"} {
		t.Run(kind, func(t *testing.T) {
			s, store := resumePreparationFixture(t, "two known")
			before := resumeStoredPage(t, s, store)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			boom := errors.New("synthetic observation failure")
			want := error(boom)
			if kind == "cancel" {
				want = context.Canceled
			}
			if kind == "invalid observation" {
				want = ErrInvalidFollowCurrent
			}
			var captured *OpenedFollowSet
			var generations []*openedGeneration
			result, err := prepareFollowResume(ctx, s.config.Identity, s.config.Path, store, testNormalizer, FollowResumeLimits{Origins: 2, Entries: 4}, func(ctx context.Context, opened *OpenedFollowSet, path string) (FollowCurrent, error) {
				captured = opened
				generations = append(generations, opened.opened...)
				if opened.Len() != 2 {
					t.Fatal("observation received partial set")
				}
				if kind == "cleanup error" {
					if err := opened.opened[0].file.Close(); err != nil {
						t.Fatal(err)
					}
					return FollowCurrent{}, boom
				}
				if kind == "invalid observation" {
					return FollowCurrent{}, nil
				}
				current, err := opened.ObserveCurrent(ctx, path)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "cancel" {
					cancel()
					return current, nil
				}
				return FollowCurrent{}, boom
			})
			if result != (FollowResume{}) || !errors.Is(err, want) || captured == nil || captured.Len() != 0 || captured.Close() != nil {
				t.Fatal("observation cleanup", result, err)
			}
			if kind == "cleanup error" && !errors.Is(err, fs.ErrClosed) {
				t.Fatal("cleanup error lost", err)
			}
			for _, g := range generations {
				if !errors.Is(g.file.Close(), fs.ErrClosed) {
					t.Fatal("observation failed to close both")
				}
			}
			if !reflect.DeepEqual(before, resumeStoredPage(t, s, store)) {
				t.Fatal("observation failure wrote state")
			}
		})
	}
}
