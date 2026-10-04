package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
)

type PathStatus string

const (
	PathSame     PathStatus = "same"
	PathMissing  PathStatus = "missing"
	PathReplaced PathStatus = "replaced"
)

var ErrPathNotRegular = errors.New("log path is not a regular file")

// PathObservation compares two metadata snapshots. Current is empty for a
// missing path. Size changes do not change physical identity or prove continuity
// of a generation; checkpoint and truncation decisions remain separate.
type PathObservation struct {
	Status  PathStatus
	Opened  Identity
	Current Identity
}

// ObservePath compares the caller's regular descriptor to the current path.
// It follows symlinks and accepts only regular targets. A missing target,
// including a dangling symlink, returns PathMissing without an error. Other
// filesystem errors and nonregular targets return no usable observation.
//
// This performs one descriptor stat and one path snapshot, with no data read
// or seek. On Windows the path snapshot opens and closes a temporary metadata
// handle to capture file IDs immediately; elsewhere it uses os.Stat only.
// The caller's descriptor is never closed, including on failure. The snapshots are
// not atomic and do not lock the path against changes after observation.
// Context is checked between calls; it cannot interrupt a filesystem syscall.
func ObservePath(ctx context.Context, f *os.File, path string) (PathObservation, error) {
	if err := ctx.Err(); err != nil {
		return PathObservation{}, err
	}
	if path == "" {
		return PathObservation{}, errors.New("log path is required")
	}
	opened, err := Inspect(f)
	if err != nil {
		return PathObservation{}, err
	}
	if err := ctx.Err(); err != nil {
		return PathObservation{}, err
	}
	info, err := statPath(path)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return PathObservation{}, ctxErr
	}
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return PathObservation{Status: PathMissing, Opened: opened}, nil
		}
		return PathObservation{}, err
	}
	if !info.Mode().IsRegular() {
		return PathObservation{}, ErrPathNotRegular
	}
	device, inode := physicalIDs(info)
	current := Identity{Device: device, Inode: inode, Size: info.Size(), info: info}
	status := PathReplaced
	if opened.SameFile(current) {
		status = PathSame
	}
	return PathObservation{Status: status, Opened: opened, Current: current}, nil
}
