//go:build linux

package auth

import (
	"os"
	"syscall"
)

func openAccountFile(root *os.Root) (*os.File, error) {
	return root.OpenFile(LocalAccountFilename, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

func syncAccountDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	syncErr := dir.Sync()
	closeErr := dir.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
