package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
)

func openTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "queueatlas.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func testBatch() source.Batch {
	line := []byte("Oct  3 12:00:00 mx-a postfix/smtp[1]: ABC123: to=<bob@example.org>, status=deferred (try later)\n")
	when := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	return source.Batch{
		Source:  source.Identity{ID: "mail", Kind: "file", Name: "mail log", TrustedHost: "mx-a"},
		Origins: []source.Origin{{ID: "gen-1", Path: "/var/log/mail.log", Device: "1", Inode: "2", Fingerprint: "sha256:abc", FirstSeen: when}},
		Records: []source.Record{{OriginID: "gen-1", Start: 0, End: int64(len(line)), Raw: line, ReadAt: when,
			Observation: model.Observation{SourceID: "mail", Host: "untrusted-syslog-host", Service: "smtp", QueueID: "ABC123", Kind: model.KindDelivery,
				Timestamp: model.Timestamp{Raw: "Oct  3 12:00:00", Value: &when, Quality: model.TimeConfiguredYearAndZone},
				Fields:    map[string]string{"to": "bob@example.org", "status": "deferred", "message-id": "<id@example.org>"},
				Present:   map[string]bool{"to": true, "status": true, "message-id": true}}}},
		Checkpoints: []source.Position{{OriginID: "gen-1", Offset: int64(len(line)), AnchorHash: "sha256:tail"}},
	}
}

func count(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	// All call sites use literal table names from this test file.
	if err := s.db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrationPragmasAndReopen(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	for pragma, want := range map[string]any{
		"user_version":   int64(1),
		"foreign_keys":   int64(1),
		"trusted_schema": int64(0),
		"synchronous":    int64(2),
		"journal_mode":   "wal",
	} {
		var got any
		if err := s.db.QueryRowContext(ctx, `PRAGMA `+pragma).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s = %v, want %v", pragma, got, want)
		}
	}
	var integrity string
	if err := s.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity_check = %q, %v", integrity, err)
	}
	if err := s.Commit(ctx, testBatch()); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if count(t, reopened, "raw_records") != 1 || count(t, reopened, "events") != 1 || count(t, reopened, "schema_migrations") != 1 {
		t.Fatal("unexpected row counts after reopen")
	}
	var instance, declaredHost, messageID, recipient, status string
	if err := reopened.db.QueryRowContext(ctx, `SELECT instance, host, message_id, recipient, status FROM events`).Scan(
		&instance, &declaredHost, &messageID, &recipient, &status); err != nil {
		t.Fatal(err)
	}
	if instance != "mx-a" || declaredHost != "untrusted-syslog-host" || messageID != "<id@example.org>" || recipient != "bob@example.org" || status != "deferred" {
		t.Fatalf("event lost trusted identity or parsed search fields: %q %q %q %q %q", instance, declaredHost, messageID, recipient, status)
	}
	// DSN PRAGMAs apply even after database/sql opens a replacement connection.
	reopened.db.SetMaxIdleConns(0)
	var fk int
	if err := reopened.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys on replacement connection = %d, %v", fk, err)
	}
}

func TestCommitIdempotenceAndCollision(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ctx, b); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if count(t, s, "raw_records") != 1 || count(t, s, "events") != 1 {
		t.Fatal("replay duplicated a record or event")
	}
	p, ok, err := s.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !ok || p.Offset != b.Checkpoints[0].Offset {
		t.Fatalf("checkpoint = %+v, %v, %v", p, ok, err)
	}
	b.Checkpoints[0].Offset = 1
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	p, ok, err = s.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !ok || p.Offset != int64(len(b.Records[0].Raw)) {
		t.Fatalf("checkpoint regressed: %+v, %v, %v", p, ok, err)
	}
	// A repeated offset with different content must not silently replace data.
	b.Records[0].Raw = bytes.Replace(b.Records[0].Raw, []byte("deferred"), []byte("sent    "), 1)
	if err := s.Commit(ctx, b); err == nil || !strings.Contains(err.Error(), "provenance collision") {
		t.Fatalf("wanted provenance collision, got %v", err)
	}
	if count(t, s, "raw_records") != 1 {
		t.Fatal("collision changed committed rows")
	}
}

func TestBatchRollbackAndForeignKeys(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	initial := b.Checkpoints[0].Offset
	b.Checkpoints[0].Offset += 100
	b.Records = append(b.Records, source.Record{OriginID: "missing-origin", Start: 0, End: 1, Raw: []byte{'x'}})
	if err := s.Commit(ctx, b); err == nil {
		t.Fatal("record with missing origin committed")
	}
	p, ok, err := s.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !ok || p.Offset != initial {
		t.Fatalf("checkpoint advanced on failed batch: %+v, %v, %v", p, ok, err)
	}
	if count(t, s, "raw_records") != 1 {
		t.Fatal("failed batch left records")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO checkpoints(source_id, generation_id, offset, anchor_hash, updated_at_ns)
		VALUES('mail', 'absent', 0, '', 0)`); err == nil {
		t.Fatal("foreign key not enforced")
	}
}

func TestNewerAndUnversionedDatabasesRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed string
		want string
	}{
		{"newer", `PRAGMA user_version = 2`, "newer than supported"},
		{"unversioned", `CREATE TABLE another_app (id INTEGER)`, "unversioned nonempty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db.sqlite")
			seed, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := seed.Exec(tc.seed); err != nil {
				t.Fatal(err)
			}
			seed.Close()
			_, err = Open(context.Background(), path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Open error = %v, want %q", err, tc.want)
			}
			check, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer check.Close()
			var mode string
			if err := check.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "delete" {
				t.Fatalf("refusal changed journal mode to %q: %v", mode, err)
			}
		})
	}
}
