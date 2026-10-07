//go:build linux

package auth

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLocalAccountLinuxPrivateModesAndUnsafePermissions(t *testing.T) {
	dir := t.TempDir()
	if err := CreateLocalAccount(dir, syntheticAccount()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, LocalAccountFilename)
	if info, err := os.Stat(path); err != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatal("credential exposed through POSIX permissions", err)
	}
	if err := os.Chmod(path, 0640); err != nil {
		t.Fatal(err)
	}
	if a, err := LoadLocalAccount(dir); a != (LocalAccount{}) || err != ErrAccountIO {
		t.Fatal("group-readable credential accepted", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0750); err != nil {
		t.Fatal(err)
	}
	if a, err := LoadLocalAccount(dir); a != (LocalAccount{}) || err != ErrAccountIO {
		t.Fatal("shared parent accepted", err)
	}
	if err := CreateLocalAccount(dir, syntheticAccount()); err != ErrAccountIO {
		t.Fatal("shared parent used for publication", err)
	}
}

func TestLocalAccountLinuxRejectsLinksAndFIFOWithoutReplacement(t *testing.T) {
	for _, kind := range []string{"symlink", "dangling", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, LocalAccountFilename)
			if kind == "fifo" {
				if err := syscall.Mkfifo(path, 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				target := filepath.Join(dir, "synthetic-private-target")
				if kind == "symlink" {
					if err := os.WriteFile(target, []byte(syntheticAccountJSON()), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if a, err := LoadLocalAccount(dir); a != (LocalAccount{}) || err != ErrAccountIO {
				t.Fatal("nonregular credential loaded", err)
			}
			// Direct open guards also refuse the link and never block on FIFO.
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			f, openErr := openAccountFile(root)
			if f != nil {
				f.Close()
			}
			root.Close()
			if kind != "fifo" && openErr == nil {
				t.Fatal("nofollow guard accepted symlink")
			}
			if err := CreateLocalAccount(dir, syntheticAccount()); err != ErrAccountExists {
				t.Fatal("nonregular destination replaced", err)
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("refusal replaced the entry", err)
			}
		})
	}
}
