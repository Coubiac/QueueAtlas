package file

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

type GenerationStart struct {
	Selection SelectionStatus
	State     *source.OriginState // nil when the selection cannot be applied
	Created   bool                // true only after successful Sink.Commit
}

// EnsureGeneration returns a verified existing origin or registers a new one
// when the complete selection found no candidate or only different candidates.
// Insufficient, ambiguous and limited selections return a nil State without
// writing. A successful registration acknowledges origin and offset zero in one
// Sink.Commit, with no log records; no file seeking or reading loop is started.
//
// The caller must serialize this source's state writes through selection and
// application, as required by SelectResume. An initial zero checkpoint cannot
// prove a generation on a later restart: it then requires an explicit decision.
// File content reads retain the bounded, non-atomic guarantees of CapturePrefix.
func EnsureGeneration(ctx context.Context, f *os.File, identity source.Identity, reader source.StateReader, sink source.Sink) (GenerationStart, error) {
	if err := ctx.Err(); err != nil {
		return GenerationStart{}, err
	}
	if identity.ID == "" || identity.Kind != "file" || identity.Name == "" || sink == nil {
		return GenerationStart{}, errors.New("file source ID, name and sink are required")
	}
	selection, err := SelectResume(ctx, f, identity.ID, reader)
	if err != nil {
		return GenerationStart{}, err
	}
	result := GenerationStart{Selection: selection.Status}
	switch selection.Status {
	case SelectionUnique:
		result.State = selection.Candidate
		return result, nil
	case SelectionInsufficient, SelectionAmbiguous, SelectionLimit:
		return result, nil
	case SelectionAbsent, SelectionDifferent:
	default:
		return GenerationStart{}, errors.New("invalid generation selection")
	}
	id, err := Inspect(f)
	if err != nil {
		return GenerationStart{}, err
	}
	if f.Name() == "" {
		return GenerationStart{}, errors.New("file path is required")
	}
	prefix, err := CapturePrefix(f)
	if err != nil {
		return GenerationStart{}, err
	}
	anchor, err := CaptureAnchor(f, 0)
	if err != nil {
		return GenerationStart{}, err
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return GenerationStart{}, err
	}
	origin := source.Origin{
		ID: "file-" + hex.EncodeToString(token[:]), Path: f.Name(),
		Device: id.Device, Inode: id.Inode, Fingerprint: prefix.String(),
		FirstSeen: time.Now().UTC(),
	}
	position := source.Position{OriginID: origin.ID, Offset: 0, AnchorHash: anchor.String()}
	if err := ctx.Err(); err != nil {
		return GenerationStart{}, err
	}
	if err := sink.Commit(ctx, source.Batch{
		Source: identity, Origins: []source.Origin{origin}, Checkpoints: []source.Position{position},
	}); err != nil {
		return GenerationStart{}, err
	}
	result.State = &source.OriginState{Origin: origin, Checkpoint: &position}
	result.Created = true
	return result, nil
}
