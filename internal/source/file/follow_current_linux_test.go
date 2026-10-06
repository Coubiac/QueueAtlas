//go:build linux

package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/model"
)

func TestReopenedSetObservesCurrentMissingCapacityAndRetainedReturn(t *testing.T) {
	path, locations := locatedOpenFixture(t)
	normalized := 0
	set, err := OpenFollowLocations(context.Background(), fileSourceIdentity(), locations, func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) })
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()
	check := func(want FollowCurrentStatus, originID string) {
		t.Helper()
		result, err := set.ObserveCurrent(context.Background(), path)
		if err != nil || result.Status != want || result.OriginID != originID || set.Len() != 2 || normalized != 0 {
			t.Fatal("current decision", result, err)
		}
		if (want == FollowCurrentMissing || want == FollowCurrentCapacity) && result.Current != (Identity{}) {
			t.Fatal("unusable decision exposed current identity", result)
		}
		for _, g := range set.opened {
			assertDescriptorPosition(t, g.file, 4)
			if g.ingestor.Position().Offset != 4 || g.ingestor.lines.offset != 4 || g.ingestor.pending != nil || !g.eofSince.IsZero() {
				t.Fatal("observation changed ingestor/grace state")
			}
		}
	}
	check(FollowCurrentKnown, "gen-2")
	if err := os.WriteFile(path, []byte("new\nlate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	check(FollowCurrentKnown, "gen-2")
	if result, err := set.ObserveCurrent(context.Background(), path); err != nil || result.Current.Size != 9 {
		t.Fatal("current metadata not refreshed", result, err)
	}
	if err := rotateTo(path, ".2", "third\n"); err != nil {
		t.Fatal(err)
	}
	check(FollowCurrentCapacity, "")
	if fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".1") != 1 || fileDescriptorCount(t, path+".2") != 1 {
		t.Fatal("capacity decision opened third file or closed retained descriptor")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	check(FollowCurrentMissing, "")
	if err := os.Rename(path+".1", path); err != nil {
		t.Fatal(err)
	}
	check(FollowCurrentKnown, "gen-1")
	if fileDescriptorCount(t, path) != 1 || fileDescriptorCount(t, path+".2") != 1 {
		t.Fatal("retained return changed descriptor ownership")
	}
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	if fileDescriptorCount(t, path) != 0 || fileDescriptorCount(t, path+".2") != 0 {
		t.Fatal("owner cleanup leaked descriptors")
	}
}

func TestReopenedSetFollowsRegularSymlinkAndKeepsOwnershipOnPathError(t *testing.T) {
	path, locations := locatedOpenFixture(t)
	set, err := OpenFollowLocations(context.Background(), fileSourceIdentity(), locations, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()
	link := filepath.Join(t.TempDir(), "current.log")
	if err := os.Symlink(path+".1", link); err != nil {
		t.Fatal(err)
	}
	if result, err := set.ObserveCurrent(context.Background(), link); err != nil || result.Status != FollowCurrentKnown || result.OriginID != "gen-1" {
		t.Fatal("regular symlink not matched by identity", result, err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(link, link); err != nil {
		t.Fatal(err)
	}
	if result, err := set.ObserveCurrent(context.Background(), link); !errors.Is(err, syscall.ELOOP) || result != (FollowCurrent{}) {
		t.Fatal("stat error lost", result, err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(link, 0600); err != nil {
		t.Fatal(err)
	}
	if result, err := set.ObserveCurrent(context.Background(), link); !errors.Is(err, ErrPathNotRegular) || result != (FollowCurrent{}) {
		t.Fatal("FIFO accepted", result, err)
	}
	if set.Len() != 2 || fileDescriptorCount(t, path) != 1 || fileDescriptorCount(t, path+".1") != 1 || fileDescriptorCount(t, link) != 0 {
		t.Fatal("error changed ownership or opened FIFO")
	}
	for _, g := range set.opened {
		assertDescriptorPosition(t, g.file, 4)
	}
}
