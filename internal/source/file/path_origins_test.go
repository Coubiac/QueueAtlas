package file

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

type pathReaderFunc func(context.Context, source.OriginPathQuery) (source.OriginPage, error)

func (r pathReaderFunc) FileOriginsByPath(ctx context.Context, q source.OriginPathQuery) (source.OriginPage, error) {
	return r(ctx, q)
}

func pathOriginStates(count int) []source.OriginState {
	states := make([]source.OriginState, count)
	for i := range states {
		id := fmt.Sprintf("gen-%04d", i)
		states[i] = source.OriginState{
			Origin:     source.Origin{ID: id, Path: "/synthetic/mail.log", Fingerprint: "unverified-prefix"},
			Checkpoint: &source.Position{OriginID: id, Offset: int64(i), AnchorHash: "unverified-anchor"},
		}
	}
	return states
}

func pathPages(t *testing.T, states []source.OriginState, pageSize int, calls *int) pathReaderFunc {
	t.Helper()
	index := 0
	return func(_ context.Context, q source.OriginPathQuery) (source.OriginPage, error) {
		(*calls)++
		if q.SourceID != "mail" || q.Path != "/synthetic/mail.log" || q.Limit < 1 || q.Limit > source.MaxOriginPageSize {
			t.Fatal("unexpected path query", q)
		}
		if index == 0 && q.AfterID != "" || index > 0 && q.AfterID != states[index-1].Origin.ID {
			t.Fatal("incorrect exclusive cursor", q.AfterID)
		}
		end := min(len(states), index+min(q.Limit, pageSize))
		page := source.OriginPage{States: states[index:end]}
		index = end
		if index < len(states) {
			page.NextID = states[index-1].Origin.ID
		}
		return page, nil
	}
}

func TestLoadPathOriginsCompletesPagesAndCopiesCheckpoints(t *testing.T) {
	states := pathOriginStates(3)
	states[0].Checkpoint = nil
	states[1].Checkpoint.Offset = 0
	calls := 0
	read := pathPages(t, states, 1, &calls)
	result, err := LoadPathOrigins(context.Background(), "mail", "/synthetic/mail.log", pathReaderFunc(func(ctx context.Context, q source.OriginPathQuery) (source.OriginPage, error) {
		// Reuse the prior page's checkpoint storage during the next request.
		if calls == 2 {
			states[1].Checkpoint.Offset = 999
		}
		return read(ctx, q)
	}), 3)
	if err != nil || result.Status != PathOriginsComplete || len(result.States) != 3 || result.Examined != 3 || calls != 3 {
		t.Fatal("completed scan", result, err, calls)
	}
	if result.States[0].Checkpoint != nil || result.States[1].Checkpoint.Offset != 0 || result.States[2].Checkpoint.Offset != 2 {
		t.Fatal("nil/zero/positive checkpoints or reused page changed result", result.States)
	}
	states[2].Checkpoint.AnchorHash = "reader mutation"
	states[2].Origin.Path = "reader mutation"
	if result.States[2].Checkpoint.AnchorHash != "unverified-anchor" || result.States[2].Origin.Path != "/synthetic/mail.log" {
		t.Fatal("reader mutation aliased returned state")
	}
	result.States[1].Checkpoint.Offset = 123
	result.States[1].Origin.Path = "consumer mutation"
	if states[1].Checkpoint.Offset != 999 || states[1].Origin.Path != "/synthetic/mail.log" {
		t.Fatal("consumer mutation aliased reader state")
	}
}

func TestLoadPathOriginsAbsenceAndTotalBudget(t *testing.T) {
	for _, tc := range []struct {
		count, limit int
		status       PathOriginsStatus
	}{{0, 1, PathOriginsAbsent}, {101, 101, PathOriginsComplete}, {102, 101, PathOriginsLimit}} {
		states, calls := pathOriginStates(tc.count), 0
		result, err := LoadPathOrigins(context.Background(), "mail", "/synthetic/mail.log", pathPages(t, states, source.MaxOriginPageSize, &calls), tc.limit)
		if err != nil || result.Status != tc.status || result.Examined != min(tc.count, tc.limit) {
			t.Fatal("budget scan", result, err)
		}
		if tc.status == PathOriginsComplete {
			if len(result.States) != tc.count || calls != 2 {
				t.Fatal("complete scan lost states", len(result.States), calls)
			}
		} else if result.States != nil || calls != max(1, (tc.limit+source.MaxOriginPageSize-1)/source.MaxOriginPageSize) {
			t.Fatal("incomplete scan exposed states or exceeded bounded requests", result, calls)
		}
	}
}

