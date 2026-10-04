//go:build windows

package file

import (
	"context"
	"errors"
	"os"
)

func rotationEntryInfo(ctx context.Context, path string, entry os.DirEntry) (os.FileInfo, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return info, nil
	}
	// NTFS enumeration already supplies IDs; some Windows enumeration fallbacks
	// defer them until SameFile. Load them before opening data, rather than letting
	// the final comparison retroactively attribute this entry to a replacement.
	// SameFile hides native errors, so failure is a safe refusal, not proof of change.
	if !os.SameFile(info, info) {
		return nil, errors.New("rotation entry physical identity unavailable")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot, err := statPath(path)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if err != nil {
		return nil, err
	}
	if !snapshot.Mode().IsRegular() {
		return snapshot, nil
	}
	if !os.SameFile(info, snapshot) {
		return nil, ErrPathChanged
	}
	return snapshot, nil
}
