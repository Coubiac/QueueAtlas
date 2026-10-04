//go:build linux

package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestContinuousPollPartialLineYieldsToSuccessor(t *testing.T) {
	f, path := testRegularFile(t, "first\n"+strings.Repeat("x", 3*64*1024)+"\n")
	s, store := rotationSource(t, path)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	var records []source.Record
	err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		if len(b.Records) == 0 {
			return nil
		}
		records = append(records, b.Records[0])
		if len(records) == 1 {
			stamp = stamp.Add(s.config.PollInterval)
			return rotateTo(path, ".1", "new\n")
		}
		if string(b.Records[0].Raw) != "new\n" || r.lines.offset > 6+64*1024 || r.Position().Offset != 6 || r.pending != nil {
			t.Fatal("partial old line monopolized the round or advanced its checkpoint")
		}
		cancel()
		return nil
	}), func(context.Context, time.Duration) error {
		t.Fatal("read progress triggered a wait")
		return nil
	}, func() time.Time { return stamp })
	if !errors.Is(err, context.Canceled) || len(records) != 2 || records[0].OriginID == records[1].OriginID {
		t.Fatalf("partial joint follow: %v, records %d", err, len(records))
	}
	position, found, err := store.Checkpoint(context.Background(), s.ID(), r.Position().OriginID)
	if err != nil || !found || position.Offset != 6 {
		t.Fatal("partial line changed durable checkpoint", position, found, err)
	}
	if fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".1") != 0 {
		t.Fatal("partial joint follow leaked a descriptor")
	}
}

func TestContinuousPollSeesRotationBeforeOldReaderEOF(t *testing.T) {
	f, path := testRegularFile(t, "old-1\nold-2\nold-3\nold-4\n")
	s, store := rotationSource(t, path)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	var records []source.Record
	err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		if len(b.Records) == 0 {
			return nil
		}
		records = append(records, b.Records[0])
		stamp = stamp.Add(s.config.PollInterval)
		if len(records) == 1 {
			return rotateTo(path, ".1", "new-1\nnew-2\nnew-3\n")
		}
		if string(b.Records[0].Raw) == "new-1\n" && r.lines.offset >= 24 {
			t.Fatal("rotation was deferred until all old lines were consumed")
		}
		if string(b.Records[0].Raw) == "new-3\n" {
			cancel()
		}
		return nil
	}), func(context.Context, time.Duration) error { t.Fatal("continuous readers waited"); return nil }, func() time.Time { return stamp })
	if !errors.Is(err, context.Canceled) || len(records) != 7 {
		t.Fatalf("continuous rotation: %v, records %+v", err, records)
	}
	ends := make(map[string]int64)
	for i, want := range []string{"old-1\n", "old-2\n", "new-1\n", "old-3\n", "new-2\n", "old-4\n", "new-3\n"} {
		record := records[i]
		if string(record.Raw) != want || record.Start != ends[record.OriginID] {
			t.Fatalf("record %d order or offset: %+v", i, record)
		}
		ends[record.OriginID] = record.End
	}
	if len(ends) != 2 {
		t.Fatal("continuous rotation mixed origins")
	}
	for originID, offset := range ends {
		position, found, err := store.Checkpoint(context.Background(), s.ID(), originID)
		if err != nil || !found || position.Offset != offset {
			t.Fatal("continuous checkpoint", position, found, err)
		}
	}
}

func TestContinuousPollExpiresOldFileWhileNewReaderHasMoreLines(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path, 2*MinPollInterval)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	records := 0
	err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		if len(b.Records) == 0 {
			return nil
		}
		records++
		stamp = stamp.Add(s.config.PollInterval)
		if records == 1 {
			return rotateTo(path, ".1", "new-1\nnew-2\nnew-3\nnew-4\nnew-5\n")
		}
		if records == 4 {
			if err := f.Close(); !errors.Is(err, fs.ErrClosed) || fileDescriptorCount(t, path+".1") != 0 {
				t.Fatal("old file not retired during new stream", err)
			}
			if b.Records[0].End != 18 {
				t.Fatal("expiry was deferred until new reader EOF")
			}
			cancel()
		}
		return nil
	}), func(context.Context, time.Duration) error { t.Fatal("new stream waited for expiry"); return nil }, func() time.Time { return stamp })
	if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || records != 4 || r.Position().Offset != 6 || fileDescriptorCount(t, path) != 0 {
		t.Fatalf("continuous expiry: %v, records %d, position %+v", err, records, r.Position())
	}
}

func TestContinuousPollSinkEOFStopsBeforeDuePathCheck(t *testing.T) {
	f, path := testRegularFile(t, "first\nnext\n")
	s, store := rotationSource(t, path)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	commits := 0
	err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		commits++
		if commits == 2 {
			stamp = stamp.Add(s.config.PollInterval)
			return io.EOF
		}
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
		return os.Mkdir(path, 0700) // a due path check would fail with ErrPathNotRegular
	}), func(context.Context, time.Duration) error { t.Fatal("sink EOF triggered a wait"); return nil }, func() time.Time { return stamp })
	if !errors.Is(err, io.EOF) || commits != 2 || r.pending == nil || r.Position().Offset != 6 || s.LastPathStatus() != PathSame {
		t.Fatalf("due poll after sink EOF: %v, commits %d, pending %v, position %+v, status %s", err, commits, r.pending != nil, r.Position(), s.LastPathStatus())
	}
	position, found, err := store.Checkpoint(context.Background(), s.ID(), r.Position().OriginID)
	if err != nil || !found || position.Offset != 6 {
		t.Fatal("sink EOF advanced durable checkpoint", position, found, err)
	}
}
