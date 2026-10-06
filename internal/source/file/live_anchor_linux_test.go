//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestPollDiagnosesRetainedAnchorRewriteBeforeExpiryOrThirdRegistration(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	s, store := rotationSource(t, path, time.Second)
	r, _, err := s.prepareGeneration(context.Background(), f, store)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stamp := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	waits, registrations := 0, 0
	var positions []source.Position
	err = s.followPathWithClock(ctx, f, r, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		registrations += len(b.Origins)
		if len(b.Records) > 0 {
			positions = append(positions, b.Checkpoints[0])
		}
		return nil
	}), func(context.Context, time.Duration) error {
		waits++
		switch waits {
		case 1:
			return rotateTo(path, ".1", "second\n")
		case 2:
			stamp = stamp.Add(s.config.RotationGrace)
			if _, err := f.WriteAt([]byte("other\n"), 0); err != nil {
				return err
			}
			return rotateTo(path, ".2", "third\n")
		default:
			t.Fatal("retained rewrite did not stop")
			return nil
		}
	}, func() time.Time { return stamp })
	if !errors.Is(err, ErrCheckpointChanged) || errors.Is(err, fs.ErrClosed) || waits != 2 || registrations != 1 || len(positions) != 2 {
		t.Fatalf("retained rewrite: %v, waits %d, registrations %d, checkpoints %+v", err, waits, registrations, positions)
	}
	for i, position := range positions {
		got, found, err := store.Checkpoint(context.Background(), s.ID(), position.OriginID)
		if err != nil || !found || got != position || got.Offset != []int64{6, 7}[i] {
			t.Fatal("rewrite changed durable checkpoint", got, found, err)
		}
	}
	if positions[0].OriginID == positions[1].OriginID {
		t.Fatal("generations share checkpoint")
	}
	for _, name := range []string{path, path + ".1", path + ".2"} {
		if fileDescriptorCount(t, name) != 0 {
			t.Fatal("rewrite leaked or opened descriptor", name)
		}
	}
}
