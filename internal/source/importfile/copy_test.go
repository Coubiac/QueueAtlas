package importfile

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestCopyInspectedBytesAndFingerprint(t *testing.T) {
	for _, content := range []string{"", "same\nsame\n", "line\r\npartial", strings.Repeat("x", 3*contentBufferBytes) + "\n"} {
		want, err := InspectPlain(context.Background(), strings.NewReader(content), int64(max(1, len(content))))
		if err != nil {
			t.Fatal(err)
		}
		for _, encoding := range []string{"plain", "gzip"} {
			var copied bytes.Buffer
			var info ContentInfo
			if encoding == "plain" {
				info, err = CopyPlain(context.Background(), strings.NewReader(content), &copied, int64(max(1, len(content))))
			} else {
				encoded := compressedFixture(t, content, gzip.BestSpeed, "synthetic.log")
				info, err = CopyGzip(context.Background(), bytes.NewReader(encoded), &copied, unlimitedFixtureLimits())
			}
			if err != nil || info != want || copied.String() != content {
				t.Fatal("copied content differs from digest input", encoding, info, want, err)
			}
		}
	}
}

func TestCopyWriteFailuresAndCancellationDiscardMetadata(t *testing.T) {
	boom := errors.New("synthetic output failure")
	for _, encoding := range []string{"plain", "gzip"} {
		for _, kind := range []string{"nil output", "short", "partial error", "full error", "negative", "excess", "cancel on write", "cancel and error", "cancel before read"} {
			t.Run(encoding+"/"+kind, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				want := error(nil)
				var output io.Writer = writerFunc(func(p []byte) (int, error) {
					calls++
					switch kind {
					case "short":
						return len(p) - 1, nil
					case "partial error":
						return len(p) - 1, boom
					case "full error":
						return len(p), boom
					case "negative":
						return -1, nil
					case "excess":
						return len(p) + 1, nil
					case "cancel on write":
						cancel()
						return len(p), nil
					case "cancel and error":
						cancel()
						return len(p), boom
					default:
						t.Fatal("invalid copy wrote output")
						return 0, nil
					}
				})
				switch kind {
				case "nil output":
					output = nil
				case "short":
					want = io.ErrShortWrite
				case "partial error", "full error":
					want = boom
				case "negative", "excess":
					want = ErrInvalidWriter
				case "cancel on write", "cancel and error":
					want = context.Canceled
				case "cancel before read":
					cancel()
					want = context.Canceled
				}
				var input io.Reader = strings.NewReader("one\nnext\n")
				if kind == "nil output" || kind == "cancel before read" {
					input = readerFunc(func([]byte) (int, error) { t.Fatal("invalid copy read input"); return 0, nil })
				}
				var info ContentInfo
				var err error
				if encoding == "plain" {
					info, err = CopyPlain(ctx, input, output, 100)
				} else {
					if kind != "nil output" && kind != "cancel before read" {
						input = bytes.NewReader(compressedFixture(t, "one\nnext\n", gzip.BestSpeed, ""))
					}
					info, err = CopyGzip(ctx, input, output, unlimitedFixtureLimits())
				}
				if err == nil || (want != nil && !errors.Is(err, want)) || info != (ContentInfo{}) {
					t.Fatal("write error metadata", info, err)
				}
				if kind == "cancel and error" && !errors.Is(err, boom) {
					t.Fatal("write cause lost on cancel", err)
				}
				wantCalls := 1
				if kind == "nil output" || kind == "cancel before read" {
					wantCalls = 0
				}
				if calls != wantCalls {
					t.Fatal("write retried or preceded validation", calls)
				}
			})
		}
	}
}

func TestCopyRefusesTentativeContentAndBudgetExcess(t *testing.T) {
	content := strings.Repeat("x", 2*contentBufferBytes) + "\n"
	limit := int64(contentBufferBytes + 10)
	for _, encoding := range []string{"plain", "gzip"} {
		var copied bytes.Buffer
		var info ContentInfo
		var err error
		if encoding == "plain" {
			info, err = CopyPlain(context.Background(), strings.NewReader(content), &copied, limit)
		} else {
			encoded := compressedFixture(t, content, gzip.BestSpeed, "")
			limits := unlimitedFixtureLimits()
			limits.ContentBytes = limit
			info, err = CopyGzip(context.Background(), bytes.NewReader(encoded), &copied, limits)
		}
		if !errors.Is(err, ErrByteLimit) || info != (ContentInfo{}) || int64(copied.Len()) > limit {
			t.Fatal("copy exceeded budget or acknowledged partial", encoding, info, err, copied.Len())
		}
	}
	encoded := compressedFixture(t, "valid first\n", gzip.BestSpeed, "")
	bad := compressedFixture(t, "tentative second\n", gzip.BestSpeed, "")
	bad[len(bad)-8] ^= 1
	var copied bytes.Buffer
	info, err := CopyGzip(context.Background(), bytes.NewReader(append(encoded, bad...)), &copied, unlimitedFixtureLimits())
	if !errors.Is(err, gzip.ErrChecksum) || info != (ContentInfo{}) || copied.Len() == 0 {
		t.Fatal("tentative copy falsely acknowledged", info, err, copied.Len())
	}
}

func TestCopyPreservesSimultaneousReadAndWriteFailures(t *testing.T) {
	readBoom, writeBoom := errors.New("synthetic read failure"), errors.New("synthetic write failure")
	for _, encoding := range []string{"plain", "gzip", "clean EOF"} {
		for _, kind := range []string{"write error", "short", "invalid", "cancel"} {
			t.Run(encoding+"/"+kind, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				wantRead, wantWrite := error(readBoom), error(writeBoom)
				var input io.Reader = readerFunc(func(p []byte) (int, error) { return copy(p, "one\n"), readBoom })
				if encoding == "gzip" {
					encoded := compressedFixture(t, "one\n", gzip.BestSpeed, "")
					encoded[len(encoded)-8] ^= 1
					input, wantRead = bytes.NewReader(encoded), gzip.ErrChecksum
				} else if encoding == "clean EOF" {
					input = readerFunc(func(p []byte) (int, error) { return copy(p, "one\n"), io.EOF })
					wantRead = nil
				}
				output := writerFunc(func(p []byte) (int, error) {
					switch kind {
					case "short":
						return len(p) - 1, nil
					case "invalid":
						return len(p) + 1, nil
					case "cancel":
						cancel()
						return len(p), nil
					default:
						return len(p), writeBoom
					}
				})
				switch kind {
				case "short":
					wantWrite = io.ErrShortWrite
				case "invalid":
					wantWrite = ErrInvalidWriter
				case "cancel":
					wantWrite = context.Canceled
				}
				var info ContentInfo
				var err error
				if encoding == "gzip" {
					info, err = CopyGzip(ctx, input, output, unlimitedFixtureLimits())
				} else {
					info, err = CopyPlain(ctx, input, output, 10)
				}
				if !errors.Is(err, wantWrite) || (wantRead != nil && !errors.Is(err, wantRead)) || info != (ContentInfo{}) {
					t.Fatal("simultaneous cause lost", info, err, wantRead, wantWrite)
				}
				if wantRead == nil && errors.Is(err, io.EOF) {
					t.Fatal("clean EOF joined as failure", err)
				}
			})
		}
	}
}
