package file

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func sourceConfig(path string) Config {
	return Config{Identity: fileSourceIdentity(), Path: path, PollInterval: MinPollInterval}
}

func TestNewFileSourceValidatesWithoutOpeningOrWriting(t *testing.T) {
	reader := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
		t.Fatal("constructor read state")
		return source.OriginPage{}, nil
	})
	path := filepath.Join(t.TempDir(), "not-created.log")
	for _, kind := range []string{"ID", "kind", "name", "path", "reader", "normalizer", "interval short", "interval long", "grace short", "grace long"} {
		cfg, stateReader, normalize := sourceConfig(path), source.StateReader(reader), Normalize(testNormalizer)
		switch kind {
		case "ID":
			cfg.Identity.ID = ""
		case "kind":
			cfg.Identity.Kind = "syslog"
		case "name":
			cfg.Identity.Name = ""
		case "path":
			cfg.Path = ""
		case "reader":
			stateReader = nil
		case "normalizer":
			normalize = nil
		case "interval short":
			cfg.PollInterval = MinPollInterval - 1
		case "interval long":
			cfg.PollInterval = MaxPollInterval + 1
		case "grace short":
			cfg.RotationGrace = MinRotationGrace - 1
		case "grace long":
			cfg.RotationGrace = MaxRotationGrace + 1
		}
		if s, err := New(cfg, stateReader, normalize); s != nil || err == nil {
			t.Fatalf("invalid %s accepted", kind)
		}
	}
	cfg := sourceConfig(path)
	cfg.PollInterval = 0
	s, err := New(cfg, reader, testNormalizer)
	if err != nil || s.ID() != cfg.Identity.ID || s.config.RotationGrace != DefaultRotationGrace {
		t.Fatalf("valid config: %v, %v", s, err)
	}
	cfg.Identity.ID, cfg.Path = "changed", "changed.log"
	if s.ID() != fileSourceIdentity().ID {
		t.Fatal("configuration was borrowed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	sink := sinkFunc(func(context.Context, source.Batch) error { t.Fatal("invalid run committed"); return nil })
	if err := s.Run(ctx, sink); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), nil); err == nil {
		t.Fatal("nil sink accepted")
	}
	if err := s.Run(context.Background(), sink); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing input error: %v", err)
	}
}

func TestRunOpenedClosesOwnedDescriptorOnCancellation(t *testing.T) {
	f, path := testRegularFile(t, "line\n")
	reader := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
		t.Fatal("cancelled run read state")
		return source.OriginPage{}, nil
	})
	s, err := New(sourceConfig(path), reader, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	waiting, err := s.runOpened(ctx, f, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("cancelled run committed"); return nil }))
	if waiting || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled open: %v, %v", waiting, err)
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("descriptor retained: %v", err)
	}
}

func TestFileSourceRejectsConcurrentRunAndReleasesGuard(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("persistent identities require Linux")
	}
	_, path := testRegularFile(t, "line\n")
	entered, release := make(chan struct{}), make(chan struct{})
	reader := originReaderFunc(func(ctx context.Context, _ source.OriginQuery) (source.OriginPage, error) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return source.OriginPage{}, ctx.Err()
		}
		return source.OriginPage{}, errors.New("stop selection")
	})
	s, err := New(sourceConfig(path), reader, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	sink := sinkFunc(func(context.Context, source.Batch) error { t.Error("blocked selection committed"); return nil })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- s.Run(ctx, sink) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("first run did not enter reader")
	}
	if err := s.Run(ctx, sink); !errors.Is(err, ErrSourceRunning) {
		t.Errorf("concurrent run: %v", err)
	}
	close(release)
	select {
	case err := <-finished:
		if err == nil {
			t.Error("reader failure lost")
		}
	case <-ctx.Done():
		t.Fatal("first run did not stop")
	}
	if !s.running.TryLock() {
		t.Fatal("execution guard retained")
	}
	s.running.Unlock()
}
