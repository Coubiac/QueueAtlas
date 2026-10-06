package importfile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func attemptFixture(t *testing.T, payload string, zipped bool) (*Attempt, *sqlite.Store, string) {
	t.Helper()
	p, identity, run, _ := importIngestorFixture(t, payload, zipped, 0)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	s, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "synthetic.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	a, err := NewAttempt(AttemptConfig{Identity: identity, RunID: run.ID, Path: run.Path,
		Prepare: PrepareOptions{Gzip: zipped, Limits: unlimitedFixtureLimits(), TempDir: temp}}, s,
		func(raw []byte) model.Observation { return model.Observation{Raw: string(raw)} })
	if err != nil {
		t.Fatal(err)
	}
	return a, s, temp
}

func attemptContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func assertAttemptCleanup(t *testing.T, temp string) {
	t.Helper()
	entries, err := os.ReadDir(temp)
	if err != nil || len(entries) != 0 {
		t.Fatal("private copy leaked", entries, err)
	}
}

func attemptRunState(t *testing.T, s *sqlite.Store, a *Attempt) source.ImportRun {
	t.Helper()
	run, found, err := s.ImportRun(context.Background(), a.ID(), a.config.RunID)
	if err != nil || !found {
		t.Fatal("missing attempt", run, found, err)
	}
	return run
}

