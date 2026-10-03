package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

var _ source.StateReader = (*Store)(nil)

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
		g.first_seen_ns, c.offset, c.anchor_hash
		FROM file_generations AS g LEFT JOIN checkpoints AS c
		ON c.source_id = g.source_id AND c.generation_id = g.id
		WHERE g.source_id = ? AND g.device = ? AND g.inode = ? AND g.id > ?
		ORDER BY g.id LIMIT ?`, q.SourceID, q.Device, q.Inode, q.AfterID, q.Limit+1)
	if err != nil {
		return source.OriginPage{}, err
	}
	defer rows.Close()
	page := source.OriginPage{States: make([]source.OriginState, 0, q.Limit)}
	for rows.Next() {
		var state source.OriginState
		var firstSeen int64
		var offset sql.NullInt64
		var anchor sql.NullString
		o := &state.Origin
		if err := rows.Scan(&o.ID, &o.Path, &o.Device, &o.Inode, &o.Fingerprint, &firstSeen, &offset, &anchor); err != nil {
			return source.OriginPage{}, err
		}
		if len(page.States) == q.Limit {
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
