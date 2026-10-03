//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
)

func TestGraceDeadlineClosesOldFileAndReusesCapacityForThirdGeneration(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path, time.Second)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	waits := 0
	var records []source.Record
	err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		records = append(records, b.Records...)
		if len(records) == 3 {
			if fileDescriptorCount(t, path+".1") != 0 || fileDescriptorCount(t, path+".2") != 1 || fileDescriptorCount(t, path) != 1 {
				t.Fatal("old retirement did not free the descriptor capacity")
			}
			cancel()
		}
		return nil
	}), func(context.Context, time.Duration) error {
		waits++
		switch waits {
		case 1:
			return rotateTo(path, ".1", "second\n")
		case 2:
			stamp = stamp.Add(s.config.RotationGrace - time.Nanosecond)
		case 3:
			if _, err := f.Stat(); err != nil {
				t.Fatal("old descriptor closed before deadline", err)
			}
			stamp = stamp.Add(time.Nanosecond)
			return rotateTo(path, ".2", "third\n")
		default:
			t.Fatal("third generation did not progress")
		}
		return nil
	}, func() time.Time { return stamp })
	if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || waits != 3 || len(records) != 3 {
		t.Fatalf("grace capacity reuse: %v, waits %d, records %+v", err, waits, records)
	}
	seen := make(map[string]bool)
	for i, want := range []string{"first\n", "second\n", "third\n"} {
		record := records[i]
		if string(record.Raw) != want || record.Start != 0 || seen[record.OriginID] {
			t.Fatalf("generation %d: %+v", i, record)
		}
		seen[record.OriginID] = true
		position, found, err := store.Checkpoint(context.Background(), s.ID(), record.OriginID)
		if err != nil || !found || position.Offset != int64(len(want)) {
			t.Fatal("retirement changed checkpoint", position, found, err)
		}
	}
	if fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".2") != 0 {
		t.Fatal("grace cleanup leaked descriptors")
	}
}

func TestGraceRechecksAppendDuringWaitAndRenewsDeadline(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path, time.Second)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	waits, records := 0, 0
	err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		records += len(b.Records)
		return store.Commit(ctx, b)
	}), func(context.Context, time.Duration) error {
		waits++
		switch waits {
		case 1:
			return rotateTo(path, ".1", "second\n")
		case 2:
			stamp = stamp.Add(s.config.RotationGrace)
			_, err := f.WriteAt([]byte("late\n"), 6)
			return err
		case 3:
			if _, err := f.Stat(); err != nil || r.Position().Offset != 11 {
				t.Fatal("append at deadline lost or closed", err, r.Position())
			}
			stamp = stamp.Add(s.config.RotationGrace - time.Nanosecond)
		case 4:
			if _, err := f.Stat(); err != nil {
				t.Fatal("renewed grace ended too early", err)
			}
			stamp = stamp.Add(time.Nanosecond)
		case 5:
			if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("renewed grace did not retire file", err)
			}
			cancel()
			return ctx.Err()
		default:
			t.Fatal("extra grace wait")
		}
		return nil
	}, func() time.Time { return stamp })
	if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || waits != 5 || records != 3 || r.Position().Offset != 11 {
		t.Fatalf("late append grace: %v, waits %d, records %d, position %+v", err, waits, records, r.Position())
	}
}

func TestGraceDoesNotRetirePartialOrOversizedPartialLine(t *testing.T) {
	for _, size := range []int{7, model.MaxLineBytes + 10} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			f, path := testRegularFile(t, "first\n"+strings.Repeat("x", size))
			s, store := rotationSource(t, path, time.Second)
			r, _, err := s.prepareGeneration(context.Background(), f, store)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
			waits, records := 0, 0
			err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
				records += len(b.Records)
				return store.Commit(ctx, b)
			}), func(context.Context, time.Duration) error {
				waits++
				if waits == 1 {
					return rotateTo(path, ".1", "second\n")
				}
				if waits != 2 {
					t.Fatal("partial protection failed")
				}
				stamp = stamp.Add(2 * s.config.RotationGrace)
				return rotateTo(path, ".2", "third\n")
			}, func() time.Time { return stamp })
			if !errors.Is(err, ErrRotationCapacity) || records != 2 || r.Position().Offset != 6 || r.lines.offset != int64(6+size) || fileDescriptorCount(t, path) != 0 {
				t.Fatalf("partial grace: %v, records %d, position %+v, consumed %d", err, records, r.Position(), r.lines.offset)
			}
		})
	}
}

func TestGraceKeepsCurrentDescriptorWhilePathIsMissing(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path, time.Second)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	waits := 0
	err = s.followPathWithClock(ctx, f, r, store, func(context.Context, time.Duration) error {
		waits++
		if waits == 1 {
			stamp = stamp.Add(100 * s.config.RotationGrace)
			return os.Rename(path, path+".1")
		}
		if waits != 2 || s.LastPathStatus() != PathMissing {
			t.Fatal("missing current file not observed")
		}
		if _, err := f.Stat(); err != nil {
			t.Fatal("current descriptor retired while path missing", err)
		}
		cancel()
		return ctx.Err()
	}, func() time.Time { return stamp })
	if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || waits != 2 {
		t.Fatal("missing path grace cancellation", err, waits)
	}
}
