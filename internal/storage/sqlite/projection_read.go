package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
)

var ErrProjectionStoredManifest = errors.New("invalid stored correlation revision manifest")

// CurrentProjection reconstructs only a verified current manifest. All reads
// share one transaction snapshot. A later commit can make this revision stale;
// the result certifies neither future freshness nor log coverage. Missing or
// explicitly invalidated current manifests return found=false, err=nil. Stale
// or malformed ones return no partial result and are never silently recalculated.
func (s *Store) CurrentProjection(ctx context.Context, scope CorrelationScope, limit int) (correlation.Projection, bool, error) {
	if limit < 1 || limit > correlation.MaxPartitionFacts {
		return correlation.Projection{}, false, correlation.ErrPartitionLimit
	}
	scope.Queues = append([]correlation.QueueKey(nil), scope.Queues...)
	scope.UnqueuedInstances = append([]string(nil), scope.UnqueuedInstances...)
	if _, _, err := correlationSelection(scope); err != nil {
		return correlation.Projection{}, false, err
	}
	parts, digest := canonicalProjectionScope(scope)
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return correlation.Projection{}, false, err
	}
	defer tx.Rollback()
	fail := func(err error) (correlation.Projection, bool, error) { return correlation.Projection{}, false, err }
	invalid := func() (correlation.Projection, bool, error) { return fail(projectionManifestError(ctx)) }
	var scopeID int64
	var revisionID sql.NullInt64
	var format int
	err = tx.QueryRowContext(ctx, `SELECT id, current_revision_id, format_version FROM projection_scopes WHERE scope_sha256 = ?`, digest).Scan(&scopeID, &revisionID, &format)
	if errors.Is(err, sql.ErrNoRows) {
		return correlation.Projection{}, false, nil
	}
	if err != nil || format != 1 || scopeID <= 0 {
		return invalid()
	}
	if !revisionID.Valid {
		return correlation.Projection{}, false, nil
	}
	var revision, input string
	var window int64
	var factCount int
	err = tx.QueryRowContext(ctx, `SELECT format_version, revision, input_revision, link_window_ns, fact_count FROM projection_revisions WHERE id = ? AND scope_id = ?`, revisionID.Int64, scopeID).Scan(&format, &revision, &input, &window, &factCount)
	if err != nil || format != 1 || revisionID.Int64 <= 0 || factCount < 0 || factCount > correlation.MaxPartitionFacts {
		return invalid()
	}
	if factCount > limit {
		return fail(correlation.ErrPartitionLimit)
	}
	if err := readProjectionScope(ctx, tx, scopeID, parts); err != nil {
		return fail(err)
	}
	opts, err := readProjectionBindings(ctx, tx, revisionID.Int64, window)
	if err != nil {
		return fail(err)
	}
	members, err := readProjectionMembers(ctx, tx, revisionID.Int64, factCount)
	if err != nil {
		return fail(err)
	}
	facts, ids, err := correlationFacts(ctx, tx, scope, limit)
	if err != nil {
		return fail(err)
	}
	if len(ids) != len(members) {
		return fail(ErrProjectionStale)
	}
	for _, id := range ids {
		if !members[id] {
			return fail(ErrProjectionStale)
		}
	}
	projection, err := correlation.BuildProjection(facts, limit, opts)
	if err != nil {
		// Inputs were checked by the same reader as106. Invalid persisted options
		// or manifest-internal inconsistencies must not expose stored values.
		return invalid()
	}
	if projection.InputRevision != input {
		return fail(ErrProjectionStale)
	}
	if projection.Revision != revision {
		return invalid()
	}
	if err := tx.Commit(); err != nil {
		return fail(err)
	}
	return projection, true, nil
}

func projectionManifestError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrProjectionStoredManifest
}

func readProjectionScope(ctx context.Context, tx *sql.Tx, scopeID int64, want []projectionScopePart) error {
	rows, err := tx.QueryContext(ctx, `SELECT ordinal, kind, instance, queue_id FROM projection_scope_parts WHERE scope_id = ? ORDER BY ordinal LIMIT 65`, scopeID)
	if err != nil {
		return projectionManifestError(ctx)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var ordinal int
		var kind string
		var instance, queue []byte
		if i >= len(want) || rows.Scan(&ordinal, &kind, &instance, &queue) != nil {
			return projectionManifestError(ctx)
		}
		p := want[i]
		if ordinal != i || kind != p.kind || string(instance) != p.instance || string(queue) != p.queue || (kind == "unqueued" && queue != nil) {
			return projectionManifestError(ctx)
		}
		i++
	}
	if rows.Err() != nil || i != len(want) {
		return projectionManifestError(ctx)
	}
	return nil
}

func readProjectionBindings(ctx context.Context, tx *sql.Tx, revisionID, window int64) (correlation.LinkOptions, error) {
	opts := correlation.LinkOptions{Window: time.Duration(window)}
	rows, err := tx.QueryContext(ctx, `SELECT ordinal, from_instance, relay, to_instance FROM projection_revision_bindings WHERE revision_id = ? ORDER BY ordinal LIMIT 65`, revisionID)
	if err != nil {
		return correlation.LinkOptions{}, projectionManifestError(ctx)
	}
	defer rows.Close()
	for rows.Next() {
		var ordinal int
		var from, relay, to []byte
		i := len(opts.SMTPBindings)
		if i >= correlation.MaxSMTPBindings || rows.Scan(&ordinal, &from, &relay, &to) != nil || ordinal != i {
			return correlation.LinkOptions{}, projectionManifestError(ctx)
		}
		b := correlation.SMTPBinding{FromInstance: string(from), Relay: string(relay), ToInstance: string(to)}
		if i > 0 {
			previous := opts.SMTPBindings[i-1]
			if previous.FromInstance > b.FromInstance || (previous.FromInstance == b.FromInstance && previous.Relay >= b.Relay) {
				return correlation.LinkOptions{}, projectionManifestError(ctx)
			}
		}
		opts.SMTPBindings = append(opts.SMTPBindings, b)
	}
	if rows.Err() != nil {
		return correlation.LinkOptions{}, projectionManifestError(ctx)
	}
	return opts, nil
}

func readProjectionMembers(ctx context.Context, tx *sql.Tx, revisionID int64, count int) (map[int64]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT raw_record_id FROM projection_revision_facts WHERE revision_id = ? LIMIT ?`, revisionID, count+1)
	if err != nil {
		return nil, projectionManifestError(ctx)
	}
	defer rows.Close()
	members := make(map[int64]bool, count)
	for rows.Next() {
		var id int64
		if len(members) == count || rows.Scan(&id) != nil || members[id] {
			return nil, projectionManifestError(ctx)
		}
		members[id] = true
	}
	if rows.Err() != nil || len(members) != count {
		return nil, projectionManifestError(ctx)
	}
	return members, nil
}
