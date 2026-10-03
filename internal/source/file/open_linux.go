//go:build linux

package file

import (
	"os"
	"syscall"
)

func openReadOnly(path string) (*os.File, error) {
	// OpenFile also sets close-on-exec. O_NONBLOCK is ignored for regular files.
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
