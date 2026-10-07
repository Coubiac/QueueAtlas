package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// MaxBytes is the maximum accepted document size, including comments.
const MaxBytes = 64 * 1024

// ErrRead reports IO failures without including paths or reader error contents.
var ErrRead = errors.New("cannot read configuration")

// Load reads one regular configuration file. Relative storage paths are resolved
// against its lexical absolute directory, not a symlink target's directory.
// It neither opens the database nor checks the database's permissions.
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" || strings.IndexByte(path, 0) >= 0 {
		return Config{}, invalid("configuration", "expected a file path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Config{}, ErrRead
	}
	// Reject ordinary non-file inputs before opening; the post-open check also
	// rejects non-regular replacements. This is not a lock against replacement.
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() {
		return Config{}, ErrRead
	}
	f, err := os.Open(abs)
	if err != nil {
		return Config{}, ErrRead
	}
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return Config{}, ErrRead
	}
	c, decodeErr := Decode(f, filepath.Dir(abs))
	closeErr := f.Close()
	if decodeErr != nil {
		return Config{}, decodeErr
	}
	if closeErr != nil {
		return Config{}, ErrRead
	}
	return c, nil
}

// Decode reads bounded UTF-8 YAML from r and returns validated settings. baseDir
// must be absolute. Only absent fields retain defaults; nulls and implicit type
// conversions are refused. Neither environment variables nor templates expand.
// The byte bound is not a deadline for an arbitrary blocking reader.
func Decode(r io.Reader, baseDir string) (Config, error) {
	if !filepath.IsAbs(baseDir) {
		return Config{}, invalid("configuration", "expected an absolute base directory")
	}
	if r == nil {
		return Config{}, ErrRead
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return Config{}, ErrRead
	}
	if len(data) > MaxBytes {
		return Config{}, invalid("yaml", "input exceeds 64 KiB")
	}
	if !utf8.Valid(data) {
		return Config{}, invalid("yaml", "expected UTF-8")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return Config{}, invalid("yaml", "expected a valid mapping document")
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		return Config{}, invalid("yaml", "expected exactly one document")
	}
	remaining := 128
	if err := checkTree(&doc, 0, &remaining); err != nil {
		return Config{}, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return Config{}, invalid("yaml", "expected a mapping document")
	}
	c := Defaults()
	err = readMapping(doc.Content[0], "yaml", func(key string, node *yaml.Node) error {
		switch key {
		case "server":
			return readMapping(node, "server", func(key string, node *yaml.Node) error {
				switch key {
				case "listen":
					return readString(node, "server.listen", &c.Server.Listen)
				case "read_header_timeout":
					return readDuration(node, "server.read_header_timeout", &c.Server.ReadHeaderTimeout)
				case "idle_timeout":
					return readDuration(node, "server.idle_timeout", &c.Server.IdleTimeout)
				case "shutdown_timeout":
					return readDuration(node, "server.shutdown_timeout", &c.Server.ShutdownTimeout)
				default:
					return invalid("server", "unknown field")
				}
			})
		case "storage":
			return readMapping(node, "storage", func(key string, node *yaml.Node) error {
				if key != "path" {
					return invalid("storage", "unknown field")
				}
				return readString(node, "storage.path", &c.Storage.Path)
			})
		default:
			return invalid("yaml", "unknown field")
		}
	})
	if err != nil {
		return Config{}, err
	}
	// Validate the original path before cleaning; otherwise a trailing separator,
	// URI or empty value could change meaning during resolution.
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	if !filepath.IsAbs(c.Storage.Path) {
		if filepath.VolumeName(c.Storage.Path) != "" || (filepath.Separator == '\\' && (strings.HasPrefix(c.Storage.Path, `\`) || strings.HasPrefix(c.Storage.Path, "/"))) {
			return Config{}, invalid("storage.path", "drive-relative or rooted paths without a drive are unsupported")
		}
		c.Storage.Path = filepath.Join(baseDir, c.Storage.Path)
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Check nodes without decoding aliases into Go values (no alias expansion).
// Parsing is already byte-bounded; this walk stops at depth4 or 128 nodes.
func checkTree(n *yaml.Node, depth int, remaining *int) error {
	*remaining -= 1
	if depth > 4 || *remaining < 0 {
		return invalid("yaml", "structure exceeds supported bounds")
	}
	if n.Kind == yaml.AliasNode || n.Anchor != "" {
		return invalid("yaml", "anchors and aliases are unsupported")
	}
	for _, child := range n.Content {
		if err := checkTree(child, depth+1, remaining); err != nil {
			return err
		}
	}
	return nil
}

func readMapping(n *yaml.Node, field string, visit func(string, *yaml.Node) error) error {
	if n.Kind != yaml.MappingNode || n.ShortTag() != "!!map" || len(n.Content)%2 != 0 {
		return invalid(field, "expected a mapping")
	}
	seen := make(map[string]bool)
	for i := 0; i < len(n.Content); i += 2 {
		key := n.Content[i]
		if key.Kind != yaml.ScalarNode || key.ShortTag() != "!!str" {
			return invalid(field, "expected a textual field name")
		}
		if seen[key.Value] {
			return invalid(field, "duplicate field")
		}
		seen[key.Value] = true
		if err := visit(key.Value, n.Content[i+1]); err != nil {
			return err
		}
	}
	return nil
}

func readString(n *yaml.Node, field string, out *string) error {
	if n.Kind != yaml.ScalarNode || n.ShortTag() != "!!str" {
		return invalid(field, "expected a textual scalar")
	}
	*out = n.Value
	return nil
}

func readDuration(n *yaml.Node, field string, out *time.Duration) error {
	var value string
	if err := readString(n, field, &value); err != nil {
		return err
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return invalid(field, "expected a duration with units")
	}
	*out = d
	return nil
}
