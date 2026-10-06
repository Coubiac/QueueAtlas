//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func rotationSource(t *testing.T, path string, grace ...time.Duration) (*FileSource, *sqlite.Store) {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	cfg := sourceConfig(path)
	if len(grace) > 0 {
		cfg.RotationGrace = grace[0]
	}
	s, err := New(cfg, store, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	return s, store
}

// Count only descriptors for this test's physical file, excluding SQLite/runtime
// descriptors. Statting proc entries neither opens the log nor changes offsets.
func fileDescriptorCount(t *testing.T, path string) int {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		opened, err := os.Stat(filepath.Join("/proc/self/fd", entry.Name()))
		if err == nil && os.SameFile(info, opened) {
			count++
		}
	}
	return count
}

func rotateTo(path, suffix, content string) error {
	if err := os.Rename(path, path+suffix); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

func TestRunOpenedSwitchesOriginsAndRetainsOldDescriptorUntilCancellation(t *testing.T) {
	f, path := testRegularFile(t, "first\npartial")
	s, store := rotationSource(t, path)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var records []source.Record
	registrations := 0
	waiting, err := s.runOpened(ctx, f, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		registrations += len(b.Origins)
		if len(b.Records) == 0 {
			return nil
		}
		records = append(records, b.Records[0])
		if len(records) == 1 {
			return rotateTo(path, ".1", "new\n")
		}
		if _, err := f.Stat(); err != nil {
			return errors.New("old descriptor closed during switch")
		}
		if fileDescriptorCount(t, path) != 1 || fileDescriptorCount(t, path+".1") != 1 {
			t.Fatal("switch did not retain exactly two descriptors")
		}
		cancel()
		return nil
	}))
	if waiting || !errors.Is(err, context.Canceled) || registrations != 2 || len(records) != 2 {
		t.Fatalf("switch: %v, waiting %v, registrations %d, records %+v", err, waiting, registrations, records)
	}
	if string(records[0].Raw) != "first\n" || string(records[1].Raw) != "new\n" || records[1].Start != 0 || records[0].OriginID == records[1].OriginID {
		t.Fatalf("generation records: %+v", records)
	}
	for i, offset := range []int64{6, 4} {
		position, found, err := store.Checkpoint(context.Background(), s.ID(), records[i].OriginID)
		if err != nil || !found || position.Offset != offset {
			t.Fatalf("checkpoint %d: %+v, %v, %v", i, position, found, err)
		}
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("initial descriptor retained after cancellation", err)
	}
	if fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".1") != 0 {
		t.Fatal("rotation leaked a descriptor")
	}
}

func TestRotationWaitsForEmptySuccessorWithoutRegistering(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path)
	r, waiting, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil || waiting {
		t.Fatal(waiting, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	waits, registrations, records := 0, 0, 0
	err = s.followPath(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		registrations += len(b.Origins)
		records += len(b.Records)
		if records == 2 {
			if string(b.Records[0].Raw) != "new\n" || b.Records[0].Start != 0 {
				t.Fatal("incorrect successor record")
			}
			cancel()
		}
		return nil
	}), func(context.Context, time.Duration) error {
		waits++
		if waits == 1 {
			return rotateTo(path, ".1", "")
		}
		if waits != 2 || registrations != 0 || records != 1 || fileDescriptorCount(t, path) != 1 {
			t.Fatal("empty successor was registered, closed or consumed")
		}
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		device, inode := physicalIDs(info)
		page, err := store.FileOrigins(ctx, source.OriginQuery{SourceID: s.ID(), Device: device, Inode: inode, Limit: 100})
		if err != nil || len(page.States) != 0 {
			t.Fatal("empty successor persisted", page, err)
		}
		return os.WriteFile(path, []byte("new\n"), 0600)
	})
	if !errors.Is(err, context.Canceled) || waits != 2 || registrations != 1 || records != 2 || fileDescriptorCount(t, path) != 0 {
		t.Fatalf("empty successor: %v, waits %d, registrations %d, records %d", err, waits, registrations, records)
	}
}

