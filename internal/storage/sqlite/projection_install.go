package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"sort"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
)

var (
	ErrProjectionStale   = errors.New("correlation projection input changed")
	ErrProjectionInstall = errors.New("correlation revision installation failed")
)

type projectionScopePart struct{ kind, instance, queue string }

// InstallProjection computes from the caller's complete scope snapshot, then
// rereads under a SQLite write lock before installing its revision manifest.
// Caller inputs must remain immutable during the call. All facts are referenced,
// including Other/Unresolved. No derived Projection is serialized. Replacement
// retains only the current manifest for this scope; other scopes are untouched.
// Future reads must check freshness again: later ingestion can add facts.
func (s *Store) InstallProjection(ctx context.Context, scope CorrelationScope, facts []correlation.Fact, limit int, opts correlation.LinkOptions) (correlation.Projection, error) {
	scope.Queues = append([]correlation.QueueKey(nil), scope.Queues...)
	scope.UnqueuedInstances = append([]string(nil), scope.UnqueuedInstances...)
	if _, _, err := correlationSelection(scope); err != nil {
		return correlation.Projection{}, err
	}
	projection, err := correlation.BuildProjection(facts, limit, opts)
	if err != nil {
		return correlation.Projection{}, err
	}
	parts, digest := canonicalProjectionScope(scope)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return correlation.Projection{}, projectionInstallError(ctx)
	}
	defer tx.Rollback()
	// Even with no matching row, this UPDATE obtains SQLite's write reservation.
	// Acquire it BEFORE reading: no deferred read-to-write upgrade race and no
	// second connection can commit new facts between revalidation and installation.
	if err := projectionWriteLock(ctx, tx); err != nil {
		return correlation.Projection{}, projectionInstallError(ctx)
	}
	current, ids, err := correlationFacts(ctx, tx, scope, limit)
	if err != nil {
		return correlation.Projection{}, err
	}
	verified, err := correlation.BuildProjection(current, limit, projection.LinkOptions)
	if err != nil {
		return correlation.Projection{}, err
	}
	if len(current) != len(facts) || !sameProjectionRefs(current, facts) || verified.Revision != projection.Revision {
		return correlation.Projection{}, ErrProjectionStale
	}
	fail := func() (correlation.Projection, error) { return correlation.Projection{}, projectionInstallError(ctx) }
	if _, err := tx.ExecContext(ctx, `INSERT INTO projection_scopes(scope_sha256, format_version) VALUES(?, 1) ON CONFLICT(scope_sha256) DO NOTHING`, digest); err != nil {
		return fail()
	}
	var scopeID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM projection_scopes WHERE scope_sha256 = ?`, digest).Scan(&scopeID); err != nil {
		return fail()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projection_scopes SET current_revision_id = NULL WHERE id = ?`, scopeID); err != nil {
		return fail()
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM projection_revisions WHERE scope_id = ?`, scopeID); err != nil {
		return fail()
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM projection_scope_parts WHERE scope_id = ?`, scopeID); err != nil {
		return fail()
	}
	for i, p := range parts {
		var queue any
		if p.kind == "queue" {
			queue = []byte(p.queue)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO projection_scope_parts(scope_id, ordinal, kind, instance, queue_id) VALUES(?, ?, ?, ?, ?)`, scopeID, i, p.kind, []byte(p.instance), queue); err != nil {
			return fail()
		}
	}
	var revisionID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO projection_revisions(scope_id, format_version, revision, input_revision, link_window_ns, fact_count, created_at_ns) VALUES(?, 1, ?, ?, ?, ?, ?) RETURNING id`,
		scopeID, projection.Revision, projection.InputRevision, int64(projection.LinkOptions.Window), len(ids), time.Now().UTC().UnixNano()).Scan(&revisionID); err != nil {
		return fail()
	}
	for i, b := range projection.LinkOptions.SMTPBindings {
		if _, err := tx.ExecContext(ctx, `INSERT INTO projection_revision_bindings(revision_id, ordinal, from_instance, relay, to_instance) VALUES(?, ?, ?, ?, ?)`, revisionID, i, []byte(b.FromInstance), []byte(b.Relay), []byte(b.ToInstance)); err != nil {
			return fail()
		}
	}
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO projection_revision_facts(revision_id, raw_record_id) VALUES(?, ?)`, revisionID, id); err != nil {
			return fail()
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projection_scopes SET current_revision_id = ? WHERE id = ?`, revisionID, scopeID); err != nil {
		return fail()
	}
	if err := tx.Commit(); err != nil {
		return fail()
	}
	return projection, nil
}

func projectionWriteLock(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE projection_scopes SET current_revision_id = current_revision_id WHERE id = -1`)
	return err
}

func projectionInstallError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrProjectionInstall
}

func sameProjectionRefs(a, b []correlation.Fact) bool {
	refs := make(map[correlation.FactRef]bool, len(a))
	for _, f := range a {
		refs[f.Ref] = true
	}
	for _, f := range b {
		if !refs[f.Ref] {
			return false
		}
	}
	return len(a) == len(b)
}

// Versioned length framing, binary ordering and kind distinguish scope sets.
// Parts and counts are included even if they currently select no facts.
func canonicalProjectionScope(scope CorrelationScope) ([]projectionScopePart, string) {
	var parts []projectionScopePart
	for _, q := range scope.Queues {
		parts = append(parts, projectionScopePart{"queue", q.Instance, q.QueueID})
	}
	for _, instance := range scope.UnqueuedInstances {
		parts = append(parts, projectionScopePart{"unqueued", instance, ""})
	}
	sort.Slice(parts, func(i, j int) bool {
		a, b := parts[i], parts[j]
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.instance != b.instance {
			return a.instance < b.instance
		}
		return a.queue < b.queue
	})
	h := sha256.New()
	projectionScopeFrame(h, "projection-scope-v1")
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(parts)))
	h.Write(count[:])
	for _, p := range parts {
		projectionScopeFrame(h, p.kind)
		projectionScopeFrame(h, p.instance)
		projectionScopeFrame(h, p.queue)
	}
	return parts, hex.EncodeToString(h.Sum(nil))
}

func projectionScopeFrame(h hash.Hash, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	h.Write(size[:])
	h.Write([]byte(value))
}
