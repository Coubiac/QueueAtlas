package importfile

import (
	"context"
	"errors"
	"io/fs"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/source/file"
)

// CheckpointReader supplies one source-scoped committed position. The caller
// serializes this source's state writes across preparation, commit and ingestion.
type CheckpointReader interface {
	Checkpoint(context.Context, string, string) (source.Position, bool, error)
}

// Binding holds a proven copy and an optional association awaiting ACK. It owns
// neither the copy nor the Sink. Only Commit exposes the Ingestor, after ACK.
// One caller must preserve this object and its copy across ambiguous retries.
type Binding struct {
	ingestor *Ingestor
	pending  *source.Batch
}

// PrepareBinding proves the current shared checkpoint against a held validated
// copy. run must be the caller's acknowledged running attempt. An unprepared run
// can attach this content at that proven position; an already prepared run must
// match content and its own exact LastOffset. Missing checkpoints are allowed
// only for an unprepared attempt, at a new explicit zero position.
// This reads state and seeks the private copy, but performs no Sink call, parsing
// or manifest change. No original path is reopened or terminal attempt revived.
func PrepareBinding(ctx context.Context, prepared *PreparedContent, identity source.Identity, run source.ImportRun, state CheckpointReader, normalize file.Normalize) (*Binding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if prepared == nil || prepared.file == nil {
		return nil, fs.ErrClosed
	}
	if state == nil || normalize == nil || identity.ID == "" || identity.Kind != "import" || identity.Name == "" ||
		run.SourceID != identity.ID || run.ID <= 0 || run.Path == "" || run.Status != source.ImportRunning || run.CompletedAt != nil ||
		!time.Unix(0, run.CreatedAt.UTC().UnixNano()).Equal(run.CreatedAt) || run.Content == nil && run.LastOffset != 0 {
		return nil, ErrImportResume
	}
	info := prepared.Info()
	originID, err := source.ImportOriginID(identity.ID, info.SHA256)
	if err != nil {
		return nil, ErrImportResume
	}
	position, found, err := state.Checkpoint(ctx, identity.ID, originID)
	if err != nil {
		return nil, err
	}
	if !found {
		if run.Content != nil {
			return nil, ErrImportResume
		}
		anchor, err := file.CaptureAnchor(prepared.file, 0)
		if err != nil {
			return nil, err
		}
		position = source.Position{OriginID: originID, AnchorHash: anchor.String()}
	}
	target := run
	if run.Content == nil {
		target.Content = &source.ImportContent{OriginID: originID, Bytes: info.Bytes, SHA256: info.SHA256, TrailingPartial: info.TrailingPartial}
		target.LastOffset = position.Offset
	}
	bound, err := NewIngestor(ctx, prepared, identity, target, position, normalize)
	if err != nil {
		return nil, err
	}
	b := &Binding{ingestor: bound}
	if run.Content == nil {
		// Include the exact checkpoint even when already persisted. This makes
		// equal-offset anchors part of the Sink's atomic association contract.
		b.pending = &source.Batch{Source: identity,
			Origins:     []source.Origin{{ID: originID, Path: run.Path, Fingerprint: "sha256:" + info.SHA256, FirstSeen: run.CreatedAt}},
			Checkpoints: []source.Position{position}, ImportChange: &source.ImportChange{Before: &run, Target: bound.RunState()}}
	}
	return b, nil
}

// Commit acknowledges the proven association before exposing the Ingestor. Sink
// errors/cancellation return nil and preserve the exact batch; retry this Binding
// before any other source write. A resumed already-associated attempt needs no
// write. After ACK, repeated calls return the same Ingestor without another commit.
func (b *Binding) Commit(ctx context.Context, sink source.Sink) (*Ingestor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if sink == nil {
		return nil, errors.New("sink is required")
	}
	if b.pending != nil {
		if err := sink.Commit(ctx, *b.pending); err != nil {
			return nil, err
		}
		b.pending = nil
	}
	return b.ingestor, nil
}
