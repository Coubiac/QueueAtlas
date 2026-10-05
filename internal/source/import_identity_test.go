package source

import (
	"errors"
	"strings"
	"testing"
)

const emptyContentSHA = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestImportOriginIDStableAndSourceScoped(t *testing.T) {
	got, err := ImportOriginID("archive", emptyContentSHA)
	const want = "import-v1:ee0ac0e8f0e3dda54e4d84e1bb6bb62b4bb219991e60966316da1a7ff4df8996"
	if err != nil || got != want {
		t.Fatalf("durable identity = %q, %v; want %q", got, err, want)
	}
	seen := map[string]bool{got: true}
	for _, id := range []string{"other", "archive ", " archive", "Archive", "arch\x00ive", "archives-é"} {
		other, err := ImportOriginID(id, emptyContentSHA)
		if err != nil || seen[other] {
			t.Fatalf("source %q collided or failed: %q, %v", id, other, err)
		}
		seen[other] = true
		again, err := ImportOriginID(id, emptyContentSHA)
		if err != nil || again != other {
			t.Fatalf("source %q changed on retry: %q, %v", id, again, err)
		}
	}
	changed, err := ImportOriginID("archive", strings.Repeat("0", 64))
	if err != nil || seen[changed] {
		t.Fatalf("different digest collided or failed: %q, %v", changed, err)
	}
}

func TestImportOriginIDRejectsNoncanonicalDigest(t *testing.T) {
	for _, digest := range []string{"", emptyContentSHA[:63], emptyContentSHA + "0",
		strings.ToUpper(emptyContentSHA), "sha256:" + emptyContentSHA,
		" " + emptyContentSHA[1:], emptyContentSHA[:63] + "g", strings.Repeat("é", 32)} {
		got, err := ImportOriginID("archive", digest)
		if got != "" || !errors.Is(err, ErrImportIdentity) {
			t.Fatalf("invalid digest returned %q, %v", got, err)
		}
		if strings.Contains(err.Error(), digest) && len(digest) > 2 {
			t.Fatal("error disclosed the supplied digest")
		}
	}
	got, err := ImportOriginID("", emptyContentSHA)
	if got != "" || !errors.Is(err, ErrImportIdentity) {
		t.Fatalf("empty source returned %q, %v", got, err)
	}
}
