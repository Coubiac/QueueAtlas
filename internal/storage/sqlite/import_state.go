package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

var _ source.ImportStateReader = (*Store)(nil)

var ErrImportState = errors.New("invalid persisted import state")

// ImportRun reads one associated attempt and its content origin in one SQL
// statement. Unassociated legacy rows and other sources are never adopted.
func (s *Store) ImportRun(ctx context.Context, sourceID string, runID int64) (source.ImportRun, bool, error) {
	if err := ctx.Err(); err != nil {
		return source.ImportRun{}, false, err
	}
	if sourceID == "" || runID <= 0 {
		return source.ImportRun{}, false, errors.New("import source ID and positive run ID are required")
	}
	var r source.ImportRun
	var created int64
	var completed, bytes, partial sql.NullInt64
	var originID, digest, fingerprint, device, inode sql.NullString
	var kind string
	err := s.db.QueryRowContext(ctx, `SELECT r.id, r.path, r.status, r.last_offset,
		r.created_at_ns, r.completed_at_ns, r.generation_id, r.byte_size, r.sha256,
		r.trailing_partial, s.kind, g.fingerprint, g.device, g.inode
		FROM import_runs AS r JOIN sources AS s ON s.id = r.source_id
		LEFT JOIN file_generations AS g ON g.source_id = r.source_id AND g.id = r.generation_id
		WHERE r.source_id = ? AND r.id = ?`, sourceID, runID).Scan(
		&r.ID, &r.Path, &r.Status, &r.LastOffset, &created, &completed, &originID, &bytes,
		&digest, &partial, &kind, &fingerprint, &device, &inode)
	if errors.Is(err, sql.ErrNoRows) {
		return source.ImportRun{}, false, nil
	}
	if err != nil {
		return source.ImportRun{}, false, err
	}
	r.SourceID, r.CreatedAt = sourceID, time.Unix(0, created).UTC()
	if completed.Valid {
		stamp := time.Unix(0, completed.Int64).UTC()
		r.CompletedAt = &stamp
	}
	if kind != "import" {
		return source.ImportRun{}, false, ErrImportState
	}
	if originID.Valid {
		id, err := source.ImportOriginID(sourceID, digest.String)
		if err != nil || id != originID.String || !bytes.Valid || !partial.Valid ||
			!fingerprint.Valid || fingerprint.String != "sha256:"+digest.String ||
			!device.Valid || device.String != "" || !inode.Valid || inode.String != "" {
			return source.ImportRun{}, false, ErrImportState
		}
		r.Content = &source.ImportContent{OriginID: id, Bytes: bytes.Int64,
			SHA256: digest.String, TrailingPartial: partial.Int64 == 1}
	} else if bytes.Valid || digest.Valid || partial.Valid {
		return source.ImportRun{}, false, ErrImportState
	}
	if err := validateImportRun(r); err != nil {
		return source.ImportRun{}, false, err
	}
	return r, true, nil
}

func validateImportRun(r source.ImportRun) error {
	if r.ID <= 0 || r.SourceID == "" || r.Path == "" || r.LastOffset < 0 {
		return ErrImportState
	}
	if r.Content == nil {
		if r.LastOffset != 0 || r.Status == source.ImportComplete {
			return ErrImportState
		}
	} else {
		id, err := source.ImportOriginID(r.SourceID, r.Content.SHA256)
		if err != nil || id != r.Content.OriginID || r.Content.Bytes < 0 || r.LastOffset > r.Content.Bytes {
			return ErrImportState
		}
		if r.Status == source.ImportComplete && (r.Content.TrailingPartial || r.LastOffset != r.Content.Bytes) {
			return ErrImportState
		}
	}
	switch r.Status {
	case source.ImportRunning:
		if r.CompletedAt != nil {
			return ErrImportState
		}
	case source.ImportComplete, source.ImportFailed:
		if r.CompletedAt == nil || r.CompletedAt.Before(r.CreatedAt) {
			return ErrImportState
		}
	default:
		return ErrImportState
	}
	return nil
}
