package auth

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func syntheticAccount() LocalAccount {
	return LocalAccount{Identity: LocalIdentity{Username: "synthetic-admin"}, PasswordHash: syntheticPasswordHash}
}

func syntheticAccountJSON() string {
	return `{"version":1,"username":"synthetic-admin","password_hash":"` + syntheticPasswordHash + `"}`
}

// testing.TempDir uses 0777 subject to umask for its numbered subdirectory.
// Account fixtures must explicitly supply the private directory required by IO.
func privateAccountTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLocalAccountCreateLoadAndNoReplacement(t *testing.T) {
	dir := privateAccountTestDir(t)
	a := syntheticAccount()
	if err := CreateLocalAccount(dir, a); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, LocalAccountFilename)
	before, err := os.ReadFile(path)
	if err != nil || string(before) != syntheticAccountJSON()+"\n" {
		t.Fatal("unexpected persisted record", err)
	}
	got, err := LoadLocalAccount(dir)
	if err != nil || got != a {
		t.Fatal("account did not round trip", err)
	}
	got.Identity.Username, got.PasswordHash = "changed", "changed"
	loaded, err := LoadLocalAccount(dir)
	if err != nil || loaded != a {
		t.Fatal("load borrowed caller state", err)
	}
	a.Identity.Username = "synthetic-other"
	if err := CreateLocalAccount(dir, a); !errors.Is(err, ErrAccountExists) {
		t.Fatal("existing credential replaced", err)
	}
	after, err := os.ReadFile(path)
	entries, dirErr := os.ReadDir(dir)
	if err != nil || !bytes.Equal(before, after) || dirErr != nil || len(entries) != 1 || entries[0].Name() != LocalAccountFilename {
		t.Fatal("replacement refusal changed file or left temporary", err, dirErr)
	}
}

func TestLocalAccountFailuresHaveNoStateOrPrivateDiagnostics(t *testing.T) {
	dir := privateAccountTestDir(t)
	for _, a := range []LocalAccount{{}, {Identity: LocalIdentity{Username: "synthetic-private/invalid"}, PasswordHash: syntheticPasswordHash}, {Identity: syntheticAccount().Identity, PasswordHash: "synthetic-private-hash"}} {
		err := CreateLocalAccount(dir, a)
		if err == nil || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatal("invalid account accepted or disclosed", err)
		}
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatal("invalid account created storage", err)
	}
	missing := filepath.Join(dir, "synthetic-private-missing")
	if err := CreateLocalAccount(missing, syntheticAccount()); err != ErrAccountIO {
		t.Fatal("missing parent created or unsafe error", err)
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("missing parent was created", err)
	}
	for _, path := range []string{"", missing} {
		got, err := LoadLocalAccount(path)
		if err != ErrAccountIO || got != (LocalAccount{}) {
			t.Fatal("missing storage returned partial account or raw IO error", err)
		}
	}
	got, err := LoadLocalAccount(dir)
	if err != ErrAccountNotFound || got != (LocalAccount{}) {
		t.Fatal("missing account invented", err)
	}
	if err := os.Mkdir(filepath.Join(dir, LocalAccountFilename), 0700); err != nil {
		t.Fatal(err)
	}
	if err := CreateLocalAccount(dir, syntheticAccount()); err != ErrAccountExists {
		t.Fatal("existing non-file destination replaced", err)
	}
	if got, err := LoadLocalAccount(dir); got != (LocalAccount{}) || err != ErrAccountIO {
		t.Fatal("directory treated as credential", err)
	}
}

