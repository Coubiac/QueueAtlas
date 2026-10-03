package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
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
	if err := retireExpired(context.Background(), &opened, current, time.Second, stamp.Add(time.Second)); err != nil || len(opened) != 2 || r.pending == nil || r.Position().Offset != 0 {
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
	if err := retireExpired(cancelled, &opened, current, time.Second, stamp.Add(2*time.Second)); !errors.Is(err, context.Canceled) || len(opened) != 2 {
		t.Fatal("cancelled retirement closed a descriptor", err)
	}
	if err := retireExpired(context.Background(), &opened, current, time.Second, stamp.Add(2*time.Second)); err != nil || len(opened) != 1 || r.Position().Offset != 5 {
		t.Fatal("acknowledged EOF did not retire", err)
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("retired descriptor stayed open", err)
	}
	if _, err := currentFile.Stat(); err != nil {
		t.Fatal("retirement closed current descriptor", err)
	}
}
