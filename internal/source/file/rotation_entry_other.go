//go:build !windows

package file

import (
	"context"
	"os"
)

func rotationEntryInfo(_ context.Context, _ string, entry os.DirEntry) (os.FileInfo, error) {
	return entry.Info()
}
