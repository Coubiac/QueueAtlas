package importfile

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/source/file"
)

var ErrImportResume = errors.New("validated import content and resume state do not match")

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
