// Package file contains the building blocks for following local log files.
package file

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
)

// LineReader reads newline-terminated physical records without interpreting
// them. It retains at most MaxLineBytes of a line, plus a fixed read buffer.
// It is intended for one consumer and does not seek or own the input reader.
type LineReader struct {
	input    *bufio.Reader
	start    int64
	offset   int64
	prefix   [model.MaxLineBytes]byte
	kept     int
	tooLong  bool
	fatalErr error
}

// NewLineReader expects input to already be positioned at startOffset, which
// must be the start of a physical line (usually a committed checkpoint).
func NewLineReader(input io.Reader, startOffset int64) (*LineReader, error) {
	if input == nil {
		return nil, errors.New("line reader input is nil")
	}
	if startOffset < 0 {
		return nil, errors.New("line reader start offset is negative")
	}
	return &LineReader{
		input: bufio.NewReaderSize(input, 4096),
		start: startOffset, offset: startOffset,
	}, nil
}

// Next returns a complete record with Raw including its LF or CRLF separator.
// Its offsets include all consumed bytes, even for an oversized line; Raw is
// then a bounded prefix and Record.Error describes the discarded suffix.
//
// At EOF, partial bytes remain pending and no record is returned. Calling Next
// again after the file grows continues the same line. Read errors and context
// cancellation also preserve partial state. Cancellation is checked between
// bounded reads; interrupting a blocking input read is the caller's concern.
// OriginID, ReadAt and Observation must be populated by the ingesting source.
func (r *LineReader) Next(ctx context.Context) (source.Record, error) {
	if r.fatalErr != nil {
		return source.Record{}, r.fatalErr
	}
	for {
		if err := ctx.Err(); err != nil {
			return source.Record{}, err
		}
		fragment, err := r.input.ReadSlice('\n')
		if int64(len(fragment)) > math.MaxInt64-r.offset {
			r.fatalErr = errors.New("line reader offset exceeds int64 range")
			return source.Record{}, r.fatalErr
		}
		r.offset += int64(len(fragment))
		copied := copy(r.prefix[r.kept:], fragment)
		r.kept += copied
		if copied < len(fragment) {
			r.tooLong = true
		}
		if err == nil {
			record := source.Record{
				Start: r.start,
				End:   r.offset,
				Raw:   append([]byte(nil), r.prefix[:r.kept]...),
			}
			if r.tooLong {
				record.Error = fmt.Sprintf("line exceeds %d-byte limit", model.MaxLineBytes)
			}
			r.start = r.offset
			r.kept = 0
			r.tooLong = false
			return record, nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return source.Record{}, err
	}
}
