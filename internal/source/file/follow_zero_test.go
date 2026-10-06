package file

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestZeroReplayPrefixRecheckUntilPositiveAcknowledgement(t *testing.T) {
	_, r, f := pathFollower(t, "first\nsecond\n")
	prefix, err := CapturePrefix(f)
	if err != nil {
		t.Fatal(err)
	}
	opened := []*openedGeneration{{file: f, ingestor: r, zeroReplayPrefix: &prefix}}
	position := r.Position()
	if _, err := f.WriteAt([]byte("X"), 7); err != nil {
		t.Fatal(err)
	}
	err = checkOpenedAnchors(context.Background(), opened)
	var decision *ResumeDecisionError
	if !errors.As(err, &decision) || decision.Status != SelectionDifferent || r.Position() != position || r.lines.offset != 0 {
		t.Fatal("zero prefix change", err)
	}
	if at, err := f.Seek(0, io.SeekCurrent); err != nil || at != 0 {
		t.Fatal("prefix check sought", at, err)
	}
	if _, err := f.WriteAt([]byte("e"), 7); err != nil {
		t.Fatal(err)
	}
	if err := r.CommitNext(context.Background(), sinkFunc(func(context.Context, source.Batch) error { return nil })); err != nil {
		t.Fatal(err)
	}
	if r.Position().Offset != 6 {
		t.Fatal("positive checkpoint missing")
	}
	// Once positive, only the normal acknowledged anchor is evidence; changing
	// the unread second line must not keep applying the original replay prefix.
	if _, err := f.WriteAt([]byte("X"), 7); err != nil {
		t.Fatal(err)
	}
	if err := checkOpenedAnchors(context.Background(), opened); err != nil {
		t.Fatal("retained zero proof after positive ack", err)
	}
	if _, err := f.WriteAt([]byte("X"), 0); err != nil {
		t.Fatal(err)
	}
	if err := checkOpenedAnchors(context.Background(), opened); !errors.Is(err, ErrCheckpointChanged) {
		t.Fatal("positive anchor not checked", err)
	}
}
