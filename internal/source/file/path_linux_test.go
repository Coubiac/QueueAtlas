//go:build linux

package file

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestObservePathRenameDisappearanceReappearanceAndReplacement(t *testing.T) {
	f, path := testRegularFile(t, "old\n")
	if _, err := f.Seek(2, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	rotated := path + ".1"
	if err := os.Rename(path, rotated); err != nil {
		t.Fatal(err)
	}
	check := func(want PathStatus) PathObservation {
		t.Helper()
		observation, err := ObservePath(context.Background(), f, path)
		if err != nil || observation.Status != want || observation.Opened.Device == "" || observation.Opened.Inode == "" {
			t.Fatalf("want %s: %+v, %v", want, observation, err)
		}
		assertDescriptorPosition(t, f, 2)
		return observation
	}
	check(PathMissing)
	if err := os.Rename(rotated, path); err != nil {
		t.Fatal(err)
	}
	check(PathSame)
	if err := os.Rename(path, rotated); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	observation := check(PathReplaced)
	if observation.Opened.Device != observation.Current.Device || observation.Opened.Inode == observation.Current.Inode {
		t.Fatal("equal bytes masked physical replacement")
	}
	if _, err := f.WriteAt([]byte("late\n"), 4); err != nil {
		t.Fatal(err)
	}
	observation = check(PathReplaced)
	if observation.Opened.Size != 9 || observation.Current.Size != 4 {
		t.Fatalf("descriptor/path snapshots: %+v", observation)
	}
	data, err := io.ReadAll(f)
	if err != nil || string(data) != "d\nlate\n" {
		t.Fatalf("old descriptor after replacement: %q, %v", data, err)
	}
}

func TestObservePathFollowsRegularSymlinksAndPropagatesStatFailure(t *testing.T) {
	f, path := testRegularFile(t, "first\n")
	_, other := testRegularFile(t, "other\n")
	link := filepath.Join(t.TempDir(), "link.log")
	if _, err := f.Seek(2, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		target string
		status PathStatus
	}{{path, PathSame}, {filepath.Join(t.TempDir(), "absent.log"), PathMissing}, {other, PathReplaced}} {
		if err := os.Symlink(tc.target, link); err != nil {
			t.Fatal(err)
		}
		observation, err := ObservePath(context.Background(), f, link)
		if err != nil || observation.Status != tc.status {
			t.Fatalf("symlink observation: %+v, %v", observation, err)
		}
		assertDescriptorPosition(t, f, 2)
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(link, link); err != nil {
		t.Fatal(err)
	}
	observation, err := ObservePath(context.Background(), f, link)
	if !errors.Is(err, syscall.ELOOP) || observation != (PathObservation{}) {
		t.Fatalf("stat failure lost or misclassified: %+v, %v", observation, err)
	}
	assertDescriptorPosition(t, f, 2)
}

func TestObservePathRejectsFIFOWithoutOpeningIt(t *testing.T) {
	f, path := testRegularFile(t, "regular\n")
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		observation, err := ObservePath(context.Background(), f, path)
		if !errors.Is(err, ErrPathNotRegular) || observation != (PathObservation{}) {
			result <- errors.New("nonregular path accepted")
			return
		}
		result <- nil
	}()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		// Release a mistakenly blocking open so the test does not leave it behind.
		writer, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err == nil {
			defer writer.Close()
		}
		select {
		case <-result:
		case <-time.After(time.Second):
		}
		t.Fatal("observation blocked on FIFO without a writer")
	}
	assertDescriptorPosition(t, f, int64(len("regular\n")))
}
