package sqlite

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
)

const MaxRetentionPreview = 256

var (
	ErrRetentionQuery           = errors.New("invalid retention preview query")
	ErrRetentionStoredCandidate = errors.New("invalid stored retention candidate")
)

// RetentionQuery uses the stored event UTC hypothesis, not ingestion time.
// Before is exclusive. No policy default or clock is chosen by the Store.
type RetentionQuery struct {
	Instance string
	Before   time.Time
	Limit    int
}

type RetentionCandidate struct {
	Ref         correlation.FactRef
	At          time.Time
	TimeQuality model.TimeQuality
}

// RetentionPreview is read-only information from one SQL snapshot. Candidates
// are not deletion authorization, a stable plan, or proof of log coverage.
// More means another matching SQL row exists, not a total or a cursor.
type RetentionPreview struct {
	Candidates []RetentionCandidate
	More       bool
}

// PreviewRetention returns old dated facts covered by a nonempty-anchor
// checkpoint of their exact source/origin. Uncovered/undated facts stay absent.
// This does not establish that the origin is finished or has no pending retry.
// A future purge must reselect/revalidate under its own write transaction.
func (s *Store) PreviewRetention(ctx context.Context, query RetentionQuery) (RetentionPreview, error) {
	before, ok := searchTimeNS(query.Before)
	if !ok || query.Instance == "" || len(query.Instance) > 1024 || strings.ContainsAny(query.Instance, "\x00\r\n\t") || query.Limit < 1 || query.Limit > MaxRetentionPreview {
		return RetentionPreview{}, ErrRetentionQuery
	}
	rows, err := s.db.QueryContext(ctx, `SELECT e.time_utc_ns,e.time_quality,
		r.source_id,r.generation_id,r.start_offset,r.end_offset,c.offset,c.anchor_hash
		FROM events e JOIN raw_records r ON r.id=e.raw_record_id
		JOIN checkpoints c ON c.source_id=r.source_id AND c.generation_id=r.generation_id
		WHERE e.instance=? AND e.time_utc_ns < ? AND r.end_offset <= c.offset AND c.anchor_hash <> ''
		ORDER BY e.time_utc_ns,r.id LIMIT ?`, query.Instance, before, query.Limit+1)
	if err != nil {
		return RetentionPreview{}, err
	}
	defer rows.Close()
	var preview RetentionPreview
	for rows.Next() {
		if len(preview.Candidates) == query.Limit {
			preview.More = true
			break
		}
		var item RetentionCandidate
		var ns, offset int64
		var anchor string
		if err := rows.Scan(&ns, &item.TimeQuality, &item.Ref.SourceID, &item.Ref.OriginID, &item.Ref.Start, &item.Ref.End, &offset, &anchor); err != nil {
			if ctx.Err() != nil {
				return RetentionPreview{}, ctx.Err()
			}
			return RetentionPreview{}, ErrRetentionStoredCandidate
		}
		if ns >= before || item.Ref.SourceID == "" || item.Ref.OriginID == "" || item.Ref.Start < 0 || item.Ref.End <= item.Ref.Start || item.Ref.End > offset || anchor == "" || !retentionTimeQuality(item.TimeQuality) {
			return RetentionPreview{}, ErrRetentionStoredCandidate
		}
		item.At = time.Unix(0, ns).UTC()
		preview.Candidates = append(preview.Candidates, item)
	}
	if err := rows.Err(); err != nil {
		return RetentionPreview{}, err
	}
	return preview, nil
}

func retentionTimeQuality(quality model.TimeQuality) bool {
	switch quality {
	case model.TimeUnknown, model.TimeWallOnly, model.TimeYearWithoutZone,
		model.TimeConfiguredYearAndZone, model.TimeInferredYearAndZone, model.TimeExplicitOffset:
		return true
	}
	return false
}
