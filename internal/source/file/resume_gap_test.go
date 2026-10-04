package file

import (
	"errors"
	"testing"
)

func TestFollowLocationErrorOnlyClassifiesCompletedUnavailableSelections(t *testing.T) {
	for _, status := range []SelectionStatus{SelectionAbsent, SelectionDifferent, SelectionInsufficient, SelectionAmbiguous, SelectionLimit} {
		err := followLocationError(status)
		wantGap := status == SelectionAbsent || status == SelectionDifferent
		var decision *ResumeDecisionError
		if !errors.As(err, &decision) || decision.Status != status || errors.Is(err, ErrFollowResumeGap) != wantGap {
			t.Fatal("selection diagnostic lost or falsely classified", status, err)
		}
		var gap *FollowResumeGapError
		if errors.As(err, &gap) != wantGap {
			t.Fatal("gap type classification", status, err)
		}
		if wantGap && (gap.Status != status || err.Error() != "persisted following generation unavailable for verified resume") {
			t.Fatal("unstable gap diagnostic", err)
		}
		if errors.Is(err, ErrCurrentMissing) {
			t.Fatal("gap confused with absent current", err)
		}
	}
}
