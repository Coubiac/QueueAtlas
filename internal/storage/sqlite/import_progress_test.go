package sqlite

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func attachedImportBatch(t *testing.T, begin source.Batch, payload string) source.Batch {
	t.Helper()
	before := begin.ImportChange.Target
	after := before
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))
	id, err := source.ImportOriginID(before.SourceID, digest)
	if err != nil {
		t.Fatal(err)
	}
	after.Content = &source.ImportContent{OriginID: id, Bytes: int64(len(payload)), SHA256: digest,
		TrailingPartial: len(payload) != 0 && payload[len(payload)-1] != '\n'}
	return source.Batch{Source: begin.Source,
		Origins:      []source.Origin{{ID: id, Path: before.Path, Fingerprint: "sha256:" + digest, FirstSeen: before.CreatedAt}},
		Checkpoints:  []source.Position{{OriginID: id, AnchorHash: "zero"}},
		ImportChange: &source.ImportChange{Before: &before, Target: after},
	}
}

func importProgressBatch(prior source.Batch, raw string, status source.ImportStatus) source.Batch {
	before := prior.ImportChange.Target
	after := before
	content := *before.Content
	after.Content = &content
	after.LastOffset += int64(len(raw))
	after.Status = status
	if status != source.ImportRunning {
		stamp := before.CreatedAt.Add(time.Second)
		after.CompletedAt = &stamp
	}
	b := source.Batch{Source: prior.Source, ImportChange: &source.ImportChange{Before: &before, Target: after}}
	if raw != "" {
		b.Records = []source.Record{{OriginID: content.OriginID, Start: before.LastOffset, End: after.LastOffset, Raw: []byte(raw), ReadAt: before.CreatedAt}}
		b.Checkpoints = []source.Position{{OriginID: content.OriginID, Offset: after.LastOffset, AnchorHash: fmt.Sprintf("tail-%d", after.LastOffset)}}
	}
	return b
}

func prepareImportFixture(t *testing.T, s *Store, payload string) source.Batch {
	t.Helper()
	begin := preparationBatch()
	if err := s.Commit(context.Background(), begin); err != nil {
		t.Fatal(err)
	}
	attached := attachedImportBatch(t, begin, payload)
	for range 2 {
		if err := s.Commit(context.Background(), attached); err != nil {
			t.Fatal("attach and retry", err)
		}
	}
	return attached
}

func assertImportPosition(t *testing.T, s *Store, want source.ImportRun, rawCount int) {
	t.Helper()
	assertImportRun(t, s, want)
	p, found, err := s.Checkpoint(context.Background(), want.SourceID, want.Content.OriginID)
	if err != nil || !found || p.Offset != want.LastOffset || count(t, s, "raw_records") != rawCount || count(t, s, "events") != rawCount {
		t.Fatal("manifest, checkpoint and records diverged", p, found, err, rawCount)
	}
}

