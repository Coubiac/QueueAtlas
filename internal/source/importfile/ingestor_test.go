package importfile

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/source/file"
)

func importIngestorFixture(t *testing.T, payload string, zipped bool, offset int64) (*PreparedContent, source.Identity, source.ImportRun, source.Position) {
	t.Helper()
	data := []byte(payload)
	if zipped {
		data = compressedFixture(t, payload, gzip.BestSpeed, "synthetic.log")
	}
	path := inputFixture(t, data)
	p, err := PrepareRegular(context.Background(), path, PrepareOptions{Gzip: zipped, Limits: unlimitedFixtureLimits(), TempDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	identity := source.Identity{ID: "archive", Kind: "import", Name: "synthetic archive", TrustedHost: "mx-a"}
	info := p.Info()
	id, err := source.ImportOriginID(identity.ID, info.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	run := source.ImportRun{ID: 101, SourceID: identity.ID, Path: path, Status: source.ImportRunning,
		CreatedAt: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), LastOffset: offset,
		Content: &source.ImportContent{OriginID: id, Bytes: info.Bytes, SHA256: info.SHA256, TrailingPartial: info.TrailingPartial}}
	anchor, err := file.CaptureAnchor(p.file, offset)
	if err != nil {
		t.Fatal(err)
	}
	return p, identity, run, source.Position{OriginID: id, Offset: offset, AnchorHash: anchor.String()}
}

func TestNewImportIngestorBindsCopyAndVerifiedPosition(t *testing.T) {
	for _, zipped := range []bool{false, true} {
		for _, tc := range []struct {
			payload string
			offset  int64
			next    string
		}{
			{"", 0, ""},
			{"same\nsame\n", 0, "same\n"},
			{"same\nsame\n", 5, "same\n"},
			{"same\nsame\n", 10, ""},
			{"line\npartial", 5, ""},
			{strings.Repeat("x", file.MaxFingerprintBytes+7) + "\nnext\n", file.MaxFingerprintBytes + 8, "next\n"},
		} {
			t.Run(fmt.Sprintf("gzip_%t/bytes_%d/offset_%d", zipped, len(tc.payload), tc.offset), func(t *testing.T) {
				p, identity, run, position := importIngestorFixture(t, tc.payload, zipped, tc.offset)
				// Starting descriptor position and original-path replacement do
				// not redirect the held private copy or choose a resume offset.
				if _, err := p.Seek(1, io.SeekStart); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(run.Path, []byte("changed original\n"), 0600); err != nil {
					t.Fatal(err)
				}
				calls := 0
				r, err := NewIngestor(context.Background(), p, identity, run, position, func([]byte) model.Observation { calls++; return model.Observation{} })
				if err != nil || r == nil || calls != 0 || r.Position() != position {
					t.Fatal("bind", r, err, calls)
				}
				if offset, err := p.Seek(0, io.SeekCurrent); err != nil || offset != tc.offset {
					t.Fatal("bound read position", offset, err)
				}
				originalBytes := run.Content.Bytes
				run.Content.Bytes++
				state := r.RunState()
				state.Content.Bytes++
				if r.RunState().Content.Bytes != originalBytes {
					t.Fatal("caller metadata pointer changed bound state")
				}
				record, err := r.lines.Next(context.Background())
				if tc.next == "" {
					if !errors.Is(err, io.EOF) {
						t.Fatal("expected no remaining complete line", record, err)
					}
				} else if err != nil || string(record.Raw) != tc.next || record.Start != tc.offset {
					t.Fatal("wrong first physical line", record, err)
				}
				if calls != 0 || r.Position() != position || r.RunState().LastOffset != position.Offset {
					t.Fatal("constructor/physical reader normalized or acknowledged a record")
				}
			})
		}
	}
}

func TestNewImportIngestorRefusesUnverifiedStateWithoutSeeking(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*source.Identity, *source.ImportRun, *source.Position)
	}{
		{"wrong_kind", func(i *source.Identity, _ *source.ImportRun, _ *source.Position) { i.Kind = "file" }},
		{"empty_name", func(i *source.Identity, _ *source.ImportRun, _ *source.Position) { i.Name = "" }},
		{"foreign_source", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.SourceID = "other" }},
		{"zero_run", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.ID = 0 }},
		{"terminal_run", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.Status = source.ImportFailed }},
		{"terminal_time", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) {
			stamp := r.CreatedAt
			r.CompletedAt = &stamp
		}},
		{"unrepresentable_time", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.CreatedAt = time.Time{} }},
		{"unprepared", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.Content = nil }},
		{"wrong_digest", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) {
			r.Content.SHA256 = strings.Repeat("0", 64)
		}},
		{"wrong_size", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.Content.Bytes++ }},
		{"wrong_partial", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.Content.TrailingPartial = true }},
		{"wrong_origin", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.Content.OriginID = "foreign" }},
		{"wrong_checkpoint_origin", func(_ *source.Identity, _ *source.ImportRun, p *source.Position) { p.OriginID = "foreign" }},
		{"offset_disagrees", func(_ *source.Identity, r *source.ImportRun, _ *source.Position) { r.LastOffset++ }},
		{"past_eof", func(_ *source.Identity, r *source.ImportRun, p *source.Position) { r.LastOffset, p.Offset = 11, 11 }},
		{"negative", func(_ *source.Identity, r *source.ImportRun, p *source.Position) { r.LastOffset, p.Offset = -1, -1 }},
		{"malformed_anchor", func(_ *source.Identity, _ *source.ImportRun, p *source.Position) { p.AnchorHash = "opaque" }},
		{"changed_anchor", func(_ *source.Identity, _ *source.ImportRun, p *source.Position) {
			p.AnchorHash = p.AnchorHash[:len(p.AnchorHash)-1] + "0"
		}},
		{"noncanonical_anchor", func(_ *source.Identity, _ *source.ImportRun, p *source.Position) {
			p.AnchorHash = strings.ToUpper(p.AnchorHash)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, identity, run, position := importIngestorFixture(t, "same\nsame\n", false, 5)
			tc.edit(&identity, &run, &position)
			if _, err := p.Seek(2, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			calls := 0
			r, err := NewIngestor(context.Background(), p, identity, run, position, func([]byte) model.Observation { calls++; return model.Observation{} })
			if r != nil || !errors.Is(err, ErrImportResume) || calls != 0 {
				t.Fatal("unverified state accepted", r, err, calls)
			}
			if offset, err := p.Seek(0, io.SeekCurrent); err != nil || offset != 2 {
				t.Fatal("refusal moved or closed caller copy", offset, err)
			}
		})
	}
	// Even a matching digest/anchor at a position in the middle of a line
	// supplies no complete physical-line boundary.
	p, identity, run, position := importIngestorFixture(t, "same\nsame\n", false, 1)
	if r, err := NewIngestor(context.Background(), p, identity, run, position, func([]byte) model.Observation { return model.Observation{} }); r != nil || !errors.Is(err, ErrImportResume) {
		t.Fatal("mid-line checkpoint accepted", r, err)
	}
}

