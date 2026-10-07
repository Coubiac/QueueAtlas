package config

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
	filesource "github.com/Coubiac/QueueAtlas/internal/source/file"
)

func TestFileSourceLibraryConfigPreservesSettingsAndOwnership(t *testing.T) {
	dir := t.TempDir()
	for _, explicit := range []bool{false, true} {
		data := syntheticSourceYAML
		want := filesource.Config{
			Identity: source.Identity{ID: "synthetic-source", Kind: "file", Name: "Synthetic Postfix"},
			Path:     filepath.Join(dir, "missing-parent", "synthetic-mail.log"), StartAt: filesource.StartAtBeginning,
			PollInterval: time.Second, RotationGrace: 30 * time.Second,
			ResumeLimits: filesource.FollowResumeLimits{Origins: 1000, Entries: 2000},
		}
		if explicit {
			data += "  trusted_host: synthetic-instance\n  start_at: end\n  poll_interval: 10ms\n  rotation_grace: 24h\n  resume_origins: 1\n  resume_entries: 1\n"
			want.Identity.TrustedHost, want.StartAt = "synthetic-instance", filesource.StartAtEnd
			want.PollInterval, want.RotationGrace = 10*time.Millisecond, 24*time.Hour
			want.ResumeLimits = filesource.FollowResumeLimits{Origins: 1, Entries: 1}
		}
		c, err := Decode(strings.NewReader(data), dir)
		if err != nil || c.Source == nil {
			t.Fatal("fixture failed", err)
		}
		before := *c.Source
		converted, err := c.Source.LibraryConfig()
		if err != nil || converted != want || *c.Source != before || converted.ResumePolicy.AllowZeroCheckpoint {
			t.Fatal("conversion changed fields, input or strict replay policy", converted, err)
		}
		c.Source.ID, c.Source.Path, c.Source.PollInterval = "changed", "changed.log", time.Minute
		if converted != want {
			t.Fatal("conversion borrowed application configuration")
		}
		converted.Identity.Name = "changed"
		if c.Source.Name != before.Name {
			t.Fatal("reverse mutation changed application configuration")
		}
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatal("conversion created files", entries, err)
	}
}

func TestFileSourceLibraryConfigRevalidatesAndReturnsZero(t *testing.T) {
	for _, tc := range []struct {
		field string
		set   func(*FileSource)
	}{
		{"source.id", func(c *FileSource) { c.ID = "synthetic-private/invalid" }},
		{"source.name", func(c *FileSource) { c.Name = "synthetic-private\n" }},
		{"source.trusted_host", func(c *FileSource) { c.TrustedHost = "synthetic-private/invalid" }},
		{"source.path", func(c *FileSource) { c.Path = "file:synthetic-private.log" }},
		{"source.path", func(c *FileSource) { c.Path = "synthetic-private-relative.log" }},
		{"source.start_at", func(c *FileSource) { c.StartAt = "synthetic-private-start" }},
		{"source.poll_interval", func(c *FileSource) { c.PollInterval = 0 }},
		{"source.rotation_grace", func(c *FileSource) { c.RotationGrace = 0 }},
		{"source.resume_origins", func(c *FileSource) { c.ResumeOrigins = 0 }},
		{"source.resume_entries", func(c *FileSource) { c.ResumeEntries = 0 }},
	} {
		c := syntheticFileSettings()
		c.Path = filepath.Join(t.TempDir(), "absent.log")
		tc.set(&c)
		before := c
		converted, err := c.LibraryConfig()
		if !errors.Is(err, ErrInvalid) || converted != (filesource.Config{}) || c != before ||
			!strings.Contains(err.Error(), tc.field+":") || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatal("invalid input produced partial settings, mutation or private diagnostic", converted, err)
		}
	}
}

func TestFileSourceLibraryConfigAcceptedByConstructorWithoutIO(t *testing.T) {
	dir := t.TempDir()
	for _, mode := range []string{"beginning", "end"} {
		c, err := Decode(strings.NewReader(syntheticSourceYAML+"  start_at: "+mode+"\n"), dir)
		if err != nil || c.Source == nil {
			t.Fatal(err)
		}
		cfg, err := c.Source.LibraryConfig()
		if err != nil {
			t.Fatal(err)
		}
		component, err := filesource.New(cfg, untouchedSourceState{t}, func([]byte) model.Observation {
			t.Fatal("constructor normalized a line")
			return model.Observation{}
		})
		if err != nil || component == nil || component.ID() != "synthetic-source" {
			t.Fatal("converted settings refused by FileSource constructor", err)
		}
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatal("construction created source or database state", entries, err)
	}
}

type untouchedSourceState struct{ t *testing.T }

func (s untouchedSourceState) FileOrigins(context.Context, source.OriginQuery) (source.OriginPage, error) {
	s.t.Fatal("constructor read origins")
	return source.OriginPage{}, nil
}

func (s untouchedSourceState) Checkpoint(context.Context, string, string) (source.Position, bool, error) {
	s.t.Fatal("constructor read checkpoint")
	return source.Position{}, false, nil
}
