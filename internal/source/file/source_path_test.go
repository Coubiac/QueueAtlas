package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func pathFollower(t *testing.T, contents string) (*FileSource, *Ingestor, *os.File) {
	t.Helper()
	f, path := testRegularFile(t, contents)
	reader := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
		t.Fatal("descriptor follow queried state")
		return source.OriginPage{}, nil
	})
	s, err := New(sourceConfig(path), reader, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	return s, r, f
}

func TestSourcePathFollowObservesBeforeConsumptionAndPreservesSinkFailure(t *testing.T) {
	for _, failure := range []error{errors.New("sink failure"), io.EOF} {
		s, r, f := pathFollower(t, "first\nnext\n")
		if s.LastPathStatus() != "" {
			t.Fatal("new source returned an observation")
		}
		commits := 0
		err := s.followPath(context.Background(), f, r, sinkFunc(func(context.Context, source.Batch) error {
			commits++
			if s.LastPathStatus() != PathSame {
				t.Fatal("initial observation missing before consumption")
			}
			return failure
		}), func(context.Context, time.Duration) error { t.Fatal("sink failure polled"); return nil })
		if !errors.Is(err, failure) || commits != 1 || r.Position().Offset != 0 || s.LastPathStatus() != PathSame {
			t.Fatalf("failed follow: %v, commits %d, position %+v, status %s", err, commits, r.Position(), s.LastPathStatus())
		}
	}
}

func TestSourcePathFollowCancellationAndNextRunReset(t *testing.T) {
	s, r, f := pathFollower(t, "partial")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits := 0
	err := s.followPath(ctx, f, r, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("partial line committed"); return nil }), func(context.Context, time.Duration) error {
		waits++
		cancel()
		return ctx.Err()
	})
	if !errors.Is(err, context.Canceled) || waits != 1 || r.Position().Offset != 0 || s.LastPathStatus() != PathSame {
		t.Fatalf("cancelled follow: %v, waits %d, position %+v, status %s", err, waits, r.Position(), s.LastPathStatus())
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal(err)
	}
	if err := os.Remove(f.Name()); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), sinkFunc(func(context.Context, source.Batch) error { t.Fatal("absent input committed"); return nil })); !errors.Is(err, ErrPathStateReaderRequired) || s.LastPathStatus() != "" {
		t.Fatalf("next run retained status: %v, %s", err, s.LastPathStatus())
	}
}

func TestSourcePathFollowReturnsObservationErrorBeforeConsuming(t *testing.T) {
	s, r, f := pathFollower(t, "line\n")
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	err := s.followPath(context.Background(), f, r, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("invalid observation consumed input"); return nil }), func(context.Context, time.Duration) error { t.Fatal("invalid observation waited"); return nil })
	if err == nil || r.Position().Offset != 0 || s.LastPathStatus() != "" {
		t.Fatalf("initial observation error: %v, position %+v, status %s", err, r.Position(), s.LastPathStatus())
	}
}
