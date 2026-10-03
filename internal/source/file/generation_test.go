package file

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
)

type sinkFunc func(context.Context, source.Batch) error

func (s sinkFunc) Commit(ctx context.Context, batch source.Batch) error { return s(ctx, batch) }

func fileSourceIdentity() source.Identity {
	return source.Identity{ID: "source-1", Kind: "file", Name: "synthetic mail log", TrustedHost: "mx.example.test"}
}

func TestEnsureGenerationRejectsInvalidInputs(t *testing.T) {
	f, _ := testRegularFile(t, "line\n")
	reader := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
		t.Fatal("invalid input queried state")
		return source.OriginPage{}, nil
	})
	sink := sinkFunc(func(context.Context, source.Batch) error { t.Fatal("invalid input committed"); return nil })
	for _, identity := range []source.Identity{
		{Kind: "file", Name: "mail"}, {ID: "mail", Kind: "file"}, {ID: "mail", Kind: "syslog", Name: "mail"},
	} {
		if _, err := EnsureGeneration(context.Background(), f, identity, reader, sink); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
	if _, err := EnsureGeneration(context.Background(), f, fileSourceIdentity(), reader, nil); err == nil {
		t.Fatal("nil sink accepted")
	}
	if _, err := EnsureGeneration(context.Background(), nil, fileSourceIdentity(), reader, sink); err == nil {
		t.Fatal("nil file accepted")
	}
	if _, err := EnsureGeneration(context.Background(), f, fileSourceIdentity(), nil, sink); err == nil {
		t.Fatal("nil reader accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := EnsureGeneration(ctx, f, fileSourceIdentity(), reader, sink); !errors.Is(err, context.Canceled) || got.State != nil || got.Created {
		t.Fatalf("cancelled start: %+v, %v", got, err)
	}
	if runtime.GOOS != "linux" {
		got, err := EnsureGeneration(context.Background(), f, fileSourceIdentity(), reader, sink)
		if err != nil || got.Selection != SelectionInsufficient || got.State != nil || got.Created {
			t.Fatalf("unsupported identity: %+v, %v", got, err)
		}
	}
}
