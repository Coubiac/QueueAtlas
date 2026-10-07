package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func assertDoctorJSON(t *testing.T, output string) {
	t.Helper()
	if len(output) > 128 || !strings.HasSuffix(output, "\n") || strings.Count(output, "\n") != 1 {
		t.Fatalf("unbounded or multiline doctor output: %q", output)
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(output), &fields); err != nil {
		t.Fatal("not a single JSON report", err)
	}
	if len(fields) != 2 || fields["configuration"] != "valid" || fields["database"] != "compatible" {
		t.Fatal("unexpected checks or values", fields)
	}
}

func TestDoctorJSONAndReadOnly(t *testing.T) {
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
	if code := run([]string{"doctor", "--config", configPath}, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatal("doctor failed", code, stderr.String())
	}
	assertDoctorJSON(t, stdout.String())
	afterDB, err := os.ReadFile(dbPath)
	if err != nil || !bytes.Equal(beforeDB, afterDB) {
		t.Fatal("doctor changed database bytes", err)
	}
	afterConfig, err := os.ReadFile(configPath)
	if err != nil || !bytes.Equal(beforeConfig, afterConfig) {
		t.Fatal("doctor changed configuration", err)
	}
}

func TestDoctorArgumentsAndHelp(t *testing.T) {
	for _, args := range [][]string{
		{"doctor"}, {"doctor", "--config"}, {"doctor", "--config", ""},
		{"doctor", "--config", "  "}, {"doctor", "synthetic-private-argument"},
		{"doctor", "--unknown", "synthetic-private-argument"},
		{"doctor", "--config=synthetic-private-argument"},
		{"doctor", "--config", "synthetic-private-argument", "extra"},
		{"doctor", "--config", "synthetic-private-argument", "--config", "other"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.String() != doctorUsage {
			t.Fatalf("usage rejection: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
	for _, flag := range []string{"--help", "-h"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"doctor", flag}, &stdout, &stderr); code != 0 || stdout.String() != doctorUsage || stderr.Len() != 0 {
			t.Fatal("doctor help failed", code, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "doctor --config <path>") || stderr.Len() != 0 {
		t.Fatal("global help omitted doctor", code)
	}
}

func TestDoctorFailureCodesAndNoCreation(t *testing.T) {
	testDiagnosticFailureCodesAndNoCreation(t, []string{"doctor"})
}

func TestDoctorOutputFailure(t *testing.T) {
	configPath, _ := dbStatsFixture(t)
	var stderr bytes.Buffer
	if code := run([]string{"doctor", "--config", configPath}, failingOutput{}, &stderr); code != 1 || stderr.String() != "queueatlas: cannot write output\n" {
		t.Fatal("success reported despite output failure", code, stderr.String())
	}
}
