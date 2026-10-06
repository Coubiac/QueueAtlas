package file

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/parser/postfix"
	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func ingestState(t *testing.T, f *os.File, offset int64) source.OriginState {
	t.Helper()
	id, err := Inspect(f)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := CapturePrefix(f)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := CaptureAnchor(f, offset)
	if err != nil {
		t.Fatal(err)
	}
	return source.OriginState{Origin: source.Origin{ID: "gen-1", Path: f.Name(), Device: id.Device, Inode: id.Inode, Fingerprint: prefix.String()}, Checkpoint: &source.Position{OriginID: "gen-1", Offset: offset, AnchorHash: anchor.String()}}
}

func testNormalizer(raw []byte) model.Observation { return postfix.Parse(raw, postfix.Options{}) }

func TestIngestorAcknowledgesOneLineAndRetriesSameBatch(t *testing.T) {
	line := "Oct  4 00:00:00 mx.example.test postfix/qmgr[1]: A1B2C3: removed\r\n"
	f, _ := testRegularFile(t, line+"next\n")
	state := ingestState(t, f, 0)
	normalizations := 0
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), state, func(raw []byte) model.Observation {
		normalizations++
		o := testNormalizer(raw)
		raw[0] = 'X' // the normalizer cannot alter the physical record
		return o
	})
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("synthetic sink failure")
	var pending source.Batch
	if err := r.CommitNext(context.Background(), sinkFunc(func(_ context.Context, batch source.Batch) error { pending = batch; return boom })); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if r.Position() != *state.Checkpoint {
		t.Fatal("unacknowledged offset advanced")
	}
	if err := r.CommitNext(context.Background(), sinkFunc(func(_ context.Context, batch source.Batch) error {
		if !reflect.DeepEqual(batch, pending) {
			t.Fatal("retry changed the batch")
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
	if normalizations != 1 || r.Position().Offset != int64(len(line)) {
		t.Fatalf("normalizations %d, position %+v", normalizations, r.Position())
	}
	record := pending.Records[0]
	if len(pending.Records) != 1 || len(pending.Checkpoints) != 1 || len(pending.Origins) != 0 || pending.Source != fileSourceIdentity() || record.OriginID != state.Origin.ID || record.Start != 0 || record.End != int64(len(line)) || string(record.Raw) != line || record.ReadAt.IsZero() || record.Observation.Kind != model.KindRemoved || record.Observation.SourceID != fileSourceIdentity().ID {
		t.Fatalf("record/batch: %+v", pending)
	}
	anchor, err := CaptureAnchor(f, record.End)
	if err != nil || pending.Checkpoints[0].AnchorHash != anchor.String() {
		t.Fatalf("checkpoint %+v, anchor %+v, %v", pending.Checkpoints[0], anchor, err)
	}
	if err := r.CommitNext(context.Background(), sinkFunc(func(_ context.Context, batch source.Batch) error {
		if batch.Records[0].Start != int64(len(line)) || string(batch.Records[0].Raw) != "next\n" {
			t.Fatalf("next record: %+v", batch)
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
}

func TestIngestorPartialEOFOversizedLineAndSeededAnchor(t *testing.T) {
	f, _ := testRegularFile(t, "prior\npartial")
	state := ingestState(t, f, 6)
	normalizations := 0
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), state, func(raw []byte) model.Observation { normalizations++; return testNormalizer(raw) })
	if err != nil {
		t.Fatal(err)
	}
	var batches []source.Batch
	sink := sinkFunc(func(_ context.Context, b source.Batch) error { batches = append(batches, b); return nil })
	for range 2 {
		if err := r.CommitNext(context.Background(), sink); !errors.Is(err, io.EOF) {
			t.Fatal(err)
		}
	}
	if len(batches) != 0 || normalizations != 0 || r.Position() != *state.Checkpoint {
		t.Fatal("partial EOF was acknowledged")
	}
	suffix := " rest\n" + strings.Repeat("X", model.MaxLineBytes+17) + "\nlast\n"
	if _, err := f.WriteAt([]byte(suffix), int64(len("prior\npartial"))); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := r.CommitNext(context.Background(), sink); err != nil {
			t.Fatal(err)
		}
	}
	if normalizations != 2 || len(batches) != 3 {
		t.Fatalf("normalizations %d, batches %d", normalizations, len(batches))
	}
	if string(batches[0].Records[0].Raw) != "partial rest\n" || string(batches[2].Records[0].Raw) != "last\n" {
		t.Fatal("partial or next line lost")
	}
	long := batches[1].Records[0]
	if len(long.Raw) != model.MaxLineBytes || long.Error == "" || long.Observation.Kind != model.KindUnknown || long.Observation.ParseError != long.Error || long.End-long.Start != model.MaxLineBytes+18 {
		t.Fatalf("oversized record: %+v", long)
	}
	for _, batch := range batches {
		a, err := CaptureAnchor(f, batch.Records[0].End)
		if err != nil || batch.Checkpoints[0].AnchorHash != a.String() {
			t.Fatalf("consumed anchor %+v, want %+v, %v", batch.Checkpoints[0], a, err)
		}
	}
}

func TestIngestorAnchorUsesConsumedBytesAfterFileRewrite(t *testing.T) {
	f, _ := testRegularFile(t, "first\nsecond\n")
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	sink := sinkFunc(func(context.Context, source.Batch) error { return nil })
	if err := r.CommitNext(context.Background(), sink); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("CHANGED"), 6); err != nil {
		t.Fatal(err)
	}
	if err := r.CommitNext(context.Background(), sinkFunc(func(_ context.Context, b source.Batch) error {
		if string(b.Records[0].Raw) != "second\n" {
			t.Fatal("expected buffered physical bytes")
		}
		a := CheckpointAnchor{Offset: 13, Length: 13, Digest: sha256.Sum256([]byte("first\nsecond\n"))}
		if b.Checkpoints[0].AnchorHash != a.String() {
			t.Fatal("anchor was recomputed from rewritten file")
		}
		match, err := a.Matches(f)
		if err != nil || match {
			t.Fatalf("rewritten file matches old bytes: %v, %v", match, err)
		}
		return nil
	})); err != nil {
		t.Fatal(err)
	}
}

func TestIngestorCancellationRetainsCompletePendingLine(t *testing.T) {
	f, _ := testRegularFile(t, "line\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	normalizations := 0
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), func(raw []byte) model.Observation { normalizations++; cancel(); return testNormalizer(raw) })
	if err != nil {
		t.Fatal(err)
	}
	if err := r.CommitNext(ctx, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("cancelled batch committed"); return nil })); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if r.Position().Offset != 0 {
		t.Fatal("cancelled offset advanced")
	}
	if err := r.CommitNext(context.Background(), sinkFunc(func(context.Context, source.Batch) error { return nil })); err != nil {
		t.Fatal(err)
	}
	if normalizations != 1 || r.Position().Offset != 5 {
		t.Fatal("pending batch not retained")
	}
}

