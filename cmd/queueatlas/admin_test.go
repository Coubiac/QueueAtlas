package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/auth"
)

func adminTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func adminArgs(dir, username string) []string {
	return []string{"admin", "create", "--directory", dir, "--username", username, "--password-stdin"}
}

type adminInputFailure struct{ reads int }

func (r *adminInputFailure) Read([]byte) (int, error) {
	r.reads++
	return 0, errors.New("synthetic-private-input-error")
}

func TestAdminArgumentsAndHelpDoNotReadInput(t *testing.T) {
	for _, args := range [][]string{
		{"admin"}, {"admin", "reset"}, {"admin", "create"},
		{"admin", "create", "--directory", "synthetic-private", "--username", "operator"},
		{"admin", "create", "--directory", "", "--username", "operator", "--password-stdin"},
		{"admin", "create", "--directory", "synthetic-private", "--username", "invalid/name", "--password-stdin"},
		{"admin", "create", "--directory=synthetic-private", "--username", "operator", "--password-stdin"},
		{"admin", "create", "--directory", "synthetic-private", "--username", "operator", "--password", "synthetic-private-secret"},
		append(adminArgs("synthetic-private", "operator"), "extra"),
		{"admin", "create", "--username", "operator", "--directory", "synthetic-private", "--password-stdin"},
	} {
		input := &adminInputFailure{}
		var stdout, stderr bytes.Buffer
		if code := runWithInput(args, input, &stdout, &stderr); code != 2 || input.reads != 0 || stdout.Len() != 0 || strings.Contains(stderr.String(), "synthetic-private") || strings.Contains(stderr.String(), "invalid/name") {
			t.Fatal("invalid args caused IO or disclosure", code)
		}
	}
	for _, args := range [][]string{{"admin", "--help"}, {"admin", "-h"}, {"admin", "create", "--help"}, {"admin", "create", "-h"}, {"--help"}} {
		input := &adminInputFailure{}
		var stdout, stderr bytes.Buffer
		if code := runWithInput(args, input, &stdout, &stderr); code != 0 || input.reads != 0 || stderr.Len() != 0 || !strings.Contains(stdout.String(), "admin create") {
			t.Fatal("help performed IO or omitted command", code)
		}
	}
}

func TestAdminPasswordTransportBoundedAndLiteral(t *testing.T) {
	secret := "  unique synthetic phrase 🔐  "
	for _, suffix := range []string{"", "\n", "\r\n"} {
		got, err := readAdminPassword(strings.NewReader(secret + suffix))
		if err != nil || string(got) != secret {
			t.Fatal("transport changed literal password", err)
		}
	}
	boundary := strings.Repeat("🔐", 256)
	if got, err := readAdminPassword(strings.NewReader(boundary + "\r\n")); err != nil || string(got) != boundary {
		t.Fatal("inclusive byte/code point bound rejected", err)
	}
	for _, input := range []string{"", "short", secret + "\n\n", secret + "\r", secret + "\nsecond", secret + string([]byte{0xff}), strings.Repeat("x", 257), strings.Repeat("🔐", 257)} {
		if got, err := readAdminPassword(strings.NewReader(input)); err != auth.ErrInvalidPassword || got != nil {
			t.Fatal("bad transport returned password", err)
		}
	}
	if got, err := readAdminPassword(nil); got != nil || err != auth.ErrInvalidPassword {
		t.Fatal("absent input accepted", err)
	}
	if got, err := readAdminPassword(&adminInputFailure{}); got != nil || err != errPasswordInputIO {
		t.Fatal("input failure returned partial secret or raw error", err)
	}
	reader := &io.LimitedReader{R: strings.NewReader(strings.Repeat("x", 10000)), N: 10000}
	if _, err := readAdminPassword(reader); err != auth.ErrInvalidPassword || reader.N != 10000-(auth.MaxPasswordBytes+3) {
		t.Fatal("input not bounded", err, reader.N)
	}
	device, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close()
	if got, err := readAdminPassword(device); got != nil || err != auth.ErrInvalidPassword {
		t.Fatal("character device accepted", err)
	}
}

func TestAdminCreatesVerifiableRecordAndRefusesReplacement(t *testing.T) {
	dir := adminTestDir(t)
	secret := "  unique synthetic phrase 🔐  "
	var stdout, stderr bytes.Buffer
	args := adminArgs(dir, "synthetic-operator")
	if code := runWithInput(args, strings.NewReader(secret+"\r\n"), &stdout, &stderr); code != 0 || stdout.String() != "Local administrator created\n" || stderr.Len() != 0 {
		t.Fatal("enrollment failed", code, stderr.String())
	}
	a, err := auth.LoadLocalAccount(dir)
	if err != nil || a.Identity.Username != "synthetic-operator" {
		t.Fatal("wrong persisted account", err)
	}
	if match, err := auth.VerifyPassword([]byte(secret), a.PasswordHash); !match || err != nil {
		t.Fatal("literal stdin password not persisted", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, auth.LocalAccountFilename))
	if err != nil || bytes.Contains(before, []byte(secret)) {
		t.Fatal("plaintext persisted", err)
	}
	input := &adminInputFailure{}
	stdout.Reset()
	stderr.Reset()
	if code := runWithInput(adminArgs(dir, "synthetic-other"), input, &stdout, &stderr); code != 1 || input.reads != 0 || stdout.Len() != 0 || stderr.String() != "queueatlas: local account already exists\n" {
		t.Fatal("repeat consumed password or claimed success", code)
	}
	after, err := os.ReadFile(filepath.Join(dir, auth.LocalAccountFilename))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("repeat changed account", err)
	}
}

