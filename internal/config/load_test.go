package config

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFileDefaultsAndNoDatabaseIO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "synthetic.yaml")
	if err := os.WriteFile(path, []byte("server:\n  listen: '[::1]:9090'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	want := Defaults()
	want.Server.Listen = "[::1]:9090"
	want.Storage.Path = filepath.Join(dir, "queueatlas.db")
	if err != nil || c != want {
		t.Fatal("omitted fields or base directory changed", c, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "synthetic.yaml" {
		t.Fatal("loading created database state", entries, err)
	}
	// Load closes its input, including on Windows where renaming an open file
	// may fail. A second load proves it doesn't retain the first configuration.
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	want.Server.Listen = Defaults().Server.Listen
	if c, err := Load(path); err != nil || c != want {
		t.Fatal("configuration leaked between loads", c, err)
	}
}

func TestDecodeExplicitValuesPathsAndNoExpansion(t *testing.T) {
	dir := t.TempDir()
	data := "server:\n  listen: '127.0.0.2:9091'\n  read_header_timeout: 2s\n  idle_timeout: '2m'\n  shutdown_timeout: 3s\nstorage:\n  path: '../state/synthetic.db'\n"
	c, err := Decode(strings.NewReader(data), dir)
	want := Config{Server: Server{Listen: "127.0.0.2:9091", ReadHeaderTimeout: 2 * time.Second, IdleTimeout: 2 * time.Minute, ShutdownTimeout: 3 * time.Second}, Storage: Storage{Path: filepath.Join(dir, "..", "state", "synthetic.db")}}
	if err != nil || c != want || !filepath.IsAbs(c.Storage.Path) {
		t.Fatal(c, err)
	}
	abs := filepath.Join(dir, "absolute-synthetic.db")
	for _, path := range []string{abs, "${SYNTHETIC_CONFIG_VARIABLE}.db", "~synthetic.db", "{{synthetic}}.db"} {
		data := "storage:\n  path: '" + path + "'\n"
		c, err := Decode(strings.NewReader(data), dir)
		expected := path
		if !filepath.IsAbs(path) {
			expected = filepath.Join(dir, path)
		}
		if err != nil || c.Storage.Path != expected {
			t.Fatal("path was expanded or rebased incorrectly", c, err)
		}
	}
}

func TestDecodeRejectsAmbiguousOrInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cases := []string{
		"", "# comment only\n", "null\n", "[]\n", "synthetic-private-token\n",
		"synthetic-private-token: value\n", "server: {synthetic-private-token: value}\n", "storage: {synthetic-private-token: value}\n",
		"server: {}\nserver: {}\n", "server: {listen: '127.0.0.1:8080', listen: '127.0.0.1:9090'}\n", "storage: {path: a.db, 'path': b.db}\n",
		"{}\n---\n{}\n", "{}\n---\n", "{}\n---\n[synthetic-private-token\n",
		"server: null\n", "storage: null\n", "server: []\n", "storage: {path: null}\n", "server: {listen: null}\n",
		"server: {read_header_timeout: null}\n", "server: {idle_timeout: null}\n", "server: {shutdown_timeout: null}\n",
		"server: {read_header_timeout: 0s}\n", "server: {idle_timeout: 0s}\n", "server: {shutdown_timeout: 0s}\n",
		"server: {idle_timeout: 10}\n", "server: {listen: true}\n", "storage: {path: 123}\n",
		"server: {idle_timeout: 'synthetic-private-token'}\n", "server: {idle_timeout: 999999999999999999999h}\n",
		"server: {listen: '0.0.0.0:8080'}\n", "storage: {path: ''}\n", "storage: {path: 'synthetic/'}\n",
		"server: &synthetic {}\n", "server: &synthetic {listen: *synthetic}\n", "server: {listen: &synthetic '127.0.0.1:8080'}\n",
		"server: {<<: {listen: '127.0.0.1:8080'}}\n", "server: !synthetic-private-token {}\n", "storage: {path: !synthetic-private-token x}\n",
		"? [synthetic-private-token]\n: value\n", "123: {}\n", "server: {listen: [synthetic-private-token]}\n",
		"server:\n\tlisten: synthetic-private-token\n", "storage: {path: 'synthetic-private-token\n", "storage: {path: '\xff'}\n",
	}
	for i, data := range cases {
		c, err := Decode(strings.NewReader(data), dir)
		if !errors.Is(err, ErrInvalid) || c != (Config{}) || strings.Contains(err.Error(), "synthetic-private-token") {
			t.Fatalf("case%d: expected zero configuration and sanitized rejection, got %#v / %v", i, c, err)
		}
	}
}

func TestDecodeBoundsInputAndStructure(t *testing.T) {
	dir := t.TempDir()
	// A comment makes the valid document exactly MaxBytes bytes.
	data := "#" + strings.Repeat("x", MaxBytes-5) + "\n{}\n"
	if len(data) != MaxBytes {
		t.Fatal("incorrect boundary fixture")
	}
	if _, err := Decode(strings.NewReader(data), dir); err != nil {
		t.Fatal("inclusive byte bound rejected", err)
	}
	r := &countReader{reader: strings.NewReader(data + strings.Repeat("x", MaxBytes))}
	c, err := Decode(r, dir)
	if !errors.Is(err, ErrInvalid) || c != (Config{}) || r.read != MaxBytes+1 {
		t.Fatal("oversize input was consumed beyond the bound or accepted", c, err, r.read)
	}
	for _, data := range []string{
		"server: {listen: {nested: {x: {y: value}}}}\n",
		"server: {listen: [" + strings.Repeat("x,", 128) + "x]}\n",
	} {
		c, err := Decode(strings.NewReader(data), dir)
		if !errors.Is(err, ErrInvalid) || c != (Config{}) || !strings.Contains(err.Error(), "structure") {
			t.Fatal("structure bound was not applied", c, err)
		}
	}
}

func TestLoadReadFailuresAndDecodeBaseDirectory(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(bad, []byte("server: null\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if c, err := Load(bad); !errors.Is(err, ErrInvalid) || c != (Config{}) {
		t.Fatal(c, err)
	}
	if err := os.Rename(bad, bad+".old"); err != nil {
		t.Fatal("invalid input was not closed", err)
	}
	for _, path := range []string{filepath.Join(dir, "synthetic-private-token.yaml"), dir} {
		c, err := Load(path)
		if !errors.Is(err, ErrRead) || c != (Config{}) || strings.Contains(err.Error(), "synthetic-private-token") {
			t.Fatal("IO failure leaked path or partial settings", c, err)
		}
	}
	if c, err := Load(""); !errors.Is(err, ErrInvalid) || c != (Config{}) {
		t.Fatal(c, err)
	}
	if c, err := Decode(failingConfigReader{}, dir); !errors.Is(err, ErrRead) || c != (Config{}) || strings.Contains(err.Error(), "synthetic-private-token") {
		t.Fatal("partial input/error contents escaped", c, err)
	}
	if c, err := Decode(nil, dir); !errors.Is(err, ErrRead) || c != (Config{}) {
		t.Fatal(c, err)
	}
	if c, err := Decode(strings.NewReader("{}"), "relative"); !errors.Is(err, ErrInvalid) || c != (Config{}) {
		t.Fatal("relative base accepted", c, err)
	}
}

func TestDecodeWindowsAmbiguousPaths(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("Windows drive-relative path semantics")
	}
	for _, path := range []string{`C:synthetic.db`, `\synthetic.db`, `/synthetic.db`} {
		c, err := Decode(strings.NewReader("storage: {path: '"+path+"'}"), t.TempDir())
		if !errors.Is(err, ErrInvalid) || c != (Config{}) {
			t.Fatal("ambiguous path accepted", c, err)
		}
	}
}

func TestLoadRepositoryExample(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "examples", "queueatlas.yaml"))
	if err != nil || !filepath.IsAbs(c.Storage.Path) || c.Server != Defaults().Server {
		t.Fatal("documented example no longer loads", c, err)
	}
}

type countReader struct {
	reader io.Reader
	read   int
}

func (r *countReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += n
	return n, err
}

type failingConfigReader struct{}

func (failingConfigReader) Read(p []byte) (int, error) {
	return copy(p, "{}"), errors.New("synthetic-private-token")
}
