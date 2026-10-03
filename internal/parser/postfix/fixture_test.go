package postfix

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSyntheticCorpus exercises the parser against independently written
// fixtures. Only the explicitly malformed or oversized scenarios may fail
// envelope parsing; every other line must remain readable as a raw record.
func TestSyntheticCorpus(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "testdata", "postfix")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 25 {
		t.Fatalf("reference corpus unexpectedly small: %d files", len(entries))
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			file, err := os.Open(filepath.Join(dir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			reader := bufio.NewReader(file)
			lines := 0
			for {
				line, readErr := reader.ReadBytes('\n')
				if len(line) > 0 {
					lines++
					got := Parse(line, Options{SourceID: "fixture"})
					if got.SourceID != "fixture" {
						t.Fatalf("line %d lost provenance", lines)
					}
					allowedError := strings.HasPrefix(entry.Name(), "26-") ||
						strings.HasPrefix(entry.Name(), "29-") ||
						strings.HasPrefix(entry.Name(), "30-")
					if got.ParseError != "" && !allowedError {
						t.Fatalf("line %d unexpectedly rejected: %s", lines, got.ParseError)
					}
				}
				if readErr == io.EOF {
					break
				}
				if readErr != nil {
					t.Fatal(readErr)
				}
			}
			if lines == 0 {
				t.Fatal("empty fixture")
			}
		})
	}
}
