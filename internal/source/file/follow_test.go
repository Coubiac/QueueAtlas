package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestFollowWaitsForAppendAndRetainsPartialLine(t *testing.T) {
	f, _ := testRegularFile(t, "first\npartial")
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits := 0
	var records []source.Record
	sink := sinkFunc(func(_ context.Context, b source.Batch) error { records = append(records, b.Records[0]); return nil })
	err = r.follow(ctx, sink, func(context.Context) error {
		waits++
		switch waits {
		case 1:
			if r.Position().Offset != 6 || len(records) != 1 {
				t.Fatal("partial line committed before waiting")
			}
			_, err := f.WriteAt([]byte(" rest\nlast\n"), 13)
			return err
		case 2:
			cancel()
			return context.Canceled
		default:
			t.Fatal("extra EOF wait")
			return nil
		}
	})
	if !errors.Is(err, context.Canceled) || waits != 2 || len(records) != 3 {
		t.Fatalf("follow: %v, waits %d, records %d", err, waits, len(records))
	}
	for i, want := range []string{"first\n", "partial rest\n", "last\n"} {
		if string(records[i].Raw) != want {
			t.Fatalf("record %d: %q", i, records[i].Raw)
		}
	}
	if r.Position().Offset != 24 {
		t.Fatalf("position %+v", r.Position())
	}
}

func TestFollowPropagatesSinkErrorsWithoutWaitingOrRetrying(t *testing.T) {
	for _, failure := range []error{errors.New("sink failed"), io.EOF, fmt.Errorf("sink wrapper: %w", io.EOF)} {
		t.Run(failure.Error(), func(t *testing.T) {
			f, _ := testRegularFile(t, "first\nnext\n")
			state := ingestState(t, f, 0)
			r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), state, testNormalizer)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			var pending source.Batch
			err = r.follow(context.Background(), sinkFunc(func(_ context.Context, b source.Batch) error { calls++; pending = b; return failure }), func(context.Context) error { t.Fatal("Sink error caused EOF waiting"); return nil })
			if !errors.Is(err, failure) || calls != 1 || r.Position() != *state.Checkpoint {
				t.Fatalf("follow: %v, calls %d, position %+v", err, calls, r.Position())
			}
			if err := r.CommitNext(context.Background(), sinkFunc(func(_ context.Context, b source.Batch) error {
				if !reflect.DeepEqual(b, pending) {
					t.Fatal("pending batch changed")
				}
				return nil
			})); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFollowCancellationInterruptsLongEOFWait(t *testing.T) {
	f, _ := testRegularFile(t, "")
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	timer := time.AfterFunc(20*time.Millisecond, cancel)
	defer timer.Stop()
	start := time.Now()
	err = r.Follow(ctx, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("EOF produced a commit"); return nil }), MaxPollInterval)
	if !errors.Is(err, context.Canceled) || time.Since(start) > 2*time.Second {
		t.Fatalf("EOF cancellation took %v: %v", time.Since(start), err)
	}
	if r.Position().Offset != 0 {
		t.Fatal("EOF changed position")
	}
}

func TestFollowTimerResumesAfterAppend(t *testing.T) {
	f, _ := testRegularFile(t, "partial")
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	written := make(chan error, 1)
	timer := time.AfterFunc(20*time.Millisecond, func() {
		_, err := f.WriteAt([]byte(" rest\n"), 7)
		written <- err
	})
	defer timer.Stop()
	calls := 0
	err = r.Follow(ctx, sinkFunc(func(_ context.Context, b source.Batch) error {
		calls++
		if string(b.Records[0].Raw) != "partial rest\n" {
			t.Fatalf("appended line: %+v", b.Records[0])
		}
		cancel()
		return nil
	}), MinPollInterval)
	if writeErr := <-written; writeErr != nil {
		t.Fatal(writeErr)
	}
	if !errors.Is(err, context.Canceled) || calls != 1 || r.Position().Offset != 13 {
		t.Fatalf("append follow: %v, calls %d, position %+v", err, calls, r.Position())
	}
}

func TestFollowPollConfigurationAndReadError(t *testing.T) {
	for _, interval := range []time.Duration{-1, MinPollInterval - 1, MaxPollInterval + 1} {
		f, _ := testRegularFile(t, "line\n")
		r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), testNormalizer)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Follow(context.Background(), sinkFunc(func(context.Context, source.Batch) error { t.Fatal("invalid poll committed"); return nil }), interval); err == nil {
			t.Fatal("invalid poll interval accepted")
		}
		if r.Position().Offset != 0 {
			t.Fatal("invalid poll read a line")
		}
	}
	for _, interval := range []time.Duration{0, MinPollInterval, MaxPollInterval} {
		f, _ := testRegularFile(t, "line\n")
		r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), testNormalizer)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Follow(context.Background(), nil, interval); err == nil {
			t.Fatal("nil sink accepted")
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if err := r.Follow(context.Background(), sinkFunc(func(context.Context, source.Batch) error { t.Fatal("closed input committed"); return nil }), interval); err == nil || errors.Is(err, io.EOF) {
			t.Fatalf("read error: %v", err)
		}
	}
}
