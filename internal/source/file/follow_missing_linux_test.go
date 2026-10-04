//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestFollowMissingReappearanceResumesSavedSQLiteCheckpoints(t *testing.T) {
	for _, count := range []int{1, 2} {
		path, locations := locatedOpenFixture(t)
		locations.Locations = locations.Locations[:count]
		s, store := rotationSource(t, path)
		for i := range locations.Locations {
			locations.Locations[i].State.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
			seedAcquisition(t, s, store, locations.Locations[i].State)
		}
		set, err := OpenFollowLocations(context.Background(), s.config.Identity, locations, testNormalizer)
		if err != nil {
			t.Fatal(err)
		}
		defer set.Close()
		generations := append([]*openedGeneration(nil), set.opened...)
		before := make([]source.OriginState, count)
		for i, g := range generations {
			before[i] = retirementState(t, s, store, g.ingestor.position.OriginID)
		}
		if err := os.Rename(path, path+".2"); err != nil {
			t.Fatal(err)
		}
		missing, err := set.ObserveCurrent(context.Background(), path)
		if err != nil || missing.Status != FollowCurrentMissing {
			t.Fatal(missing, err)
		}
		appendNewTestLine(t, path+".1", "late\n")
		if count == 2 {
			appendNewTestLine(t, path+".2", "late\n")
		}
		err = s.FollowOpened(context.Background(), set, missing, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("missing wrote state"); return nil }))
		if !errors.Is(err, ErrCurrentMissing) || set.Len() != count {
			t.Fatal("missing transferred files", err)
		}
		for i, g := range generations {
			assertDescriptorPosition(t, g.file, 4)
			if g.ingestor.Position().Offset != 4 || g.ingestor.lines.offset != 4 || g.ingestor.pending != nil || !g.eofSince.IsZero() || !reflect.DeepEqual(before[i], retirementState(t, s, store, g.ingestor.position.OriginID)) {
				t.Fatal("missing changed ingestion or durable state")
			}
		}
		// The retained old file becomes current. No current is invented while
		// absent, and its actual return requires a new physical observation.
		if err := os.Rename(path+".1", path); err != nil {
			t.Fatal(err)
		}
		if err := s.FollowOpened(context.Background(), set, missing, store); !errors.Is(err, ErrPathChanged) || set.Len() != count {
			t.Fatal("stale missing decision applied", err)
		}
		current, err := set.ObserveCurrent(context.Background(), path)
		if err != nil || current.Status != FollowCurrentKnown || current.OriginID != "gen-1" {
			t.Fatal("returned current", current, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		records := 0
		err = s.FollowOpened(ctx, set, current, sinkFunc(func(ctx context.Context, b source.Batch) error {
			if len(b.Origins)+len(b.FollowTransitions) != 0 {
				t.Fatal("returned files re-registered/acquired")
			}
			for _, record := range b.Records {
				if record.Start != 4 || record.End != 9 || string(record.Raw) != "late\n" {
					t.Fatal("replayed or skipped data", record)
				}
				records++
			}
			if err := store.Commit(ctx, b); err != nil {
				return err
			}
			if records == count {
				cancel()
			}
			return nil
		}))
		if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || records != count || set.Len() != 0 || set.Close() != nil {
			t.Fatal("returned follow", err, records)
		}
		for _, g := range generations {
			state := retirementState(t, s, store, g.ingestor.position.OriginID)
			if state.Checkpoint.Offset != 9 || state.FollowState != source.FollowFollowing || !errors.Is(g.file.Close(), fs.ErrClosed) {
				t.Fatal("returned checkpoint/state/cleanup", state)
			}
		}
		if fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".2") != 0 {
			t.Fatal("returned follow leaked descriptors")
		}
	}
}
