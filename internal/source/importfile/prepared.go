package importfile

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Coubiac/mailtrace/internal/source/file"
)

type PrepareOptions struct {
	Gzip    bool
	Limits  GzipLimits // plain uses ContentBytes; gzip requires all three positive
	TempDir string     // empty uses os.TempDir; parent must be trusted/protected
}

var ErrPreparedSize = errors.New("prepared import content size changed")

// PreparedContent owns a read-only private copy of the inspected bytes. One
// caller may read/seek it; it must not be copied or used concurrently with Close.
// Close releases the descriptor and removes only its own file/empty directory.
// The copy is ephemeral, not a durable recovery manifest or an atomic snapshot
// of the original input. Unix permissions protect it; Windows ACLs are not checked.
type PreparedContent struct {
	file *os.File
	path string
	dir  string
	info ContentInfo
}

func (p *PreparedContent) Info() ContentInfo {
	if p == nil {
		return ContentInfo{}
	}
	return p.info
}

func (p *PreparedContent) Read(b []byte) (int, error) {
	if p == nil || p.file == nil {
		return 0, fs.ErrClosed
	}
	return p.file.Read(b)
}

func (p *PreparedContent) ReadAt(b []byte, offset int64) (int, error) {
	if p == nil || p.file == nil {
		return 0, fs.ErrClosed
	}
	return p.file.ReadAt(b, offset)
}

func (p *PreparedContent) Seek(offset int64, whence int) (int64, error) {
	if p == nil || p.file == nil {
		return 0, fs.ErrClosed
	}
	return p.file.Seek(offset, whence)
}

// Close is idempotent even on errors: ownership is released before cleanup.
// Failed removals are reported, not retried or hidden as successful cleanup.
func (p *PreparedContent) Close() error {
	if p == nil {
		return nil
	}
	f, path, dir := p.file, p.path, p.dir
	p.file, p.path, p.dir = nil, "", ""
	var err error
	if f != nil {
		err = f.Close()
	}
	for _, ownedPath := range []string{path, dir} {
		if ownedPath == "" {
			continue
		}
		cause := os.Remove(ownedPath) // no recursive deletion, never the caller's TempDir
		if !errors.Is(cause, fs.ErrNotExist) {
			err = errors.Join(err, cause)
		}
	}
	return err
}

// PrepareRegular opens a regular input, copies/decompresses it into a private
// temporary file during inspection, closes the writer, then returns a read-only
// owner at offset zero. Later input changes do not alter the captured bytes.
// Any failure (including input close/cancellation) closes/removes the output and
// returns nil. Cleanup errors are joined. Partial lines remain explicit metadata,
// not evidence that an import is complete. No parser, Sink or manifest is used.
func PrepareRegular(ctx context.Context, path string, options PrepareOptions) (prepared *PreparedContent, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || options.Limits.ContentBytes < 1 || (options.Gzip && (options.Limits.CompressedBytes < 1 || options.Limits.MaxRatio < 1)) {
		return nil, errors.New("import path and positive content/encoding limits are required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	tempParent := options.TempDir
	if tempParent == "" {
		tempParent = os.TempDir()
	}
	tempParent, err = filepath.Abs(tempParent)
	if err != nil {
		return nil, err
	}
	input, _, err := file.OpenLog(ctx, absPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, input.Close(), ctx.Err())
		if err != nil && prepared != nil {
			err = errors.Join(err, prepared.Close())
			prepared = nil
		}
	}()
	dir, err := os.MkdirTemp(tempParent, "queueatlas-import-") // Unix 0700
	if err != nil {
		return nil, err
	}
	owned := &PreparedContent{dir: dir}
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, owned.Close())
		}
	}()
	writer, err := os.CreateTemp(dir, "content-") // Unix 0600
	if err != nil {
		return nil, err
	}
	owned.file, owned.path = writer, writer.Name()
	var info ContentInfo
	if options.Gzip {
		info, err = CopyGzip(ctx, input, writer, options.Limits)
	} else {
		info, err = CopyPlain(ctx, input, writer, options.Limits.ContentBytes)
	}
	if err != nil {
		return nil, err
	}
	owned.file = nil // never retry a failed writer close
	if err := writer.Close(); err != nil {
		return nil, err
	}
	reader, id, err := file.OpenLog(ctx, owned.path)
	if err != nil {
		return nil, err
	}
	owned.file = reader
	if id.Size != info.Bytes {
		return nil, ErrPreparedSize
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	owned.info = info
	keep = true
	return owned, nil
}
