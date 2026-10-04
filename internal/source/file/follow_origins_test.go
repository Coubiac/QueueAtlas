package file

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

func TestLoadFollowOriginsRequiresCompleteKnownBoundedSet(t *testing.T) {
	const (
		u = source.FollowUnknown
		f = source.FollowFollowing
		r = source.FollowRetired
	)
	for _, tc := range []struct {
		name   string
		states []source.FollowState
		want   FollowOriginsStatus
	}{
		{"empty", nil, FollowOriginsAbsent},
		{"retired only", []source.FollowState{r, r}, FollowOriginsAbsent},
		{"one following", []source.FollowState{r, f, r}, FollowOriginsComplete},
		{"two following", []source.FollowState{f, r, f}, FollowOriginsComplete},
		{"capacity", []source.FollowState{f, f, f, r}, FollowOriginsCapacity},
		{"legacy unknown", []source.FollowState{u}, FollowOriginsUnknown},
		{"unknown prevents partial plan", []source.FollowState{f, r, u}, FollowOriginsUnknown},
		{"negative state", []source.FollowState{f, -1}, FollowOriginsInvalidState},
		{"future state", []source.FollowState{3, r}, FollowOriginsInvalidState},
		{"unknown over capacity", []source.FollowState{f, f, f, u}, FollowOriginsUnknown},
		{"invalid over unknown and capacity", []source.FollowState{f, f, f, u, 3}, FollowOriginsInvalidState},
	} {
		t.Run(tc.name, func(t *testing.T) {
			states := pathOriginStates(len(tc.states))
			var want []source.OriginState
			for i, follow := range tc.states {
				states[i].FollowState = follow
				// Dates and offsets run opposite to lexical ID order; neither is
				// used to select an active generation or drop a capacity excess.
				states[i].Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, len(states)-i, 0, time.UTC)
				states[i].Checkpoint.Offset = int64(len(states) - i)
				if follow == f {
					want = append(want, states[i])
				}
			}
			calls := 0
			result, err := LoadFollowOrigins(context.Background(), "mail", "/synthetic/mail.log", pathPages(t, states, 1, &calls), MaxPathOrigins)
			if err != nil || result.Status != tc.want || result.Examined != len(states) || calls != max(1, len(states)) {
				t.Fatal("follow set", result, err, calls)
			}
			if tc.want == FollowOriginsComplete {
				if !reflect.DeepEqual(result.States, want) {
					t.Fatal("candidates reordered or metadata changed", result.States, want)
				}
			} else if result.States != nil {
				t.Fatal("decision exposed partial candidates", result)
			}
		})
	}
}

func TestLoadFollowOriginsPreservesNilZeroAndIndependentMetadata(t *testing.T) {
	states := pathOriginStates(2)
	states[0].FollowState, states[1].FollowState = source.FollowFollowing, source.FollowFollowing
	states[0].Checkpoint = nil
	states[1].Checkpoint.Offset = 0
	calls := 0
	reader := pathPages(t, states, 1, &calls)
	result, err := LoadFollowOrigins(context.Background(), "mail", "/synthetic/mail.log", reader, 2)
	if err != nil || result.Status != FollowOriginsComplete || result.States[0].Checkpoint != nil || result.States[1].Checkpoint.Offset != 0 || result.States[1].Checkpoint.AnchorHash != "unverified-anchor" {
		t.Fatal("metadata inferred or verified", result, err)
	}
	states[1].Checkpoint.Offset = 99
	states[1].Origin.Fingerprint = "reader mutation"
	if result.States[1].Checkpoint.Offset != 0 || result.States[1].Origin.Fingerprint != "unverified-prefix" {
		t.Fatal("reader aliases candidates")
	}
	result.States[1].Checkpoint.AnchorHash = "consumer mutation"
	result.States[1].FollowState = source.FollowRetired
	if states[1].Checkpoint.AnchorHash != "unverified-anchor" || states[1].FollowState != source.FollowFollowing {
		t.Fatal("candidates alias reader")
	}
}

