//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

func TestFileSourceStartsAndResumesCommittedGeneration(t *testing.T) {
	writer, path := testRegularFile(t, "first\nnext\n")
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := sourceConfig(path)
	s, err := New(cfg, store, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Path, cfg.Identity.ID = "invalid", "changed"
	var originID string
	for i, want := range []string{"first\n", "next\n"} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		calls := 0
		err := s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
			if err := store.Commit(ctx, b); err != nil {
				return err
			}
			if len(b.Records) > 0 {
				calls++
				r := b.Records[0]
				if string(r.Raw) != want || r.Start != int64(i*6) || r.Observation.SourceID != s.ID() {
					t.Fatalf("run %d record: %+v", i, r)
				}
				if i == 0 {
					originID = r.OriginID
				} else if r.OriginID != originID {
					t.Fatal("resume created another origin")
				}
				cancel()
			}
			return nil
		}))
		cancel()
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("run %d: %v, record commits %d", i, err, calls)
		}
	}
	id, err := Inspect(writer)
	if err != nil {
		t.Fatal(err)
	}
	page, err := store.FileOrigins(context.Background(), source.OriginQuery{SourceID: s.ID(), Device: id.Device, Inode: id.Inode, Limit: 100})
	if err != nil || len(page.States) != 1 || page.States[0].Checkpoint.Offset != 11 {
		t.Fatalf("resumed state: %+v, %v", page, err)
	}
}

func TestFileSourceWaitsOnEmptyInputAndRegistersAfterAppend(t *testing.T) {
	writer, path := testRegularFile(t, "")
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	queries := make(chan struct{}, 4)
	reader := originReaderFunc(func(ctx context.Context, q source.OriginQuery) (source.OriginPage, error) {
		page, err := store.FileOrigins(ctx, q)
		select {
		case queries <- struct{}{}:
		default:
		}
		return page, err
	})
	s, err := New(sourceConfig(path), statePathReader{reader, store}, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	written := make(chan error, 1)
	go func() {
		for range 2 {
			select {
			case <-queries:
			case <-ctx.Done():
				written <- ctx.Err()
				return
			}
		}
		_, err := writer.WriteAt([]byte("added\n"), 0)
		written <- err
	}()
	registrations, records := 0, 0
	err = s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		if len(b.Origins) > 0 {
			registrations++
		}
		if len(b.Records) > 0 {
			records++
			if string(b.Records[0].Raw) != "added\n" {
				t.Fatalf("appended record: %+v", b.Records[0])
			}
			cancel()
		}
		return nil
	}))
	if writeErr := <-written; writeErr != nil {
		t.Fatal(writeErr)
	}
	if !errors.Is(err, context.Canceled) || registrations != 1 || records != 1 {
		t.Fatalf("empty startup: %v, registrations %d, records %d", err, registrations, records)
	}
}

func TestFileSourceEmptyWaitClosesDescriptorAndCancels(t *testing.T) {
	f, path := testRegularFile(t, "")
	queries := 0
	reader := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
		queries++
		return source.OriginPage{}, nil
	})
	cfg := sourceConfig(path)
	cfg.PollInterval = MaxPollInterval
	s, err := New(cfg, withAbsentPathState(reader), testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	sink := sinkFunc(func(context.Context, source.Batch) error { t.Fatal("empty input committed"); return nil })
	waiting, err := s.runOpened(context.Background(), f, sink)
	if !waiting || err != nil {
		t.Fatalf("empty decision: %v, %v", waiting, err)
	}
	if err := f.Close(); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("descriptor retained during initial wait: %v", err)
	}
	queries = 0
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := s.Run(ctx, sink); !errors.Is(err, context.DeadlineExceeded) || queries != 1 {
		t.Fatalf("initial wait cancellation: %v, queries %d", err, queries)
	}
}

