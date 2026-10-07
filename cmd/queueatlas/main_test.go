package main

import (
	"bytes"
	"context"
	"errors"
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
	for _, test := range []struct {
		args []string
		code int
		out  string
	}{
		{[]string{"version"}, 0, "QueueAtlas synthetic-build\n"},
		{[]string{"synthetic-private-argument"}, 2, ""},
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
		if code != test.code || stdout.String() != test.out || (code == 0 && stderr.Len() != 0) ||
			(code == 2 && (!strings.Contains(stderr.String(), "queueatlas version") || strings.Contains(stderr.String(), test.args[0]))) {
			t.Fatalf("process: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	}
}
