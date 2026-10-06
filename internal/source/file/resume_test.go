package file

import (
	"os"
	"runtime"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestResumeCandidateReportsFileErrors(t *testing.T) {
	if _, err := VerifyCandidate(nil, source.OriginState{}); err == nil {
		t.Fatal("nil descriptor accepted")
	}
	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	if _, err := VerifyCandidate(directory, source.OriginState{}); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestResumeCandidateWithoutSupportedPhysicalIdentity(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux exposes persistent device/inode identity")
	}
	f, _ := testRegularFile(t, "line\n")
	state := source.OriginState{Origin: source.Origin{ID: "gen-1", Device: "1", Inode: "2"}}
	check, err := VerifyCandidate(f, state)
	if err != nil || check.Status != ResumeInsufficient || check.Reason != ReasonIdentityUnavailable {
		t.Fatalf("unsupported identity: %+v, %v", check, err)
	}
}
