package importfile

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
)

var ErrCompressedByteLimit = errors.New("import compressed byte limit exceeded")
var ErrRatioLimit = errors.New("import compression ratio limit exceeded")

type GzipLimits struct {
	ContentBytes    int64
	CompressedBytes int64
	MaxRatio        int64 // positive integer, expanded bytes / consumed compressed bytes
}

// InspectGzip validates all concatenated members through their CRC/size trailer
// and final EOF. Fingerprinting covers decompressed content only. Compressed
// bytes (including headers) and output are bounded independently. Ratio checks
// use bytes consumed from input, including gzip's bounded read-ahead; no product
// of byte counts is formed. A valid later member cannot excuse an earlier excess.
// Input remains caller-owned. Any failure, including cleanup, discards metadata.
func InspectGzip(ctx context.Context, input io.Reader, limits GzipLimits) (info ContentInfo, err error) {
	return inspectGzip(ctx, input, limits, nil)
}

// CopyGzip writes decompressed bytes during inspection. All members must still
// validate at EOF before metadata is returned. A checksum failure may leave a
// whole tentative payload in output: callers must discard output on any error.
// Input/output remain caller-owned, and no failed write is retried.
func CopyGzip(ctx context.Context, input io.Reader, output io.Writer, limits GzipLimits) (ContentInfo, error) {
	if err := ctx.Err(); err != nil {
		return ContentInfo{}, err
	}
	if output == nil {
		return ContentInfo{}, errors.New("gzip copy output is required")
	}
	return inspectGzip(ctx, input, limits, output)
}

func inspectGzip(ctx context.Context, input io.Reader, limits GzipLimits, output io.Writer) (info ContentInfo, err error) {
	if err := ctx.Err(); err != nil {
		return ContentInfo{}, err
	}
	if input == nil || limits.ContentBytes < 1 || limits.CompressedBytes < 1 || limits.MaxRatio < 1 {
		return ContentInfo{}, errors.New("gzip reader and positive content, compressed and ratio limits are required")
	}
	compressed := &compressedReader{ctx: ctx, input: input, limit: limits.CompressedBytes}
	decoder, err := gzip.NewReader(compressed)
	if err != nil {
		return ContentInfo{}, err
	}
	defer func() {
		err = errors.Join(err, decoder.Close(), ctx.Err())
		if err != nil {
			info = ContentInfo{}
		}
	}()
	// Keep the default multistream=true: success verifies every member, not
	// merely the first compressed payload followed by unverified trailing bytes.
	expanded := &ratioReader{input: decoder, compressed: compressed, limit: limits.ContentBytes, maxRatio: limits.MaxRatio}
	return inspectContent(ctx, expanded, limits.ContentBytes, output)
}

type compressedReader struct {
	ctx        context.Context
	input      io.Reader
	limit      int64
	bytes      int64
	emptyReads int
}

func (r *compressedReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}
	size := min(len(p), contentBufferBytes)
	remaining := r.limit - r.bytes
	if remaining < int64(size) {
		size = int(remaining) + 1
	}
	n, err := r.input.Read(p[:size])
	if cause := r.ctx.Err(); cause != nil {
		return 0, cause
	}
	if n < 0 || n > size {
		return 0, ErrInvalidReader
	}
	if int64(n) > remaining {
		return 0, ErrCompressedByteLimit
	}
	if n == 0 && err == nil {
		r.emptyReads++
		if r.emptyReads >= maxEmptyReads {
			return 0, io.ErrNoProgress
		}
	} else if n > 0 {
		r.emptyReads = 0
	}
	r.bytes += int64(n)
	return n, err
}

type ratioReader struct {
	input      io.Reader
	compressed *compressedReader
	limit      int64
	maxRatio   int64
	bytes      int64
}

func (r *ratioReader) Read(p []byte) (int, error) {
	n, err := r.input.Read(p)
	if n < 0 || n > len(p) {
		return 0, ErrInvalidReader
	}
	if int64(n) > r.limit-r.bytes {
		return n, ErrByteLimit
	}
	r.bytes += int64(n)
	if r.bytes > 0 {
		count := r.compressed.bytes
		if count == 0 || r.bytes/count > r.maxRatio || (r.bytes/count == r.maxRatio && r.bytes%count != 0) {
			return n, ErrRatioLimit
		}
	}
	return n, err
}
