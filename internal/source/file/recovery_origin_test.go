package file

import (
	"context"
	"errors"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestLoadRecoveryOriginRequiresWholeUniqueExplicitTarget(t *testing.T) {
	for _, kind := range []string{"unknown", "following retry", "nil checkpoint", "zero checkpoint", "empty", "retired only", "retired target", "wrong ID", "two unknown", "unknown and following", "two following", "invalid late", "retired target and other"} {
		t.Run(kind, func(t *testing.T) {
			states := pathOriginStates(3)
			path, target := states[0].Origin.Path, states[1].Origin.ID
			for i := range states {
				states[i].FollowState = source.FollowRetired
			}
			states[1].FollowState = source.FollowUnknown
			want := RecoveryOriginUnknown
			switch kind {
			case "following retry":
				states[1].FollowState = source.FollowFollowing
				want = RecoveryOriginFollowing
			case "nil checkpoint":
				states[1].Checkpoint = nil
			case "zero checkpoint":
				states[1].Checkpoint.Offset = 0
			case "empty":
				states = nil
				want = RecoveryOriginAbsent
			case "retired only":
				states[1].FollowState = source.FollowRetired
				target = "unlisted"
				want = RecoveryOriginAbsent
			case "retired target":
				states[1].FollowState = source.FollowRetired
				want = RecoveryOriginConflict
			case "wrong ID":
				target = "unlisted"
				want = RecoveryOriginConflict
			case "two unknown":
				states[2].FollowState = source.FollowUnknown
				want = RecoveryOriginConflict
			case "unknown and following":
				states[2].FollowState = source.FollowFollowing
				want = RecoveryOriginConflict
			case "two following":
				states[1].FollowState, states[2].FollowState = source.FollowFollowing, source.FollowFollowing
				want = RecoveryOriginConflict
			case "invalid late":
				states[2].FollowState = 99
				want = RecoveryOriginInvalidState
			case "retired target and other":
				states[1].FollowState, states[2].FollowState = source.FollowRetired, source.FollowUnknown
				want = RecoveryOriginConflict
			}
			calls := 0
			reader := pathReaderFunc(func(_ context.Context, q source.OriginPathQuery) (source.OriginPage, error) {
				calls++
				if q.SourceID != "synthetic-source" || q.Path != path || q.AfterID != "" || q.Limit != 3 {
					t.Fatal("wrong namespace or budget", q)
				}
				return source.OriginPage{States: states}, nil
			})
			result, err := LoadRecoveryOrigin(context.Background(), "synthetic-source", path, target, reader, 3)
			if err != nil || result.Status != want || result.Examined != len(states) || calls != 1 {
				t.Fatal("classification", result, err, calls)
			}
			eligible := want == RecoveryOriginUnknown || want == RecoveryOriginFollowing
			if (result.State != nil) != eligible {
				t.Fatal("partial candidate or missing eligible metadata", result)
			}
			if eligible {
				if result.State.Origin.ID != target {
					t.Fatal("target changed")
				}
				if kind == "nil checkpoint" && result.State.Checkpoint != nil {
					t.Fatal("invented checkpoint")
				}
				if states[1].Checkpoint != nil {
					before := *result.State.Checkpoint
					states[1].Checkpoint.Offset = 777
					if *result.State.Checkpoint != before {
						t.Fatal("borrowed reader checkpoint")
					}
				}
			}
		})
	}
}

func TestLoadRecoveryOriginCountsRetiredPagesBeforeEligibility(t *testing.T) {
	states := pathOriginStates(101)
	for i := range states {
		states[i].FollowState = source.FollowRetired
	}
	states[100].FollowState = source.FollowUnknown
	for _, limit := range []int{100, 101} {
		calls := 0
		reader := pathReaderFunc(func(_ context.Context, q source.OriginPathQuery) (source.OriginPage, error) {
			start := 0
			if q.AfterID != "" {
				if q.AfterID != states[99].Origin.ID {
					t.Fatal("unexpected cursor", q)
				}
				start = 100
			}
			end := min(start+q.Limit, len(states))
			calls++
			page := source.OriginPage{States: states[start:end]}
			if end < len(states) {
				page.NextID = states[end-1].Origin.ID
			}
			return page, nil
		})
		result, err := LoadRecoveryOrigin(context.Background(), "synthetic-source", states[0].Origin.Path, states[100].Origin.ID, reader, limit)
		if err != nil || result.Examined != limit {
			t.Fatal(result, err)
		}
		if limit == 100 && (result.Status != RecoveryOriginLimit || result.State != nil || calls != 1) {
			t.Fatal("limited scan exposed a plan", result, calls)
		}
		if limit == 101 && (result.Status != RecoveryOriginUnknown || result.State == nil || result.State.Origin.ID != states[100].Origin.ID || calls != 2) {
			t.Fatal("last-page target lost", result, calls)
		}
	}
}

func TestLoadRecoveryOriginRejectsInvalidInputsAndDiscardsReadFailure(t *testing.T) {
	boom := errors.New("synthetic recovery read error")
	for _, kind := range []string{"ID", "source", "path", "reader", "limit zero", "limit max", "canceled", "read error", "read cancel", "bad page"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			id, sourceID, path, limit := "target", "synthetic-source", "missing-directory/mail.log", 1
			calls := 0
			reader := source.PathStateReader(pathReaderFunc(func(context.Context, source.OriginPathQuery) (source.OriginPage, error) {
				calls++
				switch kind {
				case "read error":
					return source.OriginPage{}, boom
				case "read cancel":
					cancel()
					return source.OriginPage{}, nil
				case "bad page":
					return source.OriginPage{NextID: "not-a-row"}, nil
				default:
					t.Fatal("invalid input accessed state")
					return source.OriginPage{}, nil
				}
			}))
			var want error
			switch kind {
			case "ID":
				id = ""
			case "source":
				sourceID = ""
			case "path":
				path = ""
			case "reader":
				reader = nil
			case "limit zero":
				limit = 0
			case "limit max":
				limit = MaxPathOrigins + 1
			case "canceled":
				cancel()
				want = context.Canceled
			case "read error":
				want = boom
			case "read cancel":
				want = context.Canceled
			case "bad page":
				want = ErrInvalidPathOriginPage
			}
			result, err := LoadRecoveryOrigin(ctx, sourceID, path, id, reader, limit)
			if result != (RecoveryOrigin{}) || err == nil || want != nil && !errors.Is(err, want) {
				t.Fatal("failure exposed state or lost cause", result, err)
			}
			if calls != 0 && kind != "read error" && kind != "read cancel" && kind != "bad page" {
				t.Fatal("invalid input read state")
			}
		})
	}
}
