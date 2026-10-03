//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

func waitPathStatus(ctx context.Context, s *FileSource, want PathStatus) error {
	ticker := time.NewTicker(2 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s.LastPathStatus() == want {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func TestFileSourcePollsMissingPathKeepsPartialLineAndSeesReturn(t *testing.T) {
	writer, path := testRegularFile(t, "first\npartial")
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	s, err := New(sourceConfig(path), store, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	restored := make(chan struct{})
	mutations := make(chan error, 1)
	go func() {
		if err := waitPathStatus(ctx, s, PathMissing); err != nil {
			mutations <- err
			return
		}
		if _, err := writer.WriteAt([]byte(" rest\n"), 13); err != nil {
			mutations <- err
			return
		}
		select {
		case <-restored:
		case <-ctx.Done():
			mutations <- ctx.Err()
			return
		}
		if err := waitPathStatus(ctx, s, PathSame); err != nil {
			mutations <- err
			return
		}
		_, err := writer.WriteAt([]byte("last\n"), 19)
		mutations <- err
	}()
	var records []source.Record
	registrations := 0
	err = s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		registrations += len(b.Origins)
		if len(b.Records) == 0 {
			return nil
		}
		records = append(records, b.Records[0])
		switch len(records) {
		case 1:
			if err := os.Rename(path, path+".1"); err != nil {
				return err
			}
		case 2:
			if s.LastPathStatus() != PathMissing {
				return errors.New("missing path did not remain observable during append")
			}
			if err := os.Rename(path+".1", path); err != nil {
				return err
			}
			close(restored)
		case 3:
			cancel()
		}
		return nil
	}))
	cancel()
	if mutationErr := <-mutations; mutationErr != nil {
		t.Fatal(mutationErr)
	}
	if !errors.Is(err, context.Canceled) || registrations != 1 || len(records) != 3 || s.LastPathStatus() != PathSame {
		t.Fatalf("missing/return follow: %v, registrations %d, records %+v, status %s", err, registrations, records, s.LastPathStatus())
	}
	for i, want := range []string{"first\n", "partial rest\n", "last\n"} {
		if string(records[i].Raw) != want || records[i].OriginID != records[0].OriginID {
			t.Fatalf("record %d: %+v", i, records[i])
		}
	}
	position, found, err := store.Checkpoint(context.Background(), s.ID(), records[0].OriginID)
	if err != nil || !found || position.Offset != 24 {
		t.Fatalf("final checkpoint: %+v, %v, %v", position, found, err)
	}
}

func TestSourcePathPollObservesReplacementWithoutSwitching(t *testing.T) {
	s, r, f := pathFollower(t, "first\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	waits := 0
	var records []source.Record
	err := s.followPath(ctx, f, r, sinkFunc(func(_ context.Context, b source.Batch) error {
		records = append(records, b.Records[0])
		if len(records) == 2 {
			cancel()
		}
		return nil
	}), func(context.Context) error {
		waits++
		if waits != 1 {
			return errors.New("unexpected wait")
		}
		if err := os.Rename(f.Name(), f.Name()+".1"); err != nil {
			return err
		}
		if err := os.WriteFile(f.Name(), []byte("new generation\n"), 0600); err != nil {
			return err
		}
		_, err := f.WriteAt([]byte("late\n"), 6)
		return err
	})
	if !errors.Is(err, context.Canceled) || waits != 1 || len(records) != 2 || string(records[1].Raw) != "late\n" || s.LastPathStatus() != PathReplaced {
		t.Fatalf("replacement follow: %v, waits %d, records %+v, status %s", err, waits, records, s.LastPathStatus())
	}
	if records[0].OriginID != records[1].OriginID || r.Position().Offset != 11 {
		t.Fatal("replacement switched generations or lost old append")
	}
}

func TestRunOpenedStopsOnPathPollErrorAndClosesDescriptor(t *testing.T) {
	for _, kind := range []string{"fifo", "symlink loop"} {
		t.Run(kind, func(t *testing.T) {
			writer, path := testRegularFile(t, "first\n")
			state := ingestState(t, writer, 0)
			reader := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
				return source.OriginPage{States: []source.OriginState{state}}, nil
			})
			cfg := sourceConfig(path)
			cfg.ResumePolicy.AllowZeroCheckpoint = true
			s, err := New(cfg, reader, testNormalizer)
			if err != nil {
				t.Fatal(err)
			}
			f, _, err := OpenLog(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			commits := 0
			waiting, err := s.runOpened(ctx, f, sinkFunc(func(context.Context, source.Batch) error {
				commits++
				if err := os.Rename(path, path+".1"); err != nil {
					return err
				}
				if kind == "fifo" {
					return syscall.Mkfifo(path, 0600)
				}
				return os.Symlink(path, path)
			}))
			want := error(ErrPathNotRegular)
			if kind == "symlink loop" {
				want = syscall.ELOOP
			}
			if waiting || !errors.Is(err, want) || commits != 1 || s.LastPathStatus() != PathSame {
				t.Fatalf("poll failure: %v, waiting %v, commits %d, status %s", err, waiting, commits, s.LastPathStatus())
			}
			if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("poll error retained descriptor", err)
			}
		})
	}
}
