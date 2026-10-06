package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
)

func seedV3ProjectionStore(t *testing.T) (*sql.DB, string) {
	t.Helper()
	db, path := seedV1FollowStore(t)
	if _, err := db.Exec(schemaV2 + schemaV3 + `INSERT INTO schema_migrations VALUES(2,2),(3,3); PRAGMA user_version=3;`); err != nil {
		t.Fatal(err)
	}
	return db, path
}

func projectionSchemaScope(t *testing.T, db *sql.DB, id int, hashByte string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO projection_scopes(id,scope_sha256,format_version) VALUES(?,?,1)`, id, strings.Repeat(hashByte, 64)); err != nil {
		t.Fatal(err)
	}
}

func projectionSchemaRevision(t *testing.T, db *sql.DB, id, scope int) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO projection_revisions(id,scope_id,format_version,revision,input_revision,link_window_ns,fact_count,created_at_ns) VALUES(?,?,1,?,?,60000000000,1,1)`, id, scope, strings.Repeat("c", 64), strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionMigrationV3PreservesFactsAndReopens(t *testing.T) {
	db, path := seedV3ProjectionStore(t)
	batch := testBatch()
	scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: batch.Source.TrustedHost, QueueID: batch.Records[0].Observation.QueueID}}}
	before, err := (&Store{db: db}).CorrelationFacts(context.Background(), scope, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.CorrelationFacts(context.Background(), scope, 10)
	if err != nil || !reflect.DeepEqual(before, after) || count(t, s, "schema_migrations") != schemaVersion {
		t.Fatal("migration changed immutable facts/history", err)
	}
	for _, table := range []string{"projection_scopes", "projection_scope_parts", "projection_revisions", "projection_revision_bindings", "projection_revision_facts"} {
		if count(t, s, table) != 0 {
			t.Fatal("migration invented projection", table)
		}
	}
	position, found, err := s.Checkpoint(context.Background(), "mail", "gen-1")
	if err != nil || !found || position != testBatch().Checkpoints[0] {
		t.Fatal("migration changed checkpoint")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var version, bad int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatal("reopen version", version, err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&bad); err != nil || bad != 0 {
		t.Fatal("schema foreign key mismatch", err)
	}
}

func TestProjectionMigrationFailureRollsBackV4(t *testing.T) {
	db, _ := seedV3ProjectionStore(t)
	if _, err := db.Exec(`CREATE TRIGGER stop_v4 BEFORE INSERT ON schema_migrations WHEN NEW.version=4 BEGIN SELECT RAISE(ABORT,'synthetic v4 failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(context.Background(), db, 4); err == nil {
		t.Fatal("failed v4 migration succeeded")
	}
	var version, objects, history int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name LIKE 'projection_%'`).Scan(&objects); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if version != 3 || objects != 0 || history != 3 || count(t, &Store{db: db}, "events") != 1 {
		t.Fatal("failed migration left a partial schema or lost facts")
	}
}

func TestProjectionSchemaProtectsFactsAndScopeOwnership(t *testing.T) {
	s, _ := openTestStore(t)
	if err := s.Commit(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	projectionSchemaScope(t, s.db, 1, "a")
	projectionSchemaScope(t, s.db, 2, "b")
	projectionSchemaRevision(t, s.db, 1, 1)
	projectionSchemaRevision(t, s.db, 2, 2)
	if _, err := s.db.Exec(`UPDATE projection_scopes SET current_revision_id=2 WHERE id=1`); err == nil {
		t.Fatal("foreign scope revision became current")
	}
	if _, err := s.db.Exec(`INSERT INTO projection_revision_facts SELECT 1,id FROM raw_records; UPDATE projection_scopes SET current_revision_id=1 WHERE id=1;`); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{
		`DELETE FROM raw_records`, `DELETE FROM events`,
		`UPDATE raw_records SET observed_at_ns=0`, `UPDATE events SET message='changed'`,
		`DELETE FROM projection_revisions WHERE id=1`,
	} {
		if _, err := s.db.Exec(mutation); err == nil {
			t.Fatal("referenced proof/current revision was mutated", mutation)
		}
	}
	if count(t, s, "raw_records") != 1 || count(t, s, "events") != 1 || count(t, s, "projection_revision_facts") != 1 {
		t.Fatal("failed mutation damaged manifest")
	}
	// Explicit invalidation and retention happen in the same transaction.
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE projection_scopes SET current_revision_id=NULL WHERE id=1; DELETE FROM projection_revisions WHERE id=1; DELETE FROM raw_records;`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if count(t, s, "raw_records") != 0 || count(t, s, "events") != 0 || count(t, s, "projection_revision_facts") != 0 || count(t, s, "projection_revisions") != 1 {
		t.Fatal("invalidation did not release facts/keep foreign scope")
	}
	var current sql.NullInt64
	if err := s.db.QueryRow(`SELECT current_revision_id FROM projection_scopes WHERE id=1`).Scan(&current); err != nil || current.Valid {
		t.Fatal("purged revision still current")
	}
}

func TestProjectionSchemaFactsRequireEventAndUniqueMembership(t *testing.T) {
	s, _ := openTestStore(t)
	batch := testBatch()
	if err := s.Commit(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	projectionSchemaScope(t, s.db, 1, "a")
	projectionSchemaRevision(t, s.db, 1, 1)
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM raw_records`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM events`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO projection_revision_facts VALUES(1,?)`, id); err == nil {
		t.Fatal("raw record without event became an input fact")
	}
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := insertEvent(context.Background(), tx, id, "mx-a", batch.Records[0].Observation); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO projection_revision_facts VALUES(1,?)`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO projection_revision_facts VALUES(1,?)`, id); err == nil {
		t.Fatal("duplicate snapshot fact accepted")
	}
	if _, err := s.db.Exec(`INSERT INTO projection_revision_facts VALUES(1,999)`); err == nil {
		t.Fatal("absent raw/event accepted")
	}
}

