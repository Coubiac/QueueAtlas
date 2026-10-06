package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func transferSource(t *testing.T, path string) *FileSource {
	t.Helper()
	s, err := New(sourceConfig(path), originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
		t.Fatal("existing generation was reconsidered")
		return source.OriginPage{}, nil
	}), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestFollowOpenedRejectsBeforeTransfer(t *testing.T) {
	for _, kind := range []string{"canceled", "sink", "running", "missing decision", "new decision", "capacity decision", "origin", "snapshot", "path changed", "path missing", "source ID", "source host", "closed second", "empty set"} {
		t.Run(kind, func(t *testing.T) {
			set, paths, normalized := currentSetFixture(t, 2)
			s := transferSource(t, paths[0])
			s.setPathStatus(PathMissing)
			current, err := set.ObserveCurrent(context.Background(), paths[0])
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sink := source.Sink(sinkFunc(func(context.Context, source.Batch) error { t.Fatal("commit before transfer"); return nil }))
			want := ErrInvalidFollowCurrent
			switch kind {
			case "canceled":
				cancel()
				want = context.Canceled
			case "sink":
				sink = nil
				want = nil
			case "running":
				s.running.Lock()
				defer s.running.Unlock()
				want = ErrSourceRunning
			case "missing decision":
				current.Status = FollowCurrentMissing
			case "new decision":
				current.Status = FollowCurrentNew
			case "capacity decision":
				current.Status = FollowCurrentCapacity
			case "origin":
				current.OriginID = "b"
				want = ErrPathChanged
			case "snapshot":
				current.Current = Identity{}
				want = ErrPathChanged
			case "path changed":
				s.config.Path = paths[1]
				want = ErrPathChanged
			case "path missing":
				s.config.Path = filepath.Join(t.TempDir(), "missing")
				want = ErrPathChanged
			case "source ID":
				set.opened[1].ingestor.identity.ID = "other"
				want = ErrInvalidOpenedFollowSet
			case "source host":
				set.opened[1].ingestor.identity.TrustedHost = "other"
				want = ErrInvalidOpenedFollowSet
			case "closed second":
				if err := set.opened[1].file.Close(); err != nil {
					t.Fatal(err)
				}
				want = nil
			case "empty set":
				if err := set.Close(); err != nil {
					t.Fatal(err)
				}
				want = ErrInvalidOpenedFollowSet
			}
			err = s.FollowOpened(ctx, set, current, sink)
			if err == nil || (want != nil && !errors.Is(err, want)) {
				t.Fatal("unexpected rejection", err, want)
			}
			if s.LastPathStatus() != PathMissing {
				t.Fatal("rejected attempt reset path status")
			}
			if kind == "empty set" {
				return
			}
			if kind == "closed second" {
				if set.Len() != 2 {
					t.Fatal("lost ownership")
				}
				assertDescriptorPosition(t, set.opened[0].file, 5)
				return
			}
			assertCurrentSetUnchanged(t, set, 2, normalized)
		})
	}
}

func TestFollowOpenedClosesAfterTransferOnSinkFailure(t *testing.T) {
	for _, failure := range []error{errors.New("synthetic sink failure"), io.EOF, context.Canceled} {
		t.Run(failure.Error(), func(t *testing.T) {
			set, paths, _ := currentSetFixture(t, 2)
			s := transferSource(t, paths[1])
			current, err := set.ObserveCurrent(context.Background(), paths[1])
			if err != nil {
				t.Fatal(err)
			}
			generations := append([]*openedGeneration(nil), set.opened...)
			if _, err := generations[0].file.WriteAt([]byte("late\n"), 5); err != nil {
				t.Fatal(err)
			}
			commits := 0
			err = s.FollowOpened(context.Background(), set, current, sinkFunc(func(_ context.Context, batch source.Batch) error {
				commits++
				if set.Len() != 0 || set.Close() != nil {
					t.Fatal("ownership was not transferred before commit")
				}
				if len(batch.Origins) != 0 || len(batch.FollowTransitions) != 0 || len(batch.Records) != 1 || string(batch.Records[0].Raw) != "late\n" {
					t.Fatal("unexpected replay or acquisition", batch)
				}
				return failure
			}))
			if !errors.Is(err, failure) || errors.Is(err, fs.ErrClosed) || commits != 1 {
				t.Fatal("failure or cleanup changed", err, commits)
			}
			if set.Close() != nil {
				t.Fatal("emptied owner close failed")
			}
			for _, g := range generations {
				if !errors.Is(g.file.Close(), fs.ErrClosed) || g.ingestor.Position().Offset != 5 {
					t.Fatal("descriptor leaked or unacknowledged checkpoint advanced")
				}
			}
			if !s.running.TryLock() {
				t.Fatal("execution guard leaked")
			}
			s.running.Unlock()
		})
	}
}

func TestFollowOpenedRechecksAnchorAfterKnownDecision(t *testing.T) {
	for _, shrink := range []bool{false, true} {
		set, paths, normalized := currentSetFixture(t, 1)
		s := transferSource(t, paths[0])
		current, err := set.ObserveCurrent(context.Background(), paths[0])
		if err != nil {
			t.Fatal(err)
		}
		f := set.opened[0].file
		want := ErrCheckpointChanged
		if shrink {
			err = f.Truncate(2)
			want = ErrFileTruncated
		} else {
			_, err = f.WriteAt([]byte("edit\n"), 0)
		}
		if err != nil {
			t.Fatal(err)
		}
		err = s.followOpened(context.Background(), set, current, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("consumed changed checkpoint"); return nil }), func(context.Context, time.Duration) error { t.Fatal("wait before integrity check"); return nil }, time.Now)
		if !errors.Is(err, want) || errors.Is(err, fs.ErrClosed) || set.Len() != 0 || *normalized != 0 || !errors.Is(f.Close(), fs.ErrClosed) {
			t.Fatal("integrity/cleanup", err)
		}
	}
}
