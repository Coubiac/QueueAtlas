package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRejectUnpinnedSourcePreservesOutput(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "synthetic-source")
	output := filepath.Join(dir, "previous-output")
	const previous = "synthetic previously reviewed output\n"
	for _, tc := range []struct {
		data []byte
		want string
	}{
		{[]byte("synthetic source\n"), "unexpected corpus source size or read failure"},
		{bytes.Repeat([]byte{'x'}, sourceSize), "corpus source checksum mismatch"},
	} {
		if err := os.WriteFile(source, tc.data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, []byte(previous), 0600); err != nil {
			t.Fatal(err)
		}
		if err := generate(source, output); err == nil || err.Error() != tc.want {
			t.Fatal("unpinned source accepted or wrong diagnostic", err)
		}
		if data, err := os.ReadFile(output); err != nil || string(data) != previous {
			t.Fatal("failed import changed reviewed output", err)
		}
	}
}