func TestFileSourceAppliesExplicitZeroReplayPolicyToRetiredCurrent(t *testing.T) {
	f, path := testRegularFile(t, "seed\n")
	state := ingestState(t, f, 0)
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Commit(context.Background(), source.Batch{
		Source: fileSourceIdentity(), Origins: []source.Origin{state.Origin}, Checkpoints: []source.Position{*state.Checkpoint},
	}); err != nil {
		t.Fatal(err)
	}
	cfg := sourceConfig(path)
	cfg.ResumePolicy.AllowZeroCheckpoint = true
	s, err := New(cfg, store, testNormalizer)
	if err != nil {
		t.Fatal(err)
	}
	// A lifecycle decision is explicit here: no following set is claimed, so
	// current-file startup can apply its explicit checkpoint-zero policy.
	state.FollowState = source.FollowRetired
	seedAcquisition(t, s, store, state)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	commits := 0
	err = s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if len(b.FollowTransitions) > 0 {
			return store.Commit(ctx, b)
		}
		commits++
		if len(b.Origins) != 0 || len(b.Records) != 1 || b.Records[0].OriginID != state.Origin.ID || b.Records[0].Start != 0 || string(b.Records[0].Raw) != "seed\n" {
			t.Fatalf("zero replay batch: %+v", b)
		}
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		cancel()
		return nil
	}))
	if !errors.Is(err, context.Canceled) || commits != 1 {
		t.Fatalf("explicit replay: %v, commits %d", err, commits)
	}
	position, found, err := store.Checkpoint(context.Background(), s.ID(), state.Origin.ID)
	if err != nil || !found || position.Offset != 5 {
		t.Fatalf("replayed checkpoint: %+v, %v, %v", position, found, err)
	}
}

func TestFileSourceReturnsUnusableDecisionsAndSinkFailure(t *testing.T) {
	for _, kind := range []string{"strict zero", "ambiguous", "limit", "sink failure"} {
		t.Run(kind, func(t *testing.T) {
			f, path := testRegularFile(t, "seed\n")
			zero := ingestState(t, f, 0)
			calls := 0
			reader := originReaderFunc(func(_ context.Context, q source.OriginQuery) (source.OriginPage, error) {
				calls++
				switch kind {
				case "sink failure":
					return source.OriginPage{}, nil
				case "strict zero":
					return source.OriginPage{States: []source.OriginState{zero}}, nil
				case "ambiguous":
					positive := ingestState(t, f, 5)
					other := ingestState(t, f, 5)
					other.Origin.ID, other.Checkpoint.OriginID = "gen-2", "gen-2"
					return source.OriginPage{States: []source.OriginState{positive, other}}, nil
				case "limit":
					states := selectionStates(make([]ResumeStatus, MaxResumeCandidates+1)...)
					for i := range states {
						states[i].Origin.Device, states[i].Origin.Inode = q.Device, q.Inode
					}
					start := (calls - 1) * 100
					end := start + q.Limit
					return source.OriginPage{States: states[start:end], NextID: states[end-1].Origin.ID}, nil
				}
				panic("unknown test")
			})
			s, err := New(sourceConfig(path), withAbsentPathState(reader), testNormalizer)
			if err != nil {
				t.Fatal(err)
			}
			boom := errors.New("sink failed")
			commits := 0
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			err = s.Run(ctx, sinkFunc(func(context.Context, source.Batch) error { commits++; return boom }))
			if kind == "sink failure" {
				if !errors.Is(err, boom) || commits != 1 {
					t.Fatalf("sink failure: %v, commits %d", err, commits)
				}
			} else {
				var decision *ResumeDecisionError
				want := map[string]SelectionStatus{"strict zero": SelectionInsufficient, "ambiguous": SelectionAmbiguous, "limit": SelectionLimit}[kind]
				if !errors.As(err, &decision) || decision.Status != want || commits != 0 {
					t.Fatalf("decision: %v, commits %d", err, commits)
				}
			}
			if err := f.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
				t.Fatal(err)
			}
		})
	}
}
