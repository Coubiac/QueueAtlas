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

	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func retirementState(t *testing.T, s *FileSource, store *sqlite.Store, originID string) source.OriginState {
	t.Helper()
	page, err := store.FileOriginsByPath(context.Background(), source.OriginPathQuery{SourceID: s.ID(), Path: s.config.Path, Limit: 100})
	if err != nil || page.NextID != "" {
		t.Fatal("retirement state page", page, err)
	}
	for _, state := range page.States {
		if state.Origin.ID == originID {
			return state
		}
	}
	t.Fatal("retirement origin absent")
	return source.OriginState{}
}

func TestRetirementSinkFailureStopsBeforeOpeningThirdGeneration(t *testing.T) {
	for _, kind := range []string{"sink error", "sink EOF", "conflict", "cancel before ack", "cancel after ack", "close error"} {
		t.Run(kind, func(t *testing.T) {
			f, path := testRegularFile(t, "first\n")
			s, store := rotationSource(t, path, time.Second)
			r, _, err := s.prepareGeneration(context.Background(), f, store)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
			retirements, records, registrations, waits := 0, 0, 0, 0
			boom := errors.New("synthetic retirement failure")
			wantError, wantState := error(boom), source.FollowFollowing
			switch kind {
			case "sink EOF":
				wantError = io.EOF
			case "conflict":
				wantError = sqlite.ErrFollowStateConflict
			case "cancel before ack":
				wantError = context.Canceled
			case "cancel after ack":
				wantError, wantState = context.Canceled, source.FollowRetired
			case "close error":
				wantError, wantState = fs.ErrClosed, source.FollowRetired
			}
			var before source.OriginState
			err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
				if len(b.FollowTransitions) == 1 && b.FollowTransitions[0].To == source.FollowRetired {
					retirements++
					before = retirementState(t, s, store, r.Position().OriginID)
					if _, err := f.Stat(); err != nil || *before.Checkpoint != r.Position() || before.FollowState != source.FollowFollowing || b.Source != s.config.Identity || b.FollowTransitions[0] != (source.FollowTransition{OriginID: r.Position().OriginID, From: source.FollowFollowing, To: source.FollowRetired}) || len(b.Origins)+len(b.Checkpoints)+len(b.Records) != 0 {
						t.Fatal("retirement before validation/close or changed data", err, before, b)
					}
					switch kind {
					case "sink error", "sink EOF", "conflict":
						return wantError
					case "cancel before ack":
						cancel()
					}
					if err := store.Commit(ctx, b); err != nil {
						return err
					}
					if kind == "close error" {
						return f.Close()
					}
					cancel()
					return nil
				}
				if retirements > 0 {
					t.Fatal("retirement failure followed by extra commit", b)
				}
				records += len(b.Records)
				registrations += len(b.Origins)
				return store.Commit(ctx, b)
			}), func(context.Context, time.Duration) error {
				waits++
				switch waits {
				case 1:
					return rotateTo(path, ".1", "second\n")
				case 2:
					stamp = stamp.Add(s.config.RotationGrace)
					return rotateTo(path, ".2", "third\n")
				default:
					t.Fatal("retirement error retried")
				}
				return nil
			}, func() time.Time { return stamp })
			if !errors.Is(err, wantError) || retirements != 1 || waits != 2 || records != 2 || registrations != 1 || r.Position().Offset != 6 {
				t.Fatal("retirement failure progressed", err, retirements, waits, records, registrations, r.Position())
			}
			after := retirementState(t, s, store, r.Position().OriginID)
			before.FollowState = wantState
			if !reflect.DeepEqual(before, after) {
				t.Fatal("retirement failure changed checkpoint/provenance", before, after)
			}
			page, err := store.FileOriginsByPath(context.Background(), source.OriginPathQuery{SourceID: s.ID(), Path: s.config.Path, Limit: 100})
			if err != nil || len(page.States) != 2 {
				t.Fatal("third generation registered or states lost", page, err)
			}
			for _, state := range page.States {
				if state.Origin.ID != r.Position().OriginID && state.FollowState != source.FollowFollowing {
					t.Fatal("failure invented successor retirement", state)
				}
			}
			if fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".1") != 0 || fileDescriptorCount(t, path+".2") != 0 {
				t.Fatal("retirement failure leaked/opened descriptor")
			}
		})
	}
}