func TestImportProgressAtomicRetryReopenAndReimport(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	attached := prepareImportFixture(t, s, "same\nsame\n")
	first := importProgressBatch(attached, "same\n", source.ImportRunning)
	last := importProgressBatch(first, "same\n", source.ImportComplete)
	for i, b := range []source.Batch{first, last} {
		for range 2 {
			if err := s.Commit(ctx, b); err != nil {
				t.Fatal("atomic progress/retry", err)
			}
		}
		assertImportPosition(t, s, b.ImportChange.Target, i+1)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	assertImportPosition(t, reopened, last.ImportChange.Target, 2)
	// A renamed attempt reuses the whole-content origin and final checkpoint;
	// the two identical physical lines remain distinct records by their offsets.
	again := last.ImportChange.Target
	again.ID, again.Path, again.Status, again.CompletedAt = 202, "/synthetic/renamed.log", source.ImportRunning, nil
	reimport := source.Batch{Source: last.Source, ImportChange: &source.ImportChange{Target: again},
		Origins: []source.Origin{{ID: again.Content.OriginID, Path: again.Path, Fingerprint: "sha256:" + again.Content.SHA256, FirstSeen: again.CreatedAt}}}
	if err := reopened.Commit(ctx, reimport); err != nil {
		t.Fatal("reimport at existing verified position", err)
	}
	done := importProgressBatch(reimport, "", source.ImportComplete)
	for range 2 {
		if err := reopened.Commit(ctx, done); err != nil {
			t.Fatal("empty remaining content/retry", err)
		}
	}
	assertImportPosition(t, reopened, done.ImportChange.Target, 2)
	if count(t, reopened, "file_generations") != 1 || count(t, reopened, "import_runs") != 2 {
		t.Fatal("reimport duplicated content origin or lost attempt trace")
	}
}

func TestImportProgressFinalWriteFailureRollsBackRecordsCheckpointAndSource(t *testing.T) {
	s, _ := openTestStore(t)
	attached := prepareImportFixture(t, s, "same\nsame\n")
	first := importProgressBatch(attached, "same\n", source.ImportRunning)
	first.Source.Name = "tentative changed name"
	if _, err := s.db.Exec("CREATE TRIGGER refuse_progress BEFORE UPDATE ON import_runs BEGIN SELECT RAISE(ABORT, 'synthetic final manifest failure'); END;"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(context.Background(), first); err == nil {
		t.Fatal("final manifest failure was acknowledged")
	}
	assertImportPosition(t, s, attached.ImportChange.Target, 0)
	var name string
	if err := s.db.QueryRow("SELECT name FROM sources WHERE id = 'archive'").Scan(&name); err != nil || name != attached.Source.Name {
		t.Fatal("source mutation survived failed manifest", name, err)
	}
	if _, err := s.db.Exec("DROP TRIGGER refuse_progress"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(context.Background(), first); err != nil {
		t.Fatal("same batch after failure", err)
	}
	assertImportPosition(t, s, first.ImportChange.Target, 1)
}

func TestImportProgressRefusesDivergentProvenanceAndSkippedOffsets(t *testing.T) {
	s, _ := openTestStore(t)
	attached := prepareImportFixture(t, s, "same\nsame\n")
	first := importProgressBatch(attached, "same\n", source.ImportRunning)
	if err := s.Commit(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*source.Batch)
	}{
		{"content_size_changed", func(b *source.Batch) { b.ImportChange.Target.Content.Bytes++ }},
		{"content_partial_changed", func(b *source.Batch) { b.ImportChange.Target.Content.TrailingPartial = true }},
		{"content_removed", func(b *source.Batch) { b.ImportChange.Target.Content = nil }},
		{"missing_records", func(b *source.Batch) { b.Records = nil }},
		{"foreign_record", func(b *source.Batch) { b.Records[0].OriginID = "foreign" }},
		{"gap", func(b *source.Batch) { b.Records[0].Start++ }},
		{"short_record_end", func(b *source.Batch) { b.Records[0].Raw = []byte("x\n"); b.Records[0].End = b.Records[0].Start + 2 }},
		{"foreign_checkpoint", func(b *source.Batch) { b.Checkpoints[0].OriginID = "foreign" }},
		{"missing_checkpoint", func(b *source.Batch) { b.Checkpoints = nil }},
		{"wrong_checkpoint", func(b *source.Batch) { b.Checkpoints[0].Offset-- }},
		{"empty_anchor", func(b *source.Batch) { b.Checkpoints[0].AnchorHash = "" }},
		{"extra_checkpoint", func(b *source.Batch) { b.Checkpoints = append(b.Checkpoints, b.Checkpoints[0]) }},
		{"wrong_origin_fingerprint", func(b *source.Batch) {
			b.Origins = append([]source.Origin(nil), attached.Origins...)
			b.Origins[0].Fingerprint = "changed"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := importProgressBatch(first, "same\n", source.ImportComplete)
			tc.edit(&b)
			if err := s.Commit(context.Background(), b); err == nil {
				t.Fatal("divergent import progress acknowledged")
			}
			assertImportPosition(t, s, first.ImportChange.Target, 1)
		})
	}
	// An ignored same-offset checkpoint must not acknowledge a changed anchor.
	failed := importProgressBatch(first, "", source.ImportFailed)
	failed.Checkpoints = []source.Position{{OriginID: first.ImportChange.Target.Content.OriginID, Offset: 5, AnchorHash: "impostor"}}
	if err := s.Commit(context.Background(), failed); err == nil {
		t.Fatal("different same-offset checkpoint accepted")
	}
	assertImportPosition(t, s, first.ImportChange.Target, 1)
	if _, err := s.db.Exec("UPDATE checkpoints SET offset = 4 WHERE source_id = 'archive'"); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(context.Background(), importProgressBatch(first, "same\n", source.ImportComplete)); err == nil {
		t.Fatal("inconsistent prior checkpoint repaired implicitly")
	}
	assertImportRun(t, s, first.ImportChange.Target)
	if count(t, s, "raw_records") != 1 {
		t.Fatal("inconsistent prior position committed more records")
	}
}

func TestImportProgressEmptyCompletionAndPartialFailure(t *testing.T) {
	for _, payload := range []string{"", "ok\npartial"} {
		t.Run(fmt.Sprintf("bytes_%d", len(payload)), func(t *testing.T) {
			s, _ := openTestStore(t)
			attached := prepareImportFixture(t, s, payload)
			if payload == "" {
				done := importProgressBatch(attached, "", source.ImportComplete)
				if err := s.Commit(context.Background(), done); err != nil {
					t.Fatal("validated empty complete", err)
				}
				assertImportPosition(t, s, done.ImportChange.Target, 0)
				return
			}
			first := importProgressBatch(attached, "ok\n", source.ImportRunning)
			if err := s.Commit(context.Background(), first); err != nil {
				t.Fatal(err)
			}
			if err := s.Commit(context.Background(), importProgressBatch(first, "partial", source.ImportComplete)); err == nil {
				t.Fatal("unterminated suffix claimed complete")
			}
			assertImportPosition(t, s, first.ImportChange.Target, 1)
			failed := importProgressBatch(first, "", source.ImportFailed)
			for range 2 {
				if err := s.Commit(context.Background(), failed); err != nil {
					t.Fatal("partial failure/retry", err)
				}
			}
			assertImportPosition(t, s, failed.ImportChange.Target, 1)
		})
	}
}

func TestImportContentAssociationRequiresMatchingOriginAndPosition(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*source.Batch)
	}{
		{"missing_origin", func(b *source.Batch) { b.Origins = nil }},
		{"missing_zero_checkpoint", func(b *source.Batch) { b.Checkpoints = nil }},
		{"unproven_positive_checkpoint", func(b *source.Batch) { b.ImportChange.Target.LastOffset = 5; b.Checkpoints[0].Offset = 5 }},
		{"foreign_origin", func(b *source.Batch) { b.Origins[0].ID = "foreign" }},
		{"physical_origin", func(b *source.Batch) { b.Origins[0].Device = "physical" }},
		{"wrong_path", func(b *source.Batch) { b.Origins[0].Path = "/synthetic/other" }},
		{"immediate_complete", func(b *source.Batch) {
			b.ImportChange.Target.Status = source.ImportComplete
			stamp := b.ImportChange.Target.CreatedAt
			b.ImportChange.Target.CompletedAt = &stamp
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := openTestStore(t)
			begin := preparationBatch()
			if err := s.Commit(context.Background(), begin); err != nil {
				t.Fatal(err)
			}
			attached := attachedImportBatch(t, begin, "same\nsame\n")
			tc.edit(&attached)
			if err := s.Commit(context.Background(), attached); err == nil {
				t.Fatal("unverified association acknowledged")
			}
			assertImportRun(t, s, begin.ImportChange.Target)
			if count(t, s, "file_generations") != 0 || count(t, s, "checkpoints") != 0 {
				t.Fatal("failed association left provenance/position")
			}
		})
	}
}

