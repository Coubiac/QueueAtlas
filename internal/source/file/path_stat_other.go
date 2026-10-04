//go:build !windows

package file

import "os"

func statPath(path string) (os.FileInfo, error) { return os.Stat(path) }
