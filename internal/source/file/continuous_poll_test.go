package file

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestIdleRoundWaitsOnlyUntilScheduledPoll(t *testing.T) {
	s, r, f := pathFollower(t, "line\n")
	s.config.PollInterval = 100 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	commits, waits := 0, 0
	err := s.followPathWithClock(ctx, f, r, sinkFunc(func(context.Context, source.Batch) error {
		commits++
		stamp = stamp.Add(75 * time.Millisecond)
		return nil
	}), func(_ context.Context, delay time.Duration) error {
		waits++
		if delay != 25*time.Millisecond {
			t.Fatalf("idle delay %v; want remaining 25ms", delay)
		}
		cancel()
		return ctx.Err()
	}, func() time.Time { return stamp })
	if !errors.Is(err, context.Canceled) || commits != 1 || waits != 1 || r.Position().Offset != 5 {
		t.Fatalf("scheduled idle wait: %v, commits %d, waits %d, position %+v", err, commits, waits, r.Position())
	}
}
