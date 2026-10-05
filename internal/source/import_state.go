package source

import (
	"context"
	"time"
)

type ImportStatus string

const (
	ImportRunning  ImportStatus = "running"
	ImportComplete ImportStatus = "complete"
	ImportFailed   ImportStatus = "failed"
)

// ImportContent is metadata for a whole validated private copy. It is immutable
// within a run. Its existence alone is not proof that records were ingested.
type ImportContent struct {
	OriginID        string
	Bytes           int64
	SHA256          string
	TrailingPartial bool
}

// ImportRun describes one explicit attempt. ID is a caller-assigned positive
// integer globally unique in the database (including legacy runs), stable across
// retries; SourceID is an import source, distinct from a
// live file source. Content nil means preparation has not succeeded. LastOffset
// is acknowledged progress for this attempt, not the currently buffered offset.
// Legacy unassociated runs are not exposed by ImportStateReader. Paths are
// metadata, never proof of file identity. Returned pointers belong to the caller.
type ImportRun struct {
	ID          int64
	SourceID    string
	Path        string
	Content     *ImportContent
	Status      ImportStatus
	LastOffset  int64
	CreatedAt   time.Time
	CompletedAt *time.Time
}

// ImportStateReader performs an exact source-scoped lookup, without adoption,
// writes or a decision to resume. Absence is distinct from a run at offset zero.
// It supplies no atomic snapshot across a later separate checkpoint lookup.
type ImportStateReader interface {
	ImportRun(ctx context.Context, sourceID string, runID int64) (ImportRun, bool, error)
}
