package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

var ErrImportConflict = errors.New("import run state conflict")

func validateImportChange(b source.Batch) error {
	if b.ImportChange == nil {
		if b.Source.Kind == "import" && (len(b.Records) != 0 || len(b.Checkpoints) != 0) {
			return errors.New("import records and checkpoints require an explicit manifest change")
		}
		return nil
	}
	c := b.ImportChange
	if b.Source.Kind != "import" || c.Target.SourceID != b.Source.ID {
		return ErrImportState
	}
	if err := validateImportRun(c.Target); err != nil {
		return err
	}
	if c.Before == nil {
		if c.Target.Status != source.ImportRunning {
			return ErrImportState
		}
	} else {
		p := c.Before
		if err := validateImportRun(*p); err != nil {
			return err
		}
		if p.Status != source.ImportRunning || p.ID != c.Target.ID || p.SourceID != c.Target.SourceID ||
			p.Path != c.Target.Path || !p.CreatedAt.Equal(c.Target.CreatedAt) || c.Target.LastOffset < p.LastOffset {
			return ErrImportState
		}
		if p.Content != nil && (c.Target.Content == nil || *p.Content != *c.Target.Content) {
			return ErrImportState
		}
	}
	content := c.Target.Content
	if content == nil {
		if len(b.Origins) != 0 || len(b.Records) != 0 || len(b.Checkpoints) != 0 {
			return ErrImportState
		}
		return nil
	}
	if len(b.Origins) > 1 || len(b.Checkpoints) > 1 {
		return ErrImportState
	}
	for _, o := range b.Origins {
		if o.ID != content.OriginID || o.Path != c.Target.Path || o.Fingerprint != "sha256:"+content.SHA256 || o.Device != "" || o.Inode != "" {
			return ErrImportState
		}
	}
	for _, p := range b.Checkpoints {
		if p.OriginID != content.OriginID || p.Offset != c.Target.LastOffset || p.AnchorHash == "" {
			return ErrImportState
		}
	}
	if c.Before == nil || c.Before.Content == nil {
		if c.Target.Status != source.ImportRunning || len(b.Records) != 0 {
			return ErrImportState
		}
		return nil
	}
	offset := c.Before.LastOffset
	for _, record := range b.Records {
		if record.OriginID != content.OriginID || record.Start != offset {
			return ErrImportState
		}
		offset = record.End
	}
	if offset != c.Target.LastOffset {
		return ErrImportState
	}
	return nil
}

func sameImportRun(a, b source.ImportRun) bool {
	if a.ID != b.ID || a.SourceID != b.SourceID || a.Path != b.Path ||
		a.Status != b.Status || a.LastOffset != b.LastOffset || !a.CreatedAt.Equal(b.CreatedAt) ||
		(a.Content == nil) != (b.Content == nil) || (a.CompletedAt == nil) != (b.CompletedAt == nil) {
		return false
	}
	if a.Content != nil && *a.Content != *b.Content {
		return false
	}
	return a.CompletedAt == nil || a.CompletedAt.Equal(*b.CompletedAt)
}

func importCheckpoint(ctx context.Context, tx *sql.Tx, r source.ImportRun) (source.Position, bool, error) {
	var p source.Position
	err := tx.QueryRowContext(ctx, `SELECT offset, anchor_hash FROM checkpoints WHERE source_id = ? AND generation_id = ?`,
		r.SourceID, r.Content.OriginID).Scan(&p.Offset, &p.AnchorHash)
	if errors.Is(err, sql.ErrNoRows) {
		return source.Position{}, false, nil
	}
	if err != nil {
		return source.Position{}, false, err
	}
	p.OriginID = r.Content.OriginID
	return p, true, nil
}

