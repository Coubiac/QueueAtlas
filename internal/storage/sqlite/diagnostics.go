package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Diagnostics owns a read-only connection, separate from the writable Store.
// It exposes neither arbitrary SQL nor ingestion/migration methods.
type Diagnostics struct {
	db *sql.DB
}

var (
	// Errors deliberately omit file paths, stored values and driver messages.
	ErrDiagnosticsOpen   = errors.New("cannot open database for diagnostics")
	ErrDiagnosticsSchema = errors.New("database schema is not supported for diagnostics")
)

// OpenDiagnostics opens an existing local database without creating, migrating
// or changing its journal mode. It requires the current version and recorded
// migration history; this is not a full schema/integrity or authenticity check.
// Protect the parent directory as for Open. SQLite may create/use WAL sidecars
// and shared-memory locks even with mode=ro; this is not zero filesystem IO.
func OpenDiagnostics(ctx context.Context, path string) (*Diagnostics, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, ErrDiagnosticsOpen
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, ErrDiagnosticsOpen
	}
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		info, err := os.Stat(abs + suffix)
		if suffix != "" && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return nil, ErrDiagnosticsOpen
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
			return nil, ErrDiagnosticsOpen
		}
	}
	uriPath := filepath.ToSlash(abs)
	if filepath.VolumeName(abs) != "" {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("_busy_timeout", "5000")
	q.Set("_foreign_keys", "1")
	q.Set("_defensive", "1")
	q.Set("_dqs", "0")
	q.Set("_query_only", "1")
	q.Add("_pragma", "trusted_schema(OFF)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, ErrDiagnosticsOpen
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	d := &Diagnostics{db: db}
	if err := db.PingContext(ctx); err != nil {
		d.Close()
		return nil, diagnosticError(ctx, ErrDiagnosticsOpen)
	}
	if err := d.validateSchema(ctx); err != nil {
		d.Close()
		if errors.Is(err, ErrDiagnosticsSchema) {
			return nil, diagnosticError(ctx, ErrDiagnosticsSchema)
		}
		return nil, diagnosticError(ctx, ErrDiagnosticsOpen)
	}
	return d, nil
}

func (d *Diagnostics) Close() error { return d.db.Close() }

func diagnosticError(ctx context.Context, fallback error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fallback
}

func (d *Diagnostics) validateSchema(ctx context.Context) error {
	// A single snapshot avoids mixing version/history during another connection's
	// transaction. mode=ro enforces read-only even if a driver ignores ReadOnly.
	tx, err := d.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := validateDiagnosticSchema(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func validateDiagnosticSchema(ctx context.Context, tx *sql.Tx) error {
	var version int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version != schemaVersion {
		return ErrDiagnosticsSchema
	}
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT type FROM sqlite_master WHERE name = 'schema_migrations'`).Scan(&kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrDiagnosticsSchema
		}
		return err
	}
	if kind != "table" {
		return ErrDiagnosticsSchema
	}
	for required := 1; required <= schemaVersion; required++ {
		var recorded int
		if err := tx.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = ?`, required).Scan(&recorded); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrDiagnosticsSchema
			}
			return err
		}
	}
	return nil
}
