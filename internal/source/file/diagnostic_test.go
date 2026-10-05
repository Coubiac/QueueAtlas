package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"
)

func TestFileDiagnosticDoesNotInventGapOrRunningHealth(t *testing.T) {
	missing := Diagnose(PathMissing, nil)
	if !missing.Missing || missing.Gap || missing.Degraded || missing.Failure != DiagnosticNone {
		t.Fatal("missing current observation degraded retained-reader continuity", missing)
	}
	stopped := Diagnose(PathMissing, context.Canceled)
	if !stopped.Missing || stopped.Degraded || stopped.Failure != DiagnosticCanceled {
		t.Fatal("context stop erased observation or claimed read failure", stopped)
	}
	for _, status := range []SelectionStatus{SelectionAbsent, SelectionDifferent, SelectionInsufficient, SelectionAmbiguous, SelectionLimit} {
		err := followLocationError(status)
		got := Diagnose("", err)
		wantGap := status == SelectionAbsent || status == SelectionDifferent
		if got.Gap != wantGap || !got.Degraded || got.Missing || got.Path != "" {
			t.Fatal("scan evidence mislabeled", status, got)
		}
		if wantGap && got.Failure != DiagnosticGap || !wantGap && got.Failure != DiagnosticDecision {
			t.Fatal("incomplete/ambiguous scan became gap", status, got)
		}
	}
	for _, changed := range []error{ErrFileTruncated, ErrCheckpointChanged} {
		got := Diagnose(PathSame, changed)
		if got.Gap || !got.Degraded || got.Failure != DiagnosticChanged || got.Missing {
			t.Fatal("change mislabeled as missing or completed-search gap", got)
		}
	}
	for _, tc := range []struct {
		err     error
		cause   DiagnosticFailure
		missing bool
	}{
		{ErrCurrentMissing, DiagnosticMissing, true},
		{ErrRotationCapacity, DiagnosticCapacity, false},
		{&RecoveryDecisionError{Status: RecoveryOriginStatus("private synthetic")}, DiagnosticDecision, false},
		{fs.ErrPermission, DiagnosticError, false},
		{io.EOF, DiagnosticError, false},         // dependency EOF is an error, not healthy completion
		{fs.ErrNotExist, DiagnosticError, false}, // a DB/temp/dependency path may be absent
	} {
		got := Diagnose("", tc.err)
		if !got.Degraded || got.Gap || got.Missing != tc.missing || got.Failure != tc.cause {
			t.Fatal("unproven diagnosis", got)
		}
	}
}

func TestFileDiagnosticJoinedCancellationPreservesRealFailureWithoutPII(t *testing.T) {
	private := "synthetic-private@invalid.example /synthetic/private/origin <script>"
	for _, tc := range []struct {
		err          error
		cause        DiagnosticFailure
		gap, missing bool
	}{
		{fmt.Errorf(private+": %w", context.Canceled), DiagnosticCanceled, false, false},
		{fmt.Errorf(private+": %w", context.DeadlineExceeded), DiagnosticDeadline, false, false},
		{errors.Join(context.Canceled, fmt.Errorf(private+": %w", fs.ErrPermission)), DiagnosticError, false, false},
		{errors.Join(context.DeadlineExceeded, fmt.Errorf(private+": %w", ErrFollowResumeGap)), DiagnosticGap, true, false},
		{errors.Join(context.Canceled, ErrCurrentMissing, ErrRotationCapacity), DiagnosticCapacity, false, true},
		{errors.Join(context.Canceled, &FollowResumeGapError{Status: SelectionStatus(private)}, ErrCheckpointChanged), DiagnosticGap, true, false},
		{errors.New(private), DiagnosticError, false, false},
	} {
		got := Diagnose(PathSame, tc.err)
		if got.Failure != tc.cause || got.Gap != tc.gap || got.Missing != tc.missing ||
			got.Degraded != (tc.cause != DiagnosticCanceled && tc.cause != DiagnosticDeadline) {
			t.Fatal("mixed stop swallowed evidence", got)
		}
		data, err := json.Marshal(got)
		if err != nil || strings.Contains(string(data), private) || strings.Contains(string(data), "private") {
			t.Fatal("diagnostic leaked input", string(data), err)
		}
	}
	got := Diagnose(PathStatus(private), nil)
	if got.Path != "" || got.Failure != DiagnosticError || !got.Degraded || got.Missing || got.Gap {
		t.Fatal("arbitrary path label echoed or accepted", got)
	}
}

type diagnosticCycle struct{ calls *int }

func (d diagnosticCycle) Error() string { panic("diagnostic must not inspect error strings") }
func (d diagnosticCycle) Unwrap() error { *d.calls++; return d }

type diagnosticBranches struct {
	causes []error
	calls  *int
}

func (d diagnosticBranches) Error() string   { panic("diagnostic must not inspect error strings") }
func (d diagnosticBranches) Unwrap() []error { *d.calls++; return d.causes }

func TestFileDiagnosticBoundsMalformedErrorTrees(t *testing.T) {
	calls := 0
	got := Diagnose("", diagnosticCycle{calls: &calls})
	if calls != maxDiagnosticErrorNodes || got.Failure != DiagnosticError || !got.Degraded {
		t.Fatal("cycle traversal unbounded", calls, got)
	}
	calls = 0
	causes := make([]error, maxDiagnosticErrorNodes*2)
	// Nil children also consume the traversal budget; do not scan arbitrary
	// amounts of malformed empty branches or reach a later private failure.
	causes[len(causes)-1] = diagnosticCycle{calls: &calls}
	got = Diagnose("", diagnosticBranches{causes: causes, calls: &calls})
	if calls != 1 || got.Failure != DiagnosticError || !got.Degraded || got.Gap {
		t.Fatal("wide malformed traversal unbounded", calls, got)
	}
	calls = 0
	got = Diagnose("", diagnosticBranches{causes: []error{nil, nil}, calls: &calls})
	if calls != 1 || !got.Degraded || got.Failure != DiagnosticError {
		t.Fatal("empty wrapper hid nonnil failure", got)
	}
}

func TestFileDiagnosticIncompleteTraversalOverridesRecognizedPriority(t *testing.T) {
	for _, tc := range []struct {
		path         PathStatus
		err          func(*int) error
		gap, missing bool
	}{
		{"", func(c *int) error { return errors.Join(ErrFollowResumeGap, diagnosticCycle{calls: c}) }, true, false},
		{"", func(c *int) error { return errors.Join(ErrCurrentMissing, diagnosticCycle{calls: c}) }, false, true},
		{"", func(c *int) error { return errors.Join(ErrRotationCapacity, diagnosticBranches{calls: c}) }, false, false},
		{PathStatus("private synthetic label"), func(*int) error { return ErrFollowResumeGap }, true, false},
	} {
		calls := 0
		got := Diagnose(tc.path, tc.err(&calls))
		if got.Failure != DiagnosticError || !got.Degraded || got.Gap != tc.gap || got.Missing != tc.missing || got.Path != "" {
			t.Fatal("unverified input retained a confident failure label", got)
		}
	}
}
