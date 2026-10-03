//go:build linux

package file

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

func TestEnsureGenerationRegistrationAndDurableInitialState(t *testing.T) {
	ctx := context.Background()
	identity := fileSourceIdentity()
	for _, content := range []string{"synthetic line\n", ""} {
		t.Run(content, func(t *testing.T) {
			f, path := testRegularFile(t, content)
			if _, err := f.Seek(3, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			dbPath := filepath.Join(t.TempDir(), "state.sqlite")
			store, err := sqlite.Open(ctx, dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			calls := 0
			sink := sinkFunc(func(ctx context.Context, batch source.Batch) error {
				calls++
				if batch.Source != identity || len(batch.Origins) != 1 || len(batch.Checkpoints) != 1 || len(batch.Records) != 0 {
					t.Fatalf("initial batch: %+v", batch)
				}
				return store.Commit(ctx, batch)
			})
			before := time.Now().UTC()
			got, err := EnsureGeneration(ctx, f, identity, store, sink)
			if err != nil || !got.Created || got.State == nil || got.Selection != SelectionAbsent || calls != 1 {
				t.Fatalf("initial start: %+v, %v, calls %d", got, err, calls)
			}
			o, p := got.State.Origin, got.State.Checkpoint
			id, err := Inspect(f)
			if err != nil {
				t.Fatal(err)
			}
			if o.Path != path || o.Device != id.Device || o.Inode != id.Inode || o.FirstSeen.Before(before) || o.FirstSeen.After(time.Now()) || o.FirstSeen.Location() != time.UTC {
				t.Fatalf("origin metadata: %+v", o)
			}
			if len(o.ID) != 37 || o.ID[:5] != "file-" || p == nil || p.Offset != 0 || p.OriginID != o.ID {
				t.Fatalf("origin/checkpoint: %+v, %+v", o, p)
			}
			prefix, err := ParsePrefixFingerprint(o.Fingerprint)
			if err != nil || prefix.Length != len(content) {
				t.Fatalf("prefix: %+v, %v", prefix, err)
			}
			anchor, err := ParseCheckpointAnchor(p.AnchorHash)
			if err != nil || anchor.Offset != 0 || anchor.Length != 0 {
				t.Fatalf("anchor: %+v, %v", anchor, err)
			}
			if pos, err := f.Seek(0, io.SeekCurrent); err != nil || pos != 3 {
				t.Fatalf("read position %d, %v", pos, err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := sqlite.Open(ctx, dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			page, err := reopened.FileOrigins(ctx, source.OriginQuery{SourceID: identity.ID, Device: id.Device, Inode: id.Inode, Limit: 100})
			if err != nil || len(page.States) != 1 || !reflect.DeepEqual(page.States[0], *got.State) {
				t.Fatalf("persisted initial state: %+v, %v", page, err)
			}
			retry, err := EnsureGeneration(ctx, f, identity, reopened, sinkFunc(func(context.Context, source.Batch) error {
				t.Fatal("zero checkpoint created another generation")
				return nil
			}))
			if err != nil || retry.Selection != SelectionInsufficient || retry.Created || retry.State != nil {
				t.Fatalf("restart at zero: %+v, %v", retry, err)
			}
		})
	}
}

func TestEnsureGenerationDecisionsAndSinkFailures(t *testing.T) {
	for _, kind := range []string{"different", "unique", "incomplete", "ambiguous", "limit", "sink failure", "reader failure", "cancel before commit"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f, state := resumeFixture(t)
			boom := errors.New("synthetic failure")
			queryCalls := 0
			reader := originReaderFunc(func(_ context.Context, q source.OriginQuery) (source.OriginPage, error) {
				queryCalls++
				if q.SourceID != fileSourceIdentity().ID || q.Device != state.Origin.Device || q.Inode != state.Origin.Inode {
					t.Fatalf("wrong namespace: %+v", q)
				}
				switch kind {
				case "reader failure":
					return source.OriginPage{}, boom
				case "sink failure":
					return source.OriginPage{}, nil
				case "cancel before commit":
					cancel()
					return source.OriginPage{}, nil
				case "different":
					state.Origin.Fingerprint = "sha256:1:0000000000000000000000000000000000000000000000000000000000000000"
					return source.OriginPage{States: []source.OriginState{state}}, nil
				case "unique":
					return source.OriginPage{States: []source.OriginState{state}}, nil
				case "incomplete":
					state.Checkpoint = nil
					return source.OriginPage{States: []source.OriginState{state}}, nil
				case "ambiguous":
					other := state
					other.Origin.ID = "gen-2"
					p := *state.Checkpoint
					p.OriginID = other.Origin.ID
					other.Checkpoint = &p
					return source.OriginPage{States: []source.OriginState{state, other}}, nil
				case "limit":
					states := selectionStates(make([]ResumeStatus, MaxResumeCandidates+1)...)
					for i := range states {
						states[i].Origin.Device, states[i].Origin.Inode = state.Origin.Device, state.Origin.Inode
					}
					start := (queryCalls - 1) * 100
					end := start + q.Limit
					return source.OriginPage{States: states[start:end], NextID: states[end-1].Origin.ID}, nil
				}
				panic("unknown test")
			})
			calls := 0
			sink := sinkFunc(func(_ context.Context, batch source.Batch) error {
				calls++
				if kind == "sink failure" {
					return boom
				}
				if kind != "different" {
					t.Fatal("unsafe selection committed")
				}
				if batch.Origins[0].ID == state.Origin.ID || batch.Checkpoints[0].Offset != 0 {
					t.Fatalf("new generation reused state: %+v", batch)
				}
				return nil
			})
			got, err := EnsureGeneration(ctx, f, fileSourceIdentity(), reader, sink)
			switch kind {
			case "sink failure", "reader failure":
				if !errors.Is(err, boom) || got.State != nil || got.Created {
					t.Fatalf("failed start: %+v, %v", got, err)
				}
			case "cancel before commit":
				if !errors.Is(err, context.Canceled) || got.State != nil || got.Created {
					t.Fatalf("cancelled start: %+v, %v", got, err)
				}
			case "different":
				if err != nil || got.Selection != SelectionDifferent || !got.Created || got.State == nil {
					t.Fatalf("new generation: %+v, %v", got, err)
				}
			case "unique":
				if err != nil || got.Selection != SelectionUnique || got.Created || got.State == nil || !reflect.DeepEqual(*got.State, state) {
					t.Fatalf("existing generation: %+v, %v", got, err)
				}
			default:
				want := map[string]SelectionStatus{"incomplete": SelectionInsufficient, "ambiguous": SelectionAmbiguous, "limit": SelectionLimit}[kind]
				if err != nil || got.Selection != want || got.State != nil || got.Created {
					t.Fatalf("unresolved start: %+v, %v", got, err)
				}
			}
			wantCalls := 0
			if kind == "different" || kind == "sink failure" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("commit calls %d, want %d", calls, wantCalls)
			}
		})
	}
}
