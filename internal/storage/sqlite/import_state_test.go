package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

const importTestSHA = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func seedLegacyImportRows(t *testing.T, db *sql.DB, version int) {
	t.Helper()
	if version == 2 {
		if _, err := db.Exec(schemaV2 + `INSERT INTO schema_migrations VALUES(2, 2); PRAGMA user_version = 2;`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO import_runs VALUES
		(-1, '/synthetic/legacy', 9, 'legacy-noncanonical', 'complete', 7, 1, NULL),
		(2, '/synthetic/legacy', NULL, NULL, 'running', 0, 2, NULL),
		(3, '/synthetic/legacy', 5, 'old', 'failed', 1, 3, 4);`); err != nil {
		t.Fatal(err)
	}
}

func legacyImportValues(t *testing.T, db *sql.DB) [][]any {
	t.Helper()
	rows, err := db.Query(`SELECT id, path, byte_size, sha256, status, last_offset, created_at_ns, completed_at_ns FROM import_runs ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result [][]any
	for rows.Next() {
		values := make([]any, 8)
		pointers := make([]any, 8)
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			t.Fatal(err)
		}
		result = append(result, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestImportMigrationPreservesV1V2LegacyWithoutAssociation(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			db, path := seedV1FollowStore(t)
			seedLegacyImportRows(t, db, version)
			before := legacyImportValues(t, db)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			s, err := Open(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if after := legacyImportValues(t, s.db); !reflect.DeepEqual(before, after) {
				t.Fatal("migration changed legacy columns", before, after)
			}
			var linked, indexes, integrity int
			if err := s.db.QueryRow(`SELECT count(*) FROM import_runs WHERE source_id IS NOT NULL OR generation_id IS NOT NULL OR trailing_partial IS NOT NULL`).Scan(&linked); err != nil || linked != 0 {
				t.Fatal("migration invented an association", linked, err)
			}
			if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name IN ('import_runs_hash', 'import_runs_source_origin')`).Scan(&indexes); err != nil || indexes != 2 {
				t.Fatal("import indexes", indexes, err)
			}
			if err := s.db.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&integrity); err != nil || integrity != 0 {
				t.Fatal("foreign key check", integrity, err)
			}
			if count(t, s, "schema_migrations") != 6 || count(t, s, "raw_records") != 1 || count(t, s, "events") != 1 {
				t.Fatal("migration lost observations or history")
			}
			for _, id := range []int64{2, 3} {
				r, found, err := s.ImportRun(context.Background(), "mail", id)
				if err != nil || found || r != (source.ImportRun{}) {
					t.Fatal("legacy row adopted", r, found, err)
				}
			}
		})
	}
}

func TestImportMigrationFailureRollsBackRebuildAndHistory(t *testing.T) {
	for _, version := range []int{1, 2} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			db, _ := seedV1FollowStore(t)
			seedLegacyImportRows(t, db, version)
			before := legacyImportValues(t, db)
			if _, err := db.Exec(`CREATE TRIGGER stop_v3 BEFORE INSERT ON schema_migrations WHEN NEW.version = 3 BEGIN SELECT RAISE(ABORT, 'synthetic v3 failure'); END;`); err != nil {
				t.Fatal(err)
			}
			if err := migrate(context.Background(), db, 99); err == nil {
				t.Fatal("migration failure ignored")
			}
			var current, columns, temporary, history int
			for query, output := range map[string]*int{
				`PRAGMA user_version`: &current,
				`SELECT count(*) FROM pragma_table_info('import_runs') WHERE name = 'source_id'`: &columns,
				`SELECT count(*) FROM sqlite_master WHERE name = 'import_runs_v3'`:               &temporary,
				`SELECT count(*) FROM schema_migrations`:                                         &history,
			} {
				if err := db.QueryRow(query).Scan(output); err != nil {
					t.Fatal(err)
				}
			}
			if current != version || columns != 0 || temporary != 0 || history != version || !reflect.DeepEqual(before, legacyImportValues(t, db)) {
				t.Fatal("migration failure left partial changes", current, columns, temporary, history)
			}
		})
	}
}

func seedImportIdentity(t *testing.T, s *Store, sourceID string) string {
	t.Helper()
	id, err := source.ImportOriginID(sourceID, importTestSHA)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(context.Background(), source.Batch{
		Source:  source.Identity{ID: sourceID, Kind: "import", Name: "synthetic archive"},
		Origins: []source.Origin{{ID: id, Path: "/synthetic/original", Fingerprint: "sha256:" + importTestSHA, FirstSeen: time.Unix(0, 10)}},
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func insertImportState(t *testing.T, s *Store, r source.ImportRun) error {
	t.Helper()
	var origin, bytes, digest, partial, completed any
	if r.Content != nil {
		origin, bytes, digest, partial = r.Content.OriginID, r.Content.Bytes, r.Content.SHA256, r.Content.TrailingPartial
	}
	if r.CompletedAt != nil {
		completed = r.CompletedAt.UnixNano()
	}
	_, err := s.db.Exec(`INSERT INTO import_runs(id, path, byte_size, sha256, status, last_offset, created_at_ns, completed_at_ns, source_id, generation_id, trailing_partial) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Path, bytes, digest, r.Status, r.LastOffset, r.CreatedAt.UnixNano(), completed, r.SourceID, origin, partial)
	return err
}

func TestImportRunReadScopeAbsenceStatusesAndReopen(t *testing.T) {
	s, path := openTestStore(t)
	const sourceID = "\x00archive_%'"
	id := seedImportIdentity(t, s, sourceID)
	seedImportIdentity(t, s, "other")
	stamp := time.Unix(0, 20).UTC()
	cases := []source.ImportRun{
		{ID: 1, SourceID: sourceID, Path: "/synthetic/archive.gz", Status: source.ImportRunning, CreatedAt: stamp},
		{ID: 2, SourceID: sourceID, Path: "/synthetic/archive.gz", Status: source.ImportFailed, CreatedAt: stamp, CompletedAt: &stamp},
		{ID: 3, SourceID: sourceID, Path: "/synthetic/renamed", Status: source.ImportRunning, CreatedAt: stamp, Content: &source.ImportContent{OriginID: id, Bytes: 7, SHA256: importTestSHA, TrailingPartial: true}, LastOffset: 5},
		{ID: 4, SourceID: sourceID, Path: "/synthetic/empty", Status: source.ImportComplete, CreatedAt: stamp, CompletedAt: &stamp, Content: &source.ImportContent{OriginID: id, SHA256: importTestSHA}},
		{ID: 5, SourceID: sourceID, Path: "/synthetic/empty", Status: source.ImportFailed, CreatedAt: stamp, CompletedAt: &stamp, Content: &source.ImportContent{OriginID: id, SHA256: importTestSHA}},
	}
	for _, r := range cases {
		if err := insertImportState(t, s, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, want := range cases {
		got, found, err := reopened.ImportRun(context.Background(), sourceID, want.ID)
		if err != nil || !found || !reflect.DeepEqual(got, want) {
			t.Fatal("run lost on reopen", got, found, err, want)
		}
		for _, scope := range []string{"other", "archive", "missing"} {
			got, found, err := reopened.ImportRun(context.Background(), scope, want.ID)
			if err != nil || found || got != (source.ImportRun{}) {
				t.Fatal("foreign run exposed", scope, got, found, err)
			}
		}
	}
	got, found, err := reopened.ImportRun(context.Background(), sourceID, 99)
	if err != nil || found || got != (source.ImportRun{}) {
		t.Fatal("missing run fabricated", got, found, err)
	}
	for _, invalid := range []struct {
		source string
		id     int64
	}{{"", 1}, {sourceID, 0}, {sourceID, -1}} {
		got, found, err := reopened.ImportRun(context.Background(), invalid.source, invalid.id)
		if err == nil || found || got != (source.ImportRun{}) {
			t.Fatal("invalid query accepted", got, found, err)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	got, found, err = reopened.ImportRun(cancelled, sourceID, 1)
	if !errors.Is(err, context.Canceled) || found || got != (source.ImportRun{}) || count(t, reopened, "import_runs") != len(cases) || count(t, reopened, "raw_records") != 0 {
		t.Fatal("cancel/read changed state", got, found, err)
	}
}

func TestImportRunConstraintsAndOriginConsistency(t *testing.T) {
	s, _ := openTestStore(t)
	id := seedImportIdentity(t, s, "archive")
	foreign := seedImportIdentity(t, s, "other")
	stamp := time.Unix(0, 20).UTC()
	for _, tc := range []struct {
		name string
		edit func(*source.ImportRun)
	}{
		{"foreign_origin", func(r *source.ImportRun) { r.Content.OriginID = foreign }},
		{"missing_origin", func(r *source.ImportRun) { r.Content.OriginID = "missing" }},
		{"partial_complete", func(r *source.ImportRun) { r.Content.TrailingPartial = true }},
		{"incomplete_offset", func(r *source.ImportRun) { r.Content.Bytes = 1 }},
		{"offset_over_size", func(r *source.ImportRun) { r.LastOffset = 1 }},
		{"noncanonical_sha", func(r *source.ImportRun) { r.Content.SHA256 = strings.ToUpper(importTestSHA) }},
		{"sha_with_nul_suffix", func(r *source.ImportRun) { r.Content.SHA256 += "\x00junk" }},
		{"sha_with_nul_inside", func(r *source.ImportRun) { r.Content.SHA256 = importTestSHA[:1] + "\x00" + importTestSHA[2:] }},
		{"no_completed_time", func(r *source.ImportRun) { r.CompletedAt = nil }},
		{"running_completed", func(r *source.ImportRun) { r.Status = source.ImportRunning }},
		{"complete_unprepared", func(r *source.ImportRun) { r.Content = nil }},
		{"negative_id", func(r *source.ImportRun) { r.ID = -1 }},
		{"empty_path", func(r *source.ImportRun) { r.Path = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := source.ImportRun{ID: 1, SourceID: "archive", Path: "/synthetic/import", Status: source.ImportComplete, CreatedAt: stamp, CompletedAt: &stamp, Content: &source.ImportContent{OriginID: id, SHA256: importTestSHA}}
			tc.edit(&r)
			if err := insertImportState(t, s, r); err == nil || count(t, s, "import_runs") != 0 {
				t.Fatal("invalid linked state stored", err)
			}
		})
	}
	r := source.ImportRun{ID: 1, SourceID: "archive", Path: "/synthetic/import", Status: source.ImportRunning, CreatedAt: stamp, Content: &source.ImportContent{OriginID: id, SHA256: importTestSHA}}
	if err := insertImportState(t, s, r); err != nil {
		t.Fatal(err)
	}
	for _, corrupt := range []string{
		`UPDATE file_generations SET fingerprint = 'changed' WHERE source_id = 'archive'`,
		`UPDATE file_generations SET fingerprint = 'sha256:` + importTestSHA + `', device = 'physical' WHERE source_id = 'archive'`,
		`UPDATE sources SET kind = 'file' WHERE id = 'archive'`,
	} {
		if _, err := s.db.Exec(corrupt); err != nil {
			t.Fatal(err)
		}
		got, found, err := s.ImportRun(context.Background(), "archive", 1)
		if !errors.Is(err, ErrImportState) || found || got != (source.ImportRun{}) {
			t.Fatal("inconsistent content origin exposed", got, found, err)
		}
	}
}
