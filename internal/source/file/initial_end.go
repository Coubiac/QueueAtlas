package file

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
)

var ErrInitialEndPartial = errors.New("file initial end is not a complete line boundary")

// captureInitialEnd snapshots the descriptor's size, then captures at most one
// anchor window. It never seeks, reads lines or writes state. Later appends do
// not move the chosen boundary. An empty file returns the canonical zero anchor;
// registration still waits for a nonempty prefix. A partial final line is refused
// instead of persisting an offset that could later normalize only its suffix.
// Content inspection and capture are bounded observations, not an atomic snapshot.
func captureInitialEnd(ctx context.Context, f *os.File) (CheckpointAnchor, error) {
	if err := ctx.Err(); err != nil {
		return CheckpointAnchor{}, err
	}
	id, err := Inspect(f)
	if err != nil {
		return CheckpointAnchor{}, err
	}
	return captureInitialEndAt(ctx, f, id.Size)
}

func captureInitialEndAt(ctx context.Context, input io.ReaderAt, offset int64) (CheckpointAnchor, error) {
	if err := ctx.Err(); err != nil {
		return CheckpointAnchor{}, err
	}
	if input == nil || offset < 0 {
		return CheckpointAnchor{}, errors.New("initial end input and nonnegative offset are required")
	}
	anchor := CheckpointAnchor{Offset: offset, Length: int(min(offset, int64(MaxFingerprintBytes)))}
	var window [MaxFingerprintBytes]byte
	if anchor.Length > 0 {
		n, err := input.ReadAt(window[:anchor.Length], offset-int64(anchor.Length))
		if err := ctx.Err(); err != nil {
			return CheckpointAnchor{}, err
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return CheckpointAnchor{}, err
		}
		if n != anchor.Length {
			return CheckpointAnchor{}, io.ErrUnexpectedEOF
		}
		if window[anchor.Length-1] != '\n' {
			return CheckpointAnchor{}, ErrInitialEndPartial
		}
	}
	anchor.Digest = sha256.Sum256(window[:anchor.Length])
	return anchor, nil
}
