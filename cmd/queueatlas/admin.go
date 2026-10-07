package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Coubiac/QueueAtlas/internal/auth"
)

const adminUsage = "Usage: queueatlas admin create --directory <path> --username <name> --password-stdin\n"

var errPasswordInputIO = errors.New("cannot read password input")

func adminCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") ||
		len(args) == 2 && args[0] == "create" && (args[1] == "--help" || args[1] == "-h") {
		return writeOutput(adminUsage, stdout, stderr)
	}
	if len(args) != 6 || args[0] != "create" || args[1] != "--directory" ||
		strings.TrimSpace(args[2]) == "" || args[3] != "--username" || args[5] != "--password-stdin" {
		fmt.Fprint(stderr, adminUsage)
		return 2
	}
	identity, err := auth.NewLocalIdentity(args[4])
	if err != nil {
		fmt.Fprintln(stderr, "queueatlas: invalid local username")
		return 2
	}
	// Check storage before requesting a secret. Create still arbitrates concurrent
	// publication atomically; this preflight does not reserve the destination.
	if _, err := auth.LoadLocalAccount(args[2]); err == nil {
		fmt.Fprintln(stderr, "queueatlas: local account already exists")
		return 1
	} else if !errors.Is(err, auth.ErrAccountNotFound) {
		fmt.Fprintln(stderr, "queueatlas: cannot access local account storage")
		return 1
	}
	password, err := readAdminPassword(stdin)
	if err != nil {
		if errors.Is(err, errPasswordInputIO) {
			fmt.Fprintln(stderr, "queueatlas: cannot read password input")
			return 1
		}
		fmt.Fprintln(stderr, "queueatlas: password input must be one UTF-8 record of 15 to 256 characters (at most 1024 bytes) from a pipe or file")
		return 2
	}
	defer clear(password) // best effort; copied strings/hash implementation may remain
	if err := auth.ValidateNewPassword(password, identity); err != nil {
		if errors.Is(err, auth.ErrBlockedPassword) {
			fmt.Fprintln(stderr, "queueatlas: password is common or account-related; choose a different generated password or passphrase")
		} else {
			fmt.Fprintln(stderr, "queueatlas: password must contain 15 to 256 UTF-8 characters, at most 1024 bytes")
		}
		return 2
	}
	encoded, err := auth.HashPassword(password, auth.DefaultParameters())
	if err != nil {
		fmt.Fprintln(stderr, "queueatlas: cannot hash local password")
		return 1
	}
	err = auth.CreateLocalAccount(args[2], auth.LocalAccount{Identity: identity, PasswordHash: encoded})
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrAccountExists):
			fmt.Fprintln(stderr, "queueatlas: local account already exists")
		case errors.Is(err, auth.ErrAccountPublished):
			fmt.Fprintln(stderr, "queueatlas: local account created but storage finalization failed; inspect before retrying")
		default:
			fmt.Fprintln(stderr, "queueatlas: cannot create local account")
		}
		return 1
	}
	return writeOutput("Local administrator created\n", stdout, stderr)
}

// The transport permits one trailing LF or CRLF; all other bytes are literal.
// EOF is required, with no deadline for a stalled trusted local pipe. Character
// devices (including consoles) are refused rather than reading echoed secrets.
func readAdminPassword(stdin io.Reader) ([]byte, error) {
	if stdin == nil {
		return nil, auth.ErrInvalidPassword
	}
	if file, ok := stdin.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return nil, errPasswordInputIO
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return nil, auth.ErrInvalidPassword
		}
	}
	data, err := io.ReadAll(io.LimitReader(stdin, auth.MaxPasswordBytes+3))
	if err != nil {
		clear(data)
		return nil, errPasswordInputIO
	}
	if len(data) > auth.MaxPasswordBytes+2 {
		clear(data)
		return nil, auth.ErrInvalidPassword
	}
	password := bytes.TrimSuffix(data, []byte("\n"))
	if len(password) < len(data) {
		password = bytes.TrimSuffix(password, []byte("\r"))
	}
	if bytes.ContainsAny(password, "\r\n") || auth.ValidatePassword(password) != nil {
		clear(data)
		return nil, auth.ErrInvalidPassword
	}
	return password, nil
}
