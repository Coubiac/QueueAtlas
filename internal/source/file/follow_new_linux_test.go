//go:build linux

package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

func newCurrentFixture(t *testing.T, empty bool) (*FileSource, *sqlite.Store, *OpenedFollowSet, FollowCurrent) {
	t.Helper()
	path, locations := locatedOpenFixture(t)
	locations.Locations = locations.Locations[:1]
	s, store := rotationSource(t, path)
	locations.Locations[0].State.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	seedAcquisition(t, s, store, locations.Locations[0].State)
	set, err := OpenFollowLocations(context.Background(), s.config.Identity, locations, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { set.Close() })
	if empty {
		if err := os.Truncate(path, 0); err != nil {
			t.Fatal(err)
		}
	}
	current, err := set.ObserveCurrent(context.Background(), path)
	if err != nil || current.Status != FollowCurrentNew {
		t.Fatal(current, err)
	}
	return s, store, set, current
}

func appendNewTestLine(t *testing.T, path, line string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(line)
	if err = errors.Join(err, f.Close()); err != nil {
		t.Fatal(err)
	}
}

func TestFollowNewAcquiresBeforeRecordsAndFollowsOldLateWrites(t *testing.T) {
	for _, empty := range []bool{false, true} {
		name := "nonempty"
		if empty {
			name = "empty then append"
		}
		t.Run(name, func(t *testing.T) {
			s, store, set, current := newCurrentFixture(t, empty)
			old := set.opened[0]
			appendNewTestLine(t, old.file.Name(), "late\n")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			registrations, acquisitions, waits := 0, 0, 0
			var newID string
			var records []source.Record
			err := s.followOpened(ctx, set, current, sinkFunc(func(ctx context.Context, b source.Batch) error {
				if set.Len() != 0 || set.Close() != nil {
					t.Fatal("owner not emptied before state write")
				}
				for _, origin := range b.Origins {
					if origin.ID == "gen-1" {
						t.Fatal("old re-registered")
					}
					newID = origin.ID
					registrations++
				}
				for _, transition := range b.FollowTransitions {
					if transition.OriginID != newID || transition.From != source.FollowUnknown || transition.To != source.FollowFollowing {
						t.Fatal("old reacquired or wrong transition", transition)
					}
					acquisitions++
				}
				for _, r := range b.Records {
					if r.OriginID == "gen-1" {
						if r.Start != 4 || string(r.Raw) != "late\n" {
							t.Fatal("old replayed", r)
						}
					} else {
						state := retirementState(t, s, store, newID)
						want := "new\n"
						if empty {
							want = "next\n"
						}
						if acquisitions != 1 || state.FollowState != source.FollowFollowing || r.Start != 0 || string(r.Raw) != want {
							t.Fatal("new consumed before acquisition", state, r)
						}
					}
					records = append(records, r)
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				if len(records) == 2 {
					cancel()
				}
				return nil
			}), func(context.Context, time.Duration) error {
				waits++
				if !empty || waits != 1 || len(records) != 1 || registrations != 0 || acquisitions != 0 {
					t.Fatal("empty current registered early or unexpected wait")
				}
				appendNewTestLine(t, s.config.Path, "next\n")
				return nil
			}, time.Now)
			if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || registrations != 1 || acquisitions != 1 || len(records) != 2 {
				t.Fatal("new follow", err, registrations, acquisitions, records)
			}
			oldState := retirementState(t, s, store, "gen-1")
			newState := retirementState(t, s, store, newID)
			wantOffset := int64(4)
			if empty {
				wantOffset = 5
			}
			if oldState.Checkpoint.Offset != 9 || oldState.FollowState != source.FollowFollowing || newState.Checkpoint.Offset != wantOffset || newState.FollowState != source.FollowFollowing {
				t.Fatal("wrong durable state", oldState, newState)
			}
			if !errors.Is(old.file.Close(), fs.ErrClosed) || fileDescriptorCount(t, s.config.Path) != 0 || fileDescriptorCount(t, old.file.Name()) != 0 {
				t.Fatal("descriptor leak")
			}
		})
	}
}

func TestFollowNewPreparationFailureClosesBothWithoutConsuming(t *testing.T) {
	for _, kind := range []string{"registration error", "acquisition error", "acquisition EOF", "cancel before ack", "cancel after ack", "insufficient checkpoint"} {
		t.Run(kind, func(t *testing.T) {
			s, store, set, current := newCurrentFixture(t, false)
			old := set.opened[0]
			appendNewTestLine(t, old.file.Name(), "late\n")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			boom := errors.New("synthetic startup failure")
			want := error(boom)
			wantFollow := source.FollowUnknown
			if kind == "acquisition EOF" {
				want = io.EOF
			}
			if kind == "cancel before ack" || kind == "cancel after ack" {
				want = context.Canceled
			}
			if kind == "cancel after ack" {
				wantFollow = source.FollowFollowing
			}
			if kind == "insufficient checkpoint" {
				f, _, err := OpenLog(ctx, s.config.Path)
				if err != nil {
					t.Fatal(err)
				}
				state := ingestState(t, f, 0)
				state.Origin.ID = "new-zero"
				state.Checkpoint.OriginID = "new-zero"
				state.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
				seedAcquisition(t, s, store, state)
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			normalized, commits := 0, 0
			s.normalize = func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }
			var newID string
			err := s.FollowOpened(ctx, set, current, sinkFunc(func(ctx context.Context, b source.Batch) error {
				commits++
				if set.Len() != 0 || len(b.Records) != 0 || old.ingestor.Position().Offset != 4 || old.ingestor.lines.offset != 4 || normalized != 0 {
					t.Fatal("read before successful startup")
				}
				if len(b.Origins) != 0 {
					newID = b.Origins[0].ID
					if kind == "registration error" {
						return boom
					}
					return store.Commit(ctx, b)
				}
				if len(b.FollowTransitions) != 1 {
					t.Fatal("unexpected batch", b)
				}
				switch kind {
				case "acquisition error", "acquisition EOF":
					return want
				case "cancel before ack":
					cancel()
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				cancel()
				return nil
			}))
			if kind == "insufficient checkpoint" {
				var decision *ResumeDecisionError
				if !errors.As(err, &decision) || decision.Status != SelectionInsufficient || commits != 0 {
					t.Fatal("zero checkpoint replayed", err, commits)
				}
			} else if !errors.Is(err, want) {
				t.Fatal("lost failure", err)
			}
			if errors.Is(err, fs.ErrClosed) || set.Len() != 0 || set.Close() != nil || old.ingestor.Position().Offset != 4 || !errors.Is(old.file.Close(), fs.ErrClosed) || fileDescriptorCount(t, s.config.Path) != 0 {
				t.Fatal("startup cleanup/checkpoint", err)
			}
			if retirementState(t, s, store, "gen-1").FollowState != source.FollowFollowing {
				t.Fatal("invented retirement")
			}
			if kind != "registration error" && kind != "insufficient checkpoint" {
				state := retirementState(t, s, store, newID)
				if state.Checkpoint.Offset != 0 || state.FollowState != wantFollow || commits != 2 {
					t.Fatal("wrong startup acknowledgement", state, commits)
				}
			}
		})
	}
}
