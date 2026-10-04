//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestFollowOpenedResumesLateAppendsAndRetiresOnlyRetainedOrigin(t *testing.T) {
	for _, count := range []int{1, 2} {
		for _, reverse := range []bool{false, true} {
			if count == 1 && reverse {
				continue
			}
			t.Run(fmtTransferCase(count, reverse), func(t *testing.T) {
				path, locations := locatedOpenFixture(t)
				if count == 1 {
					locations.Locations = locations.Locations[1:]
				}
				s, store := rotationSource(t, path, time.Second)
				stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
				for i := range locations.Locations {
					locations.Locations[i].State.Origin.FirstSeen = stamp
					seedAcquisition(t, s, store, locations.Locations[i].State)
				}
				set, err := OpenFollowLocations(context.Background(), s.config.Identity, locations, testNormalizer)
				if err != nil {
					t.Fatal(err)
				}
				defer set.Close()
				if reverse && count == 2 {
					set.opened[0], set.opened[1] = set.opened[1], set.opened[0]
				}
				current, err := set.ObserveCurrent(context.Background(), path)
				if err != nil || current.OriginID != "gen-2" {
					t.Fatal(current, err)
				}
				generations := append([]*openedGeneration(nil), set.opened...)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				records, retirements, waits := 0, 0, 0
				err = s.followOpened(ctx, set, current, sinkFunc(func(ctx context.Context, b source.Batch) error {
					if set.Len() != 0 || set.Close() != nil {
						t.Fatal("owner still controls scheduler descriptors")
					}
					if len(b.Origins) != 0 {
						t.Fatal("registered existing origin again")
					}
					for _, transition := range b.FollowTransitions {
						if transition.OriginID != "gen-1" || transition.From != source.FollowFollowing || transition.To != source.FollowRetired {
							t.Fatal("wrong origin retired or reacquired", transition)
						}
						retirements++
					}
					for _, record := range b.Records {
						if record.Start != 4 || string(record.Raw) != "late\n" {
							t.Fatal("replayed prefix", record)
						}
						records++
					}
					return store.Commit(ctx, b)
				}), func(context.Context, time.Duration) error {
					waits++
					stamp = stamp.Add(2 * time.Second)
					if waits == 1 {
						for _, location := range locations.Locations {
							writer, err := os.OpenFile(location.Path, os.O_WRONLY|os.O_APPEND, 0600)
							if err != nil {
								t.Fatal(err)
							}
							_, err = writer.WriteString("late\n")
							if err = errors.Join(err, writer.Close()); err != nil {
								t.Fatal(err)
							}
						}
						return nil
					}
					if waits == 2 {
						return nil
					}
					cancel()
					return ctx.Err()
				}, func() time.Time { return stamp })
				if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || records != count || retirements != count-1 || waits != 3 {
					t.Fatal("resume/retirement", err, records, retirements, waits)
				}
				for _, g := range generations {
					if !errors.Is(g.file.Close(), fs.ErrClosed) || fileDescriptorCount(t, g.file.Name()) != 0 {
						t.Fatal("descriptor leaked")
					}
					state := retirementState(t, s, store, g.ingestor.position.OriginID)
					want := source.FollowFollowing
					if state.Origin.ID != current.OriginID {
						want = source.FollowRetired
					}
					if state.Checkpoint.Offset != 9 || state.FollowState != want {
						t.Fatal("wrong durable checkpoint/follow state", state)
					}
				}
			})
		}
	}
}

func fmtTransferCase(count int, reverse bool) string {
	if count == 1 {
		return "one current"
	}
	if reverse {
		return "current first"
	}
	return "current second"
}