func TestImportAttemptLifecycleAndTerminalIdentity(t *testing.T) {
	for _, zipped := range []bool{false, true} {
		for _, payload := range []string{"", "same\nsame\n", "partial", "line\npartial"} {
			t.Run(payload+"/"+map[bool]string{true: "gzip", false: "plain"}[zipped], func(t *testing.T) {
				a, s, temp := attemptFixture(t, payload, zipped)
				count := 0
				a.normalize = func(raw []byte) model.Observation { count++; return model.Observation{Raw: string(raw)} }
				err := a.Run(attemptContext(t), s)
				wantStatus := source.ImportComplete
				if payload != "" && !strings.HasSuffix(payload, "\n") {
					wantStatus = source.ImportFailed
					if !errors.Is(err, ErrImportPartial) {
						t.Fatal(err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				run := attemptRunState(t, s, a)
				if run.Status != wantStatus || run.CompletedAt == nil || count != strings.Count(payload, "\n") {
					t.Fatal("wrong lifecycle", run, count)
				}
				assertAttemptCleanup(t, temp)
				// Same run ID describes the existing attempt. Terminal reuse does
				// not silently reopen a replaced path or revive a failed run.
				if err := os.Remove(a.config.Path); err != nil {
					t.Fatal(err)
				}
				err = a.Run(attemptContext(t), importSinkFunc(func(context.Context, source.Batch) error { t.Fatal("terminal attempt wrote"); return nil }))
				if wantStatus == source.ImportComplete && err != nil || wantStatus == source.ImportFailed && !errors.Is(err, ErrAttemptFailed) {
					t.Fatal("terminal identity", err)
				}
				if !reflect.DeepEqual(attemptRunState(t, s, a), run) {
					t.Fatal("terminal run mutated")
				}
			})
		}
	}
}

func TestImportAttemptPendingSurvivesCopyCleanupAndLostACK(t *testing.T) {
	for _, phase := range []string{"create", "attach", "record", "terminal"} {
		for _, lostACK := range []bool{false, true} {
			t.Run(phase+"/"+map[bool]string{true: "lost_ack", false: "before_write"}[lostACK], func(t *testing.T) {
				a, s, temp := attemptFixture(t, "same\nsame\n", false)
				count := 0
				a.normalize = func(raw []byte) model.Observation { count++; return model.Observation{Raw: string(raw)} }
				var pending []byte
				first := true
				failure := io.EOF // a Sink sentinel must never produce false success
				sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
					target := b.ImportChange.Target
					match := phase == "create" && b.ImportChange.Before == nil ||
						phase == "attach" && b.ImportChange.Before != nil && b.ImportChange.Before.Content == nil ||
						phase == "record" && len(b.Records) != 0 ||
						phase == "terminal" && target.Status == source.ImportComplete
					if first && match {
						first = false
						pending, _ = json.Marshal(b)
						if lostACK {
							if err := s.Commit(ctx, b); err != nil {
								t.Fatal(err)
							}
						}
						return failure
					}
					return s.Commit(ctx, b)
				})
				if err := a.Run(attemptContext(t), sink); !errors.Is(err, failure) || a.pending == nil {
					t.Fatal("failed commit falsely completed or lost pending", err)
				}
				assertAttemptCleanup(t, temp)
				if a.pending.ImportChange.Target.Status == source.ImportComplete {
					// A retained terminal batch needs no input after successful retry.
					if err := os.Remove(a.config.Path); err != nil {
						t.Fatal(err)
					}
				}
				firstRetry := true
				retry := importSinkFunc(func(ctx context.Context, b source.Batch) error {
					if firstRetry {
						firstRetry = false
						data, _ := json.Marshal(b)
						if string(data) != string(pending) {
							t.Fatal("copy cleanup changed pending batch")
						}
						assertAttemptCleanup(t, temp) // retry precedes any new input copy
					}
					return s.Commit(ctx, b)
				})
				if err := a.Run(attemptContext(t), retry); err != nil || firstRetry || a.pending != nil {
					t.Fatal("retry did not finish", err, firstRetry)
				}
				assertAttemptCleanup(t, temp)
				run := attemptRunState(t, s, a)
				if run.Status != source.ImportComplete || run.LastOffset != 10 || count != 2 {
					t.Fatal("pending replay lost or repeated normalization", run, count)
				}
			})
		}
	}
}

func TestImportAttemptPreparationFailureTraceAndDurableRestart(t *testing.T) {
	t.Run("interrupted_preparation_stays_running", func(t *testing.T) {
		a, s, temp := attemptFixture(t, "same\n", false)
		ctx, cancel := context.WithCancel(attemptContext(t))
		defer cancel()
		sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
			err := s.Commit(ctx, b)
			if err == nil && b.ImportChange.Before == nil {
				cancel()
			}
			return err
		})
		if err := a.Run(ctx, sink); !errors.Is(err, context.Canceled) || a.pending != nil {
			t.Fatal(err)
		}
		run := attemptRunState(t, s, a)
		if run.Content != nil || run.Status != source.ImportRunning || run.CompletedAt != nil {
			t.Fatal("interruption terminalized run", run)
		}
		assertAttemptCleanup(t, temp)
		if err := a.Run(attemptContext(t), s); err != nil {
			t.Fatal("interrupted preparation could not resume", err)
		}
		assertAttemptCleanup(t, temp)
	})
	t.Run("failed_preparation_trace_retry", func(t *testing.T) {
		a, s, temp := attemptFixture(t, "same\n", true)
		if err := os.WriteFile(a.config.Path, []byte("invalid gzip header"), 0600); err != nil {
			t.Fatal(err)
		}
		failure := errors.New("synthetic failed trace ACK lost")
		var first []byte
		sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
			if b.ImportChange.Target.Status == source.ImportFailed {
				first, _ = json.Marshal(b)
				if err := s.Commit(ctx, b); err != nil {
					t.Fatal(err)
				}
				return failure
			}
			return s.Commit(ctx, b)
		})
		if err := a.Run(attemptContext(t), sink); !errors.Is(err, failure) || a.pending == nil {
			t.Fatal(err)
		}
		assertAttemptCleanup(t, temp)
		retry := importSinkFunc(func(ctx context.Context, b source.Batch) error {
			data, _ := json.Marshal(b)
			if string(data) != string(first) {
				t.Fatal("failure trace retry changed")
			}
			return s.Commit(ctx, b)
		})
		if err := a.Run(attemptContext(t), retry); !errors.Is(err, ErrAttemptFailed) {
			t.Fatal(err)
		}
		run := attemptRunState(t, s, a)
		if run.Content != nil || run.LastOffset != 0 || run.Status != source.ImportFailed {
			t.Fatal(run)
		}
		assertAttemptCleanup(t, temp)
	})
	t.Run("restart_from_durable_checkpoint", func(t *testing.T) {
		a, s, temp := attemptFixture(t, "same\nsame\n", false)
		ctx, cancel := context.WithCancel(attemptContext(t))
		sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
			err := s.Commit(ctx, b)
			if err == nil && len(b.Records) != 0 {
				cancel()
			}
			return err
		})
		if err := a.Run(ctx, sink); !errors.Is(err, context.Canceled) || a.pending != nil {
			t.Fatal(err)
		}
		run := attemptRunState(t, s, a)
		if run.Status != source.ImportRunning || run.LastOffset != 5 {
			t.Fatal(run)
		}
		assertAttemptCleanup(t, temp)
		// Simulated process loss: a new object has no memory of earlier batches.
		restarted, err := NewAttempt(a.config, s, a.normalize)
		if err != nil {
			t.Fatal(err)
		}
		original := []byte("same\nsame\n")
		if err := os.WriteFile(a.config.Path, []byte("changed\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := restarted.Run(attemptContext(t), s); !errors.Is(err, ErrImportResume) {
			t.Fatal("changed content resumed", err)
		}
		if !reflect.DeepEqual(attemptRunState(t, s, a), run) {
			t.Fatal("proof failure changed run")
		}
		assertAttemptCleanup(t, temp)
		if err := os.Remove(a.config.Path); err != nil {
			t.Fatal(err)
		}
		if err := restarted.Run(attemptContext(t), s); err == nil {
			t.Fatal("missing resume accepted")
		}
		if !reflect.DeepEqual(attemptRunState(t, s, a), run) {
			t.Fatal("unavailable resume marked failed")
		}
		if err := os.WriteFile(a.config.Path, original, 0600); err != nil {
			t.Fatal(err)
		}
		count := 0
		restarted.normalize = func(raw []byte) model.Observation { count++; return model.Observation{Raw: string(raw)} }
		if err := restarted.Run(attemptContext(t), s); err != nil || count != 1 {
			t.Fatal("durable restart replayed line", err, count)
		}
		assertAttemptCleanup(t, temp)
	})
}

