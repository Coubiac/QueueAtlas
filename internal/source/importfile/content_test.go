package importfile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func TestInspectPlainWholeContentAndBoundaries(t *testing.T) {
	for _, data := range []string{"", "abc", "same\nsame\n", "one\r\ntwo\r\n", "one\npartial", strings.Repeat("x", 2*contentBufferBytes) + "\n"} {
		t.Run(data[:min(len(data), 16)], func(t *testing.T) {
			info, err := InspectPlain(context.Background(), strings.NewReader(data), int64(max(1, len(data))))
			digest := sha256.Sum256([]byte(data))
			partial := len(data) > 0 && data[len(data)-1] != '\n'
			if err != nil || info.Bytes != int64(len(data)) || info.SHA256 != hex.EncodeToString(digest[:]) || info.TrailingPartial != partial {
				t.Fatalf("whole content: %+v, %v", info, err)
			}
		})
	}
	info, err := InspectPlain(context.Background(), readerFunc(func(p []byte) (int, error) { return copy(p, "line\n"), io.EOF }), math.MaxInt64)
	if err != nil || info.Bytes != 5 || info.TrailingPartial {
		t.Fatal("data plus EOF", info, err)
	}
}

func TestInspectPlainByteBudgetAndReadBounds(t *testing.T) {
	for _, excess := range []bool{false, true} {
		limit := int64(3*contentBufferBytes + 7)
		available := limit
		if excess {
			available++
		}
		consumed, maxRead := int64(0), 0
		info, err := InspectPlain(context.Background(), readerFunc(func(p []byte) (int, error) {
			maxRead = max(maxRead, len(p))
			n := int(min(int64(len(p)), available-consumed))
			for i := range p[:n] {
				p[i] = 'x'
			}
			consumed += int64(n)
			if consumed == available {
				return n, io.EOF
			}
			return n, nil
		}), limit)
		if maxRead > contentBufferBytes || consumed != available {
			t.Fatal("unbounded read", maxRead, consumed)
		}
		if excess {
			if !errors.Is(err, ErrByteLimit) || info != (ContentInfo{}) {
				t.Fatal("excess accepted", info, err)
			}
		} else if err != nil || info.Bytes != limit {
			t.Fatal("exact limit refused", info, err)
		}
	}
	// Many short reads must hash the same bytes without depending on chunking.
	data := "repeated\nrepeated\n"
	index := 0
	info, err := InspectPlain(context.Background(), readerFunc(func(p []byte) (int, error) {
		if index == len(data) {
			return 0, io.EOF
		}
		p[0] = data[index]
		index++
		return 1, nil
	}), int64(len(data)))
	want, _ := InspectPlain(context.Background(), strings.NewReader(data), int64(len(data)))
	if err != nil || info != want {
		t.Fatal("chunking changed fingerprint", info, want, err)
	}
}

func TestInspectPlainErrorsAndCancellationDiscardMetadata(t *testing.T) {
	boom := errors.New("synthetic input failure")
	for _, kind := range []string{"nil", "zero limit", "negative limit", "canceled", "expired", "read error", "error with bytes", "joined EOF failure", "cancel with EOF", "no progress", "negative count", "excess count"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls, limit := 0, int64(10)
			want := error(nil)
			var input io.Reader = readerFunc(func(p []byte) (int, error) {
				calls++
				if kind == "read error" && calls == 1 {
					return copy(p, "line\n"), nil
				}
				switch kind {
				case "read error":
					return 0, boom
				case "error with bytes":
					return copy(p, "line\n"), boom
				case "joined EOF failure":
					return copy(p, "line\n"), errors.Join(io.EOF, boom)
				case "cancel with EOF":
					cancel()
					return copy(p, "line\n"), io.EOF
				case "no progress":
					return 0, nil
				case "negative count":
					return -1, nil
				case "excess count":
					return len(p) + 1, nil
				default:
					t.Fatal("invalid inspection read")
					return 0, nil
				}
			})
			switch kind {
			case "nil":
				input = nil
			case "zero limit":
				limit = 0
			case "negative limit":
				limit = -1
			case "canceled":
				cancel()
				want = context.Canceled
			case "expired":
				var expireCancel context.CancelFunc
				ctx, expireCancel = context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer expireCancel()
				want = context.DeadlineExceeded
			case "read error", "error with bytes", "joined EOF failure":
				want = boom
			case "cancel with EOF":
				want = context.Canceled
			case "no progress":
				want = io.ErrNoProgress
			case "negative count", "excess count":
				want = ErrInvalidReader
			}
			info, err := InspectPlain(ctx, input, limit)
			if err == nil || (want != nil && !errors.Is(err, want)) || info != (ContentInfo{}) {
				t.Fatal("failure exposed partial metadata", info, err)
			}
			if (kind == "nil" || kind == "zero limit" || kind == "negative limit" || kind == "canceled" || kind == "expired") && calls != 0 {
				t.Fatal("validation consumed input")
			}
			if kind == "no progress" && calls != maxEmptyReads {
				t.Fatal("empty reads not bounded", calls)
			}
		})
	}
}
