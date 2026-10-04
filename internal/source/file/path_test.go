package file

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func assertDescriptorPosition(t *testing.T, f *os.File, want int64) {
	t.Helper()
	position, err := f.Seek(0, io.SeekCurrent)
	if err != nil || position != want {
		t.Fatalf("descriptor position: %d, %v; want %d", position, err, want)
	}
}

func TestObservePathSameIdentityAfterSizeChanges(t *testing.T) {
	f, path := testRegularFile(t, "seed\n")
	if _, err := f.Seek(2, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	for _, size := range []int64{5, 12, 1} {
		if err := f.Truncate(size); err != nil {
			t.Fatal(err)
		}
		observation, err := ObservePath(context.Background(), f, path)
		if err != nil || observation.Status != PathSame || !observation.Opened.SameFile(observation.Current) || observation.Opened.Size != size || observation.Current.Size != size {
			t.Fatalf("size %d observation: %+v, %v", size, observation, err)
		}
		assertDescriptorPosition(t, f, 2)
	}
}

func TestObservePathMissingReplacementAndErrorsKeepDescriptor(t *testing.T) {
	f, path := testRegularFile(t, "seed\n")
	other, otherPath := testRegularFile(t, "seed\n")
	otherID, err := Inspect(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(2, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		path   string
		status PathStatus
		want   error
	}{
		{"same", context.Background(), path, PathSame, nil},
		{"missing", context.Background(), filepath.Join(t.TempDir(), "absent.log"), PathMissing, nil},
		{"replaced", context.Background(), otherPath, PathReplaced, nil},
		{"directory", context.Background(), t.TempDir(), "", ErrPathNotRegular},
		{"cancelled", cancelled, path, "", context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observation, err := ObservePath(tc.ctx, f, tc.path)
			if tc.want != nil {
				if !errors.Is(err, tc.want) || observation != (PathObservation{}) {
					t.Fatalf("failed observation: %+v, %v", observation, err)
				}
			} else {
				if err != nil || observation.Status != tc.status || observation.Opened.info == nil {
					t.Fatalf("observation: %+v, %v", observation, err)
				}
				if tc.status == PathMissing && observation.Current != (Identity{}) {
					t.Fatal("missing path returned a current identity")
				}
				if tc.status == PathReplaced && (!observation.Current.SameFile(otherID) || observation.Opened.SameFile(observation.Current)) {
					t.Fatal("replacement returned incorrect physical identities")
				}
			}
			assertDescriptorPosition(t, f, 2)
		})
	}
	data, err := io.ReadAll(f)
	if err != nil || string(data) != "ed\n" {
		t.Fatalf("retained descriptor read: %q, %v", data, err)
	}
}

func TestObservePathRejectsUnusableInputWithoutObservation(t *testing.T) {
	f, path := testRegularFile(t, "seed\n")
	closed, _ := testRegularFile(t, "closed\n")
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	directory, err := os.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()
	for _, tc := range []struct {
		f    *os.File
		path string
	}{{nil, path}, {closed, path}, {directory, path}, {f, ""}} {
		observation, err := ObservePath(context.Background(), tc.f, tc.path)
		if err == nil || observation != (PathObservation{}) {
			t.Fatalf("unusable input returned observation: %+v, %v", observation, err)
		}
	}
	if _, err := f.Stat(); err != nil {
		t.Fatal("empty path closed caller descriptor", err)
	}
	if _, err := directory.Stat(); err != nil {
		t.Fatal("nonregular descriptor was closed", err)
	}
}