// Check expected manifest and its prior position before records/checkpoints are
// changed. Attaching content may use a matching existing checkpoint; a new zero
// position must be explicitly registered in the batch. No file proof is inferred.
func prepareImportProgress(ctx context.Context, tx *sql.Tx, b source.Batch) error {
	c := b.ImportChange
	if c.Target.Content == nil {
		return nil
	}
	current, found, err := readImportRun(ctx, tx, c.Target.SourceID, c.Target.ID)
	if err != nil {
		return err
	}
	if found && sameImportRun(current, c.Target) {
		return checkAppliedImportPosition(ctx, tx, b)
	}
	if c.Before == nil && found || c.Before != nil && (!found || !sameImportRun(current, *c.Before)) {
		return ErrImportConflict
	}
	var fingerprint, device, inode string
	if err := tx.QueryRowContext(ctx, `SELECT fingerprint, device, inode FROM file_generations WHERE source_id = ? AND id = ?`,
		c.Target.SourceID, c.Target.Content.OriginID).Scan(&fingerprint, &device, &inode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrImportConflict
		}
		return err
	}
	if fingerprint != "sha256:"+c.Target.Content.SHA256 || device != "" || inode != "" {
		return ErrImportConflict
	}
	var inconsistent bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM import_runs
		WHERE source_id = ? AND generation_id = ? AND
		(byte_size <> ? OR sha256 <> ? OR trailing_partial <> ?))`, c.Target.SourceID,
		c.Target.Content.OriginID, c.Target.Content.Bytes, c.Target.Content.SHA256,
		c.Target.Content.TrailingPartial).Scan(&inconsistent); err != nil {
		return err
	}
	if inconsistent {
		return ErrImportConflict
	}
	p, found, err := importCheckpoint(ctx, tx, c.Target)
	if err != nil {
		return err
	}
	if c.Before != nil && c.Before.Content != nil {
		if !found || p.Offset != c.Before.LastOffset {
			return ErrImportConflict
		}
	} else if found {
		if p.Offset != c.Target.LastOffset {
			return ErrImportConflict
		}
	} else if c.Target.LastOffset != 0 || len(b.Checkpoints) != 1 {
		return ErrImportConflict
	}
	return nil
}

// Later attempts can advance the shared checkpoint after this exact Target was
// applied. At the same offset, however, a different anchor is not an applied
// checkpoint and must never be acknowledged, even on a manifest retry.
func checkAppliedImportPosition(ctx context.Context, tx *sql.Tx, b source.Batch) error {
	if b.ImportChange.Target.Content == nil {
		return nil
	}
	p, found, err := importCheckpoint(ctx, tx, b.ImportChange.Target)
	if err != nil {
		return err
	}
	if !found || p.Offset < b.ImportChange.Target.LastOffset ||
		p.Offset == b.ImportChange.Target.LastOffset && len(b.Checkpoints) == 1 && p != b.Checkpoints[0] {
		return ErrImportConflict
	}
	return nil
}

func writeImportChange(ctx context.Context, tx *sql.Tx, b source.Batch) error {
	c := *b.ImportChange
	current, found, err := readImportRun(ctx, tx, c.Target.SourceID, c.Target.ID)
	if err != nil {
		return err
	}
	if found && sameImportRun(current, c.Target) {
		return checkAppliedImportPosition(ctx, tx, b)
	}
	if c.Target.Content != nil {
		p, found, err := importCheckpoint(ctx, tx, c.Target)
		if err != nil {
			return err
		}
		if !found || p.Offset != c.Target.LastOffset || len(b.Checkpoints) == 1 && p != b.Checkpoints[0] {
			return ErrImportConflict
		}
	}
	var completed any
	if c.Target.CompletedAt != nil {
		completed = c.Target.CompletedAt.UTC().UnixNano()
	}
	var originID, bytes, digest, partial any
	if c.Target.Content != nil {
		originID, bytes, digest, partial = c.Target.Content.OriginID, c.Target.Content.Bytes, c.Target.Content.SHA256, c.Target.Content.TrailingPartial
	}
	var result sql.Result
	if c.Before == nil {
		if found {
			return ErrImportConflict
		}
		result, err = tx.ExecContext(ctx, `INSERT INTO import_runs(id, path, status, last_offset,
			created_at_ns, completed_at_ns, source_id, generation_id, byte_size, sha256, trailing_partial)
			VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING`, c.Target.ID, c.Target.Path, c.Target.Status,
			c.Target.LastOffset, c.Target.CreatedAt.UTC().UnixNano(), completed, c.Target.SourceID,
			originID, bytes, digest, partial)
	} else {
		if !found || !sameImportRun(current, *c.Before) {
			return ErrImportConflict
		}
		result, err = tx.ExecContext(ctx, `UPDATE import_runs SET status = ?, completed_at_ns = ?,
			last_offset = ?, generation_id = ?, byte_size = ?, sha256 = ?, trailing_partial = ?
			WHERE source_id = ? AND id = ?`, c.Target.Status, completed, c.Target.LastOffset,
			originID, bytes, digest, partial, c.Target.SourceID, c.Target.ID)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrImportConflict // foreign/legacy global ID collision is never adopted
	}
	return nil
}
