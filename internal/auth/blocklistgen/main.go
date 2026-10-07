// Command blocklistgen imports the pinned public SecLists subset for enrollment.
// Maintainer tool only: it reads a local source, never fetches or receives secrets.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Coubiac/QueueAtlas/internal/auth"
)

const (
	sourceSHA256 = "424a3e03a17df0a2bc2b3ca749d81b04e79d59cb7aeec8876a5a3f308d0caf51"
	sourceSize   = 8557632
	sourceLines  = 1000000
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run ./internal/auth/blocklistgen <pinned-source-file> <output-file>")
		os.Exit(2)
	}
	if err := generate(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(source, output string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("cannot open corpus source")
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, sourceSize+1))
	if err != nil || len(raw) != sourceSize {
		return fmt.Errorf("unexpected corpus source size or read failure")
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != sourceSHA256 {
		return fmt.Errorf("corpus source checksum mismatch")
	}
	lines := bytes.Split(bytes.TrimSuffix(raw, []byte{'\n'}), []byte{'\n'})
	if len(lines) != sourceLines {
		return fmt.Errorf("unexpected corpus source line count")
	}
	unique := make(map[string]struct{})
	eligible := 0
	for _, line := range lines {
		value := bytes.TrimSuffix(line, []byte{'\r'})
		if auth.ValidatePassword(value) != nil {
			continue
		}
		eligible++
		candidate := strings.ToLower(strings.TrimSpace(string(value)))
		digest := sha256.Sum256([]byte(candidate))
		unique[hex.EncodeToString(digest[:])] = struct{}{}
	}
	entries := make([]string, 0, len(unique))
	for key := range unique {
		entries = append(entries, key)
	}
	sort.Strings(entries)
	result := []byte(strings.Join(entries, "\n") + "\n")
	if err := os.WriteFile(output, result, 0644); err != nil {
		return fmt.Errorf("cannot write corpus output")
	}
	resultDigest := sha256.Sum256(result)
	fmt.Printf("source lines=%d eligible=%d unique=%d output bytes=%d sha256=%x\n", len(lines), eligible, len(entries), len(result), resultDigest)
	return nil
}
