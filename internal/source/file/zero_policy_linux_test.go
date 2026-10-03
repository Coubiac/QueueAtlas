//go:build linux

package file

import (
	"context"
	"crypto/sha256"
	"io"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

func TestZeroReplayPolicyRequiresConsistentNonemptyEvidence(t *testing.T) {
	for _, kind := range []string{"valid", "append", "prefix changed", "truncated", "empty prefix", "absent checkpoint", "wrong origin", "negative offset", "anchor offset", "invalid anchor", "invalid empty digest", "physical changed", "invalid prefix"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := testRegularFile(t, "seed\n")
			state := ingestState(t, f, 0)
			if _, err := f.Seek(3, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			want, reason := ResumeInsufficient, ReasonInvalidState
			switch kind {
			case "valid":
				want, reason = ResumeRestartZero, ReasonZeroReplay
			case "append":
				if _, err := f.WriteAt([]byte("added\n"), 5); err != nil {
					t.Fatal(err)
				}
				want, reason = ResumeRestartZero, ReasonZeroReplay
			case "prefix changed":
				if _, err := f.WriteAt([]byte("X"), 0); err != nil {
					t.Fatal(err)
				}
				want, reason = ResumeDifferent, ReasonPrefixChanged
			case "truncated":
				if err := f.Truncate(1); err != nil {
					t.Fatal(err)
				}
				want, reason = ResumeDifferent, ReasonPrefixChanged
			case "empty prefix":
				state.Origin.Fingerprint = (PrefixFingerprint{Digest: sha256.Sum256(nil)}).String()
				reason = ReasonEmptyPrefix
			case "absent checkpoint":
				state.Checkpoint = nil
				reason = ReasonMissingCheckpoint
			case "wrong origin":
				state.Checkpoint.OriginID = "other"
			case "negative offset":
				state.Checkpoint.Offset = -1
			case "anchor offset":
				a, err := CaptureAnchor(f, 1)
				if err != nil {
					t.Fatal(err)
				}
				state.Checkpoint.AnchorHash = a.String()
			case "invalid anchor":
				state.Checkpoint.AnchorHash = "bad"
				reason = ReasonInvalidAnchor
			case "invalid empty digest":
				state.Checkpoint.AnchorHash = (CheckpointAnchor{}).String()
				reason = ReasonInvalidAnchor
			case "physical changed":
				state.Origin.Inode += "9"
				want, reason = ResumeDifferent, ReasonPhysicalChanged
			case "invalid prefix":
				state.Origin.Fingerprint = "bad"
				reason = ReasonInvalidPrefix
			}
			got, err := VerifyCandidateWithPolicy(f, state, ResumePolicy{AllowZeroCheckpoint: true})
			if err != nil || got.Status != want || got.Reason != reason {
				t.Fatalf("policy check: %+v, %v; want %s/%s", got, err, want, reason)
			}
			if position, err := f.Seek(0, io.SeekCurrent); err != nil || position != 3 {
				t.Fatalf("policy moved position: %d, %v", position, err)
			}
		})
	}
}

func TestZeroReplayPolicyReturnsExistingStateWithoutWritingOrSkipping(t *testing.T) {
	ctx := context.Background()
	f, _ := testRegularFile(t, "seed\n")
	state := ingestState(t, f, 0)
	state.Origin.FirstSeen = time.Now().UTC()
	dbPath := filepath.Join(t.TempDir(), "state.sqlite")
	store, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	identity := fileSourceIdentity()
	if err := store.Commit(ctx, source.Batch{Source: identity, Origins: []source.Origin{state.Origin}, Checkpoints: []source.Position{*state.Checkpoint}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	query := source.OriginQuery{SourceID: identity.ID, Device: state.Origin.Device, Inode: state.Origin.Inode, Limit: 100}
	before, err := store.FileOrigins(ctx, query)
	if err != nil {
		t.Fatal(err)
	}
	sink := sinkFunc(func(context.Context, source.Batch) error { t.Fatal("zero decision wrote state"); return nil })
	strict, err := EnsureGeneration(ctx, f, identity, store, sink)
	if err != nil || strict.Selection != SelectionInsufficient || strict.State != nil {
		t.Fatalf("strict decision: %+v, %v", strict, err)
	}
	if _, err := f.WriteAt([]byte("added\n"), 5); err != nil {
		t.Fatal(err)
	}
	replay, err := EnsureGenerationWithPolicy(ctx, f, identity, store, sink, ResumePolicy{AllowZeroCheckpoint: true})
	if err != nil || replay.Selection != SelectionRestartZero || replay.Created || replay.WaitingForContent || replay.State == nil || !reflect.DeepEqual(*replay.State, before.States[0]) {
		t.Fatalf("zero decision: %+v, %v", replay, err)
	}
	after, err := store.FileOrigins(ctx, query)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("decision changed state: %+v, %v", after, err)
	}
	r, err := NewIngestor(ctx, f, identity, *replay.State, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	if r.Position().Offset != 0 {
		t.Fatal("replay skipped the beginning")
	}
	if err := r.CommitNext(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if string(b.Records[0].Raw) != "seed\n" || b.Records[0].Start != 0 || b.Records[0].End != 5 || b.Records[0].OriginID != state.Origin.ID {
			t.Fatalf("zero replay record: %+v", b.Records[0])
		}
		return store.Commit(ctx, b)
	})); err != nil {
		t.Fatal(err)
	}
	selection, err := SelectResumeWithPolicy(ctx, f, identity.ID, store, ResumePolicy{AllowZeroCheckpoint: true})
	if err != nil || selection.Status != SelectionUnique || selection.Candidate == nil || selection.Candidate.Checkpoint.Offset != 5 {
		t.Fatalf("positive checkpoint decision: %+v, %v", selection, err)
	}
}

func TestZeroReplayPolicyRejectsCompetingRealCandidates(t *testing.T) {
	for _, otherKind := range []string{"zero", "positive", "missing", "empty"} {
		t.Run(otherKind, func(t *testing.T) {
			f, _ := testRegularFile(t, "seed\n")
			zero := ingestState(t, f, 0)
			other := ingestState(t, f, 0)
			other.Origin.ID, other.Checkpoint.OriginID = "gen-2", "gen-2"
			want := SelectionAmbiguous
			switch otherKind {
			case "positive":
				a, err := CaptureAnchor(f, 5)
				if err != nil {
					t.Fatal(err)
				}
				other.Checkpoint.Offset, other.Checkpoint.AnchorHash = 5, a.String()
			case "missing":
				other.Checkpoint = nil
				want = SelectionInsufficient
			case "empty":
				other.Origin.Fingerprint = (PrefixFingerprint{Digest: sha256.Sum256(nil)}).String()
				want = SelectionInsufficient
			}
			reader := originReaderFunc(func(_ context.Context, q source.OriginQuery) (source.OriginPage, error) {
				if q.AfterID == "" {
					return source.OriginPage{States: []source.OriginState{zero}, NextID: zero.Origin.ID}, nil
				}
				return source.OriginPage{States: []source.OriginState{other}}, nil
			})
			got, err := EnsureGenerationWithPolicy(context.Background(), f, fileSourceIdentity(), reader, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("competing candidates wrote state"); return nil }), ResumePolicy{AllowZeroCheckpoint: true})
			if err != nil || got.Selection != want || got.State != nil || got.Created || got.WaitingForContent {
				t.Fatalf("competing decision: %+v, %v", got, err)
			}
		})
	}
}
