// Package sqlite persists immutable log observations and their durable read
// positions in one transaction. Correlation tables are projections over them.
package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// ErrFollowStateConflict covers missing/foreign origins and stale expected state.
// It contains no stored identity or log content.
var ErrFollowStateConflict = errors.New("file generation follow state conflict")

// Open opens a local database. The caller must protect its parent directory;
// newly created database files get owner-only permissions where supported.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(abs); err == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("database input is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open database file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Join(errors.New("database input is not a regular file"), f.Close())
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		f.Close()
		return nil, fmt.Errorf("database file %q is accessible by group or others", abs)
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	// SQLite may reuse existing sidecars without reducing their permissions.
	// Check them before connecting, since they can contain log data too. The
	// protected parent remains required; these metadata observations are not locks.
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		info, err := os.Stat(abs + suffix)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, errors.New("database sidecar is not a regular file")
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
			return nil, errors.New("database sidecar is accessible by group or others")
		}
	}
	uriPath := filepath.ToSlash(abs)
	if filepath.VolumeName(abs) != "" {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := url.Values{}
	q.Set("_busy_timeout", "5000")
	q.Set("_foreign_keys", "1")
	q.Set("_synchronous", "FULL")
	q.Set("_defensive", "1")
	q.Set("_dqs", "0")
	q.Add("_pragma", "trusted_schema(OFF)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	// One physical connection keeps all operations serialized and makes the
	// connection-scoped security PRAGMAs straightforward to reason about.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open SQLite connection: %w", err)
	}
	// Check compatibility before changing a newer or foreign database's
	// persistent journal mode. WAL is set once; SQLite retains it on reopen.
	if err := preflight(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	var journalMode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode = WAL`).Scan(&journalMode); err != nil {
		db.Close()
		return nil, err
	}
	if journalMode != "wal" {
		db.Close()
		return nil, fmt.Errorf("SQLite refused WAL mode: %s", journalMode)
	}
	if err := migrate(ctx, db, time.Now().UTC().UnixNano()); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Checkpoint returns a committed position. An absent position is distinct from
// a committed offset zero; it supplies no evidence for an automatic restart.
// The source must apply its explicit generation and resume policy.
func (s *Store) Checkpoint(ctx context.Context, sourceID, originID string) (source.Position, bool, error) {
	var p source.Position
	err := s.db.QueryRowContext(ctx, `SELECT offset, anchor_hash FROM checkpoints WHERE source_id = ? AND generation_id = ?`, sourceID, originID).Scan(&p.Offset, &p.AnchorHash)
	if errors.Is(err, sql.ErrNoRows) {
		return source.Position{}, false, nil
	}
	if err != nil {
		return source.Position{}, false, err
	}
	p.OriginID = originID
	return p, true, nil
}

// Commit is idempotent by physical provenance. A duplicate position with
// changed bytes is an error rather than an invisible replacement.
func (s *Store) Commit(ctx context.Context, batch source.Batch) error {
	if err := validate(batch); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().UnixNano()
	result, err := tx.ExecContext(ctx, `INSERT INTO sources(id, kind, name, trusted_host, created_at_ns)
		VALUES(?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET name = excluded.name
		WHERE sources.kind = excluded.kind AND sources.trusted_host = excluded.trusted_host`,
		batch.Source.ID, batch.Source.Kind, batch.Source.Name, batch.Source.TrustedHost, now)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("source %q changed immutable kind or trusted host", batch.Source.ID)
	}
	for _, origin := range batch.Origins {
		result, err := tx.ExecContext(ctx, `INSERT INTO file_generations(id, source_id, path, device, inode, fingerprint, first_seen_ns)
			VALUES(?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET path = excluded.path
			WHERE file_generations.source_id = excluded.source_id
			  AND file_generations.device = excluded.device
			  AND file_generations.inode = excluded.inode
			  AND file_generations.fingerprint = excluded.fingerprint`,
			origin.ID, batch.Source.ID, origin.Path, origin.Device, origin.Inode, origin.Fingerprint, origin.FirstSeen.UTC().UnixNano())
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return fmt.Errorf("origin %q changed immutable identity", origin.ID)
		}
	}
	for _, transition := range batch.FollowTransitions {
		result, err := tx.ExecContext(ctx, `UPDATE file_generations SET follow_state = ?
			WHERE source_id = ? AND id = ? AND follow_state IN (?, ?)`,
			transition.To, batch.Source.ID, transition.OriginID, transition.From, transition.To)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrFollowStateConflict
		}
	}
	if batch.ImportChange != nil {
		if err := prepareImportProgress(ctx, tx, batch); err != nil {
			return err
		}
	}
	for _, record := range batch.Records {
		result, err := tx.ExecContext(ctx, `INSERT INTO raw_records(source_id, generation_id, start_offset, end_offset, raw, read_error, observed_at_ns)
			VALUES(?, ?, ?, ?, ?, ?, ?) ON CONFLICT(source_id, generation_id, start_offset) DO NOTHING`,
			batch.Source.ID, record.OriginID, record.Start, record.End, record.Raw, record.Error, record.ReadAt.UTC().UnixNano())
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			var priorEnd int64
			var priorRaw []byte
			var priorError string
			if err := tx.QueryRowContext(ctx, `SELECT end_offset, raw, read_error FROM raw_records
				WHERE source_id = ? AND generation_id = ? AND start_offset = ?`,
				batch.Source.ID, record.OriginID, record.Start).Scan(&priorEnd, &priorRaw, &priorError); err != nil {
				return err
			}
			if priorEnd != record.End || !bytes.Equal(priorRaw, record.Raw) || priorError != record.Error {
				return fmt.Errorf("provenance collision at %s:%d", record.OriginID, record.Start)
			}
			continue
		}
		id, err := result.LastInsertId()
		if err != nil {
			return err
		}
		instance := batch.Source.TrustedHost
		if instance == "" {
			instance = batch.Source.ID
		}
		if err := insertEvent(ctx, tx, id, instance, record.Observation); err != nil {
			return err
		}
	}
	for _, position := range batch.Checkpoints {
		_, err := tx.ExecContext(ctx, `INSERT INTO checkpoints(source_id, generation_id, offset, anchor_hash, updated_at_ns)
			VALUES(?, ?, ?, ?, ?) ON CONFLICT(source_id, generation_id) DO UPDATE SET
			offset = excluded.offset, anchor_hash = excluded.anchor_hash, updated_at_ns = excluded.updated_at_ns
			WHERE excluded.offset > checkpoints.offset`,
			batch.Source.ID, position.OriginID, position.Offset, position.AnchorHash, now)
		if err != nil {
			return err
		}
	}
	if batch.ImportChange != nil {
		if err := writeImportChange(ctx, tx, batch); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func validate(batch source.Batch) error {
	if batch.Source.ID == "" || batch.Source.Kind == "" || batch.Source.Name == "" {
		return errors.New("source ID, kind and name are required")
	}
	for _, origin := range batch.Origins {
		if origin.ID == "" || origin.Path == "" || origin.Fingerprint == "" {
			return errors.New("origin ID, path and fingerprint are required")
		}
	}
	if len(batch.FollowTransitions) > 0 && batch.Source.Kind != "file" {
		return errors.New("follow transitions require a file source")
	}
	seen := make(map[string]bool, len(batch.FollowTransitions))
	for _, transition := range batch.FollowTransitions {
		valid := transition.From == source.FollowUnknown && transition.To == source.FollowFollowing ||
			transition.From == source.FollowFollowing && transition.To == source.FollowRetired ||
			transition.From == source.FollowRetired && transition.To == source.FollowFollowing
		if transition.OriginID == "" || !valid || seen[transition.OriginID] {
			return errors.New("invalid or duplicate follow transition")
		}
		seen[transition.OriginID] = true
	}
	for _, record := range batch.Records {
		if record.OriginID == "" || record.Start < 0 || record.End <= record.Start || len(record.Raw) == 0 || len(record.Raw) > model.MaxLineBytes || int64(len(record.Raw)) > record.End-record.Start {
			return fmt.Errorf("invalid record at origin %q offset %d", record.OriginID, record.Start)
		}
		if record.Observation.SourceID != "" && record.Observation.SourceID != batch.Source.ID {
			return errors.New("observation source differs from batch source")
		}
	}
	for _, position := range batch.Checkpoints {
		if position.OriginID == "" || position.Offset < 0 {
			return errors.New("invalid checkpoint")
		}
	}
	return validateImportChange(batch)
}

func insertEvent(ctx context.Context, tx *sql.Tx, recordID int64, instance string, o model.Observation) error {
	fields, err := json.Marshal(struct {
		Fields  map[string]string `json:"fields"`
		Present map[string]bool   `json:"present"`
	}{o.Fields, o.Present})
	if err != nil {
		return err
	}
	var utcNS any
	if o.Timestamp.Value != nil {
		ns := o.Timestamp.Value.UTC().UnixNano()
		// Valid RFC5424 dates can lie outside int64 nanoseconds. Preserve their
		// raw date/year/zone/quality, but never index a wrapped, invented instant.
		if time.Unix(0, ns).Equal(*o.Timestamp.Value) {
			utcNS = ns
		}
	}
	quality := o.Timestamp.Quality
	if quality == "" {
		quality = model.TimeUnknown
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO events(raw_record_id, time_utc_ns, time_raw, time_quality, time_year, time_zone,
		host, instance, program, service, pid, kind, queue_id, no_queue, message,
		message_id, sender, recipient, status, fields_json, parse_error)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		recordID, utcNS, o.Timestamp.Raw, quality, o.Timestamp.Year, o.Timestamp.Zone, o.Host, instance, o.Program,
		o.Service, o.PID, o.Kind, o.QueueID, o.NoQueue, o.Message,
		fieldOrNull(o, "message-id"), fieldOrNull(o, "from"), fieldOrNull(o, "to"), fieldOrNull(o, "status"),
		string(fields), o.ParseError)
	if err != nil {
		return err
	}
	eventID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	return insertSearchDomains(ctx, tx, eventID, instance, utcNS, fieldOrNull(o, "from"), fieldOrNull(o, "to"))
}

func fieldOrNull(o model.Observation, key string) any {
	if value, ok := o.Field(key); ok {
		return value
	}
	return nil
}
