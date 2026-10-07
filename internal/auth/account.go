package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"unicode/utf8"
)

const (
	LocalAccountFilename = "local-admin.json"
	MaxAccountBytes      = 4096
)

var (
	ErrAccountExists    = errors.New("local account already exists")
	ErrAccountNotFound  = errors.New("local account not found")
	ErrInvalidAccount   = errors.New("invalid local account record")
	ErrAccountIO        = errors.New("cannot access local account storage")
	ErrAccountPublished = errors.New("local account published; cleanup or directory synchronization failed")
)

// LocalAccount contains only the operator identity and its encoded credential,
// not a plaintext password or session. Validation does not authenticate the user,
// grant roles or compute a password hash. Callers must not log the credential.
type LocalAccount struct {
	Identity     LocalIdentity
	PasswordHash string
}

func (a LocalAccount) Validate() error {
	if err := a.Identity.Validate(); err != nil {
		return err
	}
	return ValidatePasswordHash(a.PasswordHash)
}

// CreateLocalAccount publishes a complete versioned record without replacement.
// directory must already be trusted and private; no parent is created or chmoded.
// A synced temporary file is hard-linked into the fixed final name, then removed.
// Unsupported links fail closed. Concurrent creators have at most one winner.
// ErrAccountPublished means the final record exists but a later step failed:
// inspect it before retrying. There is no reset, overwrite or password hashing.
func CreateLocalAccount(directory string, a LocalAccount) error {
	if err := a.Validate(); err != nil {
		return err
	}
	root, err := openAccountRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := root.Lstat(LocalAccountFilename); err == nil {
		return ErrAccountExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return ErrAccountIO
	}
	data, err := json.Marshal(struct {
		Version      int    `json:"version"`
		Username     string `json:"username"`
		PasswordHash string `json:"password_hash"`
	}{1, a.Identity.Username, a.PasswordHash})
	if err != nil {
		return ErrInvalidAccount
	}
	data = append(data, '\n')
	name := ".local-admin-" + rand.Text() + ".tmp"
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return ErrAccountIO
	}
	defer root.Remove(name) // best effort for unpublished temporaries, never final
	n, writeErr := f.Write(data)
	if writeErr != nil || n != len(data) {
		f.Close()
		return ErrAccountIO
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return ErrAccountIO
	}
	if err := f.Close(); err != nil {
		return ErrAccountIO
	}
	if err := root.Link(name, LocalAccountFilename); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrAccountExists
		}
		return ErrAccountIO
	}
	if err := root.Remove(name); err != nil {
		return ErrAccountPublished
	}
	if err := syncAccountDirectory(root); err != nil {
		return ErrAccountPublished
	}
	return nil
}

// LoadLocalAccount reads one regular, bounded record with syntax-only credential
// validation. Linux additionally refuses symlinks with a nonblocking no-follow
// open; all platforms check type and file identity before reading. A trusted
// directory/owner remains required; this is not a lock against in-place writes.
// Any failure returns the zero account and a fixed diagnostic without path/data.
func LoadLocalAccount(directory string) (LocalAccount, error) {
	root, err := openAccountRoot(directory)
	if err != nil {
		return LocalAccount{}, err
	}
	defer root.Close()
	before, err := root.Lstat(LocalAccountFilename)
	if errors.Is(err, os.ErrNotExist) {
		return LocalAccount{}, ErrAccountNotFound
	}
	if err != nil || !privateAccountFile(before) {
		return LocalAccount{}, ErrAccountIO
	}
	f, err := openAccountFile(root)
	if err != nil {
		return LocalAccount{}, ErrAccountIO
	}
	opened, err := f.Stat()
	if err != nil || !privateAccountFile(opened) || !os.SameFile(before, opened) {
		f.Close()
		return LocalAccount{}, ErrAccountIO
	}
	a, readErr := decodeLocalAccount(f)
	after, statErr := f.Stat()
	closeErr := f.Close()
	if readErr != nil {
		return LocalAccount{}, readErr
	}
	if statErr != nil || closeErr != nil || !privateAccountFile(after) ||
		opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return LocalAccount{}, ErrAccountIO
	}
	return a, nil
}

func openAccountRoot(directory string) (*os.Root, error) {
	if directory == "" {
		return nil, ErrAccountIO
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, ErrAccountIO
	}
	info, err := root.Stat(".")
	if err != nil || !info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		root.Close()
		return nil, ErrAccountIO
	}
	return root, nil
}

func privateAccountFile(info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Size() <= MaxAccountBytes &&
		(runtime.GOOS == "windows" || info.Mode().Perm()&0077 == 0)
}

func decodeLocalAccount(reader io.Reader) (LocalAccount, error) {
	if reader == nil {
		return LocalAccount{}, ErrAccountIO
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxAccountBytes+1))
	if err != nil {
		return LocalAccount{}, ErrAccountIO
	}
	if len(data) > MaxAccountBytes || !utf8.Valid(data) {
		return LocalAccount{}, ErrInvalidAccount
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return LocalAccount{}, ErrInvalidAccount
	}
	var a LocalAccount
	seen := make(map[string]bool)
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return LocalAccount{}, ErrInvalidAccount
		}
		field, ok := key.(string)
		if !ok || seen[field] || (field != "version" && field != "username" && field != "password_hash") {
			return LocalAccount{}, ErrInvalidAccount
		}
		seen[field] = true
		value, err := dec.Token()
		if err != nil {
			return LocalAccount{}, ErrInvalidAccount
		}
		if field == "version" {
			if version, ok := value.(json.Number); !ok || version != "1" {
				return LocalAccount{}, ErrInvalidAccount
			}
			continue
		}
		text, ok := value.(string)
		if !ok {
			return LocalAccount{}, ErrInvalidAccount
		}
		if field == "username" {
			a.Identity.Username = text
		} else {
			a.PasswordHash = text
		}
	}
	if token, err := dec.Token(); err != nil || token != json.Delim('}') || len(seen) != 3 {
		return LocalAccount{}, ErrInvalidAccount
	}
	if _, err := dec.Token(); err != io.EOF || a.Validate() != nil {
		return LocalAccount{}, ErrInvalidAccount
	}
	return a, nil
}