func TestImportAttemptCleanupErrorIsReportedWithoutRecursiveRemoval(t *testing.T) {
	a, s, temp := attemptFixture(t, "line\n", false)
	var sentinel, ownedDir string
	a.normalize = func(raw []byte) model.Observation {
		entries, err := os.ReadDir(temp)
		if err != nil || len(entries) != 1 {
			t.Fatal("missing private directory", entries, err)
		}
		ownedDir = filepath.Join(temp, entries[0].Name())
		sentinel = filepath.Join(ownedDir, "foreign-sentinel")
		if err := os.WriteFile(sentinel, []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
		return model.Observation{Raw: string(raw)}
	}
	if err := a.Run(attemptContext(t), s); err == nil {
		t.Fatal("cleanup failure hidden")
	}
	run := attemptRunState(t, s, a)
	if run.Status != source.ImportComplete {
		t.Fatal("cleanup error compensated durable completion", run)
	}
	entries, err := os.ReadDir(ownedDir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "foreign-sentinel" {
		t.Fatal("cleanup recursively removed foreign file or retained descriptor copy", entries, err)
	}
	if err := os.Remove(sentinel); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(ownedDir); err != nil {
		t.Fatal(err)
	}
	assertAttemptCleanup(t, temp)
}

func TestImportAttemptConfigurationDeadlineAndConcurrentRun(t *testing.T) {
	a, s, temp := attemptFixture(t, "line\n", false)
	for _, edit := range []func(*AttemptConfig){
		func(c *AttemptConfig) { c.RunID = 0 }, func(c *AttemptConfig) { c.Identity.Kind = "file" },
		func(c *AttemptConfig) { c.Path = "" }, func(c *AttemptConfig) { c.Prepare.Limits.ContentBytes = 0 },
		func(c *AttemptConfig) { c.Prepare.Gzip = true; c.Prepare.Limits.MaxRatio = 0 },
	} {
		cfg := a.config
		edit(&cfg)
		if got, err := NewAttempt(cfg, s, a.normalize); got != nil || err == nil {
			t.Fatal("invalid config", got, err)
		}
	}
	if err := a.Run(context.Background(), s); err != ErrImportDeadlineRequired {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(attemptContext(t))
	cancel()
	if err := a.Run(ctx, s); err != context.Canceled {
		t.Fatal(err)
	}
	if err := a.Run(attemptContext(t), nil); err == nil {
		t.Fatal("nil sink accepted")
	}
	a.running.Lock()
	if err := a.Run(attemptContext(t), s); err != ErrAttemptRunning {
		t.Fatal(err)
	}
	a.running.Unlock()
	if _, found, err := s.ImportRun(context.Background(), a.ID(), a.config.RunID); err != nil || found {
		t.Fatal("refusal wrote run", found, err)
	}
	assertAttemptCleanup(t, temp)
	// Absolute path identity from construction cannot be repointed by cfg.
	cfg := a.config
	other, err := NewAttempt(cfg, s, a.normalize)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Path = "different"
	if other.config.Path != a.config.Path || other.ID() != a.ID() {
		t.Fatal("config alias")
	}
}
