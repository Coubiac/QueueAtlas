package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	filesource "github.com/Coubiac/QueueAtlas/internal/source/file"
)

// FileSource describes one application's file input. This initial pure contract
// is not yet part of Config or accepted by the YAML loader. Kind is always file;
// zero-checkpoint replay remains disabled when components are wired later.
type FileSource struct {
	ID            string
	Name          string
	TrustedHost   string // optional configured instance key, never a DNS lookup
	Path          string // literal OS path, relative to the future YAML directory
	StartAt       string
	PollInterval  time.Duration
	RotationGrace time.Duration
	ResumeOrigins int
	ResumeEntries int
}

// FileSourceDefaults supplies independent optional settings. ID, Name and Path
// remain required. Overlay only present fields, then Validate; explicit zero
// durations, budgets and an empty start mode must not silently select defaults.
func FileSourceDefaults() FileSource {
	return FileSource{
		StartAt:       string(filesource.StartAtBeginning),
		PollInterval:  filesource.DefaultPollInterval,
		RotationGrace: filesource.DefaultRotationGrace,
		ResumeOrigins: filesource.MaxPathOrigins,
		ResumeEntries: filesource.MaxFollowLocationEntries,
	}
}

// Validate checks syntax and existing library bounds without mutation, IO,
// normalization, path resolution, DNS, component construction or state reads.
func (c FileSource) Validate() error {
	if !fileSourceToken(c.ID, 128) {
		return invalid("source.id", "expected 1..128 ASCII identifier bytes: alphanumeric first, then alphanumeric, dot, underscore or hyphen")
	}
	if c.Name == "" || len(c.Name) > 128 || !utf8.ValidString(c.Name) || strings.TrimSpace(c.Name) != c.Name || strings.IndexFunc(c.Name, unicode.IsControl) >= 0 {
		return invalid("source.name", "expected 1..128 UTF-8 bytes without surrounding whitespace or control characters")
	}
	if c.TrustedHost != "" && !fileSourceToken(c.TrustedHost, 255) {
		return invalid("source.trusted_host", "expected an optional ASCII instance identifier of at most 255 bytes")
	}
	p := c.Path
	if p == "" || len(p) > 4096 || !utf8.ValidString(p) || strings.TrimSpace(p) != p || strings.IndexFunc(p, unicode.IsControl) >= 0 {
		return invalid("source.path", "expected 1..4096 UTF-8 path bytes without surrounding whitespace or control characters")
	}
	if p == ":memory:" || strings.HasPrefix(strings.ToLower(p), "file:") || strings.Contains(p, "://") || strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, "//") || strings.ContainsAny(p, "*?") {
		return invalid("source.path", "expected a literal file path, not a URI, UNC share or wildcard")
	}
	base := filepath.Base(p)
	if base == "." || base == ".." || base == string(filepath.Separator) || strings.HasSuffix(p, string(filepath.Separator)) || (filepath.Separator == '\\' && strings.HasSuffix(p, "/")) {
		return invalid("source.path", "expected a file name")
	}
	if !filepath.IsAbs(p) && (filepath.VolumeName(p) != "" || (filepath.Separator == '\\' && (strings.HasPrefix(p, `\`) || strings.HasPrefix(p, "/")))) {
		return invalid("source.path", "drive-relative or rooted paths without a drive are unsupported")
	}
	if c.StartAt != string(filesource.StartAtBeginning) && c.StartAt != string(filesource.StartAtEnd) {
		return invalid("source.start_at", "expected beginning or end")
	}
	for _, limit := range []struct {
		field string
		value time.Duration
		min   time.Duration
		max   time.Duration
	}{
		{"source.poll_interval", c.PollInterval, filesource.MinPollInterval, filesource.MaxPollInterval},
		{"source.rotation_grace", c.RotationGrace, filesource.MinRotationGrace, filesource.MaxRotationGrace},
	} {
		if limit.value < limit.min || limit.value > limit.max {
			return invalid(limit.field, fmt.Sprintf("duration must be between %s and %s", limit.min, limit.max))
		}
	}
	if c.ResumeOrigins < 1 || c.ResumeOrigins > filesource.MaxPathOrigins {
		return invalid("source.resume_origins", fmt.Sprintf("budget must be between 1 and %d", filesource.MaxPathOrigins))
	}
	if c.ResumeEntries < 1 || c.ResumeEntries > filesource.MaxFollowLocationEntries {
		return invalid("source.resume_entries", fmt.Sprintf("budget must be between 1 and %d", filesource.MaxFollowLocationEntries))
	}
	return nil
}

func fileSourceToken(value string, max int) bool {
	if len(value) == 0 || len(value) > max {
		return false
	}
	for i := 0; i < len(value); i++ {
		b := value[i]
		alphanumeric := b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
		if !alphanumeric && (i == 0 || b != '.' && b != '_' && b != '-') {
			return false
		}
	}
	return true
}
