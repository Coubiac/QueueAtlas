package file

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"
)

type initialEndReaderAt func([]byte, int64) (int, error)

func (f initialEndReaderAt) ReadAt(p []byte, offset int64) (int, error) { return f(p, offset) }

func TestCaptureInitialEndBoundaryAndPosition(t *testing.T) {
	for _, content := range []string{"", "old\n", "old\r\n", strings.Repeat("x", 3*MaxFingerprintBytes) + "\n", "partial", "old\npartial"} {
		t.Run(content[:min(len(content), 16)], func(t *testing.T) {
			f, _ := testRegularFile(t, content)
			if _, err := f.Seek(2, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			anchor, err := captureInitialEnd(context.Background(), f)
			partial := len(content) > 0 && content[len(content)-1] != '\n'
			if partial {
				if !errors.Is(err, ErrInitialEndPartial) || anchor != (CheckpointAnchor{}) {
					t.Fatalf("partial: %+v, %v", anchor, err)
				}
			} else {
				window := content[max(0, len(content)-MaxFingerprintBytes):]
				if err != nil || anchor.Offset != int64(len(content)) || anchor.Length != len(window) || anchor.Digest != sha256.Sum256([]byte(window)) {
					t.Fatalf("boundary: %+v, %v", anchor, err)
				}
			}
			if pos, err := f.Seek(0, io.SeekCurrent); err != nil || pos != 2 {
				t.Fatalf("position: %d, %v", pos, err)
			}
		})
	}
}

func TestCaptureInitialEndUsesOneBoundedSnapshotWindow(t *testing.T) {
	for _, offset := range []int64{0, 4, 100 * MaxFingerprintBytes} {
		calls := 0
		anchor, err := captureInitialEndAt(context.Background(), initialEndReaderAt(func(p []byte, start int64) (int, error) {
			calls++
			if len(p) != int(min(offset, int64(MaxFingerprintBytes))) || start != offset-int64(len(p)) {
				t.Fatal("unbounded or moved end", len(p), start)
			}
			for i := range p {
				p[i] = 'x'
			}
			p[len(p)-1] = '\n'
			return len(p), nil // appends beyond offset are never read
		}), offset)
		wantCalls := 1
		if offset == 0 {
			wantCalls = 0
		}
		if err != nil || anchor.Offset != offset || calls != wantCalls {
			t.Fatalf("capture: %+v, %v, calls %d", anchor, err, calls)
		}
	}
}

func TestCaptureInitialEndRefusesErrorsAndCancellation(t *testing.T) {
	boom := errors.New("synthetic read failure")
	for _, kind := range []string{"short", "read error", "cancel after read", "cancel before read", "nil input", "negative"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want, calls, offset := error(io.ErrUnexpectedEOF), 0, int64(4)
			var input io.ReaderAt = initialEndReaderAt(func(p []byte, _ int64) (int, error) {
				calls++
				if kind == "read error" {
					return 0, boom
				}
				if kind == "cancel after read" {
					copy(p, "old\n")
					cancel()
					return len(p), nil
				}
				if kind != "short" {
					t.Fatal("invalid capture read")
				}
				return 1, io.EOF
			})
			switch kind {
			case "read error":
				want = boom
			case "cancel after read", "cancel before read":
				want = context.Canceled
				if kind == "cancel before read" {
					cancel()
				}
			case "nil input":
				input = nil
				want = nil
			case "negative":
				offset = -1
				want = nil
			}
			anchor, err := captureInitialEndAt(ctx, input, offset)
			if err == nil || (want != nil && !errors.Is(err, want)) || anchor != (CheckpointAnchor{}) {
				t.Fatalf("failure: %+v, %v", anchor, err)
			}
			if (kind == "cancel before read" || kind == "nil input" || kind == "negative") && calls != 0 {
				t.Fatal("validation read input")
			}
		})
	}
	if _, err := captureInitialEnd(context.Background(), nil); err == nil {
		t.Fatal("nil file accepted")
	}
	f, _ := testRegularFile(t, "old\n")
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := captureInitialEnd(context.Background(), f); err == nil {
		t.Fatal(err)
	}
}
