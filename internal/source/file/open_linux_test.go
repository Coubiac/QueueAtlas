//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestOpenLogAllowsRegularSymlinkAndRejectsRetargeting(t *testing.T) {
	first, firstPath := testRegularFile(t, "first\n")
	link := filepath.Join(t.TempDir(), "mail-link.log")
	if err := os.Symlink(firstPath, link); err != nil {
		t.Fatal(err)
	}
	f, id, err := OpenLog(context.Background(), link)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := Inspect(first)
	if err != nil || !id.SameFile(expected) {
		t.Fatalf("symlink identity: %+v, %v", id, err)
	}
	before, err := os.Stat(link)
	if err != nil {
		t.Fatal(err)
	}
	_, replacement := testRegularFile(t, "replacement\n")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, link); err != nil {
		t.Fatal(err)
	}
	if got, _, err := validateOpen(context.Background(), f, link, before); got != nil || !errors.Is(err, ErrPathChanged) {
		t.Fatalf("retargeted symlink: %v, %v", got, err)
	}
	if _, err := f.Stat(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("retarget failure retained descriptor")
	}
}

func TestOpenLogRejectsRenameAndReplacementAfterOpen(t *testing.T) {
	writable, path := testRegularFile(t, "same\n")
	before, err := writable.Stat()
	if err != nil {
		t.Fatal(err)
	}
	f, err := openReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("same\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, _, err := validateOpen(context.Background(), f, path, before); got != nil || !errors.Is(err, ErrPathChanged) {
		t.Fatalf("replaced path: %v, %v", got, err)
	}
	if _, err := f.Stat(); !errors.Is(err, fs.ErrClosed) {
		t.Fatal("replacement failure retained descriptor")
	}
}

func TestOpenLogRejectsFIFOReplacementWithoutWaitingForWriter(t *testing.T) {
	f, path := testRegularFile(t, "regular\n")
	before, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if got, _, err := OpenLog(context.Background(), path); got != nil || err == nil {
		t.Fatalf("FIFO preflight: %v, %v", got, err)
	}
	result := make(chan error, 1)
	go func() {
		opened, err := openReadOnly(path) // replacement after regular preflight
		if err != nil {
			result <- err
			return
		}
		defer opened.Close()
		got, _, err := validateOpen(context.Background(), opened, path, before)
		if got != nil || err == nil {
			result <- errors.New("FIFO descriptor accepted")
			return
		}
		if _, err := opened.Stat(); !errors.Is(err, fs.ErrClosed) {
			result <- errors.New("FIFO descriptor leaked")
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
		// Release a buggy blocking open so the test does not leave it behind.
		writer, err := os.OpenFile(path, os.O_RDWR|syscall.O_NONBLOCK, 0)
		if err == nil {
			defer writer.Close()
		}
		select {
		case <-result:
		case <-time.After(time.Second):
		}
		t.Fatal("FIFO replacement blocked waiting for a writer")
	}
}
