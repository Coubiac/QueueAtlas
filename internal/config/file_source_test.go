package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	filesource "github.com/Coubiac/QueueAtlas/internal/source/file"
)

func syntheticFileSettings() FileSource {
	c := FileSourceDefaults()
	c.ID, c.Name, c.Path = "synthetic-source", "Synthetic Postfix", "synthetic-mail.log"
	return c
}

func assertFileSourceInvalid(t *testing.T, c FileSource, field string) {
	t.Helper()
	before := c
	err := c.Validate()
	if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), field+":") || strings.Contains(err.Error(), "synthetic-private") || c != before {
		t.Fatal("expected private field diagnostic without mutation", err)
	}
}

func TestFileSourceDefaultsAndIndependentValidation(t *testing.T) {
	c := syntheticFileSettings()
	before := c
	if err := c.Validate(); err != nil || c != before {
		t.Fatal("valid settings changed or refused", err)
	}
	if c.StartAt != "beginning" || c.PollInterval != time.Second || c.RotationGrace != 30*time.Second || c.ResumeOrigins != 1000 || c.ResumeEntries != 2000 || c.TrustedHost != "" {
		t.Fatal("unexpected defaults", c)
	}
	c.StartAt, c.PollInterval, c.ResumeOrigins = "end", time.Minute, 1
	other := FileSourceDefaults()
	if other.StartAt != "beginning" || other.PollInterval != time.Second || other.ResumeOrigins != 1000 || other.ID != "" || other.Name != "" || other.Path != "" {
		t.Fatal("defaults borrowed caller state or invented required input", other)
	}
	assertFileSourceInvalid(t, FileSource{}, "source.id")
	assertFileSourceInvalid(t, FileSourceDefaults(), "source.id")
	if err := c.Validate(); err != nil {
		t.Fatal("explicit end refused", err)
	}
}

func TestFileSourceIdentityAndPrivateDiagnostics(t *testing.T) {
	for _, value := range []string{"a", "0", "Synthetic-Source_1.2", strings.Repeat("s", 128)} {
		c := syntheticFileSettings()
		c.ID, c.TrustedHost = value, value
		if err := c.Validate(); err != nil {
			t.Fatal("valid identifier refused", err)
		}
	}
	for _, field := range []string{"source.id", "source.trusted_host"} {
		values := []string{" synthetic-private", "synthetic-private ", ".synthetic-private", "-synthetic-private", "synthetic-private/value", "synthetic-private\n", "synthétique-private", strings.Repeat("s", 256)}
		if field == "source.id" {
			values = append(values, "", strings.Repeat("s", 129))
		}
		for _, value := range values {
			c := syntheticFileSettings()
			if field == "source.id" {
				c.ID = value
			} else {
				c.TrustedHost = value
			}
			assertFileSourceInvalid(t, c, field)
		}
	}
	c := syntheticFileSettings()
	c.TrustedHost = strings.Repeat("s", 255)
	c.Name = "Postfix synthétique"
	if err := c.Validate(); err != nil {
		t.Fatal("valid UTF-8 name/maximum instance refused", err)
	}
	for _, value := range []string{"", " synthetic-private", "synthetic-private ", "synthetic-private\n", string([]byte{0xff}), strings.Repeat("s", 129)} {
		c.Name = value
		assertFileSourceInvalid(t, c, "source.name")
	}
	c.Name = strings.Repeat("s", 128)
	if err := c.Validate(); err != nil {
		t.Fatal("maximum name refused", err)
	}
}

func TestFileSourcePathSyntaxWithoutIO(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing-parent", "synthetic.log")
	for _, path := range []string{missing, "synthetic.log", filepath.Join("..", "synthetic.log"), "${SYNTHETIC}/synthetic.log", "~/synthetic.log", strings.Repeat("s", 4096)} {
		c := syntheticFileSettings()
		c.Path = path
		before := c
		if err := c.Validate(); err != nil || c != before {
			t.Fatal("literal path refused or normalized", err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("syntax validation created state", entries, err)
	}
	values := []string{"", " synthetic-private.log", "synthetic-private.log ", "synthetic-private\x00.log", "synthetic-private\n.log", string([]byte{0xff}), strings.Repeat("s", 4097), ".", "..", string(filepath.Separator), "synthetic-private" + string(filepath.Separator), ":memory:", "file:synthetic-private.log", "FILE:synthetic-private.log", "https://synthetic-private.invalid/log", `\\synthetic-private\share\log`, "//synthetic-private/share/log", "synthetic-private*.log", "synthetic-private?.log"}
	if runtime.GOOS == "windows" {
		values = append(values, `C:synthetic-private.log`, `\synthetic-private.log`, "/synthetic-private.log", "C:/synthetic-private/")
	}
	for _, path := range values {
		c := syntheticFileSettings()
		c.Path = path
		assertFileSourceInvalid(t, c, "source.path")
	}
}

func TestFileSourceTimingBudgetsAndExplicitZero(t *testing.T) {
	for _, tc := range []struct {
		field string
		set   func(*FileSource, time.Duration)
		min   time.Duration
		max   time.Duration
	}{
		{"source.poll_interval", func(c *FileSource, d time.Duration) { c.PollInterval = d }, filesource.MinPollInterval, filesource.MaxPollInterval},
		{"source.rotation_grace", func(c *FileSource, d time.Duration) { c.RotationGrace = d }, filesource.MinRotationGrace, filesource.MaxRotationGrace},
	} {
		for _, value := range []time.Duration{tc.min, tc.max} {
			c := syntheticFileSettings()
			tc.set(&c, value)
			if err := c.Validate(); err != nil {
				t.Fatal("inclusive timing bound refused", tc.field, err)
			}
		}
		for _, value := range []time.Duration{-time.Second, 0, tc.min - 1, tc.max + 1} {
			c := syntheticFileSettings()
			tc.set(&c, value)
			assertFileSourceInvalid(t, c, tc.field)
		}
	}
	for _, tc := range []struct {
		field string
		set   func(*FileSource, int)
		max   int
	}{
		{"source.resume_origins", func(c *FileSource, n int) { c.ResumeOrigins = n }, filesource.MaxPathOrigins},
		{"source.resume_entries", func(c *FileSource, n int) { c.ResumeEntries = n }, filesource.MaxFollowLocationEntries},
	} {
		for _, value := range []int{1, tc.max} {
			c := syntheticFileSettings()
			tc.set(&c, value)
			if err := c.Validate(); err != nil {
				t.Fatal("inclusive budget bound refused", tc.field, err)
			}
		}
		for _, value := range []int{-1, 0, tc.max + 1} {
			c := syntheticFileSettings()
			tc.set(&c, value)
			assertFileSourceInvalid(t, c, tc.field)
		}
	}
	for _, value := range []string{"", "END", " beginning", "synthetic-private-start"} {
		c := syntheticFileSettings()
		c.StartAt = value
		assertFileSourceInvalid(t, c, "source.start_at")
	}
}