func TestIngestorLostAcknowledgmentIsIdempotentWithSQLite(t *testing.T) {
	ctx := context.Background()
	f, _ := testRegularFile(t, "synthetic\n")
	state := ingestState(t, f, 0)
	dbPath := filepath.Join(t.TempDir(), "state.sqlite")
	store, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Commit(ctx, source.Batch{Source: fileSourceIdentity(), Origins: []source.Origin{state.Origin}, Checkpoints: []source.Position{*state.Checkpoint}}); err != nil {
		t.Fatal(err)
	}
	r, err := NewIngestor(ctx, f, fileSourceIdentity(), state, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("lost acknowledgment")
	if err := r.CommitNext(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		return boom
	})); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if r.Position().Offset != 0 {
		t.Fatal("lost acknowledgment advanced memory state")
	}
	if err := r.CommitNext(ctx, store); err != nil {
		t.Fatal(err)
	}
	p, ok, err := store.Checkpoint(ctx, fileSourceIdentity().ID, state.Origin.ID)
	if err != nil || !ok || p != r.Position() || p.Offset != 10 {
		t.Fatalf("checkpoint %+v, %v, %v", p, ok, err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var records, events int
	if err := db.QueryRowContext(ctx, "SELECT (SELECT count(*) FROM raw_records), (SELECT count(*) FROM events)").Scan(&records, &events); err != nil {
		t.Fatal(err)
	}
	if records != 1 || events != 1 {
		t.Fatalf("duplicate effect: records %d, events %d", records, events)
	}
}

func TestNewIngestorRejectsInvalidStartingState(t *testing.T) {
	for _, kind := range []string{"missing checkpoint", "wrong origin", "wrong anchor offset", "changed anchor", "midline", "truncated", "changed prefix"} {
		t.Run(kind, func(t *testing.T) {
			f, _ := testRegularFile(t, "first\nlast\n")
			state := ingestState(t, f, 6)
			switch kind {
			case "missing checkpoint":
				state.Checkpoint = nil
			case "wrong origin":
				state.Checkpoint.OriginID = "other"
			case "wrong anchor offset":
				state.Checkpoint.Offset++
			case "changed anchor":
				a, _ := ParseCheckpointAnchor(state.Checkpoint.AnchorHash)
				a.Digest[0]++
				state.Checkpoint.AnchorHash = a.String()
			case "midline":
				a, err := CaptureAnchor(f, 5)
				if err != nil {
					t.Fatal(err)
				}
				state.Checkpoint.Offset, state.Checkpoint.AnchorHash = 5, a.String()
			case "truncated":
				if err := f.Truncate(2); err != nil {
					t.Fatal(err)
				}
			case "changed prefix":
				if _, err := f.WriteAt([]byte("X"), 0); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := NewIngestor(context.Background(), f, fileSourceIdentity(), state, testNormalizer); err == nil {
				t.Fatal("invalid start accepted")
			}
		})
	}
}