func TestLoadFollowOriginsDoesNotPlanFromIncompleteOrFailedScan(t *testing.T) {
	states := pathOriginStates(101)
	for i := range states {
		states[i].FollowState = source.FollowRetired
	}
	states[0].FollowState = source.FollowFollowing
	states[100].FollowState = source.FollowUnknown
	for _, limit := range []int{100, 101} {
		calls := 0
		result, err := LoadFollowOrigins(context.Background(), "mail", "/synthetic/mail.log", pathPages(t, states, 100, &calls), limit)
		want := FollowOriginsLimit
		if limit == 101 {
			want = FollowOriginsUnknown
		}
		if err != nil || result.Status != want || result.Examined != limit || result.States != nil {
			t.Fatal("partial scan planned a generation", result, err)
		}
	}
	boom := errors.New("synthetic follow-state reader failure")
	for _, kind := range []string{"reader error", "invalid page", "cancel during read", "cancel before read", "invalid limit"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, limit := 0, 2
			wantError := error(boom)
			if kind == "invalid page" {
				wantError = ErrInvalidPathOriginPage
			}
			if kind == "cancel during read" || kind == "cancel before read" {
				wantError = context.Canceled
			}
			if kind == "cancel before read" {
				cancel()
			}
			if kind == "invalid limit" {
				limit = 0
			}
			result, err := LoadFollowOrigins(ctx, "mail", "/synthetic/mail.log", pathReaderFunc(func(context.Context, source.OriginPathQuery) (source.OriginPage, error) {
				calls++
				if calls == 1 {
					return source.OriginPage{States: states[:1], NextID: states[0].Origin.ID}, nil
				}
				switch kind {
				case "reader error":
					return source.OriginPage{}, boom
				case "cancel during read":
					cancel()
					return source.OriginPage{States: states[1:2]}, nil
				default:
					return source.OriginPage{States: states[:1]}, nil // stale ID
				}
			}), limit)
			if err == nil || !reflect.DeepEqual(result, FollowOrigins{}) {
				t.Fatal("failed scan exposed result", result, err)
			}
			if kind != "invalid limit" && !errors.Is(err, wantError) {
				t.Fatal("error cause lost", err)
			}
			wantCalls := 2
			if kind == "cancel before read" || kind == "invalid limit" {
				wantCalls = 0
			}
			if calls != wantCalls {
				t.Fatal("unexpected reader calls", calls)
			}
		})
	}
}

func TestLoadFollowOriginsSQLiteHistoryAndFollowingCandidates(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	states := pathOriginStates(101)
	path := filepath.Join(t.TempDir(), "unopened-mail.log")
	identity := fileSourceIdentity()
	batch := source.Batch{Source: identity}
	var retire []source.FollowTransition
	for i, state := range states {
		state.Origin.Path = path
		state.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, i, 0, time.UTC)
		state.FollowState = source.FollowFollowing
		if i == 99 {
			state.Checkpoint = nil
		}
		if i == 100 {
			state.Checkpoint.Offset = 0
		}
		states[i] = state
		batch.Origins = append(batch.Origins, state.Origin)
		if state.Checkpoint != nil {
			batch.Checkpoints = append(batch.Checkpoints, *state.Checkpoint)
		}
		batch.FollowTransitions = append(batch.FollowTransitions, source.FollowTransition{OriginID: state.Origin.ID, From: source.FollowUnknown, To: source.FollowFollowing})
		if i < 99 {
			retire = append(retire, source.FollowTransition{OriginID: state.Origin.ID, From: source.FollowFollowing, To: source.FollowRetired})
		}
	}
	if err := store.Commit(ctx, batch); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(ctx, source.Batch{Source: identity, FollowTransitions: retire}); err != nil {
		t.Fatal(err)
	}
	result, err := LoadFollowOrigins(ctx, identity.ID, path, store, 101)
	if err != nil || result.Status != FollowOriginsComplete || result.Examined != 101 || !reflect.DeepEqual(result.States, states[99:]) {
		t.Fatal("SQLite candidates", result, err)
	}
	if result, err := LoadFollowOrigins(ctx, identity.ID, path, store, 100); err != nil || result.Status != FollowOriginsLimit || result.States != nil {
		t.Fatal("SQLite partial history planned", result, err)
	}
	if result, err := LoadFollowOrigins(ctx, "foreign", path, store, 101); err != nil || result.Status != FollowOriginsAbsent || result.States != nil {
		t.Fatal("foreign source selected", result, err)
	}
	// Planning leaves the persisted follow state and checkpoints unchanged.
	again, err := LoadFollowOrigins(ctx, identity.ID, path, store, 101)
	if err != nil || !reflect.DeepEqual(again, result) {
		t.Fatal("planning changed persisted candidates", again, err)
	}
}
