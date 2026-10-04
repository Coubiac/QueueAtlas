package file

import (
	"context"
	"errors"
)

// ErrCheckpointChanged reports an observed mismatch in the last acknowledged
// anchor window of an opened file. It contains no path or log content and does
// not imply that the rest of the file was checked or identify a cause.
var ErrCheckpointChanged = errors.New("opened file checkpoint anchor changed")

// checkOpenedAnchors reads at most MaxFingerprintBytes per opened generation,
// without seeking or changing state. Check sizes first to diagnose observed
// shrink separately. Reads are not atomic with concurrent filesystem writes.
// Explicit zero replay uses its saved prefix until a positive acknowledgement;
// ordinary zero checkpoints still supply no anchor evidence.
func checkOpenedAnchors(ctx context.Context, opened []*openedGeneration) error {
	if err := checkZeroReplayPrefixes(ctx, opened); err != nil {
		return err
	}
	for _, generation := range opened {
		if err := ctx.Err(); err != nil {
			return err
		}
		if generation.ingestor == nil {
			continue
		}
		position := generation.ingestor.Position()
		if position.Offset == 0 {
			continue // a zero checkpoint has no bytes to verify
		}
		anchor, err := ParseCheckpointAnchor(position.AnchorHash)
		if err != nil || anchor.Offset != position.Offset {
			return errors.New("invalid acknowledged checkpoint anchor")
		}
		match, err := anchor.Matches(generation.file)
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !match {
			return ErrCheckpointChanged
		}
	}
	return nil
}
