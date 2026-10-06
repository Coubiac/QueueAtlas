package file

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestRecoverUnknownCurrentBlocksBeforeJournalAndPreservesStatus(t *testing.T) {
	for _, kind := range []string{"canceled", "ID", "sink", "reader", "running", "read error", "absent", "conflict", "invalid", "retired", "limit"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "not-created", "mail.log")
			states := pathOriginStates(2)
			for i := range states {
				states[i].Origin.Path = path
			}
			states[0].FollowState, states[1].FollowState = source.FollowUnknown, source.FollowRetired
			target := states[0].Origin.ID
			boom := errors.New("synthetic recovery state error")
			calls := 0
			base := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
				t.Fatal("recovery used physical selection")
				return source.OriginPage{}, nil
			})
			reader := source.StateReader(statePathReader{base, pathReaderFunc(func(_ context.Context, q source.OriginPathQuery) (source.OriginPage, error) {
				calls++
				if kind == "read error" {
					return source.OriginPage{}, boom
				}
				if kind == "absent" {
					return source.OriginPage{}, nil
				}
				if kind == "limit" {
					return source.OriginPage{States: states[:1], NextID: states[0].Origin.ID}, nil
				}
				return source.OriginPage{States: states}, nil
			})})
			cfg := sourceConfig(path)
			var want error
			var status RecoveryOriginStatus
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sink := source.Sink(sinkFunc(func(context.Context, source.Batch) error { t.Fatal("blocked recovery committed"); return nil }))
			switch kind {
			case "canceled":
				cancel()
				want = context.Canceled
			case "ID":
				target = ""
				want = ErrInvalidRecoveryOrigin
			case "sink":
				sink = nil
			case "reader":
				reader = base
				want = ErrPathStateReaderRequired
			case "running":
				want = ErrSourceRunning
			case "read error":
				want = boom
			case "absent":
				status = RecoveryOriginAbsent
			case "conflict":
				states[1].FollowState = source.FollowFollowing
				status = RecoveryOriginConflict
			case "invalid":
				states[1].FollowState = 99
				status = RecoveryOriginInvalidState
			case "retired":
				states[0].FollowState = source.FollowRetired
				status = RecoveryOriginConflict
			case "limit":
				cfg.ResumeLimits.Origins = 1
				status = RecoveryOriginLimit
			}
			s, err := New(cfg, reader, testNormalizer)
			if err != nil {
				t.Fatal(err)
			}
			s.setPathStatus(PathMissing)
			if kind == "running" {
				s.running.Lock()
			}
			err = s.recoverUnknownCurrent(ctx, target, sink, func(context.Context, string, RecoveryOrigin) (*openedRecovery, error) {
				t.Fatal("blocked recovery accessed journal")
				return nil, nil
			})
			if kind == "running" {
				s.running.Unlock()
			}
			if err == nil || want != nil && !errors.Is(err, want) || s.LastPathStatus() != PathMissing {
				t.Fatal("rejection/status", err)
			}
			if status != "" {
				var decision *RecoveryDecisionError
				if !errors.As(err, &decision) || decision.Status != status || err.Error() != "file recovery decision required: "+string(status) {
					t.Fatal("diagnostic", err)
				}
			}
			if calls != 0 && status == "" && kind != "read error" {
				t.Fatal("invalid input read state")
			}
			if !s.running.TryLock() {
				t.Fatal("guard leaked")
			}
			s.running.Unlock()
		})
	}
}
