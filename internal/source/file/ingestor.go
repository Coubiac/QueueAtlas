package file

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

// Normalize interprets one bounded complete line. Its result becomes owned by
// the ingestor; it must not be mutated after return. Oversized lines bypass it.
type Normalize func([]byte) model.Observation

// Ingestor consumes one acknowledged generation using one caller at a time.
// It owns neither the descriptor nor the Sink. No polling or rotation is done.
type Ingestor struct {
	identity  source.Identity
	lines     *LineReader
	normalize Normalize
	position  source.Position
	pending   *source.Batch
	tail      [MaxFingerprintBytes]byte
	tailLen   int
}

// NewIngestor seeks to a position supplied by the generation decision. The
// caller must retain exclusive ownership of descriptor reads/seeks and serialize
// this source's state writes. Offset zero is permitted for a freshly registered
// generation; this constructor does not resolve an insufficient resume decision.
func NewIngestor(ctx context.Context, f *os.File, identity source.Identity, state source.OriginState, normalize Normalize) (*Ingestor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if identity.ID == "" || identity.Kind != "file" || identity.Name == "" || normalize == nil {
		return nil, errors.New("file source identity and normalizer are required")
	}
	p := state.Checkpoint
	if state.Origin.ID == "" || p == nil || p.OriginID != state.Origin.ID {
		return nil, errors.New("generation and checkpoint are required")
	}
	anchor, err := ParseCheckpointAnchor(p.AnchorHash)
	if err != nil || anchor.Offset != p.Offset {
		return nil, errors.New("invalid initial checkpoint anchor")
	}
	id, err := Inspect(f)
	if err != nil {
		return nil, err
	}
	if p.Offset > id.Size {
		return nil, io.ErrUnexpectedEOF
	}
	if id.Device != "" && (id.Device != state.Origin.Device || id.Inode != state.Origin.Inode) {
		return nil, errors.New("generation physical identity changed")
	}
	prefix, err := ParsePrefixFingerprint(state.Origin.Fingerprint)
	if err != nil {
		return nil, err
	}
	if prefix.Length > 0 {
		match, err := prefix.Matches(f)
		if err != nil {
			return nil, err
		}
		if !match {
			return nil, errors.New("generation prefix changed")
		}
	}
	r := &Ingestor{identity: identity, normalize: normalize, position: *p, tailLen: anchor.Length}
	if anchor.Length > 0 {
		if _, err := f.ReadAt(r.tail[:anchor.Length], p.Offset-int64(anchor.Length)); err != nil {
			return nil, err
		}
		if sha256.Sum256(r.tail[:anchor.Length]) != anchor.Digest || r.tail[anchor.Length-1] != '\n' {
			return nil, errors.New("initial checkpoint content or line boundary changed")
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := f.Seek(p.Offset, io.SeekStart); err != nil {
		return nil, err
	}
	r.lines, err = NewLineReader(f, p.Offset)
	if err != nil {
		return nil, err
	}
	r.lines.observe = r.remember
	return r, nil
}

// Position returns the last acknowledged position, never the buffered offset.
func (r *Ingestor) Position() source.Position { return r.position }

// CommitNext commits exactly one complete record and its anchor together. EOF
// retains a partial line without a commit. Failed commits retain the same batch
// (including read time, normalization and anchor) for the next call, and no later
// line is read until acknowledgment. Cancellation before commit also retains it.
// A Sink must honor the durable contract and must not mutate the supplied batch.
func (r *Ingestor) CommitNext(ctx context.Context, sink source.Sink) error {
	return r.commitNext(ctx, sink, 0)
}

// The scheduler can yield a partial physical line without normalization or a
// commit. A pending batch always takes precedence over another read attempt.
func (r *Ingestor) commitNext(ctx context.Context, sink source.Sink, fragmentLimit int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sink == nil {
		return errors.New("sink is required")
	}
	if r.pending == nil {
		record, err := r.lines.next(ctx, fragmentLimit)
		if err != nil {
			return err
		}
		record.OriginID, record.ReadAt = r.position.OriginID, time.Now().UTC()
		if record.Error != "" {
			record.Observation = model.Observation{Kind: model.KindUnknown, Raw: string(record.Raw), ParseError: record.Error, Timestamp: model.Timestamp{Quality: model.TimeUnknown}}
		} else {
			record.Observation = r.normalize(append([]byte(nil), record.Raw...))
		}
		record.Observation.SourceID = r.identity.ID
		anchor := CheckpointAnchor{Offset: record.End, Length: r.tailLen, Digest: sha256.Sum256(r.tail[:r.tailLen])}
		r.pending = &source.Batch{Source: r.identity, Records: []source.Record{record}, Checkpoints: []source.Position{{OriginID: record.OriginID, Offset: record.End, AnchorHash: anchor.String()}}}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := sink.Commit(ctx, *r.pending); err != nil {
		return err
	}
	r.position = r.pending.Checkpoints[0]
	r.pending = nil
	return nil
}

func (r *Ingestor) remember(fragment []byte) {
	if len(fragment) >= len(r.tail) {
		copy(r.tail[:], fragment[len(fragment)-len(r.tail):])
		r.tailLen = len(r.tail)
		return
	}
	keep := min(r.tailLen, len(r.tail)-len(fragment))
	copy(r.tail[:keep], r.tail[r.tailLen-keep:r.tailLen])
	copy(r.tail[keep:], fragment)
	r.tailLen = keep + len(fragment)
}
