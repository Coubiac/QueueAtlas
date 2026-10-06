package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaVersion = 6

func preflight(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version < 0 {
		return fmt.Errorf("invalid negative database schema version")
	}
	if version > schemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, schemaVersion)
	}
	if version == 0 {
		var existing int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name NOT GLOB 'sqlite_*'`).Scan(&existing); err != nil {
			return err
		}
		if existing != 0 {
			return fmt.Errorf("unversioned nonempty SQLite database: refusing migration")
		}
	}
	return nil
}

const schemaV1 = `
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at_ns INTEGER NOT NULL
);
CREATE TABLE sources (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    trusted_host TEXT NOT NULL,
    created_at_ns INTEGER NOT NULL
);
CREATE TABLE file_generations (
    id TEXT PRIMARY KEY,
    source_id TEXT NOT NULL REFERENCES sources(id),
    path TEXT NOT NULL,
    device TEXT NOT NULL,
    inode TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    first_seen_ns INTEGER NOT NULL,
    UNIQUE(source_id, id)
);
CREATE INDEX file_generations_source_path ON file_generations(source_id, path);
CREATE TABLE checkpoints (
    source_id TEXT NOT NULL,
    generation_id TEXT NOT NULL,
    offset INTEGER NOT NULL CHECK(offset >= 0),
    anchor_hash TEXT NOT NULL,
    updated_at_ns INTEGER NOT NULL,
    PRIMARY KEY(source_id, generation_id),
    FOREIGN KEY(source_id, generation_id)
        REFERENCES file_generations(source_id, id)
);
CREATE TABLE raw_records (
    id INTEGER PRIMARY KEY,
    source_id TEXT NOT NULL,
    generation_id TEXT NOT NULL,
    start_offset INTEGER NOT NULL CHECK(start_offset >= 0),
    end_offset INTEGER NOT NULL CHECK(end_offset > start_offset),
    raw BLOB NOT NULL CHECK(length(raw) <= 65536),
    read_error TEXT NOT NULL,
    observed_at_ns INTEGER NOT NULL,
    UNIQUE(source_id, generation_id, start_offset),
    FOREIGN KEY(source_id, generation_id)
        REFERENCES file_generations(source_id, id)
);
CREATE INDEX raw_records_origin_end ON raw_records(source_id, generation_id, end_offset);
CREATE TABLE events (
    id INTEGER PRIMARY KEY,
    raw_record_id INTEGER NOT NULL UNIQUE REFERENCES raw_records(id) ON DELETE CASCADE,
    time_utc_ns INTEGER,
    time_raw TEXT NOT NULL,
    time_quality TEXT NOT NULL,
    time_year INTEGER NOT NULL,
    time_zone TEXT NOT NULL,
    host TEXT NOT NULL,
    instance TEXT NOT NULL,
    program TEXT NOT NULL,
    service TEXT NOT NULL,
    pid TEXT NOT NULL,
    kind TEXT NOT NULL,
    queue_id TEXT NOT NULL,
    no_queue INTEGER NOT NULL CHECK(no_queue IN (0, 1)),
    message TEXT NOT NULL,
    message_id TEXT,
    sender TEXT,
    recipient TEXT,
    status TEXT,
    fields_json TEXT NOT NULL CHECK(json_valid(fields_json)),
    parse_error TEXT NOT NULL
);
CREATE INDEX events_time ON events(time_utc_ns, id);
CREATE INDEX events_queue ON events(instance, queue_id, time_utc_ns, id) WHERE queue_id <> '';
CREATE INDEX events_message_id ON events(message_id, id) WHERE message_id IS NOT NULL;
CREATE INDEX events_sender ON events(sender, time_utc_ns, id) WHERE sender IS NOT NULL;
CREATE INDEX events_recipient ON events(recipient, time_utc_ns, id) WHERE recipient IS NOT NULL;
CREATE TABLE queue_instances (
    id INTEGER PRIMARY KEY,
    instance TEXT NOT NULL,
    queue_id TEXT NOT NULL,
    generation INTEGER NOT NULL CHECK(generation >= 0),
    first_seen_ns INTEGER NOT NULL,
    last_seen_ns INTEGER NOT NULL,
    closed_at_ns INTEGER,
    UNIQUE(instance, queue_id, generation)
);
CREATE TABLE journeys (
    id INTEGER PRIMARY KEY,
    overall_status TEXT NOT NULL,
    first_seen_ns INTEGER,
    last_seen_ns INTEGER
);
CREATE TABLE journey_queues (
    journey_id INTEGER NOT NULL REFERENCES journeys(id) ON DELETE CASCADE,
    queue_instance_id INTEGER NOT NULL UNIQUE REFERENCES queue_instances(id) ON DELETE CASCADE,
    PRIMARY KEY(journey_id, queue_instance_id)
);
CREATE TABLE queue_links (
    id INTEGER PRIMARY KEY,
    from_queue_id INTEGER NOT NULL REFERENCES queue_instances(id) ON DELETE CASCADE,
    to_queue_id INTEGER NOT NULL REFERENCES queue_instances(id) ON DELETE CASCADE,
    relation TEXT NOT NULL,
    evidence_event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    confidence TEXT NOT NULL CHECK(confidence IN ('candidate', 'confirmed')),
    CHECK(from_queue_id <> to_queue_id),
    UNIQUE(from_queue_id, to_queue_id, relation, evidence_event_id)
);
CREATE TABLE recipient_deliveries (
    id INTEGER PRIMARY KEY,
    queue_instance_id INTEGER NOT NULL REFERENCES queue_instances(id) ON DELETE CASCADE,
    recipient TEXT NOT NULL,
    original_recipient TEXT,
    status TEXT NOT NULL,
    last_seen_ns INTEGER
);
CREATE INDEX recipient_deliveries_address ON recipient_deliveries(recipient, id);
CREATE TABLE delivery_attempts (
    id INTEGER PRIMARY KEY,
    delivery_id INTEGER NOT NULL REFERENCES recipient_deliveries(id) ON DELETE CASCADE,
    event_id INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    time_utc_ns INTEGER,
    relay TEXT,
    dsn TEXT,
    smtp_reply TEXT,
    status TEXT NOT NULL,
    UNIQUE(delivery_id, event_id)
);
CREATE TABLE prequeue_attempts (
    id INTEGER PRIMARY KEY,
    event_id INTEGER NOT NULL UNIQUE REFERENCES events(id) ON DELETE CASCADE,
    session_key TEXT,
    recipient TEXT,
    smtp_code TEXT,
    reason TEXT
);
CREATE TABLE import_runs (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL,
    byte_size INTEGER CHECK(byte_size IS NULL OR byte_size >= 0),
    sha256 TEXT,
    status TEXT NOT NULL CHECK(status IN ('running', 'complete', 'failed')),
    last_offset INTEGER NOT NULL DEFAULT 0 CHECK(last_offset >= 0),
    created_at_ns INTEGER NOT NULL,
    completed_at_ns INTEGER
);
CREATE INDEX import_runs_hash ON import_runs(sha256) WHERE sha256 IS NOT NULL;
`

const schemaV2 = `ALTER TABLE file_generations ADD COLUMN follow_state INTEGER NOT NULL
	DEFAULT 0 CHECK(follow_state IN (0, 1, 2));`

// Preserve every v1 column and legacy row without inventing source association.
// Rebuild only import_runs to add its composite provenance FK and linked checks.
const schemaV3 = `
CREATE TABLE import_runs_v3 (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL,
    byte_size INTEGER CHECK(byte_size IS NULL OR byte_size >= 0),
    sha256 TEXT,
    status TEXT NOT NULL CHECK(status IN ('running', 'complete', 'failed')),
    last_offset INTEGER NOT NULL DEFAULT 0 CHECK(last_offset >= 0),
    created_at_ns INTEGER NOT NULL,
    completed_at_ns INTEGER,
    source_id TEXT REFERENCES sources(id),
    generation_id TEXT,
    trailing_partial INTEGER CHECK(trailing_partial IS NULL OR trailing_partial IN (0, 1)),
    FOREIGN KEY(source_id, generation_id) REFERENCES file_generations(source_id, id),
    CHECK (
      (source_id IS NULL AND generation_id IS NULL AND trailing_partial IS NULL)
      OR
      (source_id IS NOT NULL AND source_id <> '' AND id > 0 AND path <> ''
       AND ((status = 'running' AND completed_at_ns IS NULL)
            OR (status IN ('complete', 'failed') AND completed_at_ns IS NOT NULL
                AND completed_at_ns >= created_at_ns))
       AND (
         (generation_id IS NULL AND byte_size IS NULL AND sha256 IS NULL
          AND trailing_partial IS NULL AND last_offset = 0 AND status <> 'complete')
         OR
         (generation_id IS NOT NULL AND length(generation_id) > 0
          AND byte_size IS NOT NULL AND sha256 IS NOT NULL AND length(sha256) = 64
          AND length(CAST(sha256 AS BLOB)) = 64
          AND sha256 NOT GLOB '*[^0-9a-f]*' AND trailing_partial IS NOT NULL
          AND last_offset <= byte_size
          AND (status <> 'complete' OR (last_offset = byte_size AND trailing_partial = 0)))
       ))
    )
);
INSERT INTO import_runs_v3(id, path, byte_size, sha256, status, last_offset, created_at_ns, completed_at_ns)
    SELECT id, path, byte_size, sha256, status, last_offset, created_at_ns, completed_at_ns FROM import_runs;
