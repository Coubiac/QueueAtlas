package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func dbStatsFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "synthetic-private.db")
	s, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "synthetic-private.yaml")
	if err := os.WriteFile(configPath, []byte("storage:\n  path: synthetic-private.db\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return configPath, path
}

func assertDBStatsJSON(t *testing.T, output string) {
	t.Helper()
	if len(output) > 512 || !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 ||
		strings.Contains(output, "synthetic-private") {
		t.Fatalf("unbounded or private output: %q", output)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &fields); err != nil {
		t.Fatal("not a single JSON result", err)
	}
	if len(fields) != 6 {
		t.Fatal("unexpected JSON fields", fields)
	}
	for _, key := range []string{"schema_version", "sqlite_version", "journal_mode", "page_size", "page_count", "free_page_count"} {
		if _, ok := fields[key]; !ok {
			t.Fatal("missing JSON field", key)
		}
	}
	var result dbStatsOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 7 || result.SQLiteVersion == "" || result.JournalMode != "wal" ||
		result.PageSize < 512 || result.PageSize > 65536 || result.PageSize&(result.PageSize-1) != 0 ||
		result.PageCount < 1 || result.FreePageCount < 0 || result.FreePageCount > result.PageCount {
		t.Fatalf("wrong metadata: %+v", result)
	}
}

func TestDBStatsJSONAndReadOnly(t *testing.T) {
	configPath, dbPath := dbStatsFixture(t)
	beforeDB, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	beforeConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"db", "stats", "--config", configPath}, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatal("stats failed", code, stderr.String())
	}
	assertDBStatsJSON(t, stdout.String())
	var result dbStatsOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.PageCount*result.PageSize != int64(len(beforeDB)) {
		t.Fatal("wrong logical size for checkpointed fixture", result)
	}
	afterDB, err := os.ReadFile(dbPath)
	if err != nil || !bytes.Equal(beforeDB, afterDB) {
		t.Fatal("stats changed database bytes", err)
	}
	afterConfig, err := os.ReadFile(configPath)
	if err != nil || !bytes.Equal(beforeConfig, afterConfig) {
		t.Fatal("stats changed configuration", err)
	}
}

func TestDBStatsFailureCodesAndNoCreation(t *testing.T) {
	for _, tc := range []struct {
		name, config, input, diagnostic string
		code                            int
	}{
		{"missing config", "", "", "queueatlas: cannot read configuration\n", 1},
		{"invalid config", "synthetic-private-field: true\n", "", "queueatlas: invalid configuration: yaml: unknown field\n", 2},
		{"missing database", "{}\n", "", "queueatlas: cannot open database for diagnostics\n", 1},
		{"empty database", "{}\n", "empty", "queueatlas: database schema is not supported for diagnostics\n", 1},
		{"corrupt database", "{}\n", "corrupt", "queueatlas: cannot open database for diagnostics\n", 1},
		{"directory database", "{}\n", "directory", "queueatlas: cannot open database for diagnostics\n", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "synthetic-private.yaml")
			if tc.config != "" {
				if err := os.WriteFile(configPath, []byte(tc.config), 0600); err != nil {
					t.Fatal(err)
				}
			}
			dbPath := filepath.Join(dir, "queueatlas.db")
			var original []byte
			switch tc.input {
			case "empty", "corrupt":
				if tc.input == "corrupt" {
					original = []byte("synthetic-private-corrupt-input")
				}
				if err := os.WriteFile(dbPath, original, 0600); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(dbPath, 0700); err != nil {
					t.Fatal(err)
				}
			}
			var stdout, stderr bytes.Buffer
			if code := run([]string{"db", "stats", "--config", configPath}, &stdout, &stderr); code != tc.code || stdout.Len() != 0 ||
				stderr.String() != tc.diagnostic || strings.Contains(stderr.String(), "synthetic-private") {
				t.Fatal("wrong failure result", code, stdout.String(), stderr.String())
			}
			if tc.input == "" {
				if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
					t.Fatal("failure created a database", err)
				}
			} else if tc.input == "empty" || tc.input == "corrupt" {
				data, err := os.ReadFile(dbPath)
				if err != nil || !bytes.Equal(data, original) {
					t.Fatal("failure initialized or changed input", err)
				}
			}
		})
	}
}

func TestDBStatsArgumentsAndHelp(t *testing.T) {
	for _, args := range [][]string{
		{"db"}, {"db", "synthetic-private-subcommand"}, {"db", "stats"},
		{"db", "stats", "--config"}, {"db", "stats", "--config", ""},
		{"db", "stats", "--config", "  "}, {"db", "stats", "--unknown", "synthetic-private"},
		{"db", "stats", "--config=synthetic-private"},
		{"db", "stats", "--config", "synthetic-private", "extra"},
		{"db", "stats", "--config", "synthetic-private", "--config", "other"},
		{"db", "--help", "extra"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.String() != dbUsage {
			t.Fatal("wrong usage result", code, stdout.String(), stderr.String())
		}
	}
	for _, args := range [][]string{{"db", "--help"}, {"db", "-h"}, {"db", "stats", "--help"}, {"db", "stats", "-h"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 0 || stderr.Len() != 0 || stdout.String() != dbUsage {
			t.Fatal("wrong help result", code, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "db stats --config <path>") {
		t.Fatal("global help missing db stats", code)
	}
}

func TestDBStatsOutputFailure(t *testing.T) {
	configPath, _ := dbStatsFixture(t)
	var stderr bytes.Buffer
	if code := run([]string{"db", "stats", "--config", configPath}, failingOutput{}, &stderr); code != 1 || stderr.String() != "queueatlas: cannot write output\n" {
		t.Fatal("wrong output failure", code, stderr.String())
	}
}
