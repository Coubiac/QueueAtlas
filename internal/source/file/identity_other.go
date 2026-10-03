//go:build !linux

package file

import "os"

// Persistent device/inode identity is currently supported only on Linux.
func physicalIDs(os.FileInfo) (string, string) { return "", "" }
