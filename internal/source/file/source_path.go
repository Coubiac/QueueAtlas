package file

import (
	"context"
	"os"

	"github.com/Coubiac/mailtrace/internal/source"
)

// LastPathStatus returns the last successful observation in the accepted Run.
// The empty value means no observation yet. An accepted new Run resets it;
// a rejected concurrent Run does not. The value survives cancellation/errors
// and may be stale; it is neither a running state nor proof of continuity.
// It is safe to call concurrently with Run and contains no path or log content.
func (s *FileSource) LastPathStatus() PathStatus {
	s.pathMu.RLock()
	defer s.pathMu.RUnlock()
	return s.pathStatus
}

func (s *FileSource) setPathStatus(status PathStatus) {
	s.pathMu.Lock()
	s.pathStatus = status
	s.pathMu.Unlock()
}

func (s *FileSource) observePath(ctx context.Context, f *os.File) error {
	observation, err := ObservePath(ctx, f, s.config.Path)
	if err != nil {
		return err
	}
	s.setPathStatus(observation.Status)
	return nil
}

// followPath observes once before consumption and after each reader EOF wait.
// It keeps the same ingestor, including partial bytes and pending checkpoints,
// while the path is missing or replaced. Continuous input can defer EOF polling.
// Descriptor ownership remains with runOpened; errors are not retried here.
func (s *FileSource) followPath(ctx context.Context, f *os.File, ingestor *Ingestor, sink source.Sink, wait func(context.Context) error) error {
	if err := s.observePath(ctx, f); err != nil {
		return err
	}
	return ingestor.follow(ctx, sink, func(ctx context.Context) error {
		if err := wait(ctx); err != nil {
			return err
		}
		return s.observePath(ctx, f)
	})
}
