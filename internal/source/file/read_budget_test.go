package file

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
)

func TestChunkedIngestorPreservesOversizedLineAndPending(t *testing.T) {
	content := strings.Repeat("x", 2*64*1024) + "\n"
	f, _ := testRegularFile(t, content)
	normalized := 0
	r, err := NewIngestor(context.Background(), f, fileSourceIdentity(), ingestState(t, f, 0), func([]byte) model.Observation {
		normalized++
		return model.Observation{}
	})
	if err != nil {
		t.Fatal(err)
	}
	commits := 0
	var failed source.Batch
	sink := sinkFunc(func(_ context.Context, b source.Batch) error {
		commits++
		failed = b
		return io.EOF
	})
	for i := 1; i <= 2; i++ {
		if err := r.commitNext(context.Background(), sink, followReadFragments); !errors.Is(err, errReadYield) {
			t.Fatal("partial read did not yield", err)
		}
		if r.lines.offset != int64(i*64*1024) || r.Position().Offset != 0 || r.pending != nil || commits != 0 || normalized != 0 {
			t.Fatal("yield changed checkpoint, consumed too much or committed a partial line")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.commitNext(ctx, sink, followReadFragments); !errors.Is(err, context.Canceled) || r.lines.offset != 2*64*1024 {
		t.Fatal("cancelled partial read changed state", err)
	}
	if err := r.commitNext(context.Background(), sink, followReadFragments); !errors.Is(err, io.EOF) || r.pending == nil || commits != 1 || normalized != 0 || r.Position().Offset != 0 {
		t.Fatal("sink failure lost pending or advanced checkpoint", err)
	}
	record := failed.Records[0]
	if record.Start != 0 || record.End != int64(len(content)) || len(record.Raw) != model.MaxLineBytes || record.Error == "" || record.Observation.Kind != model.KindUnknown {
		t.Fatal("chunking changed oversized record", record.Start, record.End, len(record.Raw), record.Error)
	}
	if err := r.commitNext(context.Background(), sinkFunc(func(_ context.Context, b source.Batch) error {
		if !reflect.DeepEqual(b, failed) {
			t.Fatal("retry changed batch")
		}
		return nil
	}), followReadFragments); err != nil || r.Position().Offset != int64(len(content)) || r.pending != nil {
		t.Fatal("pending retry failed", err)
	}
	anchor, err := ParseCheckpointAnchor(r.Position().AnchorHash)
	if err != nil {
		t.Fatal(err)
	}
	if matches, err := anchor.Matches(f); err != nil || !matches {
		t.Fatal("chunking changed acknowledged anchor", matches, err)
	}
}
