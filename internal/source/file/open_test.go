package file

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogReturnsReadOnlyRegularDescriptorAtZero(t *testing.T) {
	writer, path := testRegularFile(t, "synthetic\n")
	expected, err := Inspect(writer)
	if err != nil {
		t.Fatal(err)
	}
	f, id, err := OpenLog(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if !id.SameFile(expected) {
		t.Fatal("descriptor identifies another file")
	}
	data, err := io.ReadAll(f)
	if err != nil || string(data) != "synthetic\n" {
		t.Fatalf("opened data %q, %v", data, err)
	}
	if _, err := f.WriteAt([]byte("X"), 0); err == nil {
		t.Fatal("descriptor is writable")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if data, err := io.ReadAll(f); err != nil || string(data) != "synthetic\n" {
		t.Fatalf("failed write changed content %q, %v", data, err)
	}
}

func TestOpenLogRejectsInvalidPathAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		ctx  context.Context
		path string
		want error
	}{
		{ctx, "unused", context.Canceled},
		{context.Background(), "", nil},
		{context.Background(), t.TempDir(), nil},
		{context.Background(), filepath.Join(t.TempDir(), "absent.log"), fs.ErrNotExist},
	} {
		f, id, err := OpenLog(tc.ctx, tc.path)
		if err == nil || f != nil || id.info != nil || tc.want != nil && !errors.Is(err, tc.want) {
			t.Fatalf("open %q: %v, %+v, %v", tc.path, f, id, err)
		}
	}
}

func TestValidateOpenClosesOnChangedIdentityAndCancellation(t *testing.T) {
	first, firstPath := testRegularFile(t, "first\n")
	before, err := first.Stat()
	if err != nil {
		t.Fatal(err)
	}
	otherPath := filepath.Join(t.TempDir(), "replacement.log")
	if err := os.WriteFile(otherPath, []byte("replacement\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"descriptor changed", "path changed", "path disappeared", "cancelled", "directory"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			openedPath, currentPath := firstPath, firstPath
			want := ErrPathChanged
			switch kind {
			case "descriptor changed":
				openedPath, currentPath = otherPath, otherPath
			case "path changed":
				currentPath = otherPath
			case "path disappeared":
				currentPath = filepath.Join(t.TempDir(), "absent.log")
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				want = context.Canceled
			case "directory":
				openedPath = t.TempDir()
				want = nil
			}
			opened, err := os.Open(openedPath)
			if err != nil {
				t.Fatal(err)
			}
			defer opened.Close()
			f, id, err := validateOpen(ctx, opened, currentPath, before)
			if err == nil || f != nil || id.info != nil || want != nil && !errors.Is(err, want) {
				t.Fatalf("validation: %v, %+v, %v", f, id, err)
			}
			if err := opened.Close(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("failed validation retained descriptor: %v", err)
			}
			if kind == "path disappeared" && !errors.Is(err, fs.ErrNotExist) {
				t.Fatal("missing path error was lost")
			}
		})
	}
}
