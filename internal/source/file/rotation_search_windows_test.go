//go:build windows

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

// Mutate at a context boundary to exercise the public opening sequence without
// racing a goroutine or adding a production filesystem hook.
type rotationMutationContext struct {
	context.Context
	calls, at int
	mutate    func() error
	err       error
}

func (c *rotationMutationContext) Err() error {
	c.calls++
	if c.calls == c.at {
		c.err = c.mutate()
	}
	if c.err != nil {
		return c.err
	}
	return c.Context.Err()
}

func TestSelectRotationRejectsDirectorySubstitutionWindows(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "rotations")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	ctx := &rotationMutationContext{Context: context.Background(), at: 2, mutate: func() error {
		if err := os.Rename(directory, directory+".old"); err != nil {
			return err
		}
		return os.Mkdir(directory, 0700)
	}}
	result, err := SelectRotation(ctx, directory, source.OriginState{}, 1)
	if !errors.Is(err, ErrPathChanged) || result != (RotationSelection{}) {
		t.Fatalf("directory substitution accepted: %+v, %v", result, err)
	}
	// Removing both directories also detects a leaked directory handle on Windows.
	for _, name := range []string{directory, directory + ".old"} {
		if err := os.Remove(name); err != nil {
			t.Fatal("directory retained after refusal", err)
		}
	}
}

func TestRotationEntryRejectsDeferredIdentitySubstitutionWindows(t *testing.T) {
	f, path := testRegularFile(t, "old\n")
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	// Simulate the Windows directory enumeration fallback with deferred file IDs.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	entry := fs.FileInfoToDirEntry(info)
	ctx := &rotationMutationContext{Context: context.Background(), at: 1, mutate: func() error {
		if err := os.Rename(path, path+".old"); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("replacement\n"), 0600)
	}}
	check, eligible, err := checkRotationEntry(ctx, path, entry, source.OriginState{})
	if !errors.Is(err, ErrPathChanged) || eligible || check != (ResumeCheck{}) {
		t.Fatalf("deferred entry substitution accepted: %+v, %v, %v", check, eligible, err)
	}
	for _, name := range []string{path, path + ".old"} {
		if err := os.Remove(name); err != nil {
			t.Fatal("entry descriptor retained after refusal", err)
		}
	}
}

func TestRotationEntryAcceptsUnchangedDeferredSnapshotWindows(t *testing.T) {
	f, path := testRegularFile(t, "unchanged\n")
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	state := source.OriginState{Origin: source.Origin{ID: "synthetic-origin"}}
	check, eligible, err := checkRotationEntry(context.Background(), path, fs.FileInfoToDirEntry(info), state)
	if err != nil || !eligible || check.Status != ResumeInsufficient || check.Reason != ReasonIdentityUnavailable {
		t.Fatalf("unchanged entry rejected: %+v, %v, %v", check, eligible, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal("unchanged entry descriptor retained", err)
	}
}
