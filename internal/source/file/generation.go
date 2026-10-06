package file

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

type GenerationStart struct {
	Selection         SelectionStatus
	State             *source.OriginState // nil when the selection cannot be applied
	Created           bool                // true only after successful Sink.Commit
	WaitingForContent bool                // new registration deferred after empty capture
}

// EnsureGeneration returns a verified existing origin or registers a new one
// when the complete selection found no candidate or only different candidates.
// Insufficient, ambiguous and limited selections return a nil State without
// writing. A successful registration acknowledges origin and offset zero in one
// Sink.Commit, with no log records; no file seeking or reading loop is started.
// An empty captured prefix instead sets WaitingForContent, with no write; the
// caller can invoke this decision again after append. Existing insufficient
// origins, including legacy empty fingerprints, are never replaced implicitly.
//
// The caller must serialize this source's state writes through selection and
// application, as required by SelectResume. An initial zero checkpoint cannot
// prove a generation on a later restart: it then requires an explicit decision.
// File content reads retain the bounded, non-atomic guarantees of CapturePrefix.
func EnsureGeneration(ctx context.Context, f *os.File, identity source.Identity, reader source.StateReader, sink source.Sink) (GenerationStart, error) {
	return EnsureGenerationWithPolicy(ctx, f, identity, reader, sink, ResumePolicy{})
}

// EnsureGenerationWithPolicy can return a zero replay decision without writing
// an origin or checkpoint. All other registration rules remain unchanged.
func EnsureGenerationWithPolicy(ctx context.Context, f *os.File, identity source.Identity, reader source.StateReader, sink source.Sink, policy ResumePolicy) (GenerationStart, error) {
	return ensureGenerationWithStart(ctx, f, identity, reader, sink, policy, StartAtBeginning)
}

// Only Run's bootstrap grants end after a complete path scan without history.
// A physical selection with any history keeps the existing beginning/resume
// rules, even if the configured path itself has no registered origin yet.
func ensureGenerationWithStart(ctx context.Context, f *os.File, identity source.Identity, reader source.StateReader, sink source.Sink, policy ResumePolicy, startAt StartAt) (GenerationStart, error) {
	if err := ctx.Err(); err != nil {
		return GenerationStart{}, err
	}
	if identity.ID == "" || identity.Kind != "file" || identity.Name == "" || sink == nil {
		return GenerationStart{}, errors.New("file source ID, name and sink are required")
	}
	mode, err := resolveStartAt(startAt)
	if err != nil {
		return GenerationStart{}, err
	}
	selection, err := SelectResumeWithPolicy(ctx, f, identity.ID, reader, policy)
	if err != nil {
		return GenerationStart{}, err
	}
	result := GenerationStart{Selection: selection.Status}
	switch selection.Status {
	case SelectionUnique, SelectionRestartZero:
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
	var anchor CheckpointAnchor
	initialEnd := mode == StartAtEnd && selection.Status == SelectionAbsent
	if initialEnd {
		anchor, err = captureInitialEnd(ctx, f)
		if err != nil {
			return GenerationStart{}, err
		}
		if anchor.Offset == 0 {
			result.WaitingForContent = true
			return result, nil
		}
	}
	prefix, err := CapturePrefix(f)
	if err != nil {
		return GenerationStart{}, err
	}
	if err := ctx.Err(); err != nil {
		return GenerationStart{}, err
	}
	if prefix.Length == 0 {
		result.WaitingForContent = true
		return result, nil
	}
	if !initialEnd {
		anchor, err = CaptureAnchor(f, 0)
		if err != nil {
			return GenerationStart{}, err
		}
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
	position := source.Position{OriginID: origin.ID, Offset: anchor.Offset, AnchorHash: anchor.String()}
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
