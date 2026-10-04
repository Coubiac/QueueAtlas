//go:build linux

package file

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
)

func TestStartAtEndBootstrapAppendAndResume(t *testing.T) {
	writer, path := testRegularFile(t, "old\n")
	s, store := rotationSource(t, path)
	s.config.StartAt = StartAtEnd
	normalized := 0
	s.normalize = func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }
	var initial source.Position
	for run := range 2 {
		if run == 1 {
			if _, err := writer.WriteAt([]byte("next\n"), 8); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		registrations, records := 0, 0
		err := s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
			if len(b.Origins) > 0 {
				registrations++
				if run != 0 || len(b.Checkpoints) != 1 || b.Checkpoints[0].Offset != 4 || len(b.Records)+len(b.FollowTransitions) != 0 || normalized != 0 {
					t.Fatal("end registration", b)
				}
				initial = b.Checkpoints[0]
				anchor, err := ParseCheckpointAnchor(initial.AnchorHash)
				if err != nil || anchor.Offset != 4 || anchor.Length != 4 {
					t.Fatal("end anchor", anchor, err)
				}
			}
			if err := store.Commit(ctx, b); err != nil {
				return err
			}
			if len(b.Origins) > 0 {
				_, err := writer.WriteAt([]byte("new\n"), 4)
				return err
			}
			if len(b.Records) > 0 {
				records++
				wantRaw, wantStart := "new\n", int64(4)
				if run == 1 {
					wantRaw, wantStart = "next\n", 8
				}
				if string(b.Records[0].Raw) != wantRaw || b.Records[0].Start != wantStart || b.Records[0].OriginID != initial.OriginID {
					t.Fatal("bootstrap/resume skipped append", b)
				}
				cancel()
			}
			return nil
		}))
		cancel()
		if !errors.Is(err, context.Canceled) || records != 1 || registrations != 1-run || normalized != run+1 {
			t.Fatalf("run %d: %v, registrations %d records %d normalizations %d", run, err, registrations, records, normalized)
		}
	}
	if state := acquiredPathState(t, s, store); state.Checkpoint.Offset != 13 || state.FollowState != source.FollowFollowing {
		t.Fatal(state)
	}
	if fileDescriptorCount(t, path) != 1 {
		t.Fatal("Run leaked descriptor")
	}
}

func TestStartAtEndEmptyThenAppendBeginsAtZero(t *testing.T) {
	writer, path := testRegularFile(t, "")
	s, store := rotationSource(t, path)
	s.config.StartAt = StartAtEnd
	queries := 0
	s.reader = statePathReader{originReaderFunc(func(ctx context.Context, q source.OriginQuery) (source.OriginPage, error) {
		queries++
		// The second physical scan happens only after the empty first attempt
		// closed and waited. Its new bytes must not become an EOF to skip.
		if queries == 2 {
			if _, err := writer.WriteAt([]byte("added\n"), 0); err != nil {
				return source.OriginPage{}, err
			}
		}
		return store.FileOrigins(ctx, q)
	}), store}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	registrations, records := 0, 0
	err := s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if len(b.Origins) > 0 {
			registrations++
			if b.Checkpoints[0].Offset != 0 {
				t.Fatal("empty bootstrap recaptured end", b)
			}
		}
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		if len(b.Records) > 0 {
			records++
			if string(b.Records[0].Raw) != "added\n" || b.Records[0].Start != 0 {
				t.Fatal("empty append", b)
			}
			cancel()
		}
		return nil
	}))
	if !errors.Is(err, context.Canceled) || queries != 2 || registrations != 1 || records != 1 {
		t.Fatalf("empty bootstrap: %v, scans %d registrations %d records %d", err, queries, registrations, records)
	}
}

func TestStartAtEndRefusesPartialAndUnknownWithoutCommit(t *testing.T) {
	for _, kind := range []string{"partial", "unknown"} {
		t.Run(kind, func(t *testing.T) {
			content := "old\npartial"
			if kind == "unknown" {
				content = "old\n"
			}
			writer, path := testRegularFile(t, content)
			s, store := rotationSource(t, path)
			s.config.StartAt = StartAtEnd
			want := error(ErrInitialEndPartial)
			if kind == "unknown" {
				state := ingestState(t, writer, 4)
				state.Origin.FirstSeen = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
				seedAcquisition(t, s, store, state)
				want = ErrUnknownFollowState
			}
			before := resumeStoredPage(t, s, store)
			err := s.Run(context.Background(), sinkFunc(func(context.Context, source.Batch) error { t.Fatal("partial/unknown committed"); return nil }))
			if !errors.Is(err, want) || !reflect.DeepEqual(before, resumeStoredPage(t, s, store)) {
				t.Fatal("refusal/state", err)
			}
			if fileDescriptorCount(t, path) != 1 {
				t.Fatal("refusal leaked descriptor")
			}
		})
	}
}

