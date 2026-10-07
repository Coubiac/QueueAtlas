package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDiagnosticsMetadataRollbackAndPrivacy(t *testing.T) {
	s, path := openTestStore(t)
	if err := s.Commit(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	// Make reusable pages without relying on the exact layout of a migration.
	if _, err := s.db.Exec(`CREATE TABLE synthetic_padding(payload BLOB);
		INSERT INTO synthetic_padding VALUES (zeroblob(131072));
		DROP TABLE synthetic_padding; PRAGMA journal_mode = DELETE`); err != nil {
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
	t.Cleanup(func() { d.Close() })
	m, err := d.Metadata(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Check against the physical image of a closed rollback database, including
	// the header encoding used for the special 65536-byte page size.
	pageSize := int64(binary.BigEndian.Uint16(before[16:18]))
	if pageSize == 1 {
		pageSize = 65536
	}
	if m.SchemaVersion != schemaVersion || m.JournalMode != "delete" ||
		m.SQLiteVersion == "" || m.PageSize != pageSize ||
		m.PageCount*m.PageSize != int64(len(before)) || m.FreePageCount < 1 ||
		m.FreePageCount > m.PageCount {
		t.Fatalf("unexpected metadata: %+v", m)
	}
	encoded, err := json.Marshal(m)
	if err != nil || len(encoded) > 512 {
		t.Fatal("metadata result is not bounded", len(encoded), err)
	}
	for _, private := range []string{path, "bob@example.org", "mx-a", "ABC123", "raw_records", "synthetic_padding"} {
		if strings.Contains(string(encoded), private) {
			t.Fatal("metadata exposed identity, path, table or log content")
		}
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, diagnosticFileBytes(t, path)) {
		t.Fatal("metadata read modified rollback database")
	}
}

func TestDiagnosticsMetadataWALSnapshot(t *testing.T) {
	ctx := context.Background()
	s, path := openTestStore(t)
	if _, err := s.db.Exec(`PRAGMA wal_autocheckpoint = 0; PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		t.Fatal(err)
	}
	base := diagnosticFileBytes(t, path)
	d, err := OpenDiagnostics(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	initial, err := d.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	readTx, err := d.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer readTx.Rollback()
	// Pin the old snapshot before an independent writer grows the image and
	// changes its version atomically. Exercise the same transaction reader used
	// by Metadata; there are no timing assumptions or test hooks in production.
	var version int
	if err := readTx.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != schemaVersion {
		t.Fatal(version, err)
	}
	writeTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer writeTx.Rollback()
	if _, err := writeTx.Exec(`CREATE TABLE synthetic_padding(payload BLOB);
		INSERT INTO synthetic_padding VALUES (zeroblob(262144)); PRAGMA user_version = 8`); err != nil {
		t.Fatal(err)
	}
	if err := writeTx.Commit(); err != nil {
		t.Fatal(err)
	}
	pinned, err := readDiagnosticMetadata(ctx, readTx)
	if err != nil || pinned != initial {
		t.Fatal("read mixed snapshots after writer committed", pinned, initial, err)
	}
	if err := readTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if m, err := d.Metadata(ctx); m != (DiagnosticMetadata{}) || err != ErrDiagnosticsSchema {
		t.Fatal("next read accepted changed version or returned partial result", m, err)
	}
	if _, err := s.db.Exec(`PRAGMA user_version = 7`); err != nil {
		t.Fatal(err)
	}
	wal := diagnosticFileBytes(t, path+"-wal")
	current, err := d.Metadata(ctx)
	if err != nil || current.JournalMode != "wal" || current.PageCount <= initial.PageCount {
		t.Fatal("committed WAL growth invisible", current, initial, err)
	}
	if current.PageCount*current.PageSize <= int64(len(base)) {
		t.Fatal("logical image incorrectly reduced to main file size")
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(base, diagnosticFileBytes(t, path)) || !bytes.Equal(wal, diagnosticFileBytes(t, path+"-wal")) {
		t.Fatal("diagnostic modified main file/WAL or checkpointed writer")
	}
}

func TestDiagnosticsMetadataRevalidatesHistory(t *testing.T) {
	s, path := openTestStore(t)
	d, err := OpenDiagnostics(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version = 4`); err != nil {
		t.Fatal(err)
	}
	if m, err := d.Metadata(context.Background()); m != (DiagnosticMetadata{}) || err != ErrDiagnosticsSchema {
		t.Fatal("history change ignored or partial result returned", m, err)
	}
}

func TestDiagnosticsMetadataCancellationAndClosedHandle(t *testing.T) {
	_, path := openTestStore(t)
	d, err := OpenDiagnostics(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if m, err := d.Metadata(cancelled); m != (DiagnosticMetadata{}) || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled read returned data or lost cancellation", m, err)
	}
	// Cancellation while waiting for the single pooled connection must neither
	// return partial data nor strand that connection for subsequent reads.
	conn, err := d.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if m, err := d.Metadata(deadline); m != (DiagnosticMetadata{}) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("waiting read lost deadline", m, err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Metadata(context.Background()); err != nil {
		t.Fatal("cancelled operation stranded connection", err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if m, err := d.Metadata(context.Background()); m != (DiagnosticMetadata{}) || err != ErrDiagnosticsRead {
		t.Fatal("closed handle leaked error or partial result", m, err)
	}
}
