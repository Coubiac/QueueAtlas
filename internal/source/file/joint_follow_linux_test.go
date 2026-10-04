//go:build linux

package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestJointFollowCommitsLatePartialAndDoesNotStarveEitherGeneration(t *testing.T) {
	f, path := testRegularFile(t, "first\npartial")
	s, store := rotationSource(t, path)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var committing atomic.Int32
	var records []source.Record
	waits := 0
	err = s.followPath(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		depth := committing.Add(1)
		defer committing.Add(-1)
		if depth != 1 {
			return errors.New("concurrent sink commit")
		}
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		if len(b.Records) == 0 {
			return nil
		}
		records = append(records, b.Records[0])
		if string(b.Records[0].Raw) == "new-1\n" {
			_, err := f.WriteAt([]byte(" rest\nlate\n"), 13)
			return err
		}
		if string(b.Records[0].Raw) == "new-3\n" {
			cancel()
		}
		return nil
	}), func(context.Context, time.Duration) error {
		waits++
		if waits != 1 {
			t.Fatal("waited while complete records were available")
		}
		return rotateTo(path, ".1", "new-1\nnew-2\nnew-3\n")
	})
	if !errors.Is(err, context.Canceled) || waits != 1 || len(records) != 6 {
		t.Fatalf("joint follow: %v, waits %d, records %+v", err, waits, records)
	}
	for i, want := range []string{"first\n", "new-1\n", "partial rest\n", "new-2\n", "late\n", "new-3\n"} {
		if string(records[i].Raw) != want {
			t.Fatalf("record %d: %q; want %q", i, records[i].Raw, want)
		}
	}
	for i := 0; i < len(records); i += 2 {
		if records[i].OriginID != records[0].OriginID || records[i+1].OriginID != records[1].OriginID || records[i].OriginID == records[i+1].OriginID {
			t.Fatal("joint follow mixed physical origins")
		}
	}
	for i, offset := range []int64{24, 18} {
		position, found, err := store.Checkpoint(context.Background(), s.ID(), records[i].OriginID)
		if err != nil || !found || position.Offset != offset {
			t.Fatalf("checkpoint %d: %+v, %v, %v", i, position, found, err)
		}
	}
	if r.Position().Offset != 24 || fileDescriptorCount(t, path) != 0 {
		t.Fatal("retained ingestor did not advance or successor leaked")
	}
}

func TestJointFollowOldAppendContinuesWhileSuccessorIsEmpty(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
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
		if len(records) == 2 {
			cancel()
		}
		return nil
	}), func(context.Context, time.Duration) error {
		waits++
		if waits == 1 {
			return rotateTo(path, ".1", "")
		}
		if waits != 2 {
			t.Fatal("old append not consumed")
		}
		_, err := f.WriteAt([]byte("late\n"), 6)
		return err
	})
	if !errors.Is(err, context.Canceled) || waits != 2 || registrations != 0 || len(records) != 2 || string(records[1].Raw) != "late\n" || records[0].OriginID != records[1].OriginID || r.Position().Offset != 11 {
		t.Fatalf("empty successor joint follow: %v, waits %d, registrations %d, records %+v", err, waits, registrations, records)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != 0 || fileDescriptorCount(t, path) != 0 {
		t.Fatal("empty successor changed or leaked", err)
	}
}

func TestJointFollowRetainedErrorStopsBeforeLaterSuccessorCommit(t *testing.T) {
	for _, kind := range []string{"sink failure", "sink EOF", "read failure"} {
		t.Run(kind, func(t *testing.T) {
			f, path := testRegularFile(t, "first\n")
			s, store := rotationSource(t, path)
			r, _, err := s.prepareGeneration(context.Background(), f, store)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			want := errors.New("late sink failure")
			if kind == "sink EOF" {
				want = io.EOF
			} else if kind == "read failure" {
				want = fs.ErrClosed
			}
			waits, failures := 0, 0
			var records []source.Record
			err = s.followPath(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
				if len(b.Records) > 0 && string(b.Records[0].Raw) == "late\n" {
					failures++
					return want
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				records = append(records, b.Records...)
				if len(b.Records) > 0 && string(b.Records[0].Raw) == "new-1\n" {
					if kind == "read failure" {
						return f.Close() // inject a retained descriptor read failure
					}
					_, err := f.WriteAt([]byte("late\n"), 6)
					return err
				}
				return nil
			}), func(context.Context, time.Duration) error {
				waits++
				if waits != 1 {
					t.Fatal("retained error waited or retried")
				}
				return rotateTo(path, ".1", "new-1\nnew-2\n")
			})
			if !errors.Is(err, want) || len(records) != 2 || waits != 1 || fileDescriptorCount(t, path) != 0 {
				t.Fatalf("retained error: %v, records %+v, waits %d", err, records, waits)
			}
			for i := range records {
				position, found, err := store.Checkpoint(context.Background(), s.ID(), records[i].OriginID)
				if err != nil || !found || position.Offset != 6 {
					t.Fatalf("unacknowledged position advanced: %+v, %v, %v", position, found, err)
				}
			}
			if kind != "read failure" {
				if failures != 1 || r.pending == nil || r.Position().Offset != 6 || string(r.pending.Records[0].Raw) != "late\n" {
					t.Fatal("failed retained batch was lost or retried")
				}
				if err := r.CommitNext(context.Background(), store); err != nil || r.Position().Offset != 11 {
					t.Fatal("retained pending batch could not be acknowledged", err)
				}
			}
		})
	}
}
