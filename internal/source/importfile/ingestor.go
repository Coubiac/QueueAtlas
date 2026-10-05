package importfile

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/source/file"
)

var ErrImportResume = errors.New("validated import content and resume state do not match")

var ErrImportPartial = errors.New("validated import ends with an incomplete line")

// Ingestor binds one validated private copy to an acknowledged import run.
// The caller retains ownership of PreparedContent and must Close it on all
// paths. Reads/seeks/Close and state writes are exclusive and serialized.
// The copy's protected directory and bytes must remain unchanged during use.
type Ingestor struct {
	prepared  *PreparedContent
	identity  source.Identity
	run       source.ImportRun
	position  source.Position
	lines     *file.LineReader
	normalize file.Normalize
	staged    *source.Record
	pending   *source.Batch
}

// NewIngestor verifies whole-copy metadata, exact source/run/content identity and
// a canonical complete-line checkpoint before seeking to it. Unlike a live-file
// zero anchor, the whole validated content digest supplies identity at offset
// zero. No normalization, Sink call or manifest change happens. A refusal leaves
// ownership with the caller; cancellation or an error after Seek can leave the
// read position changed. No original path is reopened or implicitly adopted.
func NewIngestor(ctx context.Context, prepared *PreparedContent, identity source.Identity, run source.ImportRun, position source.Position, normalize file.Normalize) (*Ingestor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if prepared == nil || prepared.file == nil {
		return nil, fs.ErrClosed
	}
	if identity.ID == "" || identity.Kind != "import" || identity.Name == "" || normalize == nil ||
		run.SourceID != identity.ID || run.ID <= 0 || run.Path == "" ||
		run.Status != source.ImportRunning || run.CompletedAt != nil || run.Content == nil ||
		!time.Unix(0, run.CreatedAt.UTC().UnixNano()).Equal(run.CreatedAt) {
		return nil, ErrImportResume
	}
	info := prepared.Info()
	id, err := source.ImportOriginID(identity.ID, info.SHA256)
	if err != nil || info.Bytes < 0 || run.Content.OriginID != id || run.Content.SHA256 != info.SHA256 ||
		run.Content.Bytes != info.Bytes || run.Content.TrailingPartial != info.TrailingPartial ||
		position.OriginID != id || run.LastOffset != position.Offset || position.Offset < 0 || position.Offset > info.Bytes {
		return nil, ErrImportResume
	}
	anchor, err := file.ParseCheckpointAnchor(position.AnchorHash)
	if err != nil || anchor.Offset != position.Offset {
		return nil, ErrImportResume
	}
	physical, err := file.Inspect(prepared.file)
	if err != nil {
		return nil, err
	}
	if physical.Size != info.Bytes {
		return nil, ErrPreparedSize
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	actual, err := file.CaptureAnchor(prepared.file, position.Offset)
	if err != nil {
		return nil, err
	}
	if actual != anchor {
		return nil, ErrImportResume
	}
	if position.Offset > 0 {
		var end [1]byte
		if _, err := prepared.ReadAt(end[:], position.Offset-1); err != nil {
			return nil, err
		}
		if end[0] != '\n' {
			return nil, ErrImportResume
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	lines, err := file.NewLineReader(prepared, position.Offset)
	if err != nil {
		return nil, err
	}
	if _, err := prepared.Seek(position.Offset, io.SeekStart); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	content := *run.Content // caller metadata pointers cannot mutate bound state
	run.Content = &content
	return &Ingestor{prepared: prepared, identity: identity, run: run, position: position,
		lines: lines, normalize: normalize}, nil
}

// Position and RunState describe the acknowledged state, not buffered reads.
func (r *Ingestor) Position() source.Position { return r.position }

func (r *Ingestor) RunState() source.ImportRun {
	state := r.run
	content := *state.Content
	state.Content = &content
	if state.CompletedAt != nil {
		stamp := *state.CompletedAt
		state.CompletedAt = &stamp
	}
	return state
}

// CommitNext applies one complete physical record, checkpoint and run offset in
// the same transaction. At finite EOF it commits a terminal run change: io.EOF
// with RunState.Status complete means success, ErrImportPartial with failed means
// an incomplete last line. Only a terminal RunState proves acknowledgment; a Sink
// can return those same errors while the run is still running and pending. The
// terminal results are returned after acknowledgment, also on later calls.
// Read, anchor, cancellation and Sink errors leave acknowledged state unchanged.
// A consumed record is staged before proof/normalization; a failed commit retains
// its exact batch, including timestamps, until retried. Never compensate an
// ambiguous commit by marking the run failed. Sink must not mutate its input.
func (r *Ingestor) CommitNext(ctx context.Context, sink source.Sink) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sink == nil {
		return errors.New("sink is required")
	}
	if r.run.Status != source.ImportRunning {
		return r.terminalResult()
	}
	if r.pending == nil {
		if r.staged == nil {
			record, err := r.lines.Next(ctx)
			if err == io.EOF {
				if !r.run.Content.TrailingPartial && r.position.Offset != r.run.Content.Bytes {
					return ErrPreparedSize
				}
				before, target := r.RunState(), r.RunState()
				stamp := time.Now().UTC()
				// Clock rollback must not produce an invalid durable timestamp.
				if stamp.Before(target.CreatedAt) {
					stamp = target.CreatedAt
				}
				target.Status, target.CompletedAt = source.ImportComplete, &stamp
				if target.Content.TrailingPartial {
					target.Status = source.ImportFailed
				}
				r.pending = &source.Batch{Source: r.identity, ImportChange: &source.ImportChange{Before: &before, Target: target}}
			} else if err != nil {
				return err
			} else {
				record.OriginID, record.ReadAt = r.position.OriginID, time.Now().UTC()
				r.staged = &record
			}
		}
		if r.staged != nil {
			if err := ctx.Err(); err != nil {
				return err
			}
			record := *r.staged
			if record.Start != r.position.Offset || record.End > r.run.Content.Bytes {
				return ErrPreparedSize
			}
			anchor, err := file.CaptureAnchor(r.prepared.file, record.End)
			if err != nil {
				return err
			}
			if record.Error != "" {
				record.Observation = model.Observation{Kind: model.KindUnknown, Raw: string(record.Raw), ParseError: record.Error, Timestamp: model.Timestamp{Quality: model.TimeUnknown}}
			} else {
				record.Observation = r.normalize(append([]byte(nil), record.Raw...))
			}
			record.Observation.SourceID = r.identity.ID
			before, target := r.RunState(), r.RunState()
			target.LastOffset = record.End
			r.pending = &source.Batch{Source: r.identity, Records: []source.Record{record},
				Checkpoints:  []source.Position{{OriginID: record.OriginID, Offset: record.End, AnchorHash: anchor.String()}},
				ImportChange: &source.ImportChange{Before: &before, Target: target}}
			r.staged = nil
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := sink.Commit(ctx, *r.pending); err != nil {
		return err
	}
	if len(r.pending.Checkpoints) != 0 {
		r.position = r.pending.Checkpoints[0]
	}
	r.run = r.pending.ImportChange.Target
	r.pending = nil
	return r.terminalResult()
}

func (r *Ingestor) terminalResult() error {
	switch r.run.Status {
	case source.ImportComplete:
		return io.EOF
	case source.ImportFailed:
		return ErrImportPartial
	default:
		return nil
	}
}
