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
