package file

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestContinuousPollBoundsPartialReadBeforeAnchorCheck(t *testing.T) {
	s, r, f := pathFollower(t, "first\n"+strings.Repeat("x", 3*64*1024))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	commits := 0
	observe := r.lines.observe
	changed := false
	r.lines.observe = func(fragment []byte) {
		observe(fragment)
		if r.lines.offset > 6 && !changed {
			changed = true
			if _, err := f.WriteAt([]byte("other\n"), 0); err != nil {
				t.Fatal(err)
			}
			stamp = stamp.Add(s.config.PollInterval)
		}
	}
	err := s.followPathWithClock(ctx, f, r, sinkFunc(func(context.Context, source.Batch) error {
		commits++
		return nil
	}), func(context.Context, time.Duration) error {
		t.Fatal("partial read delayed a due poll until waiting")
		return nil
	}, func() time.Time { return stamp })
	if !errors.Is(err, ErrCheckpointChanged) || commits != 1 || r.Position().Offset != 6 || r.pending != nil {
		t.Fatalf("partial poll: %v, commits %d, position %+v", err, commits, r.Position())
	}
	if r.lines.offset > 6+64*1024 {
		t.Fatalf("due poll waited for %d partial bytes; limit 65536", r.lines.offset-6)
	}
}

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
