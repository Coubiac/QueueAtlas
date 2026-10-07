package sqlite

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenDiagnosticsRejectsMissingAndNonregular(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.db")
	for _, path := range []string{"", " \t", missing, filepath.Join(dir, "missing-parent", "db"), dir} {
		d, err := OpenDiagnostics(ctx, path)
		if d != nil || !errors.Is(err, ErrDiagnosticsOpen) {
			t.Fatalf("accepted invalid input: %v", err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if d, err := OpenDiagnostics(cancelled, missing); d != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled open: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("refusal created files", entries, err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		t.Run(suffix, func(t *testing.T) {
			s, path := openTestStore(t)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path+suffix, 0700); err != nil {
				t.Fatal(err)
			}
			if d, err := OpenDiagnostics(ctx, path); d != nil || err != ErrDiagnosticsOpen {
				t.Fatalf("nonregular sidecar accepted: %v", err)
			}
		})
	}
}

func TestOpenDiagnosticsPreservesRollbackDatabase(t *testing.T) {
	s, path := openTestStore(t)
	if _, err := s.db.Exec(`PRAGMA journal_mode = DELETE`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	before := diagnosticFileBytes(t, path)
	d, err := OpenDiagnostics(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := d.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "delete" {
		t.Fatal("journal mode changed", mode, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, diagnosticFileBytes(t, path)) {
		t.Fatal("diagnostic changed database bytes")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatal("rollback diagnostic created sidecars", entries, err)
	}
}

func TestOpenDiagnosticsRejectsIncompatibleWithoutMigration(t *testing.T) {
	for _, tc := range []struct{ name, change string }{
		{"older", `PRAGMA user_version = 1`},
		{"newer", `PRAGMA user_version = 8`},
		{"unversioned", `PRAGMA user_version = 0`},
		{"history gap", `DELETE FROM schema_migrations WHERE version = 4`},
		{"foreign current version", `DROP TABLE schema_migrations`},
		{"history view", `DROP TABLE schema_migrations; CREATE VIEW schema_migrations AS SELECT 1 AS version`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, path := openTestStore(t)
			if _, err := s.db.Exec(tc.change); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			before := diagnosticFileBytes(t, path)
			if d, err := OpenDiagnostics(context.Background(), path); d != nil || err != ErrDiagnosticsSchema {
				t.Fatalf("incompatible schema accepted: %v", err)
			}
			if !bytes.Equal(before, diagnosticFileBytes(t, path)) {
				t.Fatal("refusal modified database")
			}
		})
	}
	for _, content := range [][]byte{nil, []byte("synthetic non-SQLite file")} {
		path := filepath.Join(t.TempDir(), "foreign.db")
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
		d, err := OpenDiagnostics(context.Background(), path)
		if d != nil || (err != ErrDiagnosticsSchema && err != ErrDiagnosticsOpen) {
			t.Fatalf("empty/corrupt file accepted: %v", err)
		}
		if !bytes.Equal(content, diagnosticFileBytes(t, path)) {
			t.Fatal("refusal initialized foreign file")
		}
	}
}

func TestOpenDiagnosticsReadsLiveWALAndRefusesWrites(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)
	if _, err := s.db.Exec(`PRAGMA wal_autocheckpoint = 0; PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	base := diagnosticFileBytes(t, path)
	if err := s.Commit(ctx, testBatch()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(base, diagnosticFileBytes(t, path)) {
		t.Fatal("fixture already checkpointed; not exercising committed WAL")
	}
	d, err := OpenDiagnostics(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	var n int
	if err := d.db.QueryRow(`SELECT count(*) FROM raw_records`).Scan(&n); err != nil || n != 1 {
		t.Fatal("committed WAL record missing", n, err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT INTO sources VALUES ('other', 'file', 'synthetic', 'mx-b', 1)`); err != nil {
		t.Fatal(err)
	}
	if err := d.db.QueryRow(`SELECT count(*) FROM sources`).Scan(&n); err != nil || n != 1 {
		t.Fatal("uncommitted row leaked or writer blocked reader", n, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := d.db.QueryRow(`SELECT count(*) FROM sources`).Scan(&n); err != nil || n != 2 {
		t.Fatal("new commit invisible on next read", n, err)
	}
	for _, replacement := range []bool{false, true} {
		if replacement {
			d.db.SetMaxIdleConns(0)
		}
		for pragma, want := range map[string]int{"query_only": 1, "trusted_schema": 0, "foreign_keys": 1, "busy_timeout": 5000} {
			var got int
			if err := d.db.QueryRow(`PRAGMA ` + pragma).Scan(&got); err != nil || got != want {
				t.Fatal("connection guard missing", replacement, pragma, got, err)
			}
		}
		conn, err := d.db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// Prove mode=ro itself, even if query_only is disabled on this connection.
		if _, err := conn.ExecContext(ctx, `PRAGMA query_only = OFF`); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		for _, statement := range []string{`DELETE FROM schema_migrations`, `PRAGMA user_version = 1`, `CREATE TABLE forbidden (id INTEGER)`} {
			if _, err := conn.ExecContext(ctx, statement); err == nil {
				conn.Close()
				t.Fatal("readonly connection permitted write", statement)
			}
		}
		conn.Close()
		// Recycle the deliberately altered connection before the next iteration.
		d.db.SetMaxIdleConns(0)
	}
	wal := diagnosticFileBytes(t, path+"-wal")
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(base, diagnosticFileBytes(t, path)) || !bytes.Equal(wal, diagnosticFileBytes(t, path+"-wal")) {
		t.Fatal("diagnostic close changed database/WAL or checkpointed writer")
	}
}

func TestOpenDiagnosticsEscapesLiteralPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace é # % &mode=rw;.db")
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	d, err := OpenDiagnostics(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`PRAGMA query_only = OFF`); err != nil {
		d.Close()
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`DELETE FROM schema_migrations`); err == nil {
		d.Close()
		t.Fatal("filename injected writable URI option")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
}

func diagnosticFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
