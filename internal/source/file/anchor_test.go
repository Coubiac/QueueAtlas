package file

import (
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestAnchorSurvivesAppendAndDetectsTailChange(t *testing.T) {
	contents := strings.Repeat("head", MaxFingerprintBytes) + "tail\n"
	f, _ := testRegularFile(t, contents)
	if _, err := f.Seek(3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	anchor, err := CaptureAnchor(f, int64(len(contents)))
	if err != nil || anchor.Length != MaxFingerprintBytes || anchor.Digest != sha256.Sum256([]byte(contents[len(contents)-MaxFingerprintBytes:])) {
		t.Fatalf("capture: %+v, %v", anchor, err)
	}
	saved, err := ParseCheckpointAnchor(anchor.String())
	if err != nil || saved != anchor {
		t.Fatalf("parse saved anchor: %+v, %v", saved, err)
	}
	if _, err := f.WriteAt([]byte("more\n"), int64(len(contents))); err != nil {
		t.Fatal(err)
	}
	if match, err := saved.Matches(f); err != nil || !match {
		t.Fatalf("append changed anchor: %v, %v", match, err)
	}
	if pos, err := f.Seek(0, io.SeekCurrent); err != nil || pos != 3 {
		t.Fatalf("anchor read moved position: %d, %v", pos, err)
	}
	if _, err := f.WriteAt([]byte("X"), 0); err != nil {
		t.Fatal(err)
	}
	if match, err := saved.Matches(f); err != nil || !match {
		t.Fatalf("bytes outside the anchor window changed its digest: %v, %v", match, err)
	}
	if _, err := f.WriteAt([]byte("X"), int64(len(contents)-2)); err != nil {
		t.Fatal(err)
	}
	if match, err := saved.Matches(f); err != nil || match {
		t.Fatalf("modified anchor window matched: %v, %v", match, err)
	}
}

func TestAnchorShortWindowAndTruncation(t *testing.T) {
	f, _ := testRegularFile(t, "line\n")
	anchor, err := CaptureAnchor(f, 5)
	if err != nil || anchor.Length != 5 || anchor.Digest != sha256.Sum256([]byte("line\n")) {
		t.Fatalf("short anchor: %+v, %v", anchor, err)
	}
	if err := f.Truncate(4); err != nil {
		t.Fatal(err)
	}
	if match, err := anchor.Matches(f); err != nil || match {
		t.Fatalf("truncated file matched: %v, %v", match, err)
	}
	if _, err := CaptureAnchor(f, 5); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("captured incomplete window: %v", err)
	}
	if _, err := CaptureAnchor(f, -1); err == nil {
		t.Fatal("negative anchor offset accepted")
	}
}

func TestZeroAnchorIsValidButProvidesNoEvidence(t *testing.T) {
	f, _ := testRegularFile(t, "")
	anchor, err := CaptureAnchor(f, 0)
	if err != nil || anchor.Length != 0 || anchor.Offset != 0 || anchor.Digest != sha256.Sum256(nil) {
		t.Fatalf("zero anchor: %+v, %v", anchor, err)
	}
	if _, err := ParseCheckpointAnchor(anchor.String()); err != nil {
		t.Fatal(err)
	}
	if match, err := anchor.Matches(f); err != nil || match {
		t.Fatalf("zero anchor claimed evidence: %v, %v", match, err)
	}
}

func TestParsePersistedFingerprints(t *testing.T) {
	f, _ := testRegularFile(t, "line\n")
	prefix, err := CapturePrefix(f)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := ParsePrefixFingerprint(prefix.String())
	if err != nil || saved != prefix {
		t.Fatalf("prefix round trip: %+v, %v", saved, err)
	}
	if match, err := saved.Matches(f); err != nil || !match {
		t.Fatalf("persisted prefix no longer matches: %v, %v", match, err)
	}
	empty := PrefixFingerprint{Digest: sha256.Sum256(nil)}
	if _, err := ParsePrefixFingerprint(empty.String()); err != nil {
		t.Fatal(err)
	}
	hexDigest := strings.Repeat("a", 64)
	for _, value := range []string{
		"", "sha512:5:" + hexDigest, "sha256:-1:" + hexDigest,
		"sha256:4097:" + hexDigest, "sha256:99999999999999999999999999:" + hexDigest,
		"sha256:05:" + hexDigest, "sha256:+5:" + hexDigest,
		"sha256:5:" + strings.ToUpper(hexDigest), "sha256:5:" + strings.Repeat("g", 64),
		"sha256:5:abc", "sha256:0:" + hexDigest, prefix.String() + ":extra",
		strings.Repeat("x", 129),
	} {
		if _, err := ParsePrefixFingerprint(value); err == nil {
			t.Fatalf("invalid prefix accepted: %q", value)
		}
	}
	for _, value := range []string{
		"", "sha512:5:5:" + hexDigest, "sha256:-1:5:" + hexDigest,
		"sha256:9223372036854775808:4096:" + hexDigest,
		"sha256:5:6:" + hexDigest, "sha256:5:-1:" + hexDigest,
		"sha256:4097:4097:" + hexDigest, "sha256:5000:4095:" + hexDigest,
		"sha256:05:5:" + hexDigest, "sha256:5:05:" + hexDigest,
		"sha256:5:5:" + strings.ToUpper(hexDigest), "sha256:5:5:" + strings.Repeat("g", 64),
		"sha256:5:5:abc", "sha256:0:0:" + hexDigest, "sha256:5:5:" + hexDigest + ":extra",
		strings.Repeat("x", 129),
	} {
		if _, err := ParseCheckpointAnchor(value); err == nil {
			t.Fatalf("invalid anchor accepted: %q", value)
		}
	}
}
