//go:build linux

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

func TestSelectRotationFindsRenamedGenerationAfterAppend(t *testing.T) {
	f, state := resumeFixture(t)
	position, origin := *state.Checkpoint, state.Origin
	path := state.Origin.Path
	if _, err := f.WriteAt([]byte("late\n"), position.Offset); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := rotateTo(path, ".1", "new generation\n"); err != nil {
		t.Fatal(err)
	}
	result, err := SelectRotation(context.Background(), filepath.Dir(path), state, 2)
	if err != nil || result.Status != SelectionUnique || result.Path != path+".1" || result.Examined != 2 {
		t.Fatal("renamed generation not selected", result, err)
	}
	if *state.Checkpoint != position || state.Origin != origin {
		t.Fatal("search changed caller state")
	}
	for _, name := range []string{path, path + ".1", filepath.Dir(path)} {
		if fileDescriptorCount(t, name) != 0 {
			t.Fatal("search retained a descriptor", name)
		}
	}
}

func TestSelectRotationNegativeDecisionsAndExcludedEntries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*os.File, *source.OriginState) error
		limit  int
		want   SelectionStatus
	}{
		{"absent", func(f *os.File, s *source.OriginState) error { return os.Remove(f.Name()) }, 10, SelectionAbsent},
		{"different_prefix", func(f *os.File, s *source.OriginState) error { _, err := f.WriteAt([]byte("changed"), 0); return err }, 10, SelectionDifferent},
		{"zero_checkpoint", func(f *os.File, s *source.OriginState) error {
			a, err := CaptureAnchor(f, 0)
			s.Checkpoint.Offset, s.Checkpoint.AnchorHash = 0, a.String()
			return err
		}, 10, SelectionInsufficient},
		{"hard_links", func(f *os.File, s *source.OriginState) error { return os.Link(f.Name(), f.Name()+".1") }, 10, SelectionAmbiguous},
		{"entry_limit", func(f *os.File, s *source.OriginState) error {
			return os.WriteFile(f.Name()+".1", []byte("other\n"), 0600)
		}, 1, SelectionLimit},
		{"gzip_name", func(f *os.File, s *source.OriginState) error { return os.Rename(f.Name(), f.Name()+".GZ") }, 10, SelectionAbsent},
		{"gzip_signature", func(f *os.File, s *source.OriginState) error { _, err := f.WriteAt([]byte{0x1f, 0x8b}, 0); return err }, 10, SelectionAbsent},
		{"subdirectory", func(f *os.File, s *source.OriginState) error {
			sub := filepath.Join(filepath.Dir(f.Name()), "rotated")
			if err := os.Mkdir(sub, 0700); err != nil {
				return err
			}
			return os.Rename(f.Name(), filepath.Join(sub, "mail.log"))
		}, 10, SelectionAbsent},
		{"symlink", func(f *os.File, s *source.OriginState) error {
			outside := filepath.Join(t.TempDir(), "outside.log")
			if err := os.Rename(f.Name(), outside); err != nil {
				return err
			}
			return os.Symlink(outside, f.Name())
		}, 10, SelectionAbsent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, state := resumeFixture(t)
			if err := tc.change(f, &state); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			position, origin := *state.Checkpoint, state.Origin
			directory := filepath.Dir(f.Name())
			result, err := SelectRotation(context.Background(), directory, state, tc.limit)
			if err != nil || result.Status != tc.want || result.Path != "" || *state.Checkpoint != position || state.Origin != origin {
				t.Fatalf("negative search: %+v, %v; want %s", result, err, tc.want)
			}
			if fileDescriptorCount(t, directory) != 0 {
				t.Fatal("directory descriptor retained")
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if fileDescriptorCount(t, filepath.Join(directory, entry.Name())) != 0 {
					t.Fatal("candidate descriptor retained", entry.Name())
				}
			}
		})
	}
}

func TestRotationEntryReplacementAndDisappearanceDoNotReturnEvidence(t *testing.T) {
	for _, missing := range []bool{false, true} {
		f, state := resumeFixture(t)
		info, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		entry := fs.FileInfoToDirEntry(info)
		path := f.Name()
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(path, path+".1"); err != nil {
			t.Fatal(err)
		}
		wantErr := error(ErrPathChanged)
		if missing {
			wantErr = fs.ErrNotExist
		} else if err := os.WriteFile(path, []byte("replacement\n"), 0600); err != nil {
			t.Fatal(err)
		}
		check, eligible, err := checkRotationEntry(context.Background(), path, entry, state)
		if !errors.Is(err, wantErr) || eligible || check != (ResumeCheck{}) || fileDescriptorCount(t, path+".1") != 0 {
			t.Fatal("changed entry yielded evidence or retained file", check, eligible, err)
		}
		if !missing && fileDescriptorCount(t, path) != 0 {
			t.Fatal("replacement descriptor retained on error")
		}
	}
}
