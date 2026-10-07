package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunVersionAndHelpSeparateOutputStreams(t *testing.T) {
	for _, command := range []string{"version", "--help", "-h"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run([]string{command}, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
				t.Fatalf("successful command: code=%d stderr=%q", code, stderr.String())
			}
			if command == "version" {
				if stdout.String() != "QueueAtlas dev\n" {
					t.Fatalf("unlabelled build version: %q", stdout.String())
				}
			} else if !strings.Contains(stdout.String(), "queueatlas version") {
				t.Fatalf("help omitted the available command: %q", stdout.String())
			}
		})
	}
}

func TestRunRejectsInvalidArgumentsWithoutEchoingThem(t *testing.T) {
	for _, args := range [][]string{nil, {"synthetic-private-argument"}, {"version", "synthetic-private-argument"}, {"--help", "extra"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 ||
			!strings.Contains(stderr.String(), "queueatlas version") || strings.Contains(stderr.String(), "synthetic-private-argument") {
			t.Fatalf("invalid arguments: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
}

type failingOutput struct{}

func (failingOutput) Write([]byte) (int, error) { return 0, errors.New("synthetic writer failure") }

func TestRunReportsOutputFailure(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"version"}, failingOutput{}, &stderr); code != 1 || stderr.String() != "queueatlas: cannot write output\n" {
		t.Fatalf("output failure: code=%d stderr=%q", code, stderr.String())
	}
}

func TestCheckConfigOutputAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	const private = "synthetic-private-value"
	valid := filepath.Join(dir, private+"-valid.yaml")
	invalid := filepath.Join(dir, private+"-invalid.yaml")
	data := "storage:\n  path: '" + private + ".db'\n"
	for path, contents := range map[string]string{valid: data, invalid: "server:\n  listen: " + private + "\n"} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		path string
		code int
		out  string
		err  string
	}{
		{valid, 0, "Configuration valid\n", ""},
		{invalid, 2, "", "queueatlas: invalid configuration: server.listen: expected a loopback IP and numeric port\n"},
		{filepath.Join(dir, private+"-missing.yaml"), 1, "", "queueatlas: cannot read configuration\n"},
	} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"check-config", "--config", tc.path}, &stdout, &stderr)
		if code != tc.code || stdout.String() != tc.out || stderr.String() != tc.err || strings.Contains(stdout.String()+stderr.String(), private) {
			t.Fatalf("check-config: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("checking created database state", entries, err)
	}
	contents, err := os.ReadFile(valid)
	if err != nil || string(contents) != data {
		t.Fatal("checking modified its input", err)
	}
}

func TestCheckConfigArgumentsAndHelp(t *testing.T) {
	for _, args := range [][]string{
		{"check-config"}, {"check-config", "--config"}, {"check-config", "--config", ""},
		{"check-config", "--config", "  "}, {"check-config", "synthetic-private-argument"},
		{"check-config", "--unknown", "synthetic-private-argument"},
		{"check-config", "--config=synthetic-private-argument"},
		{"check-config", "--config", "synthetic-private-argument", "extra"},
		{"check-config", "--config", "synthetic-private-argument", "--config", "other"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 || stdout.Len() != 0 || stderr.String() != checkConfigUsage {
			t.Fatalf("usage rejection: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
	for _, flag := range []string{"--help", "-h"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"check-config", flag}, &stdout, &stderr); code != 0 || stdout.String() != checkConfigUsage || stderr.Len() != 0 {
			t.Fatalf("command help: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--help"}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "check-config --config <path>") || stderr.Len() != 0 {
		t.Fatal("global help omitted check-config", code)
	}
}

func TestCheckConfigOutputFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "synthetic.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{"check-config", "--config", path}, failingOutput{}, &stderr); code != 1 || stderr.String() != "queueatlas: cannot write output\n" {
		t.Fatal("success reported despite output failure", code, stderr.String())
	}
}

func TestLinkedBinaryVersionAndProcessExitCodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	name := "queueatlas"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	build := exec.CommandContext(ctx, "go", "build", "-ldflags=-X main.version=synthetic-build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build command: %v\n%s", err, output)
	}
	dir := t.TempDir()
	valid := filepath.Join(dir, "synthetic-private-valid.yaml")
	invalid := filepath.Join(dir, "synthetic-private-invalid.yaml")
	for path, data := range map[string]string{valid: "{}\n", invalid: "synthetic-private-value: true\n"} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		args []string
		code int
		out  string
		err  string
	}{
		{[]string{"version"}, 0, "QueueAtlas synthetic-build\n", ""},
		{[]string{"synthetic-private-argument"}, 2, "", usage},
		{[]string{"check-config", "--config", valid}, 0, "Configuration valid\n", ""},
		{[]string{"check-config", "--config", invalid}, 2, "", "queueatlas: invalid configuration: yaml: unknown field\n"},
		{[]string{"check-config", "--config", filepath.Join(dir, "synthetic-private-missing.yaml")}, 1, "", "queueatlas: cannot read configuration\n"},
		{[]string{"check-config"}, 2, "", checkConfigUsage},
	} {
		command := exec.CommandContext(ctx, binary, test.args...)
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		if code != test.code || stdout.String() != test.out || stderr.String() != test.err || strings.Contains(stdout.String()+stderr.String(), "synthetic-private") {
			t.Fatalf("process: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("compiled check-config created state", entries, err)
	}
}
