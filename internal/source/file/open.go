package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
)

// ErrPathChanged signals a replacement or disappearance during the observed
// opening sequence. A caller may retry; no descriptor is returned on failure.
var ErrPathChanged = errors.New("log path changed during open")

// OpenLog opens a regular log in read-only mode at offset zero. It compares
// physical identity before open, on the descriptor and on the path after open.
// Symlinks to regular files are allowed; identity checks follow their targets.
// The caller owns the returned descriptor. All failures close any opened file.
//
// On Linux the open is nonblocking, so a replacement by a FIFO cannot wait for
// a writer. Elsewhere the preflight rejects known nonregular paths, but cannot
// guarantee a nonblocking open during replacement. Context is checked between
// filesystem calls; it does not interrupt a blocking filesystem syscall. These
// observations do not lock the path against later changes or prove a generation.
// Windows path snapshots use a temporary metadata handle to capture file IDs
// immediately; each temporary handle is closed before its path snapshot returns.
func OpenLog(ctx context.Context, path string) (*os.File, Identity, error) {
	if err := ctx.Err(); err != nil {
		return nil, Identity{}, err
	}
	if path == "" {
		return nil, Identity{}, errors.New("log path is required")
	}
	before, err := statPath(path)
	if err != nil {
		return nil, Identity{}, err
	}
	if !before.Mode().IsRegular() {
		return nil, Identity{}, errors.New("log input is not a regular file")
	}
	if err := ctx.Err(); err != nil {
		return nil, Identity{}, err
	}
	f, err := openReadOnly(path)
	if err != nil {
		return nil, Identity{}, err
	}
	return validateOpen(ctx, f, path, before)
}

// validateOpen owns f until every check succeeds. Keeping the post-open stage
// separate permits deterministic replacement tests without timing a race.
func validateOpen(ctx context.Context, f *os.File, path string, before os.FileInfo) (*os.File, Identity, error) {
	keep := false
	defer func() {
		if !keep && f != nil {
			f.Close()
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, Identity{}, err
	}
	id, err := Inspect(f)
	if err != nil {
		return nil, Identity{}, err
	}
	if !id.SameFile(Identity{info: before}) {
		return nil, Identity{}, ErrPathChanged
	}
	current, err := statPath(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = errors.Join(ErrPathChanged, err)
		}
		return nil, Identity{}, err
	}
	if !id.SameFile(Identity{info: current}) {
		return nil, Identity{}, ErrPathChanged
	}
	if err := ctx.Err(); err != nil {
		return nil, Identity{}, err
	}
	keep = true
	return f, id, nil
}
