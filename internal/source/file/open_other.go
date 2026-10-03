//go:build !linux

package file

import "os"

func openReadOnly(path string) (*os.File, error) { return os.Open(path) }
