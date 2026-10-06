package importfile

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

func orderedFixture(t *testing.T, payloads []string) (*ImportSource, Config, *Attempt, string) {
	t.Helper()
	a, _, temp := attemptFixture(t, "unused\n", false)
	cfg := Config{Identity: a.config.Identity, MaxFiles: len(payloads), MaxDuration: 10 * time.Second,
		Limits: unlimitedFixtureLimits(), TempDir: temp}
	for i, payload := range payloads {
		cfg.Inputs = append(cfg.Inputs, Input{RunID: int64(101 + i), Path: inputFixture(t, []byte(payload))})
	}
	s, err := New(cfg, a.state, a.normalize)
	if err != nil {
		t.Fatal(err)
	}
	return s, cfg, a, temp
}

func TestImportSourceOrderSharedDeadlineAndIdenticalRecompression(t *testing.T) {
	s, cfg, a, temp := orderedFixture(t, []string{"z-first\n", "a-second\n", "z-first\n"})
	// The input's encoding is explicit, not derived from its synthetic filename.
	cfg.Inputs[2].Gzip = true
	if err := os.WriteFile(cfg.Inputs[2].Path, compressedFixture(t, "z-first\n", 1, "renamed"), 0600); err != nil {
		t.Fatal(err)
	}
	var normalized []string
	s, err := New(cfg, a.state, func(raw []byte) model.Observation {
		normalized = append(normalized, string(raw))
		return model.Observation{Raw: string(raw)}
	})
	if err != nil {
		t.Fatal(err)
	}
	// Mutating the caller slice/config must not alter the stored order or paths.
	cfg.Inputs[0] = Input{RunID: 999, Path: "missing"}
	cfg.MaxDuration = time.Nanosecond
	var created []int64
	var deadline time.Time
	sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
		got, bounded := ctx.Deadline()
		if !bounded || !deadline.IsZero() && !deadline.Equal(got) {
			t.Fatal("deadline reset between files", deadline, got)
		}
		deadline = got
		if b.ImportChange.Before == nil {
			created = append(created, b.ImportChange.Target.ID)
		}
		// Only one private copy can be held during application.
		entries, err := os.ReadDir(temp)
		if err != nil || len(entries) > 1 {
			t.Fatal("multiple copies retained", entries, err)
		}
		return a.state.(source.Sink).Commit(ctx, b)
	})
	if err := s.Run(context.Background(), sink); err != nil {
		t.Fatal(err)
	}
	if strings.Join(normalized, "") != "z-first\na-second\n" || len(created) != 3 || created[0] != 101 || created[1] != 102 || created[2] != 103 {
		t.Fatal("order, duplicates or caller mutation changed application", normalized, created)
	}
	for _, attempt := range s.attempts {
		run, found, err := a.state.ImportRun(context.Background(), s.ID(), attempt.config.RunID)
		if err != nil || !found || run.Status != source.ImportComplete {
			t.Fatal(run, found, err)
		}
		if err := os.Remove(attempt.config.Path); err != nil {
			t.Fatal(err)
		}
	}
	assertAttemptCleanup(t, temp)
	// Retrying the complete list needs no input, Sink mutation or normalization.
	if err := s.Run(context.Background(), importSinkFunc(func(context.Context, source.Batch) error { t.Fatal("completed list wrote again"); return nil })); err != nil {
		t.Fatal(err)
	}
	if len(normalized) != 2 {
		t.Fatal(normalized)
	}
}

func TestImportSourceStopsAndRetriesInterruptedFileBeforeLaterInput(t *testing.T) {
	s, _, a, temp := orderedFixture(t, []string{"first\n", "second\n", "third\n"})
	failure := errors.New("synthetic second file ACK loss")
	var pending []byte
	sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := a.state.(source.Sink).Commit(ctx, b); err != nil {
			return err
		}
		if b.ImportChange.Target.ID == 102 && len(b.Records) != 0 {
			pending, _ = json.Marshal(b)
			return failure
		}
		return nil
	})
	if err := s.Run(context.Background(), sink); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	assertAttemptCleanup(t, temp)
	first, found, err := a.state.ImportRun(context.Background(), s.ID(), 101)
	if err != nil || !found || first.Status != source.ImportComplete {
		t.Fatal(first, found, err)
	}
	if _, found, err := a.state.ImportRun(context.Background(), s.ID(), 103); err != nil || found {
		t.Fatal("later input started", found, err)
	}
	if err := os.Remove(s.attempts[0].config.Path); err != nil {
		t.Fatal(err)
	}
	retried := false
	retry := importSinkFunc(func(ctx context.Context, b source.Batch) error {
		if !retried {
			retried = true
			data, _ := json.Marshal(b)
			if string(data) != string(pending) {
				t.Fatal("retry did not start with interrupted exact batch")
			}
			assertAttemptCleanup(t, temp)
		}
		return a.state.(source.Sink).Commit(ctx, b)
	})
	if err := s.Run(context.Background(), retry); err != nil || !retried {
		t.Fatal(err, retried)
	}
	assertAttemptCleanup(t, temp)
	for _, attempt := range s.attempts {
		run, found, err := a.state.ImportRun(context.Background(), s.ID(), attempt.config.RunID)
		if err != nil || !found || run.Status != source.ImportComplete {
			t.Fatal(run, found, err)
		}
	}
}

