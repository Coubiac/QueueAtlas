//go:build linux

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOpenRejectsNonregularDatabaseAndSidecars(t *testing.T) {
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		t.Run(suffix, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "queueatlas.db")
			if suffix != "" {
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := syscall.Mkfifo(path+suffix, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := Open(context.Background(), path)
			if err == nil {
				s.Close()
				t.Fatal("FIFO accepted as database or sidecar")
			}
			info, err := os.Stat(path + suffix)
			if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
				t.Fatal("refusal replaced FIFO", err)
			}
		})
	}
}

func TestNewDatabaseAndLiveWALArePrivate(t *testing.T) {
	s, path := openTestStore(t)
	if err := s.Commit(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		info, err := os.Stat(path + suffix)
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatal("database or sidecar exposed", suffix, info, err)
		}
	}
}
