package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func openLocationInput(path string) FollowLocations {
	origins := locationOrigins(path)
	locations := FollowLocations{Status: SelectionUnique}
	for _, state := range origins.States {
		locations.Locations = append(locations.Locations, FollowLocation{State: state, Path: filepath.Join(filepath.Dir(path), state.Origin.ID)})
	}
	return locations
}

func TestOpenFollowLocationsValidatesWholeInputBeforeOpening(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail.log")
	never := func(context.Context, string) (*os.File, Identity, error) {
		t.Fatal("invalid input opened a file")
		return nil, Identity{}, nil
	}
	for _, change := range []func(*FollowLocations){
		func(l *FollowLocations) { l.Status = SelectionLimit },
		func(l *FollowLocations) { l.Locations = nil },
		func(l *FollowLocations) { l.Locations = append(l.Locations, l.Locations[0]) },
		func(l *FollowLocations) { l.Locations[1].State.FollowState = source.FollowUnknown },
		func(l *FollowLocations) { l.Locations[1].State.FollowState = source.FollowRetired },
		func(l *FollowLocations) { l.Locations[1].State.FollowState = 3 },
		func(l *FollowLocations) { l.Locations[1].State.Origin.ID = "" },
		func(l *FollowLocations) { l.Locations[1].State.Origin.ID = l.Locations[0].State.Origin.ID },
		func(l *FollowLocations) { l.Locations[1].State.Origin.Path = "" },
		func(l *FollowLocations) { l.Locations[1].State.Origin.Path = path + ".other" },
		func(l *FollowLocations) { l.Locations[1].Path = "relative.log" },
		func(l *FollowLocations) { l.Locations[1].Path = l.Locations[0].Path },
		func(l *FollowLocations) {
			l.Locations[1].Path = filepath.Dir(path) + string(os.PathSeparator) + "unused" + string(os.PathSeparator) + ".." + string(os.PathSeparator) + l.Locations[0].State.Origin.ID
		},
	} {
		locations := openLocationInput(path)
		change(&locations)
		set, err := openFollowLocations(context.Background(), fileSourceIdentity(), locations, testNormalizer, never)
		if set != nil || !errors.Is(err, ErrInvalidFollowLocations) {
			t.Fatal("invalid set accepted", set, err)
		}
	}
	for _, kind := range []string{"source ID", "source kind", "source name", "normalizer", "cancel before"} {
		identity, normalize := fileSourceIdentity(), Normalize(testNormalizer)
		ctx, cancel := context.WithCancel(context.Background())
		switch kind {
		case "source ID":
			identity.ID = ""
		case "source kind":
			identity.Kind = "syslog"
		case "source name":
			identity.Name = ""
		case "normalizer":
			normalize = nil
		case "cancel before":
			cancel()
		}
		set, err := openFollowLocations(ctx, identity, openLocationInput(path), normalize, never)
		cancel()
		if set != nil || err == nil || kind == "cancel before" && !errors.Is(err, context.Canceled) {
			t.Fatal("invalid config/cancellation accepted", set, err)
		}
	}
}

func TestOpenedFollowSetCloseIsIdempotentAndClosesAllOnError(t *testing.T) {
	for _, failedFirst := range []bool{false, true} {
		first, _ := testRegularFile(t, "first\n")
		second, _ := testRegularFile(t, "second\n")
		set := &OpenedFollowSet{opened: []*openedGeneration{{file: first}, {file: second}}}
		if set.Len() != 2 {
			t.Fatal("owner lost descriptors")
		}
		if failedFirst {
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
		}
		err := set.Close()
		if failedFirst && !errors.Is(err, fs.ErrClosed) || !failedFirst && err != nil {
			t.Fatal("close error lost", err)
		}
		if set.Len() != 0 || set.Close() != nil {
			t.Fatal("owner retried closed descriptors")
		}
		for _, f := range []*os.File{first, second} {
			if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("descriptor stayed open", err)
			}
		}
	}
	var empty OpenedFollowSet
	var nilSet *OpenedFollowSet
	if empty.Len() != 0 || empty.Close() != nil || nilSet.Len() != 0 || nilSet.Close() != nil {
		t.Fatal("empty/nil owner not usable for cleanup")
	}
}
