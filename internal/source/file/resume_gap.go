package file

import "errors"

// ErrFollowResumeGap means a completed location scan could not find a verified
// persisted following generation. It proves neither deletion nor lost records:
// a present but changed file can also prevent verified continuity.
var ErrFollowResumeGap = errors.New("persisted following generation unavailable for verified resume")

// FollowResumeGapError keeps the precise selection diagnostic available through
// errors.As while errors.Is identifies a gap independently of that selection.
// Error contains no origin ID, path, offsets or log content.
type FollowResumeGapError struct {
	Status SelectionStatus
}

func (e *FollowResumeGapError) Error() string { return ErrFollowResumeGap.Error() }

func (e *FollowResumeGapError) Is(target error) bool { return target == ErrFollowResumeGap }

func (e *FollowResumeGapError) Unwrap() error { return &ResumeDecisionError{Status: e.Status} }

func followLocationError(status SelectionStatus) error {
	if status == SelectionAbsent || status == SelectionDifferent {
		return &FollowResumeGapError{Status: status}
	}
	return &ResumeDecisionError{Status: status}
}
