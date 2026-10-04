package file

import (
	"context"
	"errors"
)

// ErrFileTruncated stops following when an opened file is observed shorter than
// the bytes already consumed, including an incomplete line. No path or log
// content is included. A truncate followed by growth before polling can escape
// this check; it does not prove that previously consumed bytes are unchanged.
var ErrFileTruncated = errors.New("opened file is shorter than consumed offset")

// checkOpenedSizes does not seek, change checkpoints or register generations.
// The scheduler bounds opened to MaxOpenGenerations. Use the reader's consumed
// offset, not the acknowledged position or the descriptor's read-ahead position.
func checkOpenedSizes(ctx context.Context, opened []*openedGeneration) error {
	for _, generation := range opened {
		if err := ctx.Err(); err != nil {
			return err
		}
		id, err := Inspect(generation.file)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if generation.ingestor != nil && id.Size < generation.ingestor.lines.offset {
			return ErrFileTruncated
		}
	}
	return nil
}
