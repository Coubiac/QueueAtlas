package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Coubiac/mailtrace/internal/source"
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
	// Preparation-only persistence: no content provenance or progress is accepted.
	if c.Target.Content != nil || len(b.Origins) != 0 || len(b.Records) != 0 || len(b.Checkpoints) != 0 {
		return errors.New("validated import content persistence is not yet supported")
	}
	if c.Before == nil {
		if c.Target.Status != source.ImportRunning {
			return ErrImportState
		}
		return nil
	}
	p := c.Before
	if err := validateImportRun(*p); err != nil {
		return err
	}
	if p.Content != nil || p.Status != source.ImportRunning || p.ID != c.Target.ID ||
		p.SourceID != c.Target.SourceID || p.Path != c.Target.Path || !p.CreatedAt.Equal(c.Target.CreatedAt) {
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

func writeImportChange(ctx context.Context, tx *sql.Tx, c source.ImportChange) error {
	current, found, err := readImportRun(ctx, tx, c.Target.SourceID, c.Target.ID)
	if err != nil {
		return err
	}
	if found && sameImportRun(current, c.Target) {
		return nil // acknowledged effect already durable, including lost-ACK retry
	}
	var completed any
	if c.Target.CompletedAt != nil {
		completed = c.Target.CompletedAt.UTC().UnixNano()
	}
	var result sql.Result
	if c.Before == nil {
		if found {
			return ErrImportConflict
		}
		result, err = tx.ExecContext(ctx, `INSERT INTO import_runs(id, path, status, last_offset,
			created_at_ns, completed_at_ns, source_id) VALUES(?, ?, ?, 0, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING`, c.Target.ID, c.Target.Path, c.Target.Status,
			c.Target.CreatedAt.UTC().UnixNano(), completed, c.Target.SourceID)
	} else {
		if !found || !sameImportRun(current, *c.Before) {
			return ErrImportConflict
		}
		result, err = tx.ExecContext(ctx, `UPDATE import_runs SET status = ?, completed_at_ns = ?
			WHERE source_id = ? AND id = ?`, c.Target.Status, completed, c.Target.SourceID, c.Target.ID)
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