func TestNewImportIngestorCancellationClosedCopyAndSizeChange(t *testing.T) {
	p, identity, run, position := importIngestorFixture(t, "same\nsame\n", false, 0)
	normalize := func([]byte) model.Observation {
		t.Fatal("normalizer called during construction")
		return model.Observation{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, err := NewIngestor(ctx, p, identity, run, position, normalize); r != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled constructor", r, err)
	}
	if r, err := NewIngestor(context.Background(), p, identity, run, position, nil); r != nil || !errors.Is(err, ErrImportResume) {
		t.Fatal("nil normalizer accepted", r, err)
	}
	if r, err := NewIngestor(context.Background(), nil, identity, run, position, normalize); r != nil || !errors.Is(err, fs.ErrClosed) {
		t.Fatal("nil copy accepted", r, err)
	}
	if err := os.Truncate(p.path, 1); err != nil {
		t.Fatal(err)
	}
	if r, err := NewIngestor(context.Background(), p, identity, run, position, normalize); r != nil || !errors.Is(err, ErrPreparedSize) {
		t.Fatal("changed private copy size accepted", r, err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if r, err := NewIngestor(context.Background(), p, identity, run, position, normalize); r != nil || !errors.Is(err, fs.ErrClosed) {
		t.Fatal("closed copy accepted", r, err)
	}
}
