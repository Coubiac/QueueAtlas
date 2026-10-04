//go:build linux

package file

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
)

func followingLocationFixture(t *testing.T) (string, FollowOrigins) {
	t.Helper()
	f, path := testRegularFile(t, "old\n")
	old := ingestState(t, f, 4)
	old.FollowState = source.FollowFollowing
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rotateTo(path, ".1", "new\n"); err != nil {
		t.Fatal(err)
	}
	next, _, err := OpenLog(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	current := ingestState(t, next, 4)
	current.Origin.ID, current.Checkpoint.OriginID, current.FollowState = "gen-2", "gen-2", source.FollowFollowing
	return path, FollowOrigins{Status: FollowOriginsComplete, States: []source.OriginState{old, current}, Examined: 2}
}

func TestLocateFollowOriginsFindsCurrentAndRenamedFilesWithoutRetainingDescriptors(t *testing.T) {
	path, origins := followingLocationFixture(t)
	for i, want := range []string{path + ".1", path} {
		one := FollowOrigins{Status: FollowOriginsComplete, States: origins.States[i : i+1], Examined: 2}
		result, err := LocateFollowOrigins(context.Background(), path, one, 2)
		if err != nil || result.Status != SelectionUnique || result.Examined != 2 || len(result.Locations) != 1 || result.Locations[0].Path != want {
			t.Fatal("single following file not located", result, err)
		}
	}
	result, err := LocateFollowOrigins(context.Background(), path, origins, 4)
	if err != nil || result.Status != SelectionUnique || result.Examined != 4 || len(result.Locations) != 2 {
		t.Fatal("following files not located", result, err)
	}
	for i, want := range []string{path + ".1", path} {
		if result.Locations[i].Path != want || !reflect.DeepEqual(result.Locations[i].State, origins.States[i]) {
			t.Fatal("location changed provenance/checkpoint", result.Locations[i])
		}
	}
	for _, name := range []string{path, path + ".1", filepath.Dir(path)} {
		if fileDescriptorCount(t, name) != 0 {
			t.Fatal("localization retained descriptor", name)
		}
	}
}

func TestLocateFollowOriginsRealDecisionsDoNotExposePartialLocations(t *testing.T) {
	for _, kind := range []string{"absent", "changed second", "zero second", "nil second", "hard links", "shared generation", "limit"} {
		t.Run(kind, func(t *testing.T) {
			path, origins := followingLocationFixture(t)
			limit, want := 10, SelectionDifferent
			switch kind {
			case "absent":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path + ".1"); err != nil {
					t.Fatal(err)
				}
				want = SelectionAbsent
			case "changed second":
				if err := os.WriteFile(path, []byte("bad\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "zero second":
				origins.States[1].Checkpoint.Offset, origins.States[1].Checkpoint.AnchorHash = 0, (CheckpointAnchor{Digest: sha256.Sum256(nil)}).String()
				want = SelectionInsufficient
			case "nil second":
				origins.States[1].Checkpoint = nil
				want = SelectionInsufficient
			case "hard links":
				if err := os.Link(path, path+".link"); err != nil {
					t.Fatal(err)
				}
				want = SelectionAmbiguous
			case "shared generation":
				origins.States[1] = origins.States[0]
				p := *origins.States[0].Checkpoint
				origins.States[1].Checkpoint = &p
				origins.States[1].Origin.ID, origins.States[1].Checkpoint.OriginID = "gen-2", "gen-2"
				want = SelectionAmbiguous
			case "limit":
				limit, want = 3, SelectionLimit
			}
			result, err := LocateFollowOrigins(context.Background(), path, origins, limit)
			if err != nil || result.Status != want || result.Locations != nil || result.Examined > limit {
				t.Fatal("unusable search exposed locations", result, err, want)
			}
			if kind == "limit" && result.Examined != 3 {
				t.Fatal("limit not shared across scans", result)
			}
			if fileDescriptorCount(t, filepath.Dir(path)) != 0 {
				t.Fatal("directory descriptor retained")
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if fileDescriptorCount(t, filepath.Join(filepath.Dir(path), entry.Name())) != 0 {
					t.Fatal("candidate descriptor retained")
				}
			}
		})
	}
}
