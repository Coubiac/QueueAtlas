package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

// DiagnosticMetadata describes the main database's logical pages at one read
// snapshot. It contains no paths, table names, identities or log content.
// PageCount includes the committed WAL view, not just the main file on disk.
type DiagnosticMetadata struct {
	SchemaVersion int
	SQLiteVersion string
	JournalMode   string
	PageSize      int64 // Bytes per page.
	PageCount     int64
	FreePageCount int64 // Freelist pages, not deleted records or free disk space.
}

var ErrDiagnosticsRead = errors.New("cannot read database diagnostic metadata")

// Metadata returns a fixed-size result with version/history and pages read in a
// single transaction. Errors return a zero result and no driver/stored message.
// No table counts, integrity scan, filesystem size or checkpoint is performed.
func (d *Diagnostics) Metadata(ctx context.Context) (DiagnosticMetadata, error) {
	if err := ctx.Err(); err != nil {
		return DiagnosticMetadata{}, err
	}
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return DiagnosticMetadata{}, diagnosticError(ctx, ErrDiagnosticsRead)
	}
	defer tx.Rollback()
	m, err := readDiagnosticMetadata(ctx, tx)
	if err != nil {
		if errors.Is(err, ErrDiagnosticsSchema) {
			return DiagnosticMetadata{}, diagnosticError(ctx, ErrDiagnosticsSchema)
		}
		return DiagnosticMetadata{}, diagnosticError(ctx, ErrDiagnosticsRead)
	}
	if err := tx.Commit(); err != nil {
		return DiagnosticMetadata{}, diagnosticError(ctx, ErrDiagnosticsRead)
	}
	return m, nil
}

func readDiagnosticMetadata(ctx context.Context, tx *sql.Tx) (DiagnosticMetadata, error) {
	if err := validateDiagnosticSchema(ctx, tx); err != nil {
		return DiagnosticMetadata{}, err
	}
	m := DiagnosticMetadata{SchemaVersion: schemaVersion}
	if err := tx.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&m.SQLiteVersion); err != nil {
		return DiagnosticMetadata{}, err
	}
	if err := tx.QueryRowContext(ctx, `PRAGMA main.journal_mode`).Scan(&m.JournalMode); err != nil {
		return DiagnosticMetadata{}, err
	}
	if err := tx.QueryRowContext(ctx, `PRAGMA main.page_size`).Scan(&m.PageSize); err != nil {
		return DiagnosticMetadata{}, err
	}
	if err := tx.QueryRowContext(ctx, `PRAGMA main.page_count`).Scan(&m.PageCount); err != nil {
		return DiagnosticMetadata{}, err
	}
	if err := tx.QueryRowContext(ctx, `PRAGMA main.freelist_count`).Scan(&m.FreePageCount); err != nil {
		return DiagnosticMetadata{}, err
	}
	// Bound the two strings and reject impossible page values instead of
	// returning partial or misleading metadata from an unreadable database.
	if len(m.SQLiteVersion) == 0 || len(m.SQLiteVersion) > 64 ||
		m.PageSize < 512 || m.PageSize > 65536 || m.PageSize&(m.PageSize-1) != 0 ||
		m.PageCount < 1 || m.FreePageCount < 0 || m.FreePageCount > m.PageCount {
		return DiagnosticMetadata{}, ErrDiagnosticsRead
	}
	switch m.JournalMode {
	case "delete", "truncate", "persist", "memory", "wal", "off":
	default:
		return DiagnosticMetadata{}, ErrDiagnosticsRead
	}
	return m, nil
}
