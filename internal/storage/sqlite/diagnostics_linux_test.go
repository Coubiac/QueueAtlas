//go:build linux

package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOpenDiagnosticsRejectsFIFOAndExposedFiles(t *testing.T) {
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		t.Run("fifo"+suffix, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "queueatlas.db")
			if suffix != "" {
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := syscall.Mkfifo(path+suffix, 0600); err != nil {
				t.Fatal(err)
			}
			if d, err := OpenDiagnostics(context.Background(), path); d != nil || err != ErrDiagnosticsOpen {
				t.Fatalf("FIFO accepted: %v", err)
			}
		})
		t.Run("permissions"+suffix, func(t *testing.T) {
			s, path := openTestStore(t)
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			if suffix != "" {
				if err := os.WriteFile(path+suffix, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(path+suffix, 0640); err != nil {
				t.Fatal(err)
			}
			if d, err := OpenDiagnostics(context.Background(), path); d != nil || err != ErrDiagnosticsOpen {
				t.Fatalf("exposed file accepted: %v", err)
			}
		})
	}
}
