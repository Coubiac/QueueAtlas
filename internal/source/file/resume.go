package file

import (
	"errors"
	"io"
	"os"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

type ResumeStatus string

const (
	ResumeMatch        ResumeStatus = "match"
	ResumeDifferent    ResumeStatus = "different"
	ResumeInsufficient ResumeStatus = "insufficient"
	ResumeRestartZero  ResumeStatus = "restart_zero"
)

// ResumePolicy explicitly permits replay from zero with physical identity and
// nonempty prefix evidence, despite the absence of a positive checkpoint anchor.
// The zero value remains strict. Replay is not a claim of anchored generation
// continuity and retains the bounded, non-atomic prefix guarantee.
type ResumePolicy struct {
	AllowZeroCheckpoint bool
}

// ResumeReason is a fixed diagnostic code; it never contains log content.
type ResumeReason string

const (
	ReasonFingerprintsMatch   ResumeReason = "fingerprints_match"
	ReasonInvalidState        ResumeReason = "invalid_state"
	ReasonIdentityUnavailable ResumeReason = "physical_identity_unavailable"
	ReasonPhysicalChanged     ResumeReason = "physical_identity_changed"
	ReasonInvalidPrefix       ResumeReason = "invalid_prefix"
	ReasonMissingCheckpoint   ResumeReason = "missing_checkpoint"
	ReasonInvalidAnchor       ResumeReason = "invalid_anchor"
	ReasonEmptyPrefix         ResumeReason = "empty_prefix"
	ReasonZeroCheckpoint      ResumeReason = "zero_checkpoint"
	ReasonZeroReplay          ResumeReason = "zero_checkpoint_replay"
	ReasonFileShortened       ResumeReason = "file_shortened"
	ReasonPrefixChanged       ResumeReason = "prefix_changed"
	ReasonAnchorChanged       ResumeReason = "anchor_changed"
	ReasonInvalidBoundary     ResumeReason = "not_line_boundary"
)

type ResumeCheck struct {
	Status ResumeStatus
	Reason ResumeReason
}

// VerifyCandidate checks one persisted origin against an open regular file,
// without seeking or selecting a generation. Match means that physical IDs,
// both bounded fingerprints and the checkpoint line boundary agree; changes
// outside the windows and concurrent rewrites remain outside this guarantee.
// Invalid or incomplete state yields Insufficient, whereas read failures return
// an error. The caller must retain the source namespace from StateReader.
func VerifyCandidate(f *os.File, state source.OriginState) (ResumeCheck, error) {
	return VerifyCandidateWithPolicy(f, state, ResumePolicy{})
}

// VerifyCandidateWithPolicy also reports RestartZero when explicitly permitted.
// It still requires a present, consistent checkpoint and canonical empty anchor,
// matching physical identity and matching nonempty prefix. It does not seek.
func VerifyCandidateWithPolicy(f *os.File, state source.OriginState, policy ResumePolicy) (ResumeCheck, error) {
	id, err := Inspect(f)
	if err != nil {
		return ResumeCheck{}, err
	}
	o := state.Origin
	p := state.Checkpoint
	if o.ID == "" || p != nil && (p.OriginID != o.ID || p.Offset < 0) {
		return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonInvalidState}, nil
	}
	if id.Device == "" || id.Inode == "" || o.Device == "" || o.Inode == "" {
		return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonIdentityUnavailable}, nil
	}
	if id.Device != o.Device || id.Inode != o.Inode {
		return ResumeCheck{Status: ResumeDifferent, Reason: ReasonPhysicalChanged}, nil
	}
	prefix, err := ParsePrefixFingerprint(o.Fingerprint)
	if err != nil {
		return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonInvalidPrefix}, nil
	}
	if p == nil {
		return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonMissingCheckpoint}, nil
	}
	anchor, err := ParseCheckpointAnchor(p.AnchorHash)
	if err != nil {
		return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonInvalidAnchor}, nil
	}
	if anchor.Offset != p.Offset {
		return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonInvalidState}, nil
	}
	if prefix.Length == 0 {
		return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonEmptyPrefix}, nil
	}
	if p.Offset == 0 {
		if !policy.AllowZeroCheckpoint {
			return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonZeroCheckpoint}, nil
		}
		match, err := prefix.Matches(f)
		if err != nil {
			return ResumeCheck{}, err
		}
		if !match {
			return ResumeCheck{Status: ResumeDifferent, Reason: ReasonPrefixChanged}, nil
		}
		return ResumeCheck{Status: ResumeRestartZero, Reason: ReasonZeroReplay}, nil
	}
	if p.Offset > id.Size {
		return ResumeCheck{Status: ResumeDifferent, Reason: ReasonFileShortened}, nil
	}
	match, err := prefix.Matches(f)
	if err != nil {
		return ResumeCheck{}, err
	}
	if !match {
		return ResumeCheck{Status: ResumeDifferent, Reason: ReasonPrefixChanged}, nil
	}
	match, err = anchor.Matches(f)
	if err != nil {
		return ResumeCheck{}, err
	}
	if !match {
		return ResumeCheck{Status: ResumeDifferent, Reason: ReasonAnchorChanged}, nil
	}
	var separator [1]byte
	n, err := f.ReadAt(separator[:], p.Offset-1)
	if err != nil && !errors.Is(err, io.EOF) {
		return ResumeCheck{}, err
	}
	if n != 1 {
		return ResumeCheck{Status: ResumeDifferent, Reason: ReasonFileShortened}, nil
	}
	if separator[0] != '\n' {
		return ResumeCheck{Status: ResumeInsufficient, Reason: ReasonInvalidBoundary}, nil
	}
	return ResumeCheck{Status: ResumeMatch, Reason: ReasonFingerprintsMatch}, nil
}
