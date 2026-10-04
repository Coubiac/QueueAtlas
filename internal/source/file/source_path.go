package file

import (
	"context"
	"os"
)

// LastPathStatus returns the last successful observation in the accepted Run.
// The empty value means no observation yet. An accepted new Run resets it;
// a rejected concurrent Run does not. The value survives cancellation/errors
// and may be stale; it is neither a running state nor proof of continuity.
// FollowOpened resets it only after ownership transfer, before scheduler polling;
// a rejection before transfer preserves it. It is safe to call concurrently with
// Run/FollowOpened and contains no path or log content.
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

func (s *FileSource) observePath(ctx context.Context, f *os.File) (PathObservation, error) {
	observation, err := ObservePath(ctx, f, s.config.Path)
	if err != nil {
		return PathObservation{}, err
	}
	s.setPathStatus(observation.Status)
	return observation, nil
}
