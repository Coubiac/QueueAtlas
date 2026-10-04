package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestPollStopsOnCurrentAnchorRewriteWithoutChangingCheckpoint(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		truncate      bool
	}{
		{"same_size", "other\npartial", false},
		{"truncate_then_regrow", "other\npartial extra\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, f := pathFollower(t, "first\npartial")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			commits, waits := 0, 0
			var acknowledged source.Position
			err := s.followPath(ctx, f, r, sinkFunc(func(_ context.Context, b source.Batch) error {
				commits++
				acknowledged = b.Checkpoints[0]
				return nil
			}), func(context.Context, time.Duration) error {
				waits++
				if waits > 1 {
					t.Fatal("anchor rewrite kept following")
				}
				if r.lines.offset != 13 || r.Position().Offset != 6 {
					t.Fatal("test requires partial bytes beyond checkpoint")
				}
				if tc.truncate {
					if err := f.Truncate(0); err != nil {
						return err
					}
				}
				_, err := f.WriteAt([]byte(tc.content), 0)
				return err
			})
			if !errors.Is(err, ErrCheckpointChanged) || errors.Is(err, ErrFileTruncated) || commits != 1 || waits != 1 || r.Position() != acknowledged || acknowledged.Offset != 6 || r.pending != nil {
				t.Fatalf("rewrite: %v, commits %d, waits %d, position %+v", err, commits, waits, r.Position())
			}
			if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("rewrite did not close descriptor", err)
			}
		})
	}
}

func TestLiveAnchorChecksAcknowledgedWindowWithoutSeeking(t *testing.T) {
	_, r, f := pathFollower(t, strings.Repeat("x", MaxFingerprintBytes+10)+"\npart")
	ctx := context.Background()
	ack := sinkFunc(func(context.Context, source.Batch) error { return nil })
	if err := r.CommitNext(ctx, ack); err != nil {
		t.Fatal(err)
	}
	if err := r.CommitNext(ctx, ack); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	position := r.Position()
	physical, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatal(err)
	}
	// Changes outside the acknowledged window are deliberately not proven here.
	for _, offset := range []int64{0, position.Offset} {
		if _, err := f.WriteAt([]byte("z"), offset); err != nil {
			t.Fatal(err)
		}
	}
	opened := []*openedGeneration{{file: f, ingestor: r}}
	if err := checkOpenedAnchors(ctx, opened); err != nil {
		t.Fatal("checked outside the acknowledged window or used partial tail", err)
	}
	if got, err := f.Seek(0, io.SeekCurrent); err != nil || got != physical || r.Position() != position {
		t.Fatal("anchor check moved position or closed descriptor", got, err)
	}
	if _, err := f.WriteAt([]byte("z"), position.Offset-2); err != nil {
		t.Fatal(err)
	}
	if err := checkOpenedAnchors(ctx, opened); !errors.Is(err, ErrCheckpointChanged) || r.Position() != position {
		t.Fatal("window rewrite not diagnosed or changed checkpoint", err)
	}
	// Cancellation and filesystem errors must keep their actual cause.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := checkOpenedAnchors(cancelled, opened); !errors.Is(err, context.Canceled) || errors.Is(err, ErrCheckpointChanged) {
		t.Fatal("anchor check replaced cancellation", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkOpenedAnchors(ctx, opened); err == nil || errors.Is(err, ErrCheckpointChanged) {
		t.Fatal("stat failure became rewrite diagnostic", err)
	}
}

func TestLiveAnchorZeroCheckpointAndUnregisteredFileHaveNoProof(t *testing.T) {
	_, r, f := pathFollower(t, "partial")
	if _, err := f.WriteAt([]byte("changed"), 0); err != nil {
		t.Fatal(err)
	}
	position := r.Position()
	if err := checkOpenedAnchors(context.Background(), []*openedGeneration{{file: f, ingestor: r}, {file: f}}); err != nil || r.Position() != position {
		t.Fatal("zero/absent checkpoint incorrectly claimed a mismatch", err)
	}
}

func TestLiveAnchorRejectsInvalidAcknowledgedPosition(t *testing.T) {
	for _, malformed := range []bool{true, false} {
		_, r, f := pathFollower(t, "first\n")
		// The valid zero anchor is also invalid for a positive position.
		r.position.Offset = 6
		if malformed {
			r.position.AnchorHash = "invalid"
		}
		if err := checkOpenedAnchors(context.Background(), []*openedGeneration{{file: f, ingestor: r}}); err == nil || errors.Is(err, ErrCheckpointChanged) {
			t.Fatal("invalid checkpoint became a content diagnostic", err)
		}
	}
}