func TestRotationCapacityStopsBeforeOpeningThirdDistinctFile(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	waits, registrations, records := 0, 0, 0
	err = s.followPath(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		registrations += len(b.Origins)
		records += len(b.Records)
		return store.Commit(ctx, b)
	}), func(context.Context, time.Duration) error {
		waits++
		if waits == 1 {
			return rotateTo(path, ".1", "second\n")
		}
		if waits != 2 {
			t.Fatal("extra capacity polling")
		}
		return rotateTo(path, ".2", "third\n")
	})
	if !errors.Is(err, ErrRotationCapacity) || registrations != 1 || records != 2 || waits != 2 || s.LastPathStatus() != PathReplaced {
		t.Fatalf("capacity: %v, registrations %d, records %d, waits %d", err, registrations, records, waits)
	}
	if fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".2") != 0 {
		t.Fatal("capacity failure opened a third file or leaked the successor")
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("scheduler retained initial descriptor", err)
	}
}

func TestRotationSinkFailureClosesSuccessorWithoutRetry(t *testing.T) {
	for _, kind := range []string{"registration", "acquisition", "record", "cancel after registration"} {
		t.Run(kind, func(t *testing.T) {
			f, path := testRegularFile(t, "first\n")
			s, store := rotationSource(t, path)
			r, _, err := s.prepareGeneration(context.Background(), f, store)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			boom := errors.New("successor sink failure")
			failures, waits := 0, 0
			err = s.followPath(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
				if kind == "registration" && len(b.Origins) > 0 || kind == "acquisition" && len(b.FollowTransitions) > 0 || kind == "record" && len(b.Records) > 0 && string(b.Records[0].Raw) == "second\n" {
					failures++
					return boom
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				if kind == "cancel after registration" && len(b.Origins) > 0 {
					cancel()
				}
				return nil
			}), func(context.Context, time.Duration) error {
				waits++
				if waits != 1 {
					t.Fatal("successor error was retried")
				}
				return rotateTo(path, ".1", "second\n")
			})
			want := boom
			if kind == "cancel after registration" {
				want = context.Canceled
			} else if failures != 1 {
				t.Fatal("failure not injected exactly once", failures)
			}
			if !errors.Is(err, want) || waits != 1 || fileDescriptorCount(t, path) != 0 {
				t.Fatalf("successor failure: %v, waits %d", err, waits)
			}
		})
	}
}

func TestRotationReusesRetainedGenerationWithItsPartialLine(t *testing.T) {
	f, path := testRegularFile(t, "first\npartial")
	s, store := rotationSource(t, path)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	waits, registrations := 0, 0
	var records []source.Record
	err = s.followPath(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		registrations += len(b.Origins)
		records = append(records, b.Records...)
		if len(records) == 3 {
			cancel()
		}
		return nil
	}), func(context.Context, time.Duration) error {
		waits++
		if waits == 1 {
			return rotateTo(path, ".1", "second\n")
		}
		if waits != 2 {
			t.Fatal("retained generation not resumed")
		}
		if _, err := f.WriteAt([]byte(" rest\n"), 13); err != nil {
			return err
		}
		if err := os.Rename(path, path+".2"); err != nil {
			return err
		}
		return os.Rename(path+".1", path)
	})
	if !errors.Is(err, context.Canceled) || registrations != 1 || len(records) != 3 || waits != 2 {
		t.Fatalf("retained return: %v, registrations %d, records %+v, waits %d", err, registrations, records, waits)
	}
	if string(records[2].Raw) != "partial rest\n" || records[2].Start != 6 || records[2].OriginID != records[0].OriginID || r.Position().Offset != 19 {
		t.Fatalf("retained partial line: %+v, position %+v", records[2], r.Position())
	}
	if fileDescriptorCount(t, path+".2") != 0 {
		t.Fatal("retained successor leaked")
	}
}

func TestRotationUnusableSuccessorDecisionDoesNotCommit(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	queries, registrations, records := 0, 0, 0
	s.reader = originReaderFunc(func(_ context.Context, q source.OriginQuery) (source.OriginPage, error) {
		queries++
		return source.OriginPage{States: []source.OriginState{{Origin: source.Origin{ID: "unsupported", Device: q.Device, Inode: q.Inode}}}}, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = s.followPath(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		registrations += len(b.Origins)
		records += len(b.Records)
		return store.Commit(ctx, b)
	}), func(context.Context, time.Duration) error { return rotateTo(path, ".1", "second\n") })
	var decision *ResumeDecisionError
	if !errors.As(err, &decision) || decision.Status != SelectionInsufficient || queries != 1 || registrations != 0 || records != 1 {
		t.Fatalf("successor decision: %v, queries %d, registrations %d, records %d", err, queries, registrations, records)
	}
	if fileDescriptorCount(t, path) != 0 {
		t.Fatal("unusable successor leaked")
	}
}