func TestImportProgressAlreadyAppliedChecksEqualOffsetAnchor(t *testing.T) {
	for _, offset := range []int64{0, 5} {
		t.Run(fmt.Sprintf("offset_%d", offset), func(t *testing.T) {
			s, _ := openTestStore(t)
			applied := prepareImportFixture(t, s, "same\nsame\n")
			if offset == 5 {
				applied = importProgressBatch(applied, "same\n", source.ImportRunning)
				if err := s.Commit(context.Background(), applied); err != nil {
					t.Fatal(err)
				}
			}
			mutated := applied
			mutated.Checkpoints = append([]source.Position(nil), applied.Checkpoints...)
			mutated.Checkpoints[0].AnchorHash = "impostor"
			if err := s.Commit(context.Background(), mutated); err == nil {
				t.Fatal("already-applied target acknowledged a different same-offset anchor")
			}
			assertImportPosition(t, s, applied.ImportChange.Target, int(offset/5))
			if err := s.Commit(context.Background(), applied); err != nil {
				t.Fatal("exact retry rejected", err)
			}
			// Another attempt can advance the shared content checkpoint while the
			// first attempt's Target remains durable. Its exact old retry is safe.
			other := applied.ImportChange.Target
			other.ID = 202
			start := source.Batch{Source: applied.Source, ImportChange: &source.ImportChange{Target: other}}
			if err := s.Commit(context.Background(), start); err != nil {
				t.Fatal(err)
			}
			advanced := importProgressBatch(start, "same\n", source.ImportRunning)
			if err := s.Commit(context.Background(), advanced); err != nil {
				t.Fatal(err)
			}
			if err := s.Commit(context.Background(), applied); err != nil {
				t.Fatal("old exact retry after later shared progress", err)
			}
			assertImportPosition(t, s, advanced.ImportChange.Target, int(offset/5)+1)
			assertImportRun(t, s, applied.ImportChange.Target)
		})
	}
}

func TestImportContentReuseRefusesChangedMetadataAndStoredOrigin(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*source.ImportRun)
		seed string
	}{
		{name: "size", edit: func(r *source.ImportRun) { r.Content.Bytes++ }},
		{name: "partial", edit: func(r *source.ImportRun) { r.Content.TrailingPartial = true }},
		{name: "stored_fingerprint", seed: "UPDATE file_generations SET fingerprint = 'changed'"},
		{name: "stored_physical_identity", seed: "UPDATE file_generations SET device = 'physical'"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := openTestStore(t)
			attached := prepareImportFixture(t, s, "same\nsame\n")
			target := attached.ImportChange.Target
			content := *target.Content
			target.Content, target.ID = &content, 202
			if tc.edit != nil {
				tc.edit(&target)
			}
			if tc.seed != "" {
				if _, err := s.db.Exec(tc.seed); err != nil {
					t.Fatal(err)
				}
			}
			b := source.Batch{Source: attached.Source, ImportChange: &source.ImportChange{Target: target}}
			if err := s.Commit(context.Background(), b); err == nil {
				t.Fatal("inconsistent reused content accepted")
			}
			r, found, err := s.ImportRun(context.Background(), target.SourceID, target.ID)
			if err != nil || found || r != (source.ImportRun{}) || count(t, s, "import_runs") != 1 || count(t, s, "raw_records") != 0 {
				t.Fatal("inconsistent reuse left a new attempt", r, found, err)
			}
		})
	}
}
