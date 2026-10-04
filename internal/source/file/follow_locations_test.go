package file

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
)

func locationOrigins(path string) FollowOrigins {
	states := pathOriginStates(2)
	for i := range states {
		states[i].Origin.Path = path
		states[i].FollowState = source.FollowFollowing
	}
	return FollowOrigins{Status: FollowOriginsComplete, States: states, Examined: 2}
}

func TestLocateFollowOriginsSharesBudgetAndDiscardsPartialPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail.log")
	for _, budget := range []int{2, 3, 4, MaxFollowLocationEntries} {
		calls := 0
		result, err := locateFollowOrigins(context.Background(), path, locationOrigins(path), budget, func(_ context.Context, directory string, state source.OriginState, limit int) (RotationSelection, error) {
			calls++
			wantLimit := min(MaxRotationEntries, budget-2*(calls-1))
			if directory != filepath.Dir(path) || limit != wantLimit {
				t.Fatal("unbounded or repeated budget", directory, limit, wantLimit)
			}
			if limit < 2 {
				return RotationSelection{Status: SelectionLimit, Examined: limit}, nil
			}
			return RotationSelection{Status: SelectionUnique, Path: filepath.Join(directory, state.Origin.ID), Examined: 2}, nil
		})
		want := SelectionUnique
		if budget < 4 {
			want = SelectionLimit
		}
		if err != nil || result.Status != want || result.Examined != min(budget, 4) {
			t.Fatal("location budget", result, err)
		}
		if want == SelectionUnique {
			if len(result.Locations) != 2 {
				t.Fatal("complete set lost candidates", result)
			}
		} else if result.Locations != nil {
			t.Fatal("budget exposed partial paths", result)
		}
		wantCalls := 2
		if budget == 2 {
			wantCalls = 1
		}
		if calls != wantCalls {
			t.Fatal("empty remaining budget performed search", calls)
		}
	}
	for _, status := range []SelectionStatus{SelectionAbsent, SelectionDifferent, SelectionInsufficient, SelectionAmbiguous, SelectionLimit} {
		calls := 0
		result, err := locateFollowOrigins(context.Background(), path, locationOrigins(path), 10, func(context.Context, string, source.OriginState, int) (RotationSelection, error) {
			calls++
			if calls == 1 {
				return RotationSelection{Status: SelectionUnique, Path: path + ".1", Examined: 1}, nil
			}
			return RotationSelection{Status: status, Examined: 1}, nil
		})
		if err != nil || result.Status != status || result.Locations != nil || result.Examined != 2 || calls != 2 {
			t.Fatal("non-unique decision lost or exposed partial paths", result, err)
		}
	}
}

func TestLocateFollowOriginsRejectsInvalidSetsBeforeSearch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail.log")
	for _, change := range []func(*FollowOrigins){
		func(o *FollowOrigins) { o.Status = FollowOriginsUnknown },
		func(o *FollowOrigins) { o.States = nil },
		func(o *FollowOrigins) { o.States = append(o.States, o.States[0]) },
		func(o *FollowOrigins) { o.States[1].FollowState = source.FollowRetired },
		func(o *FollowOrigins) { o.States[1].FollowState = source.FollowUnknown },
		func(o *FollowOrigins) { o.States[1].FollowState = 3 },
		func(o *FollowOrigins) { o.States[1].Origin.ID = "" },
		func(o *FollowOrigins) { o.States[1].Origin.ID = o.States[0].Origin.ID },
		func(o *FollowOrigins) { o.States[1].Origin.Path = path + ".other" },
	} {
		origins := locationOrigins(path)
		change(&origins)
		result, err := locateFollowOrigins(context.Background(), path, origins, 10, func(context.Context, string, source.OriginState, int) (RotationSelection, error) {
			t.Fatal("invalid set searched disk")
			return RotationSelection{}, nil
		})
		if !errors.Is(err, ErrInvalidFollowOrigins) || !reflect.DeepEqual(result, FollowLocations{}) {
			t.Fatal("invalid set accepted", result, err)
		}
	}
	for _, tc := range []struct {
		path  string
		limit int
	}{{"", 1}, {path, 0}, {path, -1}, {path, MaxFollowLocationEntries + 1}} {
		result, err := LocateFollowOrigins(context.Background(), tc.path, locationOrigins(tc.path), tc.limit)
		if err == nil || !reflect.DeepEqual(result, FollowLocations{}) {
			t.Fatal("invalid arguments accepted", result, err)
		}
	}
}

func TestLocateFollowOriginsErrorsAndCancellationDiscardAllLocations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail.log")
	boom := errors.New("synthetic location error")
	for _, kind := range []string{"error after first", "cancel before", "cancel after first", "cancel after second"} {
		ctx, cancel := context.WithCancel(context.Background())
		calls, cancelAt := 0, 0
		wantError := error(context.Canceled)
		if kind == "error after first" {
			wantError = boom
		}
		if kind == "cancel before" {
			cancel()
		}
		if kind == "cancel after first" {
			cancelAt = 1
		}
		if kind == "cancel after second" {
			cancelAt = 2
		}
		result, err := locateFollowOrigins(ctx, path, locationOrigins(path), 10, func(context.Context, string, source.OriginState, int) (RotationSelection, error) {
			calls++
			if kind == "error after first" && calls == 2 {
				return RotationSelection{}, boom
			}
			if calls == cancelAt {
				cancel()
			}
			return RotationSelection{Status: SelectionUnique, Path: path + string(rune('0'+calls)), Examined: 1}, nil
		})
		cancel()
		if !errors.Is(err, wantError) || !reflect.DeepEqual(result, FollowLocations{}) {
			t.Fatal("failure returned partial locations", result, err)
		}
		wantCalls := 2
		if kind == "cancel before" {
			wantCalls = 0
		}
		if kind == "cancel after first" {
			wantCalls = 1
		}
		if calls != wantCalls {
			t.Fatal("cancelled search continued", calls)
		}
	}
}

func TestLocateFollowOriginsCopiesMetadataAndRejectsSharedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail.log")
	origins := locationOrigins(path)
	origins.States[0], origins.States[1] = origins.States[1], origins.States[0]
	origins.States[0].Checkpoint = nil
	result, err := locateFollowOrigins(context.Background(), path, origins, 10, func(_ context.Context, dir string, state source.OriginState, _ int) (RotationSelection, error) {
		return RotationSelection{Status: SelectionUnique, Path: filepath.Join(dir, state.Origin.ID), Examined: 1}, nil
	})
	if err != nil || result.Status != SelectionUnique || len(result.Locations) != 2 || !reflect.DeepEqual(result.Locations[0].State, origins.States[0]) || !reflect.DeepEqual(result.Locations[1].State, origins.States[1]) {
		t.Fatal("metadata or input order changed", result, err)
	}
	origins.States[1].Checkpoint.Offset = 999
	if result.Locations[1].State.Checkpoint.Offset != 0 {
		t.Fatal("result aliases caller checkpoint")
	}
	result.Locations[1].State.Checkpoint.AnchorHash = "consumer mutation"
	if origins.States[1].Checkpoint.AnchorHash != "unverified-anchor" {
		t.Fatal("caller aliases result")
	}
	result, err = locateFollowOrigins(context.Background(), path, origins, 10, func(context.Context, string, source.OriginState, int) (RotationSelection, error) {
		return RotationSelection{Status: SelectionUnique, Path: path, Examined: 1}, nil
	})
	if err != nil || result.Status != SelectionAmbiguous || result.Locations != nil || result.Examined != 2 {
		t.Fatal("two origins selected one file", result, err)
	}
}
