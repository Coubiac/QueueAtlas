//go:build windows

package file

import (
	"errors"
	"os"
	"strings"
	"syscall"
)

// os.Stat can defer loading Windows file IDs until SameFile, reopening the path
// then. A metadata handle captures attributes and identity together, so the
// returned FileInfo cannot later identify a replacement at that path.
func statPath(path string) (os.FileInfo, error) {
	name, err := metadataPath(path)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	encoded, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	handle, err := syscall.CreateFile(encoded, 0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(handle), path)
	if f == nil {
		err := errors.New("metadata handle could not be wrapped")
		return nil, errors.Join(err, syscall.CloseHandle(handle))
	}
	info, statErr := f.Stat()
	if err := errors.Join(statErr, f.Close()); err != nil {
		return nil, err
	}
	return info, nil
}

// Keep short Win32 paths in their normal namespace. Extend only long absolute
// paths, including UNC, while preserving explicitly supplied device namespaces.
func metadataPath(path string) (string, error) {
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\\.\`) || strings.HasPrefix(path, `\??\`) {
		return path, nil
	}
	absolute, err := syscall.FullPath(path)
	if err != nil {
		return "", err
	}
	if len(absolute) < 248 {
		return path, nil
	}
	if strings.HasPrefix(absolute, `\\`) {
		return `\\?\UNC\` + absolute[2:], nil
	}
	return `\\?\` + absolute, nil
}
