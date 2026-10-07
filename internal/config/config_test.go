package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultsAreValidAndIndependent(t *testing.T) {
	c := Defaults()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if c.Server.Listen != "127.0.0.1:8080" || c.Storage.Path != "queueatlas.db" || c.Server.ReadHeaderTimeout != 5*time.Second || c.Server.IdleTimeout != time.Minute || c.Server.ShutdownTimeout != 15*time.Second {
		t.Fatalf("unexpected defaults: %#v", c)
	}
	c.Server.Listen = "[::1]:9090"
	c.Storage.Path = "synthetic.db"
	if Defaults().Server.Listen != "127.0.0.1:8080" || Defaults().Storage.Path != "queueatlas.db" {
		t.Fatal("a caller changed another caller's defaults")
	}
	if err := (Config{}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatal("zero configuration silently defaulted", err)
	}
}

func TestListenRequiresLiteralLoopbackAndUsablePort(t *testing.T) {
	for _, listen := range []string{"127.0.0.1:8080", "127.0.0.2:1", "[::1]:65535", "[::ffff:127.0.0.1]:8080"} {
		c := Defaults()
		c.Server.Listen = listen
		before := c
		if err := c.Validate(); err != nil || c != before {
			t.Fatal("valid loopback changed or refused", listen, err)
		}
	}
	for _, listen := range []string{"", ":8080", "localhost:8080", "0.0.0.0:8080", "[::]:8080", "192.0.2.1:8080", "[2001:db8::1]:8080", "[::1%synthetic]:8080", "::1:8080", "127.0.0.1", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:http", "127.0.0.1:+80", "127.0.0.1:-1", "127.0.0.1:999999999999999999999999", "127.0.0.1: 80"} {
		t.Run(listen, func(t *testing.T) {
			c := Defaults()
			c.Server.Listen = listen
			assertInvalid(t, c, "server.listen")
		})
	}
}

func TestTimeoutBoundsAndExplicitZero(t *testing.T) {
	for _, tc := range []struct {
		field string
		set   func(*Config, time.Duration)
		max   time.Duration
	}{
		{"server.read_header_timeout", func(c *Config, d time.Duration) { c.Server.ReadHeaderTimeout = d }, time.Minute},
		{"server.idle_timeout", func(c *Config, d time.Duration) { c.Server.IdleTimeout = d }, 10 * time.Minute},
		{"server.shutdown_timeout", func(c *Config, d time.Duration) { c.Server.ShutdownTimeout = d }, time.Minute},
	} {
		t.Run(tc.field, func(t *testing.T) {
			for _, d := range []time.Duration{time.Millisecond, tc.max} {
				c := Defaults()
				tc.set(&c, d)
				if err := c.Validate(); err != nil {
					t.Fatal("inclusive bound refused", d, err)
				}
			}
			for _, d := range []time.Duration{-time.Second, 0, time.Millisecond - 1, tc.max + 1} {
				c := Defaults()
				tc.set(&c, d)
				assertInvalid(t, c, tc.field)
			}
		})
	}
}

func TestStoragePathSyntaxWithoutIO(t *testing.T) {
	// The parent deliberately does not exist. Validation accepts syntax only.
	missing := filepath.Join(t.TempDir(), "missing-parent", "synthetic.db")
	for _, path := range []string{"queueatlas.db", filepath.Join("missing-synthetic-parent", "queueatlas.db"), missing} {
		c := Defaults()
		c.Storage.Path = path
		before := c
		if err := c.Validate(); err != nil || c != before {
			t.Fatal("file path changed or refused", err)
		}
	}
	if _, err := os.Stat(filepath.Dir(missing)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("validation created state or required an existing parent", err)
	}
	for _, path := range []string{"", " ", " synthetic.db", "synthetic.db ", "synthetic\x00.db", "synthetic\n.db", ":memory:", "file:synthetic.db", "FILE:synthetic.db", "https://synthetic.invalid/db", `\\synthetic\share\db`, "//synthetic/share/db", ".", "..", string(filepath.Separator), "synthetic" + string(filepath.Separator)} {
		t.Run(path, func(t *testing.T) {
			c := Defaults()
			c.Storage.Path = path
			assertInvalid(t, c, "storage.path")
		})
	}
}

func TestDiagnosticsDoNotEchoSuppliedValues(t *testing.T) {
	for _, field := range []string{"server.listen", "storage.path"} {
		c := Defaults()
		const private = "synthetic-private-token"
		if field == "server.listen" {
			c.Server.Listen = private
		} else {
			c.Storage.Path = "file:" + private
		}
		err := c.Validate()
		if err == nil || strings.Contains(err.Error(), private) {
			t.Fatal("invalid value was accepted or echoed", err)
		}
		assertInvalid(t, c, field)
	}
}

func assertInvalid(t *testing.T, c Config, field string) {
	t.Helper()
	before := c
	err := c.Validate()
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), field+":") || c != before {
		t.Fatal("expected a field diagnostic without configuration mutation", err)
	}
}
