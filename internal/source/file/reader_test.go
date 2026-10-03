package file

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
)

func TestCompleteLinesKeepSeparatorsAndOffsets(t *testing.T) {
	r, err := NewLineReader(strings.NewReader("one\ntwo\r\n\n"), 37)
	if err != nil {
		t.Fatal(err)
	}
	offset := int64(37)
	var first []byte
	for i, want := range []string{"one\n", "two\r\n", "\n"} {
		record, err := r.Next(context.Background())
		if err != nil || string(record.Raw) != want || record.Error != "" || record.Start != offset || record.End != offset+int64(len(want)) {
			t.Fatalf("record %d = %+v, %v", i, record, err)
		}
		if i == 0 {
			first = record.Raw
		}
		offset = record.End
	}
	if string(first) != "one\n" {
		t.Fatal("a later read mutated an earlier record")
	}
	record, err := r.Next(context.Background())
	if !errors.Is(err, io.EOF) || len(record.Raw) != 0 {
		t.Fatalf("after last line: %+v, %v", record, err)
	}
}

func openGrowingFile(t *testing.T, initial string) (*LineReader, *os.File) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mail.log")
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { input.Close() })
	writer, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close() })
	r, err := NewLineReader(input, 0)
	if err != nil {
		t.Fatal(err)
	}
	return r, writer
}

func assertPending(t *testing.T, r *LineReader) {
	t.Helper()
	record, err := r.Next(context.Background())
	if !errors.Is(err, io.EOF) || len(record.Raw) != 0 || record.End != 0 {
		t.Fatalf("partial line produced a record: %+v, %v", record, err)
	}
}

func TestPartialLineContinuesAfterAppend(t *testing.T) {
	r, writer := openGrowingFile(t, "prefix")
	assertPending(t, r)
	assertPending(t, r)
	if _, err := writer.WriteString(" suffix\r"); err != nil {
		t.Fatal(err)
	}
	assertPending(t, r)
	if _, err := writer.WriteString("\nnext\n"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		raw        string
		start, end int64
	}{
		{"prefix suffix\r\n", 0, 15},
		{"next\n", 15, 20},
	} {
		record, err := r.Next(context.Background())
		if err != nil || string(record.Raw) != want.raw || record.Start != want.start || record.End != want.end {
			t.Fatalf("after append: %+v, %v; want %+v", record, err, want)
		}
	}
}

func TestLineSizeBoundaryIncludesSeparator(t *testing.T) {
	for _, size := range []int{model.MaxLineBytes - 1, model.MaxLineBytes, model.MaxLineBytes + 1} {
		line := strings.Repeat("x", size-1) + "\n"
		r, err := NewLineReader(strings.NewReader(line+"ok\n"), 0)
		if err != nil {
			t.Fatal(err)
		}
		record, err := r.Next(context.Background())
		if err != nil || record.End != int64(size) || len(record.Raw) != min(size, model.MaxLineBytes) || (record.Error != "") != (size > model.MaxLineBytes) {
			t.Fatalf("size %d: retained=%d, end=%d, record error=%q, read error=%v", size, len(record.Raw), record.End, record.Error, err)
		}
		next, err := r.Next(context.Background())
		if err != nil || string(next.Raw) != "ok\n" || next.Start != int64(size) || next.End != int64(size+3) || next.Error != "" {
			t.Fatalf("line after size %d: %+v, %v", size, next, err)
		}
	}
}

func TestOversizedPartialLineWaitsForSeparator(t *testing.T) {
	size := 16*model.MaxLineBytes + 17
	r, writer := openGrowingFile(t, strings.Repeat("x", size))
	assertPending(t, r)
	assertPending(t, r)
	if _, err := writer.WriteString("\nok\n"); err != nil {
		t.Fatal(err)
	}
	record, err := r.Next(context.Background())
	if err != nil || record.Start != 0 || record.End != int64(size+1) || len(record.Raw) != model.MaxLineBytes || record.Error == "" || string(record.Raw) != strings.Repeat("x", model.MaxLineBytes) {
		t.Fatalf("oversized line: retained=%d, start=%d, end=%d, record error=%q, read error=%v", len(record.Raw), record.Start, record.End, record.Error, err)
	}
	next, err := r.Next(context.Background())
	if err != nil || string(next.Raw) != "ok\n" || next.Start != int64(size+1) || next.End != int64(size+4) || next.Error != "" {
		t.Fatalf("line after discarded suffix: %+v, %v", next, err)
	}
}

func TestCancellationPreservesPendingLine(t *testing.T) {
	r, writer := openGrowingFile(t, "partial")
	assertPending(t, r)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read: %v", err)
	}
	if _, err := writer.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
	record, err := r.Next(context.Background())
	if err != nil || string(record.Raw) != "partial\n" || record.Start != 0 || record.End != 8 {
		t.Fatalf("resume after cancellation: %+v, %v", record, err)
	}
}

type interruptedReader struct {
	tail        *strings.Reader
	interrupted bool
	failure     error
}

func (r *interruptedReader) Read(p []byte) (int, error) {
	if !r.interrupted {
		r.interrupted = true
		return copy(p, "part"), r.failure
	}
	return r.tail.Read(p)
}

func TestReadFailurePreservesConsumedBytes(t *testing.T) {
	failure := errors.New("temporary read failure")
	r, err := NewLineReader(&interruptedReader{tail: strings.NewReader("ial\n"), failure: failure}, 0)
	if err != nil {
		t.Fatal(err)
	}
	record, err := r.Next(context.Background())
	if !errors.Is(err, failure) || len(record.Raw) != 0 {
		t.Fatalf("read failure produced a record: %+v, %v", record, err)
	}
	record, err = r.Next(context.Background())
	if err != nil || string(record.Raw) != "partial\n" || record.Start != 0 || record.End != 8 {
		t.Fatalf("retry lost bytes: %+v, %v", record, err)
	}
}

func TestInvalidStartAndOffsetOverflow(t *testing.T) {
	if _, err := NewLineReader(strings.NewReader("x\n"), -1); err == nil {
		t.Fatal("negative offset accepted")
	}
	r, err := NewLineReader(strings.NewReader("x\n"), math.MaxInt64-1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(context.Background()); err == nil {
		t.Fatal("offset overflow accepted")
	}
}
