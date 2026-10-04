package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
)

func TestPollStopsOnCurrentFileShrinkIncludingPartialBytes(t *testing.T) {
	for _, tc := range []struct {
		name, tail string
		size       int64
	}{
		{"complete", "", 3},
		{"partial_above_checkpoint", "partial", 10},
		{"oversized_partial", strings.Repeat("x", model.MaxLineBytes+1), model.MaxLineBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, f := pathFollower(t, "first\n"+tc.tail)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			commits, waits := 0, 0
			var acknowledged source.Position
			err := s.followPath(ctx, f, r, sinkFunc(func(_ context.Context, b source.Batch) error {
				commits++
				acknowledged = b.Checkpoints[0]
				return nil
			}), func(context.Context, time.Duration) error {
				waits++
				if waits > 1 {
					t.Fatal("truncated file kept following")
				}
				if r.lines.offset != int64(6+len(tc.tail)) {
					t.Fatal("partial bytes were not consumed", r.lines.offset)
				}
				return f.Truncate(tc.size)
			})
			if !errors.Is(err, ErrFileTruncated) || commits != 1 || waits != 1 || r.Position() != acknowledged || acknowledged.Offset != 6 || r.pending != nil {
				t.Fatalf("shrink: %v, commits %d, waits %d, position %+v", err, commits, waits, r.Position())
			}
			if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("truncation did not close descriptor", err)
			}
		})
	}
}

func TestSizeCheckUsesConsumedOffsetWithoutSeekingOrClosing(t *testing.T) {
	_, r, f := pathFollower(t, "first\nnext\n")
	if err := r.CommitNext(context.Background(), sinkFunc(func(context.Context, source.Batch) error { return nil })); err != nil {
		t.Fatal(err)
	}
	physical, err := f.Seek(0, io.SeekCurrent)
	if err != nil || physical <= r.lines.offset {
		t.Fatal("test requires unread buffered bytes", physical, err)
	}
	// Shrinking only unread bytes is not evidence of consumed data loss.
	if err := f.Truncate(r.lines.offset); err != nil {
		t.Fatal(err)
	}
	position := r.Position()
	if err := checkOpenedSizes(context.Background(), []*openedGeneration{{file: f, ingestor: r}}); err != nil {
		t.Fatal("read-ahead position caused a false diagnostic", err)
	}
	if got, err := f.Seek(0, io.SeekCurrent); err != nil || got != physical || r.Position() != position {
		t.Fatal("size check moved position or closed descriptor", got, err)
	}
	if err := f.Truncate(0); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opened := []*openedGeneration{{file: f, ingestor: r}}
	if err := checkOpenedSizes(ctx, opened); !errors.Is(err, context.Canceled) || errors.Is(err, ErrFileTruncated) {
		t.Fatal("size check replaced cancellation", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkOpenedSizes(context.Background(), opened); err == nil || errors.Is(err, ErrFileTruncated) {
		t.Fatal("stat failure became truncation diagnostic", err)
	}
}

func TestPollAllowsAppendAfterPartialEOF(t *testing.T) {
	s, r, f := pathFollower(t, "first\npart")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	commits, waits := 0, 0
	err := s.followPath(ctx, f, r, sinkFunc(func(_ context.Context, b source.Batch) error {
		commits++
		if commits == 2 {
			if string(b.Records[0].Raw) != "partial\n" {
				t.Fatal("partial append changed", b.Records[0])
			}
			cancel()
		}
		return nil
	}), func(context.Context, time.Duration) error {
		waits++
		if waits > 1 {
			t.Fatal("append did not progress")
		}
		_, err := f.WriteAt([]byte("ial\n"), 10)
		return err
	})
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrFileTruncated) || commits != 2 || waits != 1 || r.Position().Offset != 14 {
		t.Fatalf("normal append: %v, commits %d, waits %d, position %+v", err, commits, waits, r.Position())
	}
}
