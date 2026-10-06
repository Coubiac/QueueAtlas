package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestGracePreservesUnacknowledgedBatchBeforeRetiring(t *testing.T) {
	f, _ := testRegularFile(t, "line\n")
	currentFile, _ := testRegularFile(t, "current\n")
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("unacknowledged line")
	if err := r.CommitNext(context.Background(), sinkFunc(func(context.Context, source.Batch) error { return boom })); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	current := &openedGeneration{file: currentFile}
	// A stale expiry candidate must still recheck the current ingestor state.
	retired := &openedGeneration{file: f, ingestor: r, eofSince: stamp, eofOffset: 5}
	opened := []*openedGeneration{retired, current}
	retirements := 0
	sink := sinkFunc(func(_ context.Context, b source.Batch) error {
		retirements++
		if b.Source != fileSourceIdentity() || len(b.FollowTransitions) != 1 || b.FollowTransitions[0] != (source.FollowTransition{OriginID: r.Position().OriginID, From: source.FollowFollowing, To: source.FollowRetired}) || len(b.Origins)+len(b.Records)+len(b.Checkpoints) != 0 {
			t.Fatal("retirement changed data", b)
		}
		if _, err := f.Stat(); err != nil {
			t.Fatal("descriptor closed before acknowledgement", err)
		}
		return nil
	})
	if err := retireExpired(context.Background(), &opened, current, time.Second, stamp.Add(time.Second), sink); err != nil || len(opened) != 2 || r.pending == nil || r.Position().Offset != 0 || retirements != 0 {
		t.Fatal("pending batch retired or advanced", err)
	}
	if _, err := f.Stat(); err != nil {
		t.Fatal("pending descriptor closed", err)
	}
	ack := sinkFunc(func(context.Context, source.Batch) error { return nil })
	if err := r.CommitNext(context.Background(), ack); err != nil {
		t.Fatal(err)
	}
	if err := r.CommitNext(context.Background(), ack); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	retired.observeEOF(current, stamp.Add(time.Second))
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := retireExpired(cancelled, &opened, current, time.Second, stamp.Add(2*time.Second), sink); !errors.Is(err, context.Canceled) || len(opened) != 2 || retirements != 0 {
		t.Fatal("cancelled retirement closed a descriptor", err)
	}
	if err := retireExpired(context.Background(), &opened, current, time.Second, stamp.Add(2*time.Second), sink); err != nil || len(opened) != 1 || r.Position().Offset != 5 || retirements != 1 {
		t.Fatal("acknowledged EOF did not retire", err)
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("retired descriptor stayed open", err)
	}
	if _, err := currentFile.Stat(); err != nil {
		t.Fatal("retirement closed current descriptor", err)
	}
}

func TestGraceClosesUnregisteredEmptyFileWithoutTransition(t *testing.T) {
	f, _ := testRegularFile(t, "")
	currentFile, _ := testRegularFile(t, "current\n")
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	current := &openedGeneration{file: currentFile}
	old := &openedGeneration{file: f, eofSince: stamp}
	opened := []*openedGeneration{old, current}
	sink := sinkFunc(func(context.Context, source.Batch) error {
		t.Fatal("empty unregistered file retired durably")
		return nil
	})
	if err := retireExpired(context.Background(), &opened, current, time.Second, stamp.Add(time.Second), sink); err != nil || len(opened) != 1 {
		t.Fatal("empty expiry failed", err)
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("empty descriptor retained", err)
	}
}

func TestGraceRemovesDescriptorAfterAcknowledgementEvenIfCloseFails(t *testing.T) {
	f, _ := testRegularFile(t, "line\n")
	currentFile, _ := testRegularFile(t, "current\n")
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 5), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	current := &openedGeneration{file: currentFile}
	old := &openedGeneration{file: f, ingestor: r, eofSince: stamp, eofOffset: 5}
	opened := []*openedGeneration{old, current}
	commits := 0
	err = retireExpired(context.Background(), &opened, current, time.Second, stamp.Add(time.Second), sinkFunc(func(_ context.Context, b source.Batch) error {
		commits++
		if len(b.FollowTransitions) != 1 || b.FollowTransitions[0].To != source.FollowRetired {
			t.Fatal("invalid retirement", b)
		}
		// Inject a failed close after the Sink acknowledgement. The scheduler
		// must remove the descriptor even when its own Close reports an error.
		return f.Close()
	}))
	if !errors.Is(err, fs.ErrClosed) || commits != 1 || len(opened) != 1 || opened[0] != current || r.Position().Offset != 5 {
		t.Fatal("acknowledged close error lost ownership/checkpoint", err, commits, opened)
	}
	if _, err := currentFile.Stat(); err != nil {
		t.Fatal("current descriptor closed", err)
	}
}
