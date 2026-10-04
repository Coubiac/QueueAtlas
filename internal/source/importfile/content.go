// Package importfile contains building blocks for bounded historical imports.
package importfile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

const contentBufferBytes = 32 * 1024
const maxEmptyReads = 100

var ErrByteLimit = errors.New("import content byte limit exceeded")
var ErrInvalidReader = errors.New("invalid import content reader result")

// ContentInfo describes the whole stream, including all separators and a
// possible unterminated suffix. TrailingPartial must be handled explicitly by
// the importer; this result alone never acknowledges records or a complete run.
type ContentInfo struct {
	Bytes           int64
	SHA256          string // lowercase hex, only populated after a successful EOF
	TrailingPartial bool   // false for empty content or a final LF (including CRLF)
}

// InspectPlain hashes a complete, finite stream with fixed memory. It consumes
// at most maxBytes+1 bytes to distinguish an exact limit from excess input, and
// leaves closing the reader to the caller. Errors/cancellation discard metadata.
// Context is checked between bounded reads; callers provide the import deadline
// and a regular-file input. A blocking Reader cannot be interrupted by this loop.
// No seeking, line parsing, Sink call, checkpoint or manifest update occurs.
func InspectPlain(ctx context.Context, input io.Reader, maxBytes int64) (ContentInfo, error) {
	if err := ctx.Err(); err != nil {
		return ContentInfo{}, err
	}
	if input == nil || maxBytes < 1 {
		return ContentInfo{}, errors.New("import reader and positive byte limit are required")
	}
	hash := sha256.New()
	var buffer [contentBufferBytes]byte
	var info ContentInfo
	emptyReads := 0
	for {
		if err := ctx.Err(); err != nil {
			return ContentInfo{}, err
		}
		readSize := len(buffer)
		remaining := maxBytes - info.Bytes
		if remaining < int64(readSize) {
			readSize = int(remaining) + 1 // at most the small fixed buffer; no int64 overflow
		}
		n, err := input.Read(buffer[:readSize])
		if cause := ctx.Err(); cause != nil {
			return ContentInfo{}, cause
		}
		if n < 0 || n > readSize {
			return ContentInfo{}, ErrInvalidReader
		}
		if int64(n) > remaining {
			return ContentInfo{}, ErrByteLimit
		}
		if n > 0 {
			_, _ = hash.Write(buffer[:n])
			info.Bytes += int64(n)
			info.TrailingPartial = buffer[n-1] != '\n'
			emptyReads = 0
		}
		if err == io.EOF {
			info.SHA256 = hex.EncodeToString(hash.Sum(nil))
			return info, nil
		}
		if err != nil {
			return ContentInfo{}, err
		}
		if n == 0 {
			emptyReads++
			if emptyReads >= maxEmptyReads {
				return ContentInfo{}, io.ErrNoProgress
			}
		}
	}
}
