package importfile

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

func compressedFixture(t *testing.T, content string, level int, name string) []byte {
	t.Helper()
	var output bytes.Buffer
	w, err := gzip.NewWriterLevel(&output, level)
	if err != nil {
		t.Fatal(err)
	}
	w.Name, w.ModTime = name, time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func unlimitedFixtureLimits() GzipLimits {
	return GzipLimits{ContentBytes: math.MaxInt64, CompressedBytes: math.MaxInt64, MaxRatio: math.MaxInt64}
}

func TestInspectGzipContentIdentityAndMultipleMembers(t *testing.T) {
	for _, content := range []string{"", "one\nsame\nsame\n", "line\r\npartial", strings.Repeat("a", 3*contentBufferBytes) + "\n"} {
		want, err := InspectPlain(context.Background(), strings.NewReader(content), int64(max(1, len(content))))
		if err != nil {
			t.Fatal(err)
		}
		for _, level := range []int{gzip.NoCompression, gzip.BestCompression} {
			encoded := compressedFixture(t, content, level, "synthetic.log")
			got, err := InspectGzip(context.Background(), bytes.NewReader(encoded), unlimitedFixtureLimits())
			if err != nil || got != want {
				t.Fatal("gzip identity", got, want, err)
			}
		}
	}
	first := compressedFixture(t, "first\n", gzip.BestSpeed, "first.log")
	second := compressedFixture(t, "next\n", gzip.BestCompression, "renamed.log")
	got, err := InspectGzip(context.Background(), bytes.NewReader(append(first, second...)), unlimitedFixtureLimits())
	want, _ := InspectPlain(context.Background(), strings.NewReader("first\nnext\n"), 11)
	if err != nil || got != want {
		t.Fatal("members not all inspected", got, want, err)
	}
}

func TestInspectGzipRefusesCorruptionTruncationAndTrailingBytes(t *testing.T) {
	first := compressedFixture(t, "first\n", gzip.BestSpeed, "")
	for _, kind := range []string{"empty input", "header", "CRC", "size", "truncated", "bad second member", "trailing junk", "joined EOF failure"} {
		t.Run(kind, func(t *testing.T) {
			encoded := bytes.Clone(first)
			var want error
			switch kind {
			case "empty input":
				encoded = nil
				want = io.EOF
			case "header":
				encoded[0] = 0
				want = gzip.ErrHeader
			case "CRC":
				encoded[len(encoded)-8] ^= 1
				want = gzip.ErrChecksum
			case "size":
				encoded[len(encoded)-1] ^= 1
				want = gzip.ErrChecksum
			case "truncated":
				encoded = encoded[:len(encoded)-1]
				want = io.ErrUnexpectedEOF
			case "bad second member":
				second := compressedFixture(t, "second\n", gzip.BestCompression, "")
				second[len(second)-8] ^= 1
				encoded = append(encoded, second...)
				want = gzip.ErrChecksum
			case "trailing junk":
				encoded = append(encoded, 'x')
			}
			var input io.Reader = bytes.NewReader(encoded)
			if kind == "joined EOF failure" {
				boom := errors.New("synthetic EOF error")
				want = boom
				input = readerFunc(func(p []byte) (int, error) { return copy(p, encoded), errors.Join(io.EOF, boom) })
			}
			info, err := InspectGzip(context.Background(), input, unlimitedFixtureLimits())
			if err == nil || (want != nil && !errors.Is(err, want)) || info != (ContentInfo{}) {
				t.Fatal("corruption returned fingerprint", info, err)
			}
		})
	}
}

func TestInspectGzipBudgetsAndRatio(t *testing.T) {
	content := strings.Repeat("x", 8*contentBufferBytes) + "\n"
	encoded := compressedFixture(t, content, gzip.BestCompression, "")
	for _, kind := range []string{"exact", "expanded limit", "compressed limit", "ratio"} {
		t.Run(kind, func(t *testing.T) {
			limits := GzipLimits{ContentBytes: int64(len(content)), CompressedBytes: int64(len(encoded)), MaxRatio: int64((len(content) + len(encoded) - 1) / len(encoded))}
			var want error
			switch kind {
			case "expanded limit":
				limits.ContentBytes--
				want = ErrByteLimit
			case "compressed limit":
				limits.CompressedBytes--
				want = ErrCompressedByteLimit
			case "ratio":
				limits.MaxRatio = 2
				want = ErrRatioLimit
			}
			consumed, maxRead := 0, 0
			reader := bytes.NewReader(encoded)
			info, err := InspectGzip(context.Background(), readerFunc(func(p []byte) (int, error) {
				maxRead = max(maxRead, len(p))
				n, err := reader.Read(p)
				consumed += n
				return n, err
			}), limits)
			if int64(consumed) > limits.CompressedBytes+1 || maxRead > contentBufferBytes {
				t.Fatal("compressed reads unbounded", consumed, maxRead)
			}
			if want != nil {
				if !errors.Is(err, want) || info != (ContentInfo{}) {
					t.Fatal("budget accepted", info, err, want)
				}
			} else if err != nil || info.Bytes != int64(len(content)) {
				t.Fatal("exact budget rejected", info, err)
			}
		})
	}
	// Metadata-only headers need a compressed budget even before any output.
	limits := unlimitedFixtureLimits()
	limits.CompressedBytes = 4
	if info, err := InspectGzip(context.Background(), bytes.NewReader(encoded), limits); !errors.Is(err, ErrCompressedByteLimit) || info != (ContentInfo{}) {
		t.Fatal("header ignored compressed bound", info, err)
	}
}

func TestInspectGzipInputValidationAndCancellation(t *testing.T) {
	for _, kind := range []string{"nil", "content", "compressed", "ratio", "canceled", "cancel on read", "invalid reader", "no progress"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			limits, calls := unlimitedFixtureLimits(), 0
			var want error
			var input io.Reader = readerFunc(func(p []byte) (int, error) {
				calls++
				switch kind {
				case "cancel on read":
					cancel()
					return 0, context.Canceled
				case "invalid reader":
					return len(p) + 1, nil
				case "no progress":
					return 0, nil
				default:
					t.Fatal("invalid gzip inspection read")
					return 0, nil
				}
			})
			switch kind {
			case "nil":
				input = nil
			case "content":
				limits.ContentBytes = 0
			case "compressed":
				limits.CompressedBytes = -1
			case "ratio":
				limits.MaxRatio = 0
			case "canceled":
				cancel()
				want = context.Canceled
			case "cancel on read":
				want = context.Canceled
			case "invalid reader":
				want = ErrInvalidReader
			case "no progress":
				want = io.ErrNoProgress
			}
			info, err := InspectGzip(ctx, input, limits)
			if err == nil || (want != nil && !errors.Is(err, want)) || info != (ContentInfo{}) {
				t.Fatal("invalid input metadata", info, err)
			}
			if (kind == "nil" || kind == "content" || kind == "compressed" || kind == "ratio" || kind == "canceled") && calls != 0 {
				t.Fatal("invalid input read")
			}
			if kind == "no progress" && calls > maxEmptyReads {
				t.Fatal("unbounded empty reads", calls)
			}
		})
	}
}
