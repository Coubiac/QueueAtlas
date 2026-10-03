package file

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/Coubiac/mailtrace/internal/source"
)

const MaxOpenGenerations = 2

// ErrRotationCapacity stops ingestion before opening a third distinct file.
// It contains no path or log content. Retained files have no expiry yet.
var ErrRotationCapacity = errors.New("file rotation descriptor capacity reached")

type openedGeneration struct {
	file     *os.File
	identity Identity
	ingestor *Ingestor // nil while a new empty file waits for content
}

// followPath owns only the successor descriptors it opens, closing them on every
// return. The caller owns the initial descriptor. A switched-away ingestor stays
// intact but is not polled for late writes in this version. No grace expiry yet.
// Observation occurs at startup and after EOF waits; continuous input can defer it.
func (s *FileSource) followPath(ctx context.Context, f *os.File, ingestor *Ingestor, sink source.Sink, wait func(context.Context) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	id, err := Inspect(f)
	if err != nil {
		return err
	}
	opened := []*openedGeneration{{file: f, identity: id, ingestor: ingestor}}
	defer func() {
		for _, generation := range opened[1:] {
			err = errors.Join(err, generation.file.Close())
		}
	}()
	active := opened[0]
	poll := func() error {
		observation, err := s.observePath(ctx, active.file)
		if err != nil || observation.Status != PathReplaced {
			return err
		}
		// A known descriptor can become current again without spending capacity
		// or repeating its generation decision/checkpoint registration.
		for _, generation := range opened {
			if generation.identity.SameFile(observation.Current) {
				active = generation
				return nil
			}
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
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if active.ingestor == nil {
			var waiting bool
			active.ingestor, waiting, err = s.prepareGeneration(ctx, active.file, sink)
			if err != nil {
				return err
			}
			if !waiting {
				continue
			}
		} else {
			err := active.ingestor.CommitNext(ctx, sink)
			if err == nil {
				continue
			}
			if !errors.Is(err, io.EOF) || active.ingestor.pending != nil {
				return err
			}
		}
		if err := wait(ctx); err != nil {
			return err
		}
		if err := poll(); err != nil {
			return err
		}
	}
}
