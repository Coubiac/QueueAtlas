//go:build windows

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenPathSnapshotDoesNotFollowLaterWindowsReplacement(t *testing.T) {
	writer, path := testRegularFile(t, "before\n")
	before, err := statPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement\n"), 0600); err != nil {
		t.Fatal(err)
	}
	opened, err := openReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	got, _, err := validateOpen(context.Background(), opened, path, before)
	if got != nil || !errors.Is(err, ErrPathChanged) {
		t.Fatalf("replacement against path snapshot: %v, %v", got, err)
	}
	if err := opened.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("replacement failure retained descriptor", err)
	}
}

func TestOpenLogWindowsLongPathSnapshots(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("a", 100), strings.Repeat("b", 100), strings.Repeat("c", 100))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "mail.log")
	if err := os.WriteFile(path, []byte("synthetic\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{path, `\\?\` + path} {
		t.Run(name, func(t *testing.T) {
			f, id, err := OpenLog(context.Background(), name)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			observation, err := ObservePath(context.Background(), f, name)
			if err != nil || observation.Status != PathSame || !id.SameFile(observation.Current) {
				t.Fatalf("long path identity: %+v, %v", observation, err)
			}
		})
	}
}
