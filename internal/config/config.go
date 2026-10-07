// Package config defines the initial application settings without performing IO.
// Loading a configuration file and starting components are separate operations.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ErrInvalid identifies a rejected configuration. Diagnostics contain field
// names and rules, never the supplied values (which may contain private data).
var ErrInvalid = errors.New("invalid configuration")

type Config struct {
	Server  Server  `yaml:"server"`
	Storage Storage `yaml:"storage"`
}

type Server struct {
	Listen            string        `yaml:"listen"`
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
}

type Storage struct {
	// A relative path is intended to be resolved against the configuration file's
	// directory by the future loader, before passing it to SQLite. No resolution
	// or filesystem validation happens here.
	Path string `yaml:"path"`
}

// Defaults returns independent settings for the initial local server contract.
// Callers overlay only present fields, then Validate; explicit zero values are
// invalid, rather than silently replaced with defaults.
func Defaults() Config {
	return Config{
		Server: Server{
			Listen:            "127.0.0.1:8080",
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       time.Minute,
			ShutdownTimeout:   15 * time.Second,
		},
		Storage: Storage{Path: "queueatlas.db"},
	}
}

// Validate checks the initial contract without normalizing the caller's values,
// opening files, resolving DNS or binding sockets. Loopback does not waive the
// future server's authentication requirements. Remote listening is unsupported
// until authentication and TLS configuration are implemented together.
func (c Config) Validate() error {
	host, port, err := net.SplitHostPort(c.Server.Listen)
	if err != nil {
		return invalid("server.listen", "expected a loopback IP and numeric port")
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || addr.Zone() != "" || !addr.Unmap().IsLoopback() {
		return invalid("server.listen", "expected a literal loopback IP")
	}
	if port == "" || strings.IndexFunc(port, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return invalid("server.listen", "port must be numeric and between 1 and 65535")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return invalid("server.listen", "port must be numeric and between 1 and 65535")
	}
	for _, limit := range []struct {
		field string
		value time.Duration
		max   time.Duration
	}{
		{"server.read_header_timeout", c.Server.ReadHeaderTimeout, time.Minute},
		{"server.idle_timeout", c.Server.IdleTimeout, 10 * time.Minute},
		{"server.shutdown_timeout", c.Server.ShutdownTimeout, time.Minute},
	} {
		if limit.value < time.Millisecond || limit.value > limit.max {
			return invalid(limit.field, fmt.Sprintf("duration must be between 1ms and %s", limit.max))
		}
	}
	p := c.Storage.Path
	if p == "" || strings.TrimSpace(p) != p || strings.IndexFunc(p, unicode.IsControl) >= 0 {
		return invalid("storage.path", "expected a nonempty path without surrounding whitespace or control characters")
	}
	if p == ":memory:" || strings.HasPrefix(strings.ToLower(p), "file:") || strings.Contains(p, "://") || strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") {
		return invalid("storage.path", "expected a file path, not a URI, memory database or UNC share")
	}
	base := filepath.Base(p)
	if base == "." || base == ".." || base == string(filepath.Separator) || strings.HasSuffix(p, string(filepath.Separator)) || (filepath.Separator == '\\' && strings.HasSuffix(p, "/")) {
		return invalid("storage.path", "expected a file name")
	}
	return nil
}

func invalid(field, rule string) error {
	return fmt.Errorf("%w: %s: %s", ErrInvalid, field, rule)
}
