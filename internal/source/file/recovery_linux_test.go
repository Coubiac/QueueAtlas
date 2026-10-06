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

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func recoveryStoredFixture(t *testing.T, offset int64) (*FileSource, *sqlite.Store, source.OriginState, *int) {
	t.Helper()
	path, selected := recoveryOpenFixture(t, offset)
	state := *selected.State
	state.Origin.FirstSeen = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	s, store := rotationSource(t, path)
	if err := store.Commit(context.Background(), source.Batch{Source: s.config.Identity, Origins: []source.Origin{state.Origin}, Checkpoints: []source.Position{*state.Checkpoint}}); err != nil {
		t.Fatal(err)
	}
	normalized := 0
	s.normalize = func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }
	s.setPathStatus(PathMissing)
	return s, store, state, &normalized
}

func assertRecoveryTransition(t *testing.T, s *FileSource, state source.OriginState, batch source.Batch) {
	t.Helper()
	if batch.Source != s.config.Identity || len(batch.Origins)+len(batch.Records)+len(batch.Checkpoints) != 0 || !reflect.DeepEqual(batch.FollowTransitions, []source.FollowTransition{{OriginID: state.Origin.ID, From: source.FollowUnknown, To: source.FollowFollowing}}) {
		t.Fatal("recovery changed ingestion/provenance", batch)
	}
	if s.running.TryLock() {
		s.running.Unlock()
		t.Fatal("commit outside guard")
	}
}

func TestRecoverUnknownCurrentAcquiresOnlyLifecycleAndRechecksRetry(t *testing.T) {
	for _, offset := range []int64{0, 5} {
		s, store, before, normalized := recoveryStoredFixture(t, offset)
		commits := 0
		err := s.RecoverUnknownCurrent(context.Background(), before.Origin.ID, sinkFunc(func(ctx context.Context, batch source.Batch) error {
			commits++
			assertRecoveryTransition(t, s, before, batch)
			return store.Commit(ctx, batch)
		}))
		if err != nil || commits != 1 {
			t.Fatal(err, commits)
		}
		after := retirementState(t, s, store, before.Origin.ID)
		if after.Origin != before.Origin || *after.Checkpoint != *before.Checkpoint || after.FollowState != source.FollowFollowing || *normalized != 0 || s.LastPathStatus() != PathMissing || fileDescriptorCount(t, s.config.Path) != 0 {
			t.Fatal("recovery changed more than lifecycle", after)
		}
		if err := s.RecoverUnknownCurrent(context.Background(), before.Origin.ID, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("already following committed"); return nil })); err != nil {
			t.Fatal("retry", err)
		}
		if offset == 0 {
			err := s.Run(context.Background(), sinkFunc(func(context.Context, source.Batch) error {
				t.Fatal("lifecycle recovery implicitly replayed zero")
				return nil
			}))
			var decision *ResumeDecisionError
			if !errors.As(err, &decision) || decision.Status != SelectionInsufficient {
				t.Fatal("zero replay policy bypassed", err)
			}
		}
		// A retry must obtain fresh proof rather than trust the prior ACK.
		if err := rotateTo(s.config.Path, ".1", "seed\n"); err != nil {
			t.Fatal(err)
		}
		err = s.RecoverUnknownCurrent(context.Background(), before.Origin.ID, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("replacement reacquired"); return nil }))
		var decision *ResumeDecisionError
		if !errors.As(err, &decision) || decision.Status != SelectionDifferent {
			t.Fatal("retry used stale proof", err)
		}
	}
}

func TestRecoverUnknownCurrentFailureAckLossCancellationAndClose(t *testing.T) {
	for _, kind := range []string{"sink error", "sink EOF", "lost ACK", "cancel before commit", "cancel before ACK", "cancel after ACK", "close after ACK", "retired before retry"} {
		t.Run(kind, func(t *testing.T) {
			s, store, before, normalized := recoveryStoredFixture(t, 0)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			boom := errors.New("synthetic recovery ACK error")
			want := error(boom)
			wantState := source.FollowUnknown
			switch kind {
			case "sink EOF":
				want = io.EOF
			case "lost ACK":
				wantState = source.FollowFollowing
			case "cancel before commit", "cancel before ACK":
				want = context.Canceled
			case "cancel after ACK":
				want = context.Canceled
				wantState = source.FollowFollowing
			case "close after ACK":
				want = fs.ErrClosed
				wantState = source.FollowFollowing
			case "retired before retry":
				wantState = source.FollowRetired
			}
			var captured *openedRecovery
			commits := 0
			err := s.recoverUnknownCurrent(ctx, before.Origin.ID, sinkFunc(func(ctx context.Context, batch source.Batch) error {
				commits++
				assertRecoveryTransition(t, s, before, batch)
				switch kind {
				case "sink error", "sink EOF":
					return want
				case "cancel before ACK":
					cancel()
					return store.Commit(ctx, batch)
				}
				if err := store.Commit(ctx, batch); err != nil {
					return err
				}
				switch kind {
				case "lost ACK":
					return boom
				case "cancel after ACK":
					cancel()
				case "close after ACK":
					if err := captured.file.Close(); err != nil {
						t.Fatal(err)
					}
				case "retired before retry":
					if err := store.Commit(ctx, source.Batch{Source: s.config.Identity, FollowTransitions: []source.FollowTransition{{OriginID: before.Origin.ID, From: source.FollowFollowing, To: source.FollowRetired}}}); err != nil {
						t.Fatal(err)
					}
					return boom
				}
				return nil
			}), func(ctx context.Context, path string, selected RecoveryOrigin) (*openedRecovery, error) {
				owned, err := prepareRecoveryCurrent(ctx, path, selected)
				captured = owned
				if err == nil && kind == "cancel before commit" {
					cancel()
				}
				return owned, err
			})
			wantCommits := 1
			if kind == "cancel before commit" {
				wantCommits = 0
			}
			if !errors.Is(err, want) || commits != wantCommits || captured == nil || captured.file != nil || captured.Close() != nil {
				t.Fatal("failure/cleanup/retry count", err, commits)
			}
			after := retirementState(t, s, store, before.Origin.ID)
			if after.FollowState != wantState || after.Origin != before.Origin || *after.Checkpoint != *before.Checkpoint || *normalized != 0 || s.LastPathStatus() != PathMissing || fileDescriptorCount(t, s.config.Path) != 0 {
				t.Fatal("failure changed durable state", after)
			}
			if !s.running.TryLock() {
				t.Fatal("guard leaked")
			}
			s.running.Unlock()
			// Explicit invocation reloads authoritative state; no automatic retry.
			retryCommits := 0
			err = s.RecoverUnknownCurrent(context.Background(), before.Origin.ID, sinkFunc(func(ctx context.Context, batch source.Batch) error {
				retryCommits++
				assertRecoveryTransition(t, s, before, batch)
				return store.Commit(ctx, batch)
			}))
			if wantState == source.FollowRetired {
				var decision *RecoveryDecisionError
				if !errors.As(err, &decision) || decision.Status != RecoveryOriginConflict || retryCommits != 0 || retirementState(t, s, store, before.Origin.ID).FollowState != source.FollowRetired {
					t.Fatal("retired target reactivated", err)
				}
			} else {
				wantRetries := 0
				if wantState == source.FollowUnknown {
					wantRetries = 1
				}
				if err != nil || retryCommits != wantRetries || retirementState(t, s, store, before.Origin.ID).FollowState != source.FollowFollowing {
					t.Fatal("explicit retry", err, retryCommits)
				}
			}
		})
	}
}
