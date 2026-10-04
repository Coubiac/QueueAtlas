//go:build linux

package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

func acquiredPathState(t *testing.T, s *FileSource, store *sqlite.Store) source.OriginState {
	t.Helper()
	page, err := store.FileOriginsByPath(context.Background(), source.OriginPathQuery{SourceID: s.ID(), Path: s.config.Path, Limit: 100})
	if err != nil || len(page.States) != 1 {
		t.Fatalf("acquisition state: %+v, %v", page, err)
	}
	return page.States[0]
}

func seedAcquisition(t *testing.T, s *FileSource, store *sqlite.Store, state source.OriginState) {
	t.Helper()
	batch := source.Batch{Source: s.config.Identity, Origins: []source.Origin{state.Origin}, Checkpoints: []source.Position{*state.Checkpoint}}
	if state.FollowState != source.FollowUnknown {
		batch.FollowTransitions = []source.FollowTransition{{OriginID: state.Origin.ID, From: source.FollowUnknown, To: source.FollowFollowing}}
	}
	if err := store.Commit(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if state.FollowState == source.FollowRetired {
		if err := store.Commit(context.Background(), source.Batch{Source: s.config.Identity, FollowTransitions: []source.FollowTransition{{OriginID: state.Origin.ID, From: source.FollowFollowing, To: source.FollowRetired}}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFileSourceAcquiresVerifiedGenerationBeforeFirstRecord(t *testing.T) {
	for _, kind := range []string{"new", "positive", "following", "retired", "explicit zero"} {
		t.Run(kind, func(t *testing.T) {
			f, path := testRegularFile(t, "first\nnext\n")
			s, store := rotationSource(t, path)
			offset := int64(6)
			if kind == "new" || kind == "explicit zero" {
				offset = 0
			}
			state := ingestState(t, f, offset)
			state.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
			if kind == "following" {
				state.FollowState = source.FollowFollowing
			} else if kind == "retired" {
				state.FollowState = source.FollowRetired
			}
			if kind != "new" {
				seedAcquisition(t, s, store, state)
			}
			s.config.ResumePolicy.AllowZeroCheckpoint = kind == "explicit zero"
			normalized, acquisitions, registrations, records := 0, 0, 0, 0
			s.normalize = func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			waiting, err := s.runOpened(ctx, f, sinkFunc(func(ctx context.Context, b source.Batch) error {
				registrations += len(b.Origins)
				if len(b.FollowTransitions) > 0 {
					acquisitions++
					position, err := f.Seek(0, io.SeekCurrent)
					before := acquiredPathState(t, s, store)
					want := source.FollowTransition{OriginID: before.Origin.ID, From: state.FollowState, To: source.FollowFollowing}
					if err != nil || position != offset || normalized != 0 || b.Source != s.config.Identity || !reflect.DeepEqual(b.FollowTransitions, []source.FollowTransition{want}) || len(b.Records)+len(b.Origins)+len(b.Checkpoints) != 0 {
						t.Fatalf("acquisition consumed or changed data: %+v, position %d, normalized %d, error %v", b, position, normalized, err)
					}
					if err := store.Commit(ctx, b); err != nil {
						return err
					}
					after := acquiredPathState(t, s, store)
					before.FollowState = source.FollowFollowing
					if !reflect.DeepEqual(before, after) {
						t.Fatal("acquisition changed provenance or checkpoint", before, after)
					}
					return nil
				}
				if len(b.Records) > 0 {
					records++
					if acquiredPathState(t, s, store).FollowState != source.FollowFollowing || b.Records[0].Start != offset {
						t.Fatal("record preceded durable acquisition or changed offset", b)
					}
					if kind != "new" && b.Records[0].OriginID != state.Origin.ID {
						t.Fatal("resume changed origin")
					}
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				if len(b.Records) > 0 {
					cancel()
				}
				return nil
			}))
			wantAcquisitions, wantRegistrations := 1, 0
			if kind == "following" {
				wantAcquisitions = 0
			}
			if kind == "new" {
				wantRegistrations = 1
			}
			if waiting || !errors.Is(err, context.Canceled) || acquisitions != wantAcquisitions || registrations != wantRegistrations || records != 1 || normalized != 1 {
				t.Fatalf("acquisition: %v, waiting %v, acquisitions %d, registrations %d, records %d, normalized %d", err, waiting, acquisitions, registrations, records, normalized)
			}
			if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("descriptor retained", err)
			}
		})
	}
}

func TestAcquisitionFailureStopsBeforeReadingAndClosesDescriptor(t *testing.T) {
	for _, kind := range []string{"sink error", "sink EOF", "conflict", "cancel before ack", "cancel after ack"} {
		t.Run(kind, func(t *testing.T) {
			f, path := testRegularFile(t, "first\nnext\n")
			s, store := rotationSource(t, path)
			state := ingestState(t, f, 6)
			state.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
			seedAcquisition(t, s, store, state)
			normalized, commits := 0, 0
			s.normalize = func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			boom := errors.New("synthetic acquisition failure")
			wantError, wantState := error(boom), source.FollowUnknown
			switch kind {
			case "sink EOF":
				wantError = io.EOF
			case "conflict":
				wantError, wantState = sqlite.ErrFollowStateConflict, source.FollowRetired
			case "cancel before ack":
				wantError = context.Canceled
			case "cancel after ack":
				wantError, wantState = context.Canceled, source.FollowFollowing
			}
			waiting, err := s.runOpened(ctx, f, sinkFunc(func(ctx context.Context, b source.Batch) error {
				commits++
				position, err := f.Seek(0, io.SeekCurrent)
				if err != nil || position != 6 || normalized != 0 || len(b.FollowTransitions) != 1 || len(b.Records)+len(b.Checkpoints)+len(b.Origins) != 0 {
					t.Fatal("read or write before acquisition", b, position, err)
				}
				switch kind {
				case "sink error", "sink EOF":
					return wantError
				case "conflict":
					// Simulate state changing after selection; the stale acquisition
					// must fail rather than consume a line. Writers must serialize.
					seedAcquisition(t, s, store, source.OriginState{Origin: state.Origin, Checkpoint: state.Checkpoint, FollowState: source.FollowRetired})
				case "cancel before ack":
					cancel()
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				cancel()
				return nil
			}))
			if waiting || !errors.Is(err, wantError) || commits != 1 || normalized != 0 {
				t.Fatalf("failure: %v, waiting %v, commits %d, normalized %d", err, waiting, commits, normalized)
			}
			after := acquiredPathState(t, s, store)
			state.FollowState = wantState
			if !reflect.DeepEqual(after, state) {
				t.Fatal("failure changed provenance/checkpoint", after, state)
			}
			if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("failed acquisition retained descriptor", err)
			}
		})
	}
}

func TestAcquisitionRequiresConstructorVerification(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path)
	registrations, normalized := 0, 0
	s.normalize = func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }
	waiting, err := s.runOpened(context.Background(), f, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if len(b.Origins) != 1 {
			t.Fatal("acquisition preceded verification", b)
		}
		registrations++
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		_, err := f.WriteAt([]byte("other\n"), 0)
		return err
	}))
	if waiting || err == nil || registrations != 1 || normalized != 0 {
		t.Fatal("changed prefix acquired", waiting, err, registrations, normalized)
	}
	state := acquiredPathState(t, s, store)
	if state.FollowState != source.FollowUnknown || state.Checkpoint.Offset != 0 {
		t.Fatal("verification failure changed state", state)
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("verification failure retained descriptor", err)
	}
}