func TestProjectionSchemaBoundsAndBinaryConfiguration(t *testing.T) {
	s, _ := openTestStore(t)
	projectionSchemaScope(t, s.db, 1, "a")
	projectionSchemaRevision(t, s.db, 1, 1)
	instance, queue, relay, target := []byte{0xff, 'i'}, []byte{0xfe, 'q'}, []byte{0xfe, 'r'}, []byte{0xfd, 't'}
	if _, err := s.db.Exec(`INSERT INTO projection_scope_parts VALUES(1,0,'queue',?,?)`, instance, queue); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO projection_revision_bindings VALUES(1,0,?,?,?)`, instance, relay, target); err != nil {
		t.Fatal(err)
	}
	var gotInstance, gotQueue, gotRelay, gotTarget []byte
	if err := s.db.QueryRow(`SELECT instance,queue_id FROM projection_scope_parts`).Scan(&gotInstance, &gotQueue); err != nil || !bytes.Equal(gotInstance, instance) || !bytes.Equal(gotQueue, queue) {
		t.Fatal("scope bytes canonicalized")
	}
	if err := s.db.QueryRow(`SELECT relay,to_instance FROM projection_revision_bindings`).Scan(&gotRelay, &gotTarget); err != nil || !bytes.Equal(gotRelay, relay) || !bytes.Equal(gotTarget, target) {
		t.Fatal("binding bytes canonicalized")
	}
	for _, invalid := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO projection_scope_parts VALUES(1,1,'queue',?,?)`, []any{instance, queue}},
		{`INSERT INTO projection_scope_parts VALUES(1,64,'queue',X'61',X'41')`, nil},
		{`INSERT INTO projection_scope_parts VALUES(1,1,'queue','text',X'41')`, nil},
		{`INSERT INTO projection_scope_parts VALUES(1,1,'queue',X'6100',X'41')`, nil},
		{`INSERT INTO projection_scope_parts VALUES(1,1,'queue',X'61',NULL)`, nil},
		{`INSERT INTO projection_scope_parts VALUES(1,1,'unqueued',X'61',X'41')`, nil},
		{`INSERT INTO projection_revision_bindings VALUES(1,1,?,?,?)`, []any{instance, relay, target}},
		{`INSERT INTO projection_revision_bindings VALUES(1,64,X'61',X'62',X'63')`, nil},
		{`INSERT INTO projection_revision_bindings VALUES(1,1,X'',X'62',X'63')`, nil},
		{`INSERT INTO projection_revision_bindings VALUES(1,1,X'61','text',X'63')`, nil},
	} {
		if _, err := s.db.Exec(invalid.sql, invalid.args...); err == nil {
			t.Fatal("invalid scope/binding accepted", invalid.sql)
		}
	}
	for _, change := range []string{
		`fact_count=4097`, `fact_count=-1`, `link_window_ns=0`, `link_window_ns=86400000000001`,
		`link_window_ns='not-a-duration'`, `format_version=2`, `revision='wrong'`,
		`input_revision='wrong'`, `scope_id=999`,
	} {
		if _, err := s.db.Exec(`UPDATE projection_revisions SET ` + change + ` WHERE id=1`); err == nil {
			t.Fatal("invalid revision metadata accepted", change)
		}
	}
	if _, err := s.db.Exec(`INSERT INTO projection_scopes(scope_sha256,format_version) VALUES(?,1)`, strings.Repeat("G", 64)); err == nil {
		t.Fatal("noncanonical scope digest accepted")
	}
}
