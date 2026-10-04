//go:build linux

package file

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

func TestEnsureGenerationDefersEmptyCaptureThenRegistersAfterAppend(t *testing.T) {
	for _, prior := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "different"}[prior], func(t *testing.T) {
			ctx := context.Background()
			f, _ := testRegularFile(t, "prior\n")
			id, err := Inspect(f)
			if err != nil {
				t.Fatal(err)
			}
			store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			if prior {
				old := ingestState(t, f, 6)
				if err := store.Commit(ctx, source.Batch{Source: fileSourceIdentity(), Origins: []source.Origin{old.Origin}, Checkpoints: []source.Position{*old.Checkpoint}}); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.Truncate(0); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Seek(3, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			query := source.OriginQuery{SourceID: fileSourceIdentity().ID, Device: id.Device, Inode: id.Inode, Limit: 100}
			before, err := store.FileOrigins(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			sink := sinkFunc(func(ctx context.Context, b source.Batch) error { calls++; return store.Commit(ctx, b) })
			want := SelectionAbsent
			if prior {
				want = SelectionDifferent
			}
			for range 2 {
				got, err := EnsureGeneration(ctx, f, fileSourceIdentity(), store, sink)
				if err != nil || !got.WaitingForContent || got.Created || got.State != nil || got.Selection != want || calls != 0 {
					t.Fatalf("empty decision: %+v, %v, commits %d", got, err, calls)
				}
			}
			after, err := store.FileOrigins(ctx, query)
			if err != nil || !reflect.DeepEqual(after, before) {
				t.Fatalf("empty capture changed persisted state: %+v, %v", after, err)
			}
			if position, err := f.Seek(0, io.SeekCurrent); err != nil || position != 3 {
				t.Fatalf("empty decision moved position: %d, %v", position, err)
			}
			if _, err := f.WriteAt([]byte("new\n"), 0); err != nil {
				t.Fatal(err)
			}
			got, err := EnsureGeneration(ctx, f, fileSourceIdentity(), store, sink)
			if err != nil || got.WaitingForContent || !got.Created || got.State == nil || got.Selection != want || calls != 1 {
				t.Fatalf("append decision: %+v, %v, commits %d", got, err, calls)
			}
			prefix, err := ParsePrefixFingerprint(got.State.Origin.Fingerprint)
			if err != nil || prefix.Length != 4 {
				t.Fatalf("registered prefix: %+v, %v", prefix, err)
			}
			after, err = store.FileOrigins(ctx, query)
			if err != nil || len(after.States) != len(before.States)+1 {
				t.Fatalf("registration: %+v, %v", after, err)
			}
		})
	}
}

func TestEnsureGenerationKeepsLegacyEmptyOriginInsufficient(t *testing.T) {
	ctx := context.Background()
	f, _ := testRegularFile(t, "")
	legacy := ingestState(t, f, 0)
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Commit(ctx, source.Batch{Source: fileSourceIdentity(), Origins: []source.Origin{legacy.Origin}, Checkpoints: []source.Position{*legacy.Checkpoint}}); err != nil {
		t.Fatal(err)
	}
	query := source.OriginQuery{SourceID: fileSourceIdentity().ID, Device: legacy.Origin.Device, Inode: legacy.Origin.Inode, Limit: 100}
	before, err := store.FileOrigins(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	sink := sinkFunc(func(context.Context, source.Batch) error { t.Fatal("legacy empty origin replaced"); return nil })
	for _, content := range []string{"", "new\n"} {
		if _, err := f.WriteAt([]byte(content), 0); err != nil {
			t.Fatal(err)
		}
		got, err := EnsureGeneration(ctx, f, fileSourceIdentity(), store, sink)
		if err != nil || got.Selection != SelectionInsufficient || got.WaitingForContent || got.Created || got.State != nil {
			t.Fatalf("legacy decision: %+v, %v", got, err)
		}
		after, err := store.FileOrigins(ctx, query)
		if err != nil || !reflect.DeepEqual(after, before) {
			t.Fatalf("legacy state changed: %+v, %v", after, err)
		}
	}
}