func TestLocalAccountConcurrentPublicationHasOneWinner(t *testing.T) {
	dir := privateAccountTestDir(t)
	start := make(chan struct{})
	type result struct {
		account LocalAccount
		err     error
	}
	results := make(chan result, 8)
	var writers sync.WaitGroup
	for i := 0; i < 8; i++ {
		writers.Add(1)
		go func(index int) {
			defer writers.Done()
			a := syntheticAccount()
			a.Identity.Username = fmt.Sprintf("synthetic-admin-%d", index)
			<-start
			results <- result{a, CreateLocalAccount(dir, a)}
		}(i)
	}
	close(start)
	for i := 0; i < 100; i++ {
		got, err := LoadLocalAccount(dir)
		if err != nil && err != ErrAccountNotFound || err == nil && got.Validate() != nil {
			t.Fatal("reader observed a partial or invalid published record", err)
		}
	}
	writers.Wait()
	close(results)
	var winner LocalAccount
	wins := 0
	for r := range results {
		if r.err == nil {
			wins++
			winner = r.account
		} else if r.err != ErrAccountExists {
			t.Fatal("loser failed outside nonreplacement contract", r.err)
		}
	}
	got, err := LoadLocalAccount(dir)
	if wins != 1 || err != nil || got != winner {
		t.Fatal("publication did not preserve exactly one winner", wins, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Fatal("concurrent creators left temporary records", err)
	}
}

func TestLocalAccountDecoderStrictAndBounded(t *testing.T) {
	valid := syntheticAccountJSON()
	for _, data := range []string{valid, valid + strings.Repeat(" ", MaxAccountBytes-len(valid))} {
		got, err := decodeLocalAccount(strings.NewReader(data))
		if err != nil || got != syntheticAccount() {
			t.Fatal("valid/inclusive bound refused", err)
		}
	}
	cases := []string{
		"", "null", "[]", "{}", valid + "{}", valid + "synthetic-private",
		valid + strings.Repeat(" ", MaxAccountBytes-len(valid)+1),
		strings.Replace(valid, `"version":1`, `"version":2`, 1),
		strings.Replace(valid, `"version":1`, `"version":1.0`, 1),
		strings.Replace(valid, `"version":1`, `"version":"1"`, 1),
		strings.Replace(valid, `"version":1`, `"version":null`, 1),
		strings.Replace(valid, `"version":1`, `"version":1,"version":1`, 1),
		strings.Replace(valid, `"username":`, `"Username":`, 1),
		strings.Replace(valid, `"username":"synthetic-admin"`, `"username":null`, 1),
		strings.Replace(valid, `"username":"synthetic-admin"`, `"username":{}`, 1),
		strings.Replace(valid, `"username":"synthetic-admin"`, `"username":"synthetic-admin","username":"other"`, 1),
		strings.Replace(valid, `"username":"synthetic-admin"`, `"username":"synthetic-private/invalid"`, 1),
		strings.Replace(valid, `"password_hash":`, `"password":`, 1),
		strings.Replace(valid, syntheticPasswordHash, "synthetic-private-hash", 1),
		strings.Replace(valid, `"version":1`, `"version":1,"role":"admin"`, 1),
		valid[:len(valid)-1], valid[:len(valid)-1] + ",}", valid + string([]byte{0xff}),
	}
	for i, data := range cases {
		got, err := decodeLocalAccount(strings.NewReader(data))
		if got != (LocalAccount{}) || err != ErrInvalidAccount || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatalf("case%d: malformed input returned partial account or unsafe error", i)
		}
	}
	for _, r := range []io.Reader{nil, failedRandom{}} {
		if got, err := decodeLocalAccount(r); got != (LocalAccount{}) || err != ErrAccountIO {
			t.Fatal("read failure disclosed dependency error", err)
		}
	}
	// A reader is consumed only up to the document bound plus one byte.
	r := &accountCountingReader{r: strings.NewReader(strings.Repeat(" ", MaxAccountBytes+100))}
	if _, err := decodeLocalAccount(r); err != ErrInvalidAccount || r.n != MaxAccountBytes+1 {
		t.Fatal("reader bound not enforced", err, r.n)
	}
}

type accountCountingReader struct {
	r io.Reader
	n int
}

func (r *accountCountingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += n
	return n, err
}

func TestLocalAccountLoadRejectsMalformedOrOversizedFiles(t *testing.T) {
	dir := privateAccountTestDir(t)
	path := filepath.Join(dir, LocalAccountFilename)
	for _, data := range []string{"synthetic-private-content", syntheticAccountJSON() + "{}", strings.Repeat(" ", MaxAccountBytes+1)} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := LoadLocalAccount(dir)
		if err == nil || got != (LocalAccount{}) || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatal("invalid file loaded or disclosed", err)
		}
		if err := CreateLocalAccount(dir, syntheticAccount()); err != ErrAccountExists {
			t.Fatal("invalid existing record was overwritten", err)
		}
		if after, err := os.ReadFile(path); err != nil || string(after) != data {
			t.Fatal("refusal changed malformed record", err)
		}
	}
}
