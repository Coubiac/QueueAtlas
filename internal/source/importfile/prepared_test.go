package importfile

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func inputFixture(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.log")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertTempEmpty(t *testing.T, parent string) {
	t.Helper()
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("temporary output not cleaned", entries, err)
	}
}

func TestPrepareRegularOwnsValidatedReadOnlyCopy(t *testing.T) {
	for _, encoding := range []string{"plain", "gzip"} {
		for _, content := range []string{"", "same\nsame\n", "line\npartial", strings.Repeat("x", 2*contentBufferBytes) + "\n"} {
			data := []byte(content)
			if encoding == "gzip" {
				data = compressedFixture(t, content, gzip.BestSpeed, "synthetic.log")
			}
			path, tempParent := inputFixture(t, data), t.TempDir()
			prepared, err := PrepareRegular(context.Background(), path, PrepareOptions{Gzip: encoding == "gzip", Limits: unlimitedFixtureLimits(), TempDir: tempParent})
			if err != nil || prepared == nil {
				t.Fatal("prepare", err)
			}
			copyPath, copyDir := prepared.path, prepared.dir
			if filepath.Dir(copyPath) != copyDir || filepath.Dir(copyDir) != tempParent {
				t.Fatal("copy escaped parent", copyPath, copyDir)
			}
			if runtime.GOOS != "windows" {
				for _, privatePath := range []string{copyPath, copyDir} {
					stat, err := os.Stat(privatePath)
					if err != nil || stat.Mode().Perm()&0077 != 0 {
						t.Fatal("copy permissions not private", privatePath, stat, err)
					}
				}
			}
			want, _ := InspectPlain(context.Background(), strings.NewReader(content), int64(max(1, len(content))))
			if prepared.Info() != want {
				t.Fatal("metadata", prepared.Info(), want)
			}
			// The original is no longer consulted and cannot redirect this copy.
			if err := os.WriteFile(path, []byte("changed input\n"), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(prepared)
			if err != nil || string(got) != content {
				t.Fatal("copy followed changed input", string(got), err)
			}
			if _, err := prepared.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			window := make([]byte, min(4, len(content)))
			if n, err := prepared.ReadAt(window, 0); err != nil || n != len(window) || string(window) != content[:len(window)] {
				t.Fatal("owned ReadAt", n, err)
			}
			if _, err := prepared.file.Write([]byte("x")); err == nil {
				t.Fatal("sealed descriptor writable")
			}
			if err := prepared.Close(); err != nil {
				t.Fatal(err)
			}
			if err := prepared.Close(); err != nil {
				t.Fatal("close not idempotent", err)
			}
			if _, err := prepared.Read(make([]byte, 1)); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("closed read", err)
			}
			if _, err := prepared.Seek(0, io.SeekStart); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("closed seek", err)
			}
			if _, err := prepared.ReadAt(make([]byte, 1), 0); !errors.Is(err, fs.ErrClosed) {
				t.Fatal("closed ReadAt", err)
			}
			assertTempEmpty(t, tempParent)
		}
	}
}

func TestPrepareRegularFailureDiscardsPrivateCopy(t *testing.T) {
	for _, kind := range []string{"empty path", "zero limit", "gzip limit", "gzip ratio", "canceled", "missing", "directory", "temp missing", "plain excess", "gzip excess", "gzip CRC", "gzip header"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tempParent := t.TempDir()
			options := PrepareOptions{Limits: unlimitedFixtureLimits(), TempDir: tempParent}
			data := []byte("valid\n")
			var want error
			switch kind {
			case "zero limit":
				options.Limits.ContentBytes = 0
			case "gzip limit":
				options.Gzip = true
				options.Limits.CompressedBytes = 0
			case "gzip ratio":
				options.Gzip = true
				options.Limits.MaxRatio = 0
			case "canceled":
				cancel()
				want = context.Canceled
			case "temp missing":
				options.TempDir = filepath.Join(tempParent, "absent")
				want = fs.ErrNotExist
			case "plain excess":
				options.Limits.ContentBytes = 2
				want = ErrByteLimit
			case "gzip excess":
				options.Gzip = true
				options.Limits.ContentBytes = 2
				data = compressedFixture(t, "valid\n", gzip.BestSpeed, "")
				want = ErrByteLimit
			case "gzip CRC":
				options.Gzip = true
				data = compressedFixture(t, "valid\n", gzip.BestSpeed, "")
				data[len(data)-8] ^= 1
				want = gzip.ErrChecksum
			case "gzip header":
				options.Gzip = true
				data = []byte("invalid gzip header\n")
				want = gzip.ErrHeader
			}
			path := inputFixture(t, data)
			switch kind {
			case "empty path":
				path = ""
			case "missing":
				path = filepath.Join(t.TempDir(), "absent")
				want = fs.ErrNotExist
			case "directory":
				path = t.TempDir()
			}
			prepared, err := PrepareRegular(ctx, path, options)
			if err == nil || prepared != nil || (want != nil && !errors.Is(err, want)) {
				t.Fatal("failure returned owner", prepared, err)
			}
			assertTempEmpty(t, tempParent)
		})
	}
}

func TestPreparedContentCloseReportsErrorsWithoutRemovingForeignFiles(t *testing.T) {
	var absent *PreparedContent
	if absent.Close() != nil || absent.Info() != (ContentInfo{}) {
		t.Fatal("nil owner")
	}
	if (&PreparedContent{}).Close() != nil {
		t.Fatal("zero owner")
	}
	for _, kind := range []string{"already closed", "foreign file"} {
		t.Run(kind, func(t *testing.T) {
			tempParent := t.TempDir()
			prepared, err := PrepareRegular(context.Background(), inputFixture(t, []byte("old\n")), PrepareOptions{Limits: unlimitedFixtureLimits(), TempDir: tempParent})
			if err != nil {
				t.Fatal(err)
			}
			dir := prepared.dir
			if kind == "already closed" {
				if err := prepared.file.Close(); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(filepath.Join(dir, "foreign"), []byte("synthetic sentinel"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := prepared.Close(); err == nil {
				t.Fatal("cleanup failure hidden")
			}
			if prepared.file != nil || prepared.path != "" || prepared.dir != "" || prepared.Close() != nil {
				t.Fatal("owner retained or cleanup retried")
			}
			if kind == "already closed" {
				assertTempEmpty(t, tempParent)
			} else {
				if data, err := os.ReadFile(filepath.Join(dir, "foreign")); err != nil || !bytes.Equal(data, []byte("synthetic sentinel")) {
					t.Fatal("foreign file removed", err)
				}
				if err := os.Remove(filepath.Join(dir, "foreign")); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(dir); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
