package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

var _ source.StateReader = (*Store)(nil)
var _ source.PathStateReader = (*Store)(nil)

// FileOrigins reads candidate origins and their optional checkpoints in one
// SQL statement, so a page cannot mix metadata and positions from different
// commits. The caller must verify fingerprints and anchors before resuming.
func (s *Store) FileOrigins(ctx context.Context, q source.OriginQuery) (source.OriginPage, error) {
	if q.SourceID == "" || q.Device == "" || q.Inode == "" {
		return source.OriginPage{}, errors.New("source ID, device and inode are required")
	}
	if q.Limit < 1 || q.Limit > source.MaxOriginPageSize {
		return source.OriginPage{}, fmt.Errorf("origin page limit must be between 1 and %d", source.MaxOriginPageSize)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT g.id, g.path, g.device, g.inode, g.fingerprint,
		g.first_seen_ns, g.follow_state, c.offset, c.anchor_hash
		FROM file_generations AS g LEFT JOIN checkpoints AS c
		ON c.source_id = g.source_id AND c.generation_id = g.id
		WHERE g.source_id = ? AND g.device = ? AND g.inode = ? AND g.id > ?
		ORDER BY g.id LIMIT ?`, q.SourceID, q.Device, q.Inode, q.AfterID, q.Limit+1)
	if err != nil {
		return source.OriginPage{}, err
	}
	return readOriginPage(rows, q.Limit)
}

// FileOriginsByPath reads exact stored path candidates and optional checkpoints
// in one statement. It uses the existing source/path index and writes no state.
// A result is persisted metadata, not a decision to resume a file generation.
func (s *Store) FileOriginsByPath(ctx context.Context, q source.OriginPathQuery) (source.OriginPage, error) {
	if q.SourceID == "" || q.Path == "" {
		return source.OriginPage{}, errors.New("source ID and path are required")
	}
	if q.Limit < 1 || q.Limit > source.MaxOriginPageSize {
		return source.OriginPage{}, fmt.Errorf("origin page limit must be between 1 and %d", source.MaxOriginPageSize)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT g.id, g.path, g.device, g.inode, g.fingerprint,
		g.first_seen_ns, g.follow_state, c.offset, c.anchor_hash
		FROM file_generations AS g LEFT JOIN checkpoints AS c
		ON c.source_id = g.source_id AND c.generation_id = g.id
		WHERE g.source_id = ? AND g.path = ? AND g.id > ?
		ORDER BY g.id LIMIT ?`, q.SourceID, q.Path, q.AfterID, q.Limit+1)
	if err != nil {
		return source.OriginPage{}, err
	}
	return readOriginPage(rows, q.Limit)
}

func readOriginPage(rows *sql.Rows, limit int) (source.OriginPage, error) {
	defer rows.Close()
	page := source.OriginPage{States: make([]source.OriginState, 0, limit)}
	for rows.Next() {
		var state source.OriginState
		var firstSeen int64
		var offset sql.NullInt64
		var anchor sql.NullString
		o := &state.Origin
		if err := rows.Scan(&o.ID, &o.Path, &o.Device, &o.Inode, &o.Fingerprint, &firstSeen, &state.FollowState, &offset, &anchor); err != nil {
			return source.OriginPage{}, err
		}
		if len(page.States) == limit {
			page.NextID = page.States[len(page.States)-1].Origin.ID
			continue
		}
		o.FirstSeen = time.Unix(0, firstSeen).UTC()
		if offset.Valid {
			state.Checkpoint = &source.Position{OriginID: o.ID, Offset: offset.Int64, AnchorHash: anchor.String}
		}
		page.States = append(page.States, state)
	}
	if err := rows.Err(); err != nil {
		return source.OriginPage{}, err
	}
	return page, nil
}
