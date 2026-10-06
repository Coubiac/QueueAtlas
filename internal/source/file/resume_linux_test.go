//go:build linux

package file

import (
	"crypto/sha256"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func resumeFixture(t *testing.T) (*os.File, source.OriginState) {
	t.Helper()
	f, path := testRegularFile(t, strings.Repeat("header\n", 1024)+strings.Repeat("tail\n", 1024))
	id, err := Inspect(f)
	if err != nil {
		t.Fatal(err)
	}
	prefix, err := CapturePrefix(f)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := CaptureAnchor(f, id.Size)
	if err != nil {
		t.Fatal(err)
	}
	return f, source.OriginState{
		Origin:     source.Origin{ID: "gen-1", Path: path, Device: id.Device, Inode: id.Inode, Fingerprint: prefix.String()},
		Checkpoint: &source.Position{OriginID: "gen-1", Offset: id.Size, AnchorHash: anchor.String()},
	}
}

func TestResumeCandidateAfterAppendAndWithRenamedPath(t *testing.T) {
	f, state := resumeFixture(t)
	state.Origin.Path += ".1" // the old path is not evidence of physical identity
	if _, err := f.WriteAt([]byte("next\n"), state.Checkpoint.Offset); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	check, err := VerifyCandidate(f, state)
	if err != nil || check.Status != ResumeMatch || check.Reason != ReasonFingerprintsMatch {
		t.Fatalf("candidate after append: %+v, %v", check, err)
	}
	if pos, err := f.Seek(0, io.SeekCurrent); err != nil || pos != 3 {
		t.Fatalf("verification moved read position: %d, %v", pos, err)
	}
}

func TestResumeCandidateRejectsChangedOrIncompleteEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*os.File, *source.OriginState) error
		status ResumeStatus
		reason ResumeReason
	}{
		{"physical", func(_ *os.File, s *source.OriginState) error { s.Origin.Inode += "9"; return nil }, ResumeDifferent, ReasonPhysicalChanged},
		{"prefix", func(f *os.File, _ *source.OriginState) error { _, err := f.WriteAt([]byte("X"), 0); return err }, ResumeDifferent, ReasonPrefixChanged},
		{"anchor", func(f *os.File, s *source.OriginState) error {
			_, err := f.WriteAt([]byte("X"), s.Checkpoint.Offset-2)
			return err
		}, ResumeDifferent, ReasonAnchorChanged},
		{"truncated", func(f *os.File, s *source.OriginState) error { return f.Truncate(s.Checkpoint.Offset - 1) }, ResumeDifferent, ReasonFileShortened},
		{"missing checkpoint", func(_ *os.File, s *source.OriginState) error { s.Checkpoint = nil; return nil }, ResumeInsufficient, ReasonMissingCheckpoint},
		{"zero checkpoint", func(f *os.File, s *source.OriginState) error {
			a, err := CaptureAnchor(f, 0)
			s.Checkpoint.Offset, s.Checkpoint.AnchorHash = 0, a.String()
			return err
		}, ResumeInsufficient, ReasonZeroCheckpoint},
		{"empty prefix", func(_ *os.File, s *source.OriginState) error {
			s.Origin.Fingerprint = (PrefixFingerprint{Digest: sha256.Sum256(nil)}).String()
			return nil
		}, ResumeInsufficient, ReasonEmptyPrefix},
		{"invalid prefix", func(_ *os.File, s *source.OriginState) error { s.Origin.Fingerprint = "bad"; return nil }, ResumeInsufficient, ReasonInvalidPrefix},
		{"invalid anchor", func(_ *os.File, s *source.OriginState) error { s.Checkpoint.AnchorHash = "bad"; return nil }, ResumeInsufficient, ReasonInvalidAnchor},
		{"origin ID", func(_ *os.File, s *source.OriginState) error { s.Checkpoint.OriginID = "other"; return nil }, ResumeInsufficient, ReasonInvalidState},
		{"negative offset", func(_ *os.File, s *source.OriginState) error { s.Checkpoint.Offset = -1; return nil }, ResumeInsufficient, ReasonInvalidState},
		{"inconsistent offset", func(_ *os.File, s *source.OriginState) error { s.Checkpoint.Offset--; return nil }, ResumeInsufficient, ReasonInvalidState},
		{"line boundary", func(f *os.File, s *source.OriginState) error {
			s.Checkpoint.Offset--
			a, err := CaptureAnchor(f, s.Checkpoint.Offset)
			s.Checkpoint.AnchorHash = a.String()
			return err
		}, ResumeInsufficient, ReasonInvalidBoundary},
		{"missing identity", func(_ *os.File, s *source.OriginState) error { s.Origin.Device = ""; return nil }, ResumeInsufficient, ReasonIdentityUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, state := resumeFixture(t)
			if err := tc.change(f, &state); err != nil {
				t.Fatal(err)
			}
			check, err := VerifyCandidate(f, state)
			if err != nil || check.Status != tc.status || check.Reason != tc.reason {
				t.Fatalf("check = %+v, %v; want %s/%s", check, err, tc.status, tc.reason)
			}
		})
	}
}
