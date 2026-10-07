package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const syntheticSourceYAML = "source:\n  id: synthetic-source\n  name: Synthetic Postfix\n  path: missing-parent/synthetic-mail.log\n"

func TestLoadSourceDefaultsAndNoSourceIO(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "synthetic.yaml")
	if err := os.WriteFile(path, []byte(syntheticSourceYAML), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	want := syntheticFileSettings()
	want.Path = filepath.Join(dir, "missing-parent", "synthetic-mail.log")
	if err != nil || c.Source == nil || *c.Source != want || c.Server != Defaults().Server || c.Storage.Path != filepath.Join(dir, "queueatlas.db") {
		t.Fatal("source defaults or YAML directory incorrect", c, err)
	}
	before := *c.Source
	if err := c.Validate(); err != nil || *c.Source != before {
		t.Fatal("whole configuration validation changed source", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "synthetic.yaml" {
		t.Fatal("loading created source/database state", entries, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != syntheticSourceYAML {
		t.Fatal("loading changed configuration", err)
	}
	// An absent section remains nil, so existing diagnostic-only configs work.
	absent, err := Decode(strings.NewReader("{}\n"), dir)
	if err != nil || absent.Source != nil || Defaults().Source != nil {
		t.Fatal("source section was invented", absent, err)
	}
	c.Source.PollInterval = 0
	assertInvalid(t, c, "source.poll_interval")
}

func TestDecodeSourceExplicitValuesPathsAndOwnership(t *testing.T) {
	dir := t.TempDir()
	data := "source:\n  id: synthetic-source\n  name: Synthetic Postfix\n  trusted_host: synthetic-instance\n  path: ../synthetic-mail.log\n  start_at: end\n  poll_interval: 10ms\n  rotation_grace: 24h\n  resume_origins: 1\n  resume_entries: 2000\n"
	c, err := Decode(strings.NewReader(data), dir)
	want := FileSource{
		ID: "synthetic-source", Name: "Synthetic Postfix", TrustedHost: "synthetic-instance",
		Path: filepath.Join(dir, "..", "synthetic-mail.log"), StartAt: "end",
		PollInterval: 10 * time.Millisecond, RotationGrace: 24 * time.Hour, ResumeOrigins: 1, ResumeEntries: 2000,
	}
	if err != nil || c.Source == nil || *c.Source != want {
		t.Fatal("explicit source settings incorrect", c, err)
	}
	other, err := Decode(strings.NewReader(data), dir)
	if err != nil || other.Source == nil || other.Source == c.Source {
		t.Fatal("source allocation borrowed between loads", err)
	}
	c.Source.ID = "changed"
	if *other.Source != want {
		t.Fatal("mutating one load changed another")
	}
	for _, path := range []string{filepath.Join(dir, "absolute.log"), "${SYNTHETIC}/synthetic.log", "~/synthetic.log", "{{synthetic}}.log"} {
		data := strings.ReplaceAll(syntheticSourceYAML, "missing-parent/synthetic-mail.log", "'"+path+"'")
		c, err := Decode(strings.NewReader(data), dir)
		expected := path
		if !filepath.IsAbs(path) {
			expected = filepath.Join(dir, path)
		}
		if err != nil || c.Source == nil || c.Source.Path != expected {
			t.Fatal("path expanded or rebased incorrectly", err)
		}
	}
}

func TestDecodeSourceRejectsInvalidYAMLAndReturnsZero(t *testing.T) {
	dir := t.TempDir()
	cases := []string{
		"source: null\n", "source: []\n", "source: synthetic-private-token\n", "source: {}\n",
		"source: {}\nsource: {}\n", "source: !synthetic-private-token {}\n",
		"source: &synthetic-private-token {}\n", "sources: []\n",
		syntheticSourceYAML + "  synthetic-private-token: value\n",
		syntheticSourceYAML + "  id: synthetic-other\n",
		syntheticSourceYAML + "  type: file\n",
		syntheticSourceYAML + "  allow_zero_checkpoint: true\n",
		strings.ReplaceAll(syntheticSourceYAML, "synthetic-source", "123"),
		strings.ReplaceAll(syntheticSourceYAML, "Synthetic Postfix", "null"),
		strings.ReplaceAll(syntheticSourceYAML, "synthetic-source", "!synthetic-private-token value"),
		strings.ReplaceAll(syntheticSourceYAML, "synthetic-source", "&synthetic-private-token value"),
		strings.ReplaceAll(syntheticSourceYAML, "synthetic-source", "*synthetic-private-token"),
		strings.ReplaceAll(syntheticSourceYAML, "missing-parent/synthetic-mail.log", "'synthetic-private-token/'"),
		strings.ReplaceAll(syntheticSourceYAML, "missing-parent/synthetic-mail.log", "'file:synthetic-private-token'"),
		strings.ReplaceAll(syntheticSourceYAML, "missing-parent/synthetic-mail.log", "'//synthetic-private-token/share/log'"),
		strings.ReplaceAll(syntheticSourceYAML, "missing-parent/synthetic-mail.log", "null"),
	}
	for _, field := range []string{"trusted_host", "start_at", "poll_interval", "rotation_grace", "resume_origins", "resume_entries"} {
		cases = append(cases, syntheticSourceYAML+"  "+field+": null\n")
	}
	for _, value := range []string{"0", "-1", "+1", "01", "0x10", "0o10", "1_000", "'10'", "!!int '10'", "!!int \"10\"", "true", "1.5", "[1]", "{value: 1}", "99999999999999999999999999999", "!synthetic-private-token 1", "'synthetic-private-token'"} {
		for _, field := range []string{"resume_origins", "resume_entries"} {
			cases = append(cases, syntheticSourceYAML+"  "+field+": "+value+"\n")
		}
	}
	cases = append(cases, syntheticSourceYAML+"  resume_origins: 1001\n", syntheticSourceYAML+"  resume_entries: 2001\n")
	for _, field := range []string{"poll_interval", "rotation_grace"} {
		for _, value := range []string{"0s", "-1s", "1ns", "25h", "synthetic-private-token", "999999999999999999999h", "1"} {
			cases = append(cases, syntheticSourceYAML+"  "+field+": "+value+"\n")
		}
	}
	for _, value := range []string{"''", "END", "synthetic-private-token"} {
		cases = append(cases, syntheticSourceYAML+"  start_at: "+value+"\n")
	}
	if filepath.Separator == '\\' {
		for _, path := range []string{`C:synthetic-private-token.log`, `\synthetic-private-token.log`, "/synthetic-private-token.log"} {
			cases = append(cases, strings.ReplaceAll(syntheticSourceYAML, "missing-parent/synthetic-mail.log", "'"+path+"'"))
		}
	}
	// This path meets the original bound but exceeds it after adding baseDir.
	cases = append(cases, strings.ReplaceAll(syntheticSourceYAML, "missing-parent/synthetic-mail.log", strings.Repeat("s", 4096)))
	for i, data := range cases {
		c, err := Decode(strings.NewReader(data), dir)
		if !errors.Is(err, ErrInvalid) || c != (Config{}) || strings.Contains(err.Error(), "synthetic-private-token") {
			t.Fatalf("case%d: expected zero config and safe rejection, got %#v / %v", i, c, err)
		}
	}
}

func TestDecodeSourcePreservesByteAndStructureBounds(t *testing.T) {
	dir := t.TempDir()
	data := "#" + strings.Repeat("x", MaxBytes-len(syntheticSourceYAML)-2) + "\n" + syntheticSourceYAML
	if len(data) != MaxBytes {
		t.Fatal("incorrect byte boundary fixture")
	}
	if c, err := Decode(strings.NewReader(data), dir); err != nil || c.Source == nil {
		t.Fatal("inclusive byte boundary refused", err)
	}
	r := &countReader{reader: strings.NewReader(data + "x")}
	if c, err := Decode(r, dir); !errors.Is(err, ErrInvalid) || c != (Config{}) || r.read != MaxBytes+1 {
		t.Fatal("byte bound expanded or partial config returned", err, r.read)
	}
	for _, data := range []string{
		syntheticSourceYAML + "  resume_entries: {nested: {nested: value}}\n",
		syntheticSourceYAML + "  resume_entries: [" + strings.Repeat("1,", 128) + "1]\n",
	} {
		if c, err := Decode(strings.NewReader(data), dir); !errors.Is(err, ErrInvalid) || c != (Config{}) || !strings.Contains(err.Error(), "structure") {
			t.Fatal("structure bounds expanded", err)
		}
	}
}

func TestLoadRepositorySourceExample(t *testing.T) {
	c, err := Load(filepath.Join("..", "..", "examples", "queueatlas-source.yaml"))
	if err != nil || c.Source == nil || !filepath.IsAbs(c.Source.Path) || c.Source.ID != "synthetic-postfix" {
		t.Fatal("documented source example no longer loads", c, err)
	}
}
