package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestFollowMissingKeepsAllDescriptorsAndRequiresFreshDecision(t *testing.T) {
	for _, count := range []int{1, 2} {
		set, paths, normalized := currentSetFixture(t, count)
		path := filepath.Join(t.TempDir(), "missing.log")
		s := transferSource(t, path)
		s.setPathStatus(PathSame)
		missing, err := set.ObserveCurrent(context.Background(), path)
		if err != nil || missing != (FollowCurrent{Status: FollowCurrentMissing}) {
			t.Fatal(missing, err)
		}
		sink := sinkFunc(func(context.Context, source.Batch) error { t.Fatal("missing startup wrote state"); return nil })
		for attempt := 0; attempt < 2; attempt++ {
			err = s.followOpenedWithOpener(context.Background(), set, missing, sink, func(context.Context, time.Duration) error { t.Fatal("missing startup waited"); return nil }, time.Now, func(context.Context, string) (*os.File, Identity, error) {
				t.Fatal("missing startup opened a file")
				return nil, Identity{}, nil
			})
			if !errors.Is(err, ErrCurrentMissing) || err.Error() != "configured current log is missing at startup" {
				t.Fatal("missing diagnostic", err)
			}
			assertCurrentSetUnchanged(t, set, count, normalized)
			if s.LastPathStatus() != PathSame {
				t.Fatal("missing reset last status")
			}
			if !s.running.TryLock() {
				t.Fatal("missing leaked execution guard")
			}
			s.running.Unlock()
		}
		// Reusing a missing decision after a regular file appears cannot silently
		// start ingestion. The caller must observe the new physical identity.
		if err := os.WriteFile(path, []byte("fresh\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := s.FollowOpened(context.Background(), set, missing, sink); !errors.Is(err, ErrPathChanged) {
			t.Fatal("missing decision accepted a new file", err)
		}
		returned, err := set.ObserveCurrent(context.Background(), path)
		want := FollowCurrentNew
		if count == 2 {
			want = FollowCurrentCapacity
		}
		if err != nil || returned.Status != want || returned.OriginID != "" {
			t.Fatal("new file return decision", returned, err)
		}
		assertCurrentSetUnchanged(t, set, count, normalized)
		s.config.Path = paths[0]
		if err := s.FollowOpened(context.Background(), set, missing, sink); !errors.Is(err, ErrPathChanged) {
			t.Fatal("stale missing decision accepted", err)
		}
		assertCurrentSetUnchanged(t, set, count, normalized)
		fresh, err := set.ObserveCurrent(context.Background(), paths[0])
		if err != nil || fresh.Status != FollowCurrentKnown || fresh.OriginID != "a" {
			t.Fatal("fresh current decision", fresh, err)
		}
	}
}

func TestFollowMissingValidatesDecisionAndOwner(t *testing.T) {
	for _, kind := range []string{"origin supplied", "snapshot supplied", "source mismatch", "closed second", "nil set", "canceled", "concurrent"} {
		t.Run(kind, func(t *testing.T) {
			set, _, normalized := currentSetFixture(t, 2)
			s := transferSource(t, filepath.Join(t.TempDir(), "absent.log"))
			decision := FollowCurrent{Status: FollowCurrentMissing}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := ErrInvalidFollowCurrent
			input := set
			switch kind {
			case "origin supplied":
				decision.OriginID = "a"
			case "snapshot supplied":
				decision.Current.Size = 5
			case "source mismatch":
				set.opened[1].ingestor.identity.TrustedHost = "other"
				want = ErrInvalidOpenedFollowSet
			case "closed second":
				if err := set.opened[1].file.Close(); err != nil {
					t.Fatal(err)
				}
				want = nil
			case "nil set":
				input = nil
				want = ErrInvalidOpenedFollowSet
			case "canceled":
				cancel()
				want = context.Canceled
			case "concurrent":
				s.running.Lock()
				defer s.running.Unlock()
				want = ErrSourceRunning
			}
			err := s.FollowOpened(ctx, input, decision, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("invalid missing wrote"); return nil }))
			if err == nil || errors.Is(err, ErrCurrentMissing) || want != nil && !errors.Is(err, want) {
				t.Fatal("missing hides validation error", err)
			}
			if kind == "closed second" {
				assertDescriptorPosition(t, set.opened[0].file, 5)
				if set.Len() != 2 {
					t.Fatal("lost owner")
				}
				return
			}
			assertCurrentSetUnchanged(t, set, 2, normalized)
		})
	}
}