func TestLoadPathOriginsRejectsIncoherentPagesWithoutPartialResult(t *testing.T) {
	states := pathOriginStates(3)
	for _, page := range []source.OriginPage{
		{States: states[:2]}, // exceeds remaining budget after the first page
		{States: states[:1]}, // stale ID from the first page
		{States: []source.OriginState{{Origin: source.Origin{ID: "gen-0001", Path: "other"}}}},
		{States: []source.OriginState{{Origin: source.Origin{Path: "/synthetic/mail.log"}}}},
		{States: states[1:2], NextID: "unknown"},
		{NextID: "gen-0001"},
	} {
		calls := 0
		result, err := LoadPathOrigins(context.Background(), "mail", "/synthetic/mail.log", pathReaderFunc(func(context.Context, source.OriginPathQuery) (source.OriginPage, error) {
			calls++
			if calls == 1 {
				return source.OriginPage{States: states[:1], NextID: states[0].Origin.ID}, nil
			}
			return page, nil
		}), 2)
		if !errors.Is(err, ErrInvalidPathOriginPage) || !reflect.DeepEqual(result, PathOrigins{}) || calls != 2 {
			t.Fatal("invalid page exposed a partial result", result, err, calls)
		}
	}
	for _, page := range []source.OriginPage{
		{States: []source.OriginState{states[1], states[0]}},
		{States: []source.OriginState{states[0], states[0]}},
	} {
		if err := validatePathOriginPage(page, source.OriginPathQuery{Path: "/synthetic/mail.log", Limit: 2}); !errors.Is(err, ErrInvalidPathOriginPage) {
			t.Fatal("unordered/duplicate IDs accepted", err)
		}
	}
}

func TestLoadPathOriginsValidatesArgumentsAndDiscardsErrorsOrCancellation(t *testing.T) {
	never := pathReaderFunc(func(context.Context, source.OriginPathQuery) (source.OriginPage, error) {
		t.Fatal("invalid request called reader")
		return source.OriginPage{}, nil
	})
	for _, tc := range []struct {
		sourceID, path string
		reader         source.PathStateReader
		limit          int
	}{{"", "/synthetic/mail.log", never, 1}, {"mail", "", never, 1}, {"mail", "/synthetic/mail.log", nil, 1}, {"mail", "/synthetic/mail.log", never, 0}, {"mail", "/synthetic/mail.log", never, -1}, {"mail", "/synthetic/mail.log", never, MaxPathOrigins + 1}} {
		if result, err := LoadPathOrigins(context.Background(), tc.sourceID, tc.path, tc.reader, tc.limit); err == nil || !reflect.DeepEqual(result, PathOrigins{}) {
			t.Fatal("invalid arguments", result, err)
		}
	}
	boom := errors.New("synthetic reader failure")
	for _, cancelDuringRead := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		states, calls := pathOriginStates(2), 0
		result, err := LoadPathOrigins(ctx, "mail", "/synthetic/mail.log", pathReaderFunc(func(context.Context, source.OriginPathQuery) (source.OriginPage, error) {
			calls++
			if calls == 1 {
				return source.OriginPage{States: states[:1], NextID: states[0].Origin.ID}, nil
			}
			if cancelDuringRead {
				cancel()
				return source.OriginPage{States: states[1:]}, nil
			}
			return source.OriginPage{}, boom
		}), 2)
		cancel()
		want := boom
		if cancelDuringRead {
			want = context.Canceled
		}
		if !errors.Is(err, want) || !reflect.DeepEqual(result, PathOrigins{}) || calls != 2 {
			t.Fatal("error/cancellation exposed partial result", result, err, calls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := LoadPathOrigins(ctx, "mail", "/synthetic/mail.log", never, 1); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, PathOrigins{}) {
		t.Fatal("pre-cancelled scan called reader", result, err)
	}
}
