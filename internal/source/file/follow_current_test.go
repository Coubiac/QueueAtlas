package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
)

func currentSetFixture(t *testing.T, count int) (*OpenedFollowSet, []string, *int) {
	t.Helper()
	set := &OpenedFollowSet{}
	var paths []string
	normalized := new(int)
	for i := 0; i < count; i++ {
		f, path := testRegularFile(t, "same\n")
		state := ingestState(t, f, 5)
		state.Origin.ID, state.Checkpoint.OriginID = string(rune('a'+i)), string(rune('a'+i))
		r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), state, func(raw []byte) model.Observation { *normalized++; return testNormalizer(raw) })
		if err != nil {
			t.Fatal(err)
		}
		id, err := Inspect(f)
		if err != nil {
			t.Fatal(err)
		}
		set.opened = append(set.opened, &openedGeneration{file: f, identity: id, ingestor: r})
		paths = append(paths, path)
	}
	t.Cleanup(func() { set.Close() })
	return set, paths, normalized
}

func assertCurrentSetUnchanged(t *testing.T, set *OpenedFollowSet, count int, normalized *int) {
	t.Helper()
	if set.Len() != count || *normalized != 0 {
		t.Fatal("observation changed ownership or consumed a line", set.Len(), *normalized)
	}
	for _, g := range set.opened {
		assertDescriptorPosition(t, g.file, 5)
		if g.ingestor.Position().Offset != 5 || g.ingestor.lines.offset != 5 || g.ingestor.pending != nil || !g.eofSince.IsZero() {
			t.Fatal("observation changed ingestion/grace state")
		}
	}
}

func TestObserveCurrentFindsPhysicalIdentityWithoutChoosingByOrder(t *testing.T) {
	set, paths, normalized := currentSetFixture(t, 2)
	for i, path := range paths {
		result, err := set.ObserveCurrent(context.Background(), path)
		id, statErr := Inspect(set.opened[i].file)
		if err != nil || statErr != nil || result.Status != FollowCurrentKnown || result.OriginID != set.opened[i].ingestor.position.OriginID || !result.Current.SameFile(id) {
			t.Fatal("wrong current origin", result, err, statErr)
		}
		assertCurrentSetUnchanged(t, set, 2, normalized)
	}
	// The second descriptor becomes first in memory; current selection still
	// follows physical identity rather than ingestion order or a lexical ID.
	set.opened[0], set.opened[1] = set.opened[1], set.opened[0]
	result, err := set.ObserveCurrent(context.Background(), paths[0])
	if err != nil || result.Status != FollowCurrentKnown || result.OriginID != "a" {
		t.Fatal("ordering selected the current file", result, err)
	}
	assertCurrentSetUnchanged(t, set, 2, normalized)
}

func TestObserveCurrentMissingNewAndCapacityKeepOwnership(t *testing.T) {
	for _, count := range []int{1, 2} {
		set, _, normalized := currentSetFixture(t, count)
		missing := filepath.Join(t.TempDir(), "absent.log")
		result, err := set.ObserveCurrent(context.Background(), missing)
		if err != nil || result != (FollowCurrent{Status: FollowCurrentMissing}) {
			t.Fatal("missing path invented a current", result, err)
		}
		other, path := testRegularFile(t, "same\n")
		otherID, err := Inspect(other)
		if err != nil {
			t.Fatal(err)
		}
		result, err = set.ObserveCurrent(context.Background(), path)
		if err != nil || result.OriginID != "" {
			t.Fatal("new file reused an origin", result, err)
		}
		if count == 1 {
			if result.Status != FollowCurrentNew || !result.Current.SameFile(otherID) {
				t.Fatal("spare capacity refused new current", result)
			}
		} else if result != (FollowCurrent{Status: FollowCurrentCapacity}) {
			t.Fatal("capacity exposed usable current identity", result)
		}
		assertCurrentSetUnchanged(t, set, count, normalized)
	}
}

func TestObserveCurrentErrorsAndCancellationKeepOwnership(t *testing.T) {
	set, paths, normalized := currentSetFixture(t, 2)
	for _, path := range []string{"", "relative.log", t.TempDir()} {
		result, err := set.ObserveCurrent(context.Background(), path)
		if err == nil || result != (FollowCurrent{}) {
			t.Fatal("invalid path returned decision", result, err)
		}
		assertCurrentSetUnchanged(t, set, 2, normalized)
	}
	boom := errors.New("synthetic current observation failure")
	for _, kind := range []string{"error", "cancel before", "cancel after observation"} {
		ctx, cancel := context.WithCancel(context.Background())
		wantError := error(context.Canceled)
		if kind == "error" {
			wantError = boom
		}
		if kind == "cancel before" {
			cancel()
		}
		calls := 0
		result, err := set.observeCurrent(ctx, paths[1], func(ctx context.Context, f *os.File, path string) (PathObservation, error) {
			calls++
			if kind == "error" {
				return PathObservation{}, boom
			}
			observation, err := ObservePath(ctx, f, path)
			cancel()
			return observation, err
		})
		cancel()
		if !errors.Is(err, wantError) || result != (FollowCurrent{}) || kind == "cancel before" && calls != 0 {
			t.Fatal("error/cancellation returned current", result, err, calls)
		}
		assertCurrentSetUnchanged(t, set, 2, normalized)
	}
}

func TestObserveCurrentRejectsUnusableOwnerAndChecksSecondDescriptor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.log")
	var nilSet *OpenedFollowSet
	for _, set := range []*OpenedFollowSet{nilSet, {}, {opened: []*openedGeneration{nil}}, {opened: []*openedGeneration{{}, {}, {}}}} {
		result, err := set.ObserveCurrent(context.Background(), path)
		if !errors.Is(err, ErrInvalidOpenedFollowSet) || result != (FollowCurrent{}) {
			t.Fatal("unusable owner accepted", result, err)
		}
	}
	set, paths, _ := currentSetFixture(t, 2)
	if err := set.opened[1].file.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := set.ObserveCurrent(context.Background(), paths[0]); err == nil || result != (FollowCurrent{}) || set.Len() != 2 {
		t.Fatal("closed second descriptor ignored or owner closed", result, err)
	}
	if _, err := set.opened[0].file.Stat(); err != nil {
		t.Fatal("failure closed first descriptor", err)
	}
	if err := set.Close(); err == nil || set.Len() != 0 {
		t.Fatal("owner lost cleanup error", err)
	}
	if result, err := set.ObserveCurrent(context.Background(), paths[0]); !errors.Is(err, ErrInvalidOpenedFollowSet) || result != (FollowCurrent{}) {
		t.Fatal("closed owner usable", result, err)
	}
}
