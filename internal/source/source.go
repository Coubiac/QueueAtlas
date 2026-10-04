// Package source defines the durable ingestion contract shared by log sources
// and sinks. A successful Commit acknowledges both records and checkpoints.
package source

import (
	"context"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
)

type Source interface {
	ID() string
	Run(ctx context.Context, sink Sink) error
}

type Sink interface {
	Commit(ctx context.Context, batch Batch) error
}

// StateReader exposes committed ingestion state without coupling sources to
// a storage implementation. FileOrigins returns candidates, not a verdict
// that the observed file belongs to any particular generation.
type StateReader interface {
	FileOrigins(ctx context.Context, query OriginQuery) (OriginPage, error)
	Checkpoint(ctx context.Context, sourceID, originID string) (Position, bool, error)
}

// PathStateReader supplies persisted candidates for restart planning, without
// requiring an observed physical identity. It does not choose which generation
// to resume or prove that an origin can still be found on disk.
type PathStateReader interface {
	FileOriginsByPath(ctx context.Context, query OriginPathQuery) (OriginPage, error)
}

const MaxOriginPageSize = 100

// OriginQuery selects one source's physical file identity. Limit must be in
// [1, MaxOriginPageSize]. AfterID is an exclusive cursor, empty for the first
// page; results are sorted by ID, not by creation time.
type OriginQuery struct {
	SourceID string
	Device   string
	Inode    string
	AfterID  string
	Limit    int
}

// OriginPathQuery filters by nonempty source ID and exact stored path. Paths are
// not normalized, case-folded or interpreted as patterns. Limit and AfterID have
// the same bounds and exclusive ID ordering as OriginQuery; a cursor need not
// identify an existing row. ID ordering is not generation chronology. Pages do
// not form a global snapshot when state changes between calls.
type OriginPathQuery struct {
	SourceID string
	Path     string
	AfterID  string
	Limit    int
}

type OriginState struct {
	Origin     Origin
	Checkpoint *Position // nil means no committed position; offset zero is valid
}

// OriginPage is bounded. Pass NextID as AfterID for the next page; an empty
// NextID ends the scan. Each page reflects one committed storage snapshot.
type OriginPage struct {
	States []OriginState
	NextID string
}

type Identity struct {
	ID          string
	Kind        string
	Name        string
	TrustedHost string
}

// Origin identifies one immutable file generation. A rename may change Path,
// but neither its ID nor Fingerprint. A truncation needs a new origin ID.
type Origin struct {
	ID          string
	Path        string
	Device      string
	Inode       string
	Fingerprint string
	FirstSeen   time.Time
}

// Record covers a complete physical line. End is the byte after its newline.
// Raw may be a bounded prefix when Error describes an oversized line.
type Record struct {
	OriginID    string
	Start       int64
	End         int64
	Raw         []byte
	Error       string
	ReadAt      time.Time
	Observation model.Observation
}

type Position struct {
	OriginID   string
	Offset     int64
	AnchorHash string
}

type Batch struct {
	Source      Identity
	Origins     []Origin
	Records     []Record
	Checkpoints []Position
}
