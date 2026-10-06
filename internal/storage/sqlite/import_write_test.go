package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func preparationBatch() source.Batch {
	return source.Batch{
		Source: source.Identity{ID: "archive", Kind: "import", Name: "synthetic import"},
		ImportChange: &source.ImportChange{Target: source.ImportRun{
			ID: 101, SourceID: "archive", Path: "/synthetic/archive.gz",
			Status: source.ImportRunning, CreatedAt: time.Date(2026, 10, 5, 12, 0, 0, 17, time.UTC),
		}},
	}
}

func failedPreparation(b source.Batch) source.Batch {
	before := b.ImportChange.Target
	after := before
	completed := before.CreatedAt.Add(time.Second)
	after.Status, after.CompletedAt = source.ImportFailed, &completed
	b.ImportChange = &source.ImportChange{Before: &before, Target: after}
	return b
}

func assertImportRun(t *testing.T, s *Store, want source.ImportRun) {
	t.Helper()
	got, found, err := s.ImportRun(context.Background(), want.SourceID, want.ID)
	if err != nil || !found || !sameImportRun(got, want) {
		t.Fatal("unexpected durable import run", got, found, err, want)
	}
}

func TestImportPreparationAttemptLifecycleLostAckAndReopen(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	begin := preparationBatch()
	failed := failedPreparation(begin)
	ackLost := errors.New("synthetic lost acknowledgment")
	for _, b := range []source.Batch{begin, failed} {
		// The caller cannot infer failure from an error after a durable Commit.
		err := func() error {
			if err := s.Commit(ctx, b); err != nil {
				return err
			}
			return ackLost
		}()
		if !errors.Is(err, ackLost) {
			t.Fatal(err)
		}
		assertImportRun(t, s, b.ImportChange.Target)
		if err := s.Commit(ctx, b); err != nil {
			t.Fatal("identical lost-ACK retry", err)
		}
	}
	if count(t, s, "import_runs") != 1 || count(t, s, "sources") != 1 ||
		count(t, s, "file_generations") != 0 || count(t, s, "raw_records") != 0 || count(t, s, "checkpoints") != 0 {
		t.Fatal("preparation attempt duplicated or created ingestion state")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	assertImportRun(t, reopened, failed.ImportChange.Target)
	if err := reopened.Commit(ctx, failed); err != nil {
		t.Fatal("terminal retry after reopen", err)
	}
}

func TestImportPreparationExpectedStateAndGlobalIDConflicts(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := preparationBatch()
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	// A duplicate global ID must not be attributed to a different source, even
	// though its exact scoped lookup sees no row. Source insertion rolls back.
	foreign := preparationBatch()
	foreign.Source.ID, foreign.ImportChange.Target.SourceID = "other", "other"
	if err := s.Commit(ctx, foreign); !errors.Is(err, ErrImportConflict) || count(t, s, "sources") != 1 {
		t.Fatal("foreign global run adopted", err)
	}
	if _, err := s.db.Exec(`INSERT INTO import_runs(id, path, status, created_at_ns) VALUES(202, '/synthetic/legacy', 'running', 0)`); err != nil {
		t.Fatal(err)
	}
	legacy := preparationBatch()
	legacy.ImportChange.Target.ID = 202
	if err := s.Commit(ctx, legacy); !errors.Is(err, ErrImportConflict) {
		t.Fatal("legacy global run adopted", err)
	}
	missing := failedPreparation(preparationBatch())
	missing.ImportChange.Before.ID, missing.ImportChange.Target.ID = 303, 303
	if err := s.Commit(ctx, missing); !errors.Is(err, ErrImportConflict) {
		t.Fatal("missing expected state created", err)
	}
	failed := failedPreparation(b)
	if err := s.Commit(ctx, failed); err != nil {
		t.Fatal(err)
	}
	for _, stale := range []source.Batch{
		b, // an old create cannot revive a failed attempt
		func() source.Batch {
			x := failedPreparation(b)
			stamp := x.ImportChange.Target.CompletedAt.Add(time.Second)
			x.ImportChange.Target.CompletedAt = &stamp
			return x
		}(),
	} {
		if err := s.Commit(ctx, stale); !errors.Is(err, ErrImportConflict) {
			t.Fatal("stale target overwritten", err)
		}
		assertImportRun(t, s, failed.ImportChange.Target)
	}
}

func TestImportPreparationSQLFailureAndCancellationRollback(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := preparationBatch()
	if _, err := s.db.Exec(`CREATE TRIGGER refuse_import_insert BEFORE INSERT ON import_runs BEGIN SELECT RAISE(ABORT, 'synthetic manifest insertion failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ctx, b); err == nil || count(t, s, "sources") != 0 || count(t, s, "import_runs") != 0 {
		t.Fatal("failed insertion left source/run", err)
	}
	if _, err := s.db.Exec(`DROP TRIGGER refuse_import_insert`); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER refuse_import_failure BEFORE UPDATE ON import_runs BEGIN SELECT RAISE(ABORT, 'synthetic manifest update failure'); END;`); err != nil {
		t.Fatal(err)
	}
	failed := failedPreparation(b)
	failed.Source.Name = "tentative renamed source"
	if err := s.Commit(ctx, failed); err == nil {
		t.Fatal("failed manifest update committed")
	}
	assertImportRun(t, s, b.ImportChange.Target)
	var name string
	if err := s.db.QueryRow(`SELECT name FROM sources WHERE id = 'archive'`).Scan(&name); err != nil || name != b.Source.Name {
		t.Fatal("manifest failure left source mutation", name, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Commit(cancelled, failed); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled commit", err)
	}
	assertImportRun(t, s, b.ImportChange.Target)
}

func TestImportPreparationRefusesUnprovenContentAndInvalidChanges(t *testing.T) {
	s, _ := openTestStore(t)
	for _, tc := range []struct {
		name string
		edit func(*source.Batch)
	}{
		{"zero_id", func(b *source.Batch) { b.ImportChange.Target.ID = 0 }},
		{"foreign_source", func(b *source.Batch) { b.ImportChange.Target.SourceID = "other" }},
		{"wrong_kind", func(b *source.Batch) { b.Source.Kind = "file" }},
		{"empty_path", func(b *source.Batch) { b.ImportChange.Target.Path = "" }},
		{"unknown_status", func(b *source.Batch) { b.ImportChange.Target.Status = "unknown" }},
		{"complete_unprepared", func(b *source.Batch) { b.ImportChange.Target.Status = source.ImportComplete }},
		{"failed_creation", func(b *source.Batch) { *b = failedPreparation(*b); b.ImportChange.Before = nil }},
		{"unrepresentable_created", func(b *source.Batch) { b.ImportChange.Target.CreatedAt = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC) }},
		{"unrepresentable_completed", func(b *source.Batch) {
			*b = failedPreparation(*b)
			stamp := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
			b.ImportChange.Target.CompletedAt = &stamp
		}},
		{"backwards_completed", func(b *source.Batch) {
			*b = failedPreparation(*b)
			stamp := b.ImportChange.Target.CreatedAt.Add(-time.Second)
			b.ImportChange.Target.CompletedAt = &stamp
		}},
		{"changed_path", func(b *source.Batch) { *b = failedPreparation(*b); b.ImportChange.Target.Path += ".changed" }},
		{"changed_created", func(b *source.Batch) {
			*b = failedPreparation(*b)
			b.ImportChange.Target.CreatedAt = b.ImportChange.Target.CreatedAt.Add(time.Nanosecond)
		}},
		{"terminal_before", func(b *source.Batch) {
			*b = failedPreparation(*b)
			prior := b.ImportChange.Target
			b.ImportChange.Before = &prior
		}},
		{"unproven_content", func(b *source.Batch) {
			id, _ := source.ImportOriginID("archive", importTestSHA)
			b.ImportChange.Target.Content = &source.ImportContent{OriginID: id, SHA256: importTestSHA}
		}},
		{"origins_before_preparation", func(b *source.Batch) { b.Origins = testBatch().Origins }},
		{"records_before_preparation", func(b *source.Batch) { b.Records = testBatch().Records; b.Records[0].Observation.SourceID = "archive" }},
		{"checkpoint_before_preparation", func(b *source.Batch) { b.Checkpoints = testBatch().Checkpoints }},
		{"checkpoint_without_manifest", func(b *source.Batch) { b.Checkpoints = testBatch().Checkpoints; b.ImportChange = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := preparationBatch()
			tc.edit(&b)
			if err := s.Commit(context.Background(), b); err == nil {
				t.Fatal("invalid preparation change committed")
			}
			if count(t, s, "sources") != 0 || count(t, s, "import_runs") != 0 || count(t, s, "file_generations") != 0 || count(t, s, "raw_records") != 0 || count(t, s, "checkpoints") != 0 {
				t.Fatal("invalid preparation created durable state")
			}
		})
	}
}