DROP TABLE import_runs;
ALTER TABLE import_runs_v3 RENAME TO import_runs;
CREATE INDEX import_runs_hash ON import_runs(sha256) WHERE sha256 IS NOT NULL;
CREATE INDEX import_runs_source_origin ON import_runs(source_id, generation_id, id) WHERE source_id IS NOT NULL;
`

func migrate(ctx context.Context, db *sql.DB, nowNS int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version < 0 {
		return fmt.Errorf("invalid negative database schema version")
	}
	if version > schemaVersion {
		return fmt.Errorf("database schema version %d is newer than supported version %d", version, schemaVersion)
	}
	if version == 0 {
		var existing int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE name NOT GLOB 'sqlite_*'`).Scan(&existing); err != nil {
			return err
		}
		if existing != 0 {
			return fmt.Errorf("unversioned nonempty SQLite database: refusing migration")
		}
		if _, err := tx.ExecContext(ctx, schemaV1); err != nil {
			return fmt.Errorf("apply schema v1: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at_ns) VALUES(1, ?)`, nowNS); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 1`); err != nil {
			return err
		}
		version = 1
	}
	for required := 1; required <= version; required++ {
		var recorded int
		if err := tx.QueryRowContext(ctx, `SELECT version FROM schema_migrations WHERE version = ?`, required).Scan(&recorded); err != nil {
			return fmt.Errorf("invalid schema migration history: %w", err)
		}
	}
	if version < 2 {
		if _, err := tx.ExecContext(ctx, schemaV2); err != nil {
			return fmt.Errorf("apply schema v2: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at_ns) VALUES(2, ?)`, nowNS); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 2`); err != nil {
			return err
		}
	}
	if version < 3 {
		if _, err := tx.ExecContext(ctx, schemaV3); err != nil {
			return fmt.Errorf("apply schema v3: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at_ns) VALUES(3, ?)`, nowNS); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 3`); err != nil {
			return err
		}
	}
	if version < 4 {
		if _, err := tx.ExecContext(ctx, schemaV4); err != nil {
			return fmt.Errorf("apply schema v4: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at_ns) VALUES(4, ?)`, nowNS); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 4`); err != nil {
			return err
		}
	}
	if version < 5 {
		if _, err := tx.ExecContext(ctx, schemaV5); err != nil {
			return fmt.Errorf("apply schema v5: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at_ns) VALUES(5, ?)`, nowNS); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 5`); err != nil {
			return err
		}
	}
	if version < 6 {
		if _, err := tx.ExecContext(ctx, schemaV6); err != nil {
			return fmt.Errorf("apply schema v6: %w", err)
		}
		if err := backfillSearchDomains(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at_ns) VALUES(6, ?)`, nowNS); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `PRAGMA user_version = 6`); err != nil {
			return err
		}
	}
	return tx.Commit()
}
