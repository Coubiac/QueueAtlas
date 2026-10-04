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

// Sink atomically persists a batch: nil acknowledges all records, checkpoints
// and state changes together. An error does not acknowledge anything to the
// caller, even if the durable commit succeeded but its acknowledgement was lost.
// Sources must retry the same batch before advancing; sinks must accept an
// identical retry without duplicating effects. A sink must not mutate the batch
// or its referenced slices and observations, which remain owned by the source.
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
	Origin      Origin
	Checkpoint  *Position // nil means no committed position; offset zero is valid
	FollowState FollowState
}

// FollowState values are persisted. Zero means unknown, including legacy state;
// chronology and checkpoint offsets must never be used to infer this state.
type FollowState int

const (
	FollowUnknown FollowState = iota
	FollowFollowing
	FollowRetired
)

// FollowTransition is an explicit, source-scoped expected-state change. A Sink
// accepts the expected state or an already applied target for idempotent retry.
// A source must serialize changes and retries and justify acquisition/retirement
// using its file lifecycle; persistence alone does not verify the file.
// Allowed pairs are unknown->following, following->retired and retired->following,
// with at most one transition per origin in a batch.
type FollowTransition struct {
	OriginID string
	From     FollowState
	To       FollowState
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

// Record covers a complete physical line in [Start, End), with 0 <= Start < End.
// End is the byte after its newline. Raw is bounded by model.MaxLineBytes and
// may be only a prefix when Error describes an oversized line; End still counts
// all consumed bytes. The source is responsible for these provenance bounds.
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
	Source            Identity
	Origins           []Origin
	Records           []Record
	Checkpoints       []Position
	FollowTransitions []FollowTransition // acknowledged in the same transaction
}