func TestStartAtEndKeepsHistoryAndRotationAtBeginning(t *testing.T) {
	for _, kind := range []string{"retired path", "different physical", "rotation"} {
		t.Run(kind, func(t *testing.T) {
			writer, path := testRegularFile(t, "old\n")
			s, store := rotationSource(t, path)
			s.config.StartAt = StartAtEnd
			if kind != "rotation" {
				state := ingestState(t, writer, 4)
				state.Origin.FirstSeen = time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
				state.FollowState = source.FollowRetired
				if kind == "different physical" {
					state.Origin.Path = path + ".alias"
				}
				seedAcquisition(t, s, store, state)
				if kind == "retired path" {
					if err := rotateTo(path, ".1", "new\n"); err != nil {
						t.Fatal(err)
					}
				} else if _, err := writer.WriteAt([]byte("new\n"), 0); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			registrations, records := 0, 0
			err := s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
				if len(b.Origins) > 0 {
					registrations++
					wantOffset := int64(0)
					if kind == "rotation" && registrations == 1 {
						wantOffset = 4
					}
					if b.Checkpoints[0].Offset != wantOffset {
						t.Fatal("history/rotation skipped", b)
					}
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				if kind == "rotation" && len(b.Origins) > 0 && registrations == 1 {
					return rotateTo(path, ".1", "new\n")
				}
				if len(b.Records) > 0 {
					records++
					if string(b.Records[0].Raw) != "new\n" || b.Records[0].Start != 0 {
						t.Fatal("not read from beginning", b)
					}
					cancel()
				}
				return nil
			}))
			wantRegistrations := 1
			if kind == "rotation" {
				wantRegistrations = 2
			}
			if !errors.Is(err, context.Canceled) || registrations != wantRegistrations || records != 1 {
				t.Fatalf("%s: %v, registrations %d records %d", kind, err, registrations, records)
			}
		})
	}
}

func TestStartAtEndRegistrationAcknowledgementFailures(t *testing.T) {
	boom := errors.New("synthetic ACK failure")
	for _, kind := range []string{"sink error", "sink EOF", "lost registration ACK", "cancel after registration", "lost acquisition ACK"} {
		t.Run(kind, func(t *testing.T) {
			writer, path := testRegularFile(t, "old\n")
			s, store := rotationSource(t, path)
			s.config.StartAt = StartAtEnd
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			commits := 0
			want, wantCommits := error(boom), 1
			if kind == "sink EOF" {
				want = io.EOF
			}
			if kind == "cancel after registration" {
				want = context.Canceled
			}
			if kind == "lost acquisition ACK" {
				wantCommits = 2
			}
			err := s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
				commits++
				if len(b.Records) > 0 {
					t.Fatal("failed ACK consumed record")
				}
				if kind == "sink error" || kind == "sink EOF" {
					return want
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				if kind == "cancel after registration" {
					cancel()
					return nil
				}
				if kind == "lost acquisition ACK" && commits == 1 {
					return nil
				}
				return boom
			}))
			if !errors.Is(err, want) || commits != wantCommits {
				t.Fatal("ACK error/retry", err, commits)
			}
			page := resumeStoredPage(t, s, store)
			if kind == "sink error" || kind == "sink EOF" {
				if len(page.States) != 0 {
					t.Fatal("failed commit registered origin", page)
				}
				return
			}
			state := acquiredPathState(t, s, store)
			wantFollow := source.FollowUnknown
			if kind == "lost acquisition ACK" {
				wantFollow = source.FollowFollowing
			}
			if state.FollowState != wantFollow || state.Checkpoint.Offset != 4 {
				t.Fatal("durable ACK state", state)
			}
			if wantFollow == source.FollowUnknown {
				if err := s.Run(context.Background(), store); !errors.Is(err, ErrUnknownFollowState) {
					t.Fatal("unknown restarted automatically", err)
				}
				if err := s.RecoverUnknownCurrent(context.Background(), state.Origin.ID, store); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := writer.WriteAt([]byte("new\n"), 4); err != nil {
				t.Fatal(err)
			}
			retryCtx, retryCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer retryCancel()
			err = s.Run(retryCtx, sinkFunc(func(ctx context.Context, b source.Batch) error {
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				if len(b.Records) > 0 {
					if b.Records[0].Start != 4 || string(b.Records[0].Raw) != "new\n" || b.Records[0].OriginID != state.Origin.ID {
						t.Fatal("ACK retry skipped or changed generation", b)
					}
					retryCancel()
				}
				return nil
			}))
			if !errors.Is(err, context.Canceled) {
				t.Fatal("ACK resumed", err)
			}
		})
	}
}
