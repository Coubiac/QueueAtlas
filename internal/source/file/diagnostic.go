package file

import "context"

type DiagnosticFailure string

const (
	DiagnosticNone     DiagnosticFailure = "none"
	DiagnosticCanceled DiagnosticFailure = "canceled"
	DiagnosticDeadline DiagnosticFailure = "deadline"
	DiagnosticError    DiagnosticFailure = "error"
	DiagnosticMissing  DiagnosticFailure = "missing"
	DiagnosticCapacity DiagnosticFailure = "capacity"
	DiagnosticDecision DiagnosticFailure = "decision_required"
	DiagnosticChanged  DiagnosticFailure = "content_changed"
	DiagnosticGap      DiagnosticFailure = "resume_gap"
)

// Diagnostic has a fixed vocabulary suitable for metric labels and a future
// doctor command. It contains no paths, origin IDs, offsets or error/log strings.
// Missing describes a reported path observation or explicit startup refusal;
// Gap describes reported failure of verified continuity, not proven lost bytes.
// Degraded means a non-context failure was reported. A missing current while an
// older descriptor is still readable need not be degraded.
type Diagnostic struct {
	Path     PathStatus
	Failure  DiagnosticFailure
	Missing  bool
	Gap      bool
	Degraded bool
}

const maxDiagnosticErrorNodes = 64

// Diagnose summarizes an observation and returned error without reading inputs,
// state writes or error-string inspection. LastPathStatus can be stale; separate
// observation/error reads are not an atomic snapshot or proof of running health.
// Typed causes can come from dependencies, so labels are reports, never authority
// to skip bytes/recover a generation. Insufficient, ambiguous or bounded scans
// are decisions, not gaps. Content changes alone do not prove a resume gap.
// Mixed cancellation+failure preserves the failure. Unknown path labels and
// malformed/too deep error trees degrade to fixed error, never echo input.
// Custom Unwrap methods must terminate; at most 64 error nodes are inspected.
func Diagnose(path PathStatus, err error) Diagnostic {
	d := Diagnostic{Failure: DiagnosticNone}
	priority := 0
	unverified := false
	mark := func(cause DiagnosticFailure, rank int) {
		if rank > priority {
			d.Failure, priority = cause, rank
		}
		if rank >= 3 {
			d.Degraded = true
		}
	}
	switch path {
	case "", PathSame, PathMissing, PathReplaced:
		d.Path, d.Missing = path, path == PathMissing
	default:
		unverified = true
		mark(DiagnosticError, 3)
	}
	budget := maxDiagnosticErrorNodes
	var visit func(error)
	visit = func(e error) {
		if budget == 0 {
			unverified = true
			mark(DiagnosticError, 3)
			return
		}
		budget--
		if e == nil {
			return
		}
		switch e {
		case context.Canceled:
			mark(DiagnosticCanceled, 1)
			return
		case context.DeadlineExceeded:
			mark(DiagnosticDeadline, 2)
			return
		case ErrFollowResumeGap:
			d.Gap = true
			mark(DiagnosticGap, 8)
			return
		case ErrCheckpointChanged, ErrFileTruncated:
			mark(DiagnosticChanged, 7)
			return
		case ErrCurrentMissing:
			d.Missing = true
			mark(DiagnosticMissing, 4)
			return
		case ErrRotationCapacity:
			mark(DiagnosticCapacity, 5)
			return
		}
		switch e.(type) {
		case *FollowResumeGapError:
			d.Gap = true
			mark(DiagnosticGap, 8)
			return
		case *ResumeDecisionError, *RecoveryDecisionError:
			mark(DiagnosticDecision, 6)
			return
		}
		if joined, ok := e.(interface{ Unwrap() []error }); ok {
			causes := joined.Unwrap()
			some := false
			for _, cause := range causes {
				if budget == 0 {
					unverified = true
					mark(DiagnosticError, 3)
					break
				}
				if cause != nil {
					some = true
				}
				visit(cause)
			}
			if some {
				return
			}
			unverified = true
		} else if wrapped, ok := e.(interface{ Unwrap() error }); ok {
			if cause := wrapped.Unwrap(); cause != nil {
				visit(cause)
				return
			}
			unverified = true
		}
		mark(DiagnosticError, 3)
	}
	visit(err)
	if unverified {
		d.Failure, d.Degraded = DiagnosticError, true
	}
	return d
}