func TestImportSourcePartialFileStopsListWithoutRevival(t *testing.T) {
	s, _, a, temp := orderedFixture(t, []string{"first\n", "partial", "third\n"})
	if err := s.Run(context.Background(), a.state.(source.Sink)); !errors.Is(err, ErrImportPartial) {
		t.Fatal(err)
	}
	assertAttemptCleanup(t, temp)
	if _, found, err := a.state.ImportRun(context.Background(), s.ID(), 103); err != nil || found {
		t.Fatal("partial continued list", found, err)
	}
	if err := s.Run(context.Background(), a.state.(source.Sink)); !errors.Is(err, ErrAttemptFailed) {
		t.Fatal("failed file revived", err)
	}
	if _, found, err := a.state.ImportRun(context.Background(), s.ID(), 103); err != nil || found {
		t.Fatal("retry skipped failed file", found, err)
	}
}

type importBlockingState struct{}

func (importBlockingState) ImportRun(ctx context.Context, _ string, _ int64) (source.ImportRun, bool, error) {
	<-ctx.Done()
	return source.ImportRun{}, false, ctx.Err()
}
func (importBlockingState) Checkpoint(context.Context, string, string) (source.Position, bool, error) {
	panic("deadline refusal must precede checkpoint lookup")
}

func TestImportSourceGlobalTimeoutAndParentContext(t *testing.T) {
	s, cfg, _, temp := orderedFixture(t, []string{"first\n"})
	cfg.MaxDuration = 25 * time.Millisecond
	s, err := New(cfg, importBlockingState{}, func([]byte) model.Observation { t.Fatal("expired input parsed"); return model.Observation{} })
	if err != nil {
		t.Fatal(err)
	}
	sink := importSinkFunc(func(context.Context, source.Batch) error { t.Fatal("expired lookup wrote state"); return nil })
	if err := s.Run(context.Background(), sink); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	assertAttemptCleanup(t, temp)
	cfg.MaxDuration = time.Hour
	s, err = New(cfg, importBlockingState{}, s.attempts[0].normalize)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	if err := s.Run(ctx, sink); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("caller deadline expanded", err)
	}
	ctx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if err := s.Run(ctx, sink); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestImportSourceListBoundsValidationAndConcurrentRun(t *testing.T) {
	s, cfg, a, temp := orderedFixture(t, []string{"line\n"})
	for _, edit := range []func(*Config){
		func(c *Config) { c.Inputs = nil }, func(c *Config) { c.MaxFiles = 0 },
		func(c *Config) { c.MaxFiles = MaxImportFiles + 1 }, func(c *Config) { c.MaxDuration = 0 },
		func(c *Config) { c.MaxDuration = -1 },
		func(c *Config) { c.Inputs = append(append([]Input(nil), c.Inputs...), c.Inputs[0]); c.MaxFiles = 2 },
		func(c *Config) {
			c.Inputs = append(append([]Input(nil), c.Inputs...), Input{RunID: 202, Path: "second"})
			c.MaxFiles = 1
		},
		func(c *Config) { c.Inputs = []Input{{RunID: 0, Path: "synthetic"}} },
		func(c *Config) { c.Inputs = []Input{{RunID: 202, Path: ""}} },
		func(c *Config) {
			c.Inputs = []Input{{RunID: 202, Path: "synthetic", Gzip: true}}
			c.Limits.MaxRatio = 0
		},
	} {
		bad := cfg
		edit(&bad)
		if got, err := New(bad, a.state, a.normalize); got != nil || err == nil {
			t.Fatal("invalid list accepted", got, err)
		}
	}
	if _, found, err := a.state.ImportRun(context.Background(), s.ID(), 101); err != nil || found {
		t.Fatal("configuration wrote state", found, err)
	}
	if err := s.Run(context.Background(), nil); err == nil {
		t.Fatal("nil sink accepted")
	}
	s.running.Lock()
	if err := s.Run(context.Background(), a.state.(source.Sink)); err != ErrImportSourceRunning {
		t.Fatal(err)
	}
	s.running.Unlock()
	assertAttemptCleanup(t, temp)
}
