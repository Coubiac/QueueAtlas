package file

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

const MaxOpenGenerations = 2

// ErrRotationCapacity stops ingestion before opening a third distinct file.
// It contains no path or log content. Partial lines and unexpired grace can
// keep the capacity occupied.
var ErrRotationCapacity = errors.New("file rotation descriptor capacity reached")

type openedGeneration struct {
	file             *os.File
	identity         Identity
	ingestor         *Ingestor // nil while a new empty file waits for content
	eofSince         time.Time
	eofOffset        int64
	zeroReplayPrefix *PrefixFingerprint // only an explicitly prepared zero replay
}

// followPath owns the initial descriptor and every successor, closing each once
// on retirement or return. Each round attempts at most one record and consumes
// at most 64 KiB per opened generation, serially, including partial lines.
// Path checks and grace expiry run when due between rounds, including during
// continuous input. Only idle rounds wait, until the next scheduled check.
func (s *FileSource) followPath(ctx context.Context, f *os.File, ingestor *Ingestor, sink source.Sink, wait func(context.Context, time.Duration) error) (err error) {
	return s.followPathWithClock(ctx, f, ingestor, sink, wait, time.Now)
}

func (s *FileSource) followPathWithClock(ctx context.Context, f *os.File, ingestor *Ingestor, sink source.Sink, wait func(context.Context, time.Duration) error, now func() time.Time) (err error) {
	opened := []*openedGeneration{{file: f, ingestor: ingestor}}
	return s.followGenerations(ctx, opened, opened[0], sink, wait, now)
}

// followGenerations takes exclusive ownership immediately, including on errors
// during initial inspection. Retirements remove their descriptor before cleanup.
func (s *FileSource) followGenerations(ctx context.Context, opened []*openedGeneration, active *openedGeneration, sink source.Sink, wait func(context.Context, time.Duration) error, now func() time.Time) (err error) {
	defer func() {
		for _, generation := range opened {
			if generation.file != nil {
				err = errors.Join(err, generation.file.Close())
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, generation := range opened {
		id, err := Inspect(generation.file)
		if err != nil {
			return err
		}
		generation.identity = id
	}
	poll := func() error {
		// Diagnose observed shrink before retiring any descriptor or switching
		// generations, including retained files and unacknowledged partial bytes.
		if err := checkOpenedSizes(ctx, opened); err != nil {
			return err
		}
		if err := checkOpenedAnchors(ctx, opened); err != nil {
			return err
		}
		observation, err := s.observePath(ctx, active.file)
		if err != nil {
			return err
		}
		// A known descriptor can become current again without spending capacity
		// or repeating its generation decision/checkpoint registration.
		known := observation.Status != PathReplaced
		if !known {
			for _, generation := range opened {
				if generation.identity.SameFile(observation.Current) {
					active = generation
					known = true
					break
				}
			}
		}
		active.eofSince = time.Time{}
		if err := retireExpired(ctx, &opened, active, s.config.RotationGrace, now(), sink); err != nil {
			return err
		}
		if known {
			return nil
		}
		if len(opened) >= MaxOpenGenerations {
			return ErrRotationCapacity
		}
		next, nextID, err := OpenLog(ctx, s.config.Path)
		if err != nil {
			return err
		}
		if !nextID.SameFile(observation.Current) {
			return errors.Join(ErrPathChanged, next.Close())
		}
		active = &openedGeneration{file: next, identity: nextID}
		opened = append(opened, active)
		return nil
	}
	if err := poll(); err != nil {
		return err
	}
	nextPoll := now().Add(s.config.PollInterval)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		progress := false
		for _, generation := range opened {
			if err := ctx.Err(); err != nil {
				return err
			}
			if generation.ingestor == nil {
				var waiting bool
				generation.ingestor, waiting, err = s.prepareGeneration(ctx, generation.file, sink)
				if err != nil {
					return err
				}
				if waiting {
					generation.observeEOF(active, now())
					continue
				}
			}
			err := generation.ingestor.commitNext(ctx, sink, followReadFragments)
			if err == nil {
				generation.eofSince = time.Time{}
				progress = true
				continue
			}
			if errors.Is(err, errReadYield) && generation.ingestor.pending == nil {
				// Bytes progressed, but no complete line or EOF was observed.
				// Keep the fragments and let the other generation/poll proceed.
				generation.eofSince = time.Time{}
				progress = true
				continue
			}
			if !errors.Is(err, io.EOF) || generation.ingestor.pending != nil {
				return err
			}
			generation.observeEOF(active, now())
		}
		remaining := nextPoll.Sub(now())
		if progress && remaining > 0 {
			continue
		}
		if !progress && remaining > 0 {
			if err := wait(ctx, remaining); err != nil {
				return err
			}
		}
		if err := poll(); err != nil {
			return err
		}
		nextPoll = now().Add(s.config.PollInterval)
	}
}