func TestAdminRefusalsLeaveNoAccountAndNoPrivateDiagnostics(t *testing.T) {
	dir := adminTestDir(t)
	for _, tc := range []struct {
		dir   string
		input io.Reader
		code  int
	}{
		{filepath.Join(dir, "synthetic-private-missing"), &adminInputFailure{}, 1},
		{dir, &adminInputFailure{}, 1},
		{dir, strings.NewReader("synthetic-private\nsecond"), 2},
		{dir, strings.NewReader("passwordpassword"), 2},
		{dir, strings.NewReader("qwertyuiopasdfghjkl"), 2},
		{dir, strings.NewReader("  QWERTYUIOPASDFGHJKL  "), 2},
		{dir, strings.NewReader(strings.Repeat(" ", 15)), 2},
		{dir, strings.NewReader("synthetic-operator2026!"), 2},
	} {
		var stdout, stderr bytes.Buffer
		if code := runWithInput(adminArgs(tc.dir, "synthetic-operator"), tc.input, &stdout, &stderr); code != tc.code || stdout.Len() != 0 || stderr.Len() == 0 || strings.Contains(stderr.String(), "synthetic-private") || strings.Contains(stderr.String(), "synthetic-operator") {
			t.Fatal("unsafe enrollment failure", code)
		}
		if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
			t.Fatal("refusal created state", err)
		}
		if strings.Contains(stderr.String(), "qwerty") || strings.Contains(stderr.String(), "QWERTY") {
			t.Fatal("enrollment diagnostic exposed synthetic password")
		}
	}
	path := filepath.Join(dir, auth.LocalAccountFilename)
	const corrupt = "synthetic-private-corrupt-account"
	if err := os.WriteFile(path, []byte(corrupt), 0600); err != nil {
		t.Fatal(err)
	}
	input := &adminInputFailure{}
	var stdout, stderr bytes.Buffer
	if code := runWithInput(adminArgs(dir, "synthetic-operator"), input, &stdout, &stderr); code != 1 || input.reads != 0 || stdout.Len() != 0 || stderr.String() != "queueatlas: cannot access local account storage\n" {
		t.Fatal("corrupt occupied storage consumed secret or claimed success", code)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != corrupt {
		t.Fatal("corrupt occupied storage overwritten", err)
	}
}

func TestAdminOutputFailurePreservesPublishedAccount(t *testing.T) {
	dir := adminTestDir(t)
	var stderr bytes.Buffer
	if code := runWithInput(adminArgs(dir, "synthetic-operator"), strings.NewReader("unique synthetic phrase"), failingOutput{}, &stderr); code != 1 || stderr.String() != "queueatlas: cannot write output\n" {
		t.Fatal("stdout failure claimed success", code)
	}
	if _, err := auth.LoadLocalAccount(dir); err != nil {
		t.Fatal("stdout failure rolled back published account", err)
	}
}

// Called by the existing linked-binary test, reusing its single build.
func assertAdminBinaryEnrollment(t *testing.T, ctx context.Context, binary string) {
	t.Helper()
	dir := adminTestDir(t)
	secret := "  unique synthetic binary phrase 🔐  "
	for _, tc := range []struct {
		args  []string
		input string
		code  int
		out   string
	}{
		{adminArgs(dir, "synthetic-binary"), "passwordpassword\n", 2, ""},
		{adminArgs(dir, "synthetic-binary"), "  QWERTYUIOPASDFGHJKL  \n", 2, ""},
		{adminArgs(dir, "synthetic-binary"), strings.Repeat(" ", 15) + "\n", 2, ""},
		{adminArgs(filepath.Join(dir, "synthetic-private-missing"), "synthetic-binary"), secret, 1, ""},
		{adminArgs(dir, "synthetic-binary"), secret + "\r\n", 0, "Local administrator created\n"},
		{adminArgs(dir, "synthetic-other"), "synthetic-private", 1, ""},
	} {
		command := exec.CommandContext(ctx, binary, tc.args...)
		command.Stdin = strings.NewReader(tc.input)
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
		if code != tc.code || stdout.String() != tc.out || (tc.code == 0 && stderr.Len() != 0) || (tc.code != 0 && stderr.Len() == 0) || strings.Contains(stderr.String(), "synthetic") || strings.Contains(stderr.String(), "QWERTY") {
			t.Fatal("binary enrollment streams/codes incorrect", code)
		}
	}
	a, err := auth.LoadLocalAccount(dir)
	if err != nil || a.Identity.Username != "synthetic-binary" {
		t.Fatal("binary created wrong account", err)
	}
	if match, err := auth.VerifyPassword([]byte(secret), a.PasswordHash); !match || err != nil {
		t.Fatal("binary password transport changed bytes", err)
	}
}
