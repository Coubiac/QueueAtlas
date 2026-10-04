//go:build linux

package file

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
)

func locatedOpenFixture(t *testing.T) (string, FollowLocations) {
	t.Helper()
	path, origins := followingLocationFixture(t)
	locations, err := LocateFollowOrigins(context.Background(), path, origins, 4)
	if err != nil || locations.Status != SelectionUnique {
		t.Fatal("fixture locations", locations, err)
	}
	return path, locations
}

func TestOpenFollowLocationsReverifiesAndSeeksWithoutConsuming(t *testing.T) {
	for _, count := range []int{1, 2} {
		path, locations := locatedOpenFixture(t)
		locations.Locations = locations.Locations[:count]
		for _, location := range locations.Locations {
			f, err := os.OpenFile(location.Path, os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := f.WriteString("late\n")
			if err := errors.Join(writeErr, f.Close()); err != nil {
				t.Fatal(err)
			}
		}
		normalized := 0
		set, err := OpenFollowLocations(context.Background(), fileSourceIdentity(), locations, func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) })
		if err != nil || set.Len() != count || normalized != 0 {
			t.Fatal("reopen failed or consumed a line", set, err, normalized)
		}
		defer set.Close()
		for i, generation := range set.opened {
			position, err := generation.file.Seek(0, io.SeekCurrent)
			if err != nil || position != 4 || generation.ingestor.Position() != *locations.Locations[i].State.Checkpoint || generation.ingestor.lines.offset != 4 || generation.ingestor.pending != nil || generation.file.Name() != locations.Locations[i].Path {
				t.Fatal("reopen changed checkpoint/read position", generation.ingestor.Position(), position, err)
			}
			if _, err := generation.file.Write([]byte("x")); err == nil {
				t.Fatal("reopened descriptor writable")
			}
			if fileDescriptorCount(t, locations.Locations[i].Path) != 1 {
				t.Fatal("owner did not keep exactly one descriptor per file")
			}
		}
		locations.Locations[0].State.Checkpoint.Offset = 999
		if set.opened[0].ingestor.Position().Offset != 4 {
			t.Fatal("caller aliases owned checkpoint")
		}
		if err := set.Close(); err != nil {
			t.Fatal(err)
		}
		if set.Len() != 0 || set.Close() != nil || fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".1") != 0 {
			t.Fatal("close leaked/retried a descriptor")
		}
	}
}

func TestOpenFollowLocationsFailureClosesEntireSet(t *testing.T) {
	for _, kind := range []string{"missing second", "replaced second", "rewritten second", "truncated second", "nil second", "zero second", "invalid anchor", "physical collision", "open error second", "cancel after first", "cancel before second", "cancel after second", "cleanup error"} {
		t.Run(kind, func(t *testing.T) {
			path, locations := locatedOpenFixture(t)
			var wantStatus SelectionStatus
			var wantError error
			boom := errors.New("synthetic second open failure")
			switch kind {
			case "missing second":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				wantError = fs.ErrNotExist
			case "replaced second":
				if err := os.Rename(path, path+".gone"); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("new\n"), 0600); err != nil {
					t.Fatal(err)
				}
				wantStatus = SelectionDifferent
			case "rewritten second":
				if err := os.WriteFile(path, []byte("bad\n"), 0600); err != nil {
					t.Fatal(err)
				}
				wantStatus = SelectionDifferent
			case "truncated second":
				if err := os.Truncate(path, 2); err != nil {
					t.Fatal(err)
				}
				wantStatus = SelectionDifferent
			case "nil second":
				locations.Locations[1].State.Checkpoint = nil
				wantStatus = SelectionInsufficient
			case "zero second":
				locations.Locations[1].State.Checkpoint.Offset, locations.Locations[1].State.Checkpoint.AnchorHash = 0, (CheckpointAnchor{Digest: sha256.Sum256(nil)}).String()
				wantStatus = SelectionInsufficient
			case "invalid anchor":
				locations.Locations[1].State.Checkpoint.AnchorHash = "invalid"
				wantStatus = SelectionInsufficient
			case "physical collision":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(path+".1", path); err != nil {
					t.Fatal(err)
				}
				wantStatus = SelectionAmbiguous
			case "open error second", "cleanup error":
				wantError = boom
			default:
				wantError = context.Canceled
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var acquired []*os.File
			calls, normalized := 0, 0
			set, err := openFollowLocations(ctx, fileSourceIdentity(), locations, func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }, func(ctx context.Context, path string) (*os.File, Identity, error) {
				calls++
				if calls == 2 {
					switch kind {
					case "open error second":
						return nil, Identity{}, boom
					case "cleanup error":
						if err := acquired[0].Close(); err != nil {
							t.Fatal(err)
						}
						return nil, Identity{}, boom
					case "cancel before second":
						cancel()
						return nil, Identity{}, ctx.Err()
					}
				}
				f, id, err := OpenLog(ctx, path)
				if err != nil {
					return nil, Identity{}, err
				}
				acquired = append(acquired, f)
				if kind == "cancel after first" && calls == 1 || kind == "cancel after second" && calls == 2 {
					cancel()
				}
				return f, id, nil
			})
			if set != nil || err == nil || normalized != 0 {
				t.Fatal("failed reopen exposed/consumed partial set", set, err, normalized)
			}
			if wantError != nil && !errors.Is(err, wantError) {
				t.Fatal("failure cause lost", err, wantError)
			}
			if wantStatus != "" {
				var decision *ResumeDecisionError
				if !errors.As(err, &decision) || decision.Status != wantStatus {
					t.Fatal("verification decision lost", err, wantStatus)
				}
			}
			if kind == "cleanup error" && !errors.Is(err, fs.ErrClosed) {
				t.Fatal("cleanup error lost", err)
			}
			wantCalls := 2
			if kind == "cancel after first" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatal("open was retried or continued after cancellation", calls)
			}
			for _, f := range acquired {
				if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
					t.Fatal("failed reopen retained descriptor", err)
				}
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if fileDescriptorCount(t, filepath.Join(filepath.Dir(path), entry.Name())) != 0 {
					t.Fatal("failed set leaked physical descriptor")
				}
			}
		})
	}
}

func TestOpenFollowLocationsCopiesWholeInputBeforeFirstOpen(t *testing.T) {
	_, locations := locatedOpenFixture(t)
	calls := 0
	set, err := openFollowLocations(context.Background(), fileSourceIdentity(), locations, testNormalizer, func(ctx context.Context, path string) (*os.File, Identity, error) {
		calls++
		if calls == 1 {
			locations.Locations[1].State.Checkpoint.Offset = 999
		}
		return OpenLog(ctx, path)
	})
	if err != nil || set.Len() != 2 || set.opened[1].ingestor.Position().Offset != 4 {
		t.Fatal("later checkpoint aliased caller", set, err)
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
}
