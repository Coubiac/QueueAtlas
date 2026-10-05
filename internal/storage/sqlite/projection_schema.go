package sqlite

// Revision manifests preserve exact binary scope/options and every input fact.
// No derived Projection is serialized here. Referenced raw records AND events
// must be invalidated explicitly before retention can delete or mutate them.
const schemaV4 = `
CREATE TABLE projection_scopes (
    id INTEGER PRIMARY KEY CHECK(id > 0),
    scope_sha256 TEXT NOT NULL UNIQUE CHECK(
      typeof(scope_sha256) = 'text' AND length(scope_sha256) = 64
      AND length(CAST(scope_sha256 AS BLOB)) = 64 AND scope_sha256 NOT GLOB '*[^0-9a-f]*'),
    format_version INTEGER NOT NULL CHECK(format_version = 1),
    current_revision_id INTEGER,
    FOREIGN KEY(id, current_revision_id) REFERENCES projection_revisions(scope_id, id) ON DELETE RESTRICT
);
CREATE TABLE projection_scope_parts (
    scope_id INTEGER NOT NULL REFERENCES projection_scopes(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK(typeof(ordinal) = 'integer' AND ordinal BETWEEN 0 AND 63),
    kind TEXT NOT NULL CHECK(kind IN ('queue', 'unqueued')),
    instance BLOB NOT NULL CHECK(typeof(instance) = 'blob' AND length(instance) BETWEEN 1 AND 1024
      AND instr(instance, X'00') = 0 AND instr(instance, X'09') = 0
      AND instr(instance, X'0a') = 0 AND instr(instance, X'0d') = 0),
    queue_id BLOB,
    PRIMARY KEY(scope_id, ordinal),
    CHECK((kind = 'unqueued' AND queue_id IS NULL) OR
      (kind = 'queue' AND typeof(queue_id) = 'blob' AND length(queue_id) BETWEEN 1 AND 32
       AND instr(queue_id, X'00') = 0 AND instr(queue_id, X'09') = 0
       AND instr(queue_id, X'0a') = 0 AND instr(queue_id, X'0d') = 0))
);
CREATE UNIQUE INDEX projection_scope_queues ON projection_scope_parts(scope_id, instance, queue_id) WHERE kind = 'queue';
CREATE UNIQUE INDEX projection_scope_unqueued ON projection_scope_parts(scope_id, instance) WHERE kind = 'unqueued';
CREATE TABLE projection_revisions (
    id INTEGER PRIMARY KEY CHECK(id > 0),
    scope_id INTEGER NOT NULL REFERENCES projection_scopes(id) ON DELETE CASCADE,
    format_version INTEGER NOT NULL CHECK(format_version = 1),
    revision TEXT NOT NULL CHECK(typeof(revision) = 'text' AND length(revision) = 64
      AND length(CAST(revision AS BLOB)) = 64 AND revision NOT GLOB '*[^0-9a-f]*'),
    input_revision TEXT NOT NULL CHECK(typeof(input_revision) = 'text' AND length(input_revision) = 64
      AND length(CAST(input_revision AS BLOB)) = 64 AND input_revision NOT GLOB '*[^0-9a-f]*'),
    link_window_ns INTEGER NOT NULL CHECK(typeof(link_window_ns) = 'integer' AND link_window_ns BETWEEN 1 AND 86400000000000),
    fact_count INTEGER NOT NULL CHECK(typeof(fact_count) = 'integer' AND fact_count BETWEEN 0 AND 4096),
    created_at_ns INTEGER NOT NULL CHECK(typeof(created_at_ns) = 'integer'),
    UNIQUE(scope_id, id),
    UNIQUE(scope_id, revision)
);
CREATE TABLE projection_revision_bindings (
    revision_id INTEGER NOT NULL REFERENCES projection_revisions(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK(typeof(ordinal) = 'integer' AND ordinal BETWEEN 0 AND 63),
    from_instance BLOB NOT NULL CHECK(typeof(from_instance) = 'blob' AND length(from_instance) BETWEEN 1 AND 1024
      AND instr(from_instance, X'00') = 0 AND instr(from_instance, X'09') = 0 AND instr(from_instance, X'0a') = 0 AND instr(from_instance, X'0d') = 0),
    relay BLOB NOT NULL CHECK(typeof(relay) = 'blob' AND length(relay) BETWEEN 1 AND 1024
      AND instr(relay, X'00') = 0 AND instr(relay, X'09') = 0 AND instr(relay, X'0a') = 0 AND instr(relay, X'0d') = 0),
    to_instance BLOB NOT NULL CHECK(typeof(to_instance) = 'blob' AND length(to_instance) BETWEEN 1 AND 1024
      AND instr(to_instance, X'00') = 0 AND instr(to_instance, X'09') = 0 AND instr(to_instance, X'0a') = 0 AND instr(to_instance, X'0d') = 0),
    PRIMARY KEY(revision_id, ordinal),
    UNIQUE(revision_id, from_instance, relay)
);
CREATE TABLE projection_revision_facts (
    revision_id INTEGER NOT NULL REFERENCES projection_revisions(id) ON DELETE CASCADE,
    raw_record_id INTEGER NOT NULL,
    PRIMARY KEY(revision_id, raw_record_id),
    FOREIGN KEY(raw_record_id) REFERENCES raw_records(id) ON DELETE RESTRICT,
    FOREIGN KEY(raw_record_id) REFERENCES events(raw_record_id) ON DELETE RESTRICT
);
CREATE INDEX projection_facts_record ON projection_revision_facts(raw_record_id, revision_id);
CREATE INDEX projection_scopes_current ON projection_scopes(current_revision_id);
CREATE TRIGGER projection_preserve_raw BEFORE UPDATE ON raw_records
  WHEN EXISTS(SELECT 1 FROM projection_revision_facts WHERE raw_record_id = OLD.id)
  BEGIN SELECT RAISE(ABORT, 'referenced projection fact is immutable'); END;
CREATE TRIGGER projection_preserve_event BEFORE UPDATE ON events
  WHEN EXISTS(SELECT 1 FROM projection_revision_facts WHERE raw_record_id = OLD.raw_record_id)
  BEGIN SELECT RAISE(ABORT, 'referenced projection fact is immutable'); END;
`
