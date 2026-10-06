package sqlite

import (
	"context"
	"strings"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
)

// PurgeResult describes a committed batch only. A failed transaction returns
// zero, even if it had already staged markers or invalidations.
type PurgeResult struct {
	Deleted              int
	InvalidatedRevisions int64
	More                 bool
}

// PurgeRetention reselects candidates under a write reservation. File origins
// must be retired; content imports must have only complete associated attempts
// at their final checkpoint. This is a storage policy, not log-coverage proof.
// Callers choose the explicit cutoff and supply a deadline. No clock/default,
// scheduling, physical erasure or global budget is inferred.
func (s *Store) PurgeRetention(ctx context.Context, query RetentionQuery) (PurgeResult, error) {
	before, ok := searchTimeNS(query.Before)
	if !ok || query.Instance == "" || len(query.Instance) > 1024 || strings.ContainsAny(query.Instance, "\x00\r\n\t") || query.Limit < 1 || query.Limit > MaxRetentionPreview {
		return PurgeResult{}, ErrRetentionQuery
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return PurgeResult{}, err
	}
	defer tx.Rollback()
	// Reserve the SQLite writer before reading facts or eligibility. A second
	// connection cannot change the checkpoint/state or install a manifest between
	// this selection and deletion. No row is updated by this statement.
	if _, err := tx.ExecContext(ctx, `UPDATE raw_records SET id=id WHERE 0`); err != nil {
		return PurgeResult{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.source_id,r.generation_id,r.start_offset,r.end_offset,e.time_utc_ns,e.time_quality,c.offset,c.anchor_hash
		FROM events e JOIN raw_records r ON r.id=e.raw_record_id
		JOIN checkpoints c ON c.source_id=r.source_id AND c.generation_id=r.generation_id
		JOIN file_generations g ON g.source_id=r.source_id AND g.id=r.generation_id
		JOIN sources s ON s.id=r.source_id
		WHERE e.instance=? AND e.time_utc_ns<? AND r.end_offset<=c.offset AND c.anchor_hash<>''
		AND ((s.kind='file' AND g.follow_state=2) OR (s.kind='import'
			AND EXISTS(SELECT 1 FROM import_runs i WHERE i.source_id=r.source_id AND i.generation_id=r.generation_id)
			AND NOT EXISTS(SELECT 1 FROM import_runs i WHERE i.source_id=r.source_id AND i.generation_id=r.generation_id
				AND (i.status<>'complete' OR i.last_offset<>i.byte_size OR i.trailing_partial<>0
					OR i.byte_size<>c.offset OR g.fingerprint<>'sha256:'||i.sha256 OR g.device<>'' OR g.inode<>'')) ))
		ORDER BY e.time_utc_ns,r.id LIMIT ?`, query.Instance, before, query.Limit+1)
	if err != nil {
		return PurgeResult{}, err
	}
	ids := make([]int64, 0, query.Limit)
	var result PurgeResult
	for rows.Next() {
		if len(ids) == query.Limit {
			result.More = true
			break
		}
		var id, ns, offset int64
		var ref correlation.FactRef
		var quality model.TimeQuality
		var anchor string
		if err := rows.Scan(&id, &ref.SourceID, &ref.OriginID, &ref.Start, &ref.End, &ns, &quality, &offset, &anchor); err != nil {
			rows.Close()
			if ctx.Err() != nil {
				return PurgeResult{}, ctx.Err()
			}
			return PurgeResult{}, ErrRetentionStoredCandidate
		}
		if id <= 0 || ref.SourceID == "" || ref.OriginID == "" || ref.Start < 0 || ref.End <= ref.Start || ref.End > offset || anchor == "" || ns >= before || !retentionTimeQuality(quality) {
			rows.Close()
			return PurgeResult{}, ErrRetentionStoredCandidate
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return PurgeResult{}, err
	}
	if err := rows.Close(); err != nil {
		return PurgeResult{}, err
	}
	if len(ids) > 0 {
		args := make([]any, len(ids))
		marks := make([]string, len(ids))
		for i, id := range ids {
			if err := rememberPurgedRecord(ctx, tx, id); err != nil {
				return PurgeResult{}, err
			}
			args[i], marks[i] = id, "?"
		}
		// Invalidate every revision referencing any selected physical fact,
		// including historical revisions and scopes spanning multiple queues.
		affected := `SELECT revision_id FROM projection_revision_facts WHERE raw_record_id IN (` + strings.Join(marks, ",") + `)`
		if _, err := tx.ExecContext(ctx, `UPDATE projection_scopes SET current_revision_id=NULL WHERE current_revision_id IN (`+affected+`)`, args...); err != nil {
			return PurgeResult{}, err
		}
		deleted, err := tx.ExecContext(ctx, `DELETE FROM projection_revisions WHERE id IN (`+affected+`)`, args...)
		if err != nil {
			return PurgeResult{}, err
		}
		result.InvalidatedRevisions, err = deleted.RowsAffected()
		if err != nil {
			return PurgeResult{}, err
		}
		deleted, err = tx.ExecContext(ctx, `DELETE FROM raw_records WHERE id IN (`+strings.Join(marks, ",")+`)`, args...)
		if err != nil {
			return PurgeResult{}, err
		}
		n, err := deleted.RowsAffected()
		if err != nil {
			return PurgeResult{}, err
		}
		if n != int64(len(ids)) {
			return PurgeResult{}, ErrPurgedRecordState
		}
		result.Deleted = len(ids)
	}
	if err := tx.Commit(); err != nil {
		return PurgeResult{}, err
	}
	return result, nil
}
