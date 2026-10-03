package file

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testRegularFile(t *testing.T, contents string) (*os.File, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mail.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	if _, err := f.WriteString(contents); err != nil {
		t.Fatal(err)
	}
	return f, path
}

func TestIdentitySurvivesRenameButNotReplacement(t *testing.T) {
	f, path := testRegularFile(t, "same prefix\n")
	before, err := Inspect(f)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" && (before.Device == "" || before.Inode == "") {
		t.Fatal("Linux device/inode identifiers missing")
	}
	if (Identity{}).SameFile(before) || before.SameFile(Identity{}) {
		t.Fatal("empty identity matched a file")
	}
	// Go's normal Windows file handles prevent renaming while open. Keep the
	// descriptor open on Linux, the target platform for active log rotation.
	if runtime.GOOS == "windows" {
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	rotated, err := os.Open(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	defer rotated.Close()
	after, err := Inspect(rotated)
	if err != nil || !before.SameFile(after) {
		t.Fatalf("rename changed physical identity: %+v, %v", after, err)
	}
	replacement, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	if _, err := replacement.WriteString("same prefix\n"); err != nil {
		t.Fatal(err)
	}
	newID, err := Inspect(replacement)
	if err != nil || before.SameFile(newID) {
		t.Fatalf("replacement retained physical identity: %+v, %v", newID, err)
	}
	// Identical prefix bytes are insufficient evidence for the same file.
	prefix, err := CapturePrefix(rotated)
	if err != nil {
		t.Fatal(err)
	}
	if match, err := prefix.Matches(replacement); err != nil || !match {
		t.Fatalf("identical content should have identical prefix: %v, %v", match, err)
	}
}

func TestPrefixUsesSavedWindowAfterAppendWithoutSeeking(t *testing.T) {
	contents := "initial\n"
	f, _ := testRegularFile(t, contents)
	if _, err := f.Seek(3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	prefix, err := CapturePrefix(f)
	if err != nil || prefix.Length != len(contents) || prefix.Digest != sha256.Sum256([]byte(contents)) {
		t.Fatalf("prefix = %+v, %v", prefix, err)
	}
	before, err := Inspect(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte(strings.Repeat("x", MaxFingerprintBytes)), int64(len(contents))); err != nil {
		t.Fatal(err)
	}
	if match, err := prefix.Matches(f); err != nil || !match {
		t.Fatalf("append changed saved prefix: %v, %v", match, err)
	}
	after, err := Inspect(f)
	if err != nil || !before.SameFile(after) || after.Size != int64(len(contents)+MaxFingerprintBytes) {
		t.Fatalf("append changed identity or size: %+v, %v", after, err)
	}
	if pos, err := f.Seek(0, io.SeekCurrent); err != nil || pos != 3 {
		t.Fatalf("fingerprinting moved read position to %d: %v", pos, err)
	}
}

func TestPrefixDetectsChangesAndTruncation(t *testing.T) {
	f, _ := testRegularFile(t, "initial\n")
	prefix, err := CapturePrefix(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("changed"), 0); err != nil {
		t.Fatal(err)
	}
	if match, err := prefix.Matches(f); err != nil || match {
		t.Fatalf("modified prefix matched: %v, %v", match, err)
	}
	if _, err := f.WriteAt([]byte("initial\n"), 0); err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(2); err != nil {
		t.Fatal(err)
	}
	if match, err := prefix.Matches(f); err != nil || match {
		t.Fatalf("shortened file matched: %v, %v", match, err)
	}
}

func TestFingerprintWindowIsBoundedAndEmptyWindowNeverMatches(t *testing.T) {
	f, _ := testRegularFile(t, strings.Repeat("x", 2*MaxFingerprintBytes))
	prefix, err := CapturePrefix(f)
	if err != nil || prefix.Length != MaxFingerprintBytes {
		t.Fatalf("prefix exceeded its window: %+v, %v", prefix, err)
	}
	if _, err := f.WriteAt([]byte("changed"), MaxFingerprintBytes); err != nil {
		t.Fatal(err)
	}
	if match, err := prefix.Matches(f); err != nil || !match {
		t.Fatalf("bytes outside the window changed the fingerprint: %v, %v", match, err)
	}
	empty, _ := testRegularFile(t, "")
	prefix, err = CapturePrefix(empty)
	if err != nil || prefix.Length != 0 {
		t.Fatalf("empty file fingerprint: %+v, %v", prefix, err)
	}
	if match, err := prefix.Matches(empty); err != nil || match {
		t.Fatalf("empty prefix claimed identity evidence: %v, %v", match, err)
	}
	for _, length := range []int{-1, MaxFingerprintBytes + 1} {
		invalid := PrefixFingerprint{Length: length}
		if _, err := invalid.Matches(f); err == nil {
			t.Fatalf("invalid fingerprint window %d accepted", length)
		}
	}
}

func TestInspectRejectsNonRegularInput(t *testing.T) {
	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	if _, err := Inspect(directory); err == nil {
		t.Fatal("directory accepted as a log file")
	}
	if _, err := CapturePrefix(directory); err == nil {
		t.Fatal("directory accepted for fingerprinting")
	}
	if _, err := Inspect(nil); err == nil {
		t.Fatal("nil file accepted")
	}
}
