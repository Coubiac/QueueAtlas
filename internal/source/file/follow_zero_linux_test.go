//go:build linux

package file

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestRunZeroReplayAfterDurableAcquisitionBeforeFirstRecord(t *testing.T) {
	f, path := testRegularFile(t, "seed\n")
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	s, store := rotationSource(t, path)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	acquisitions := 0
	err := s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if len(b.Records) != 0 {
			t.Fatal("first line consumed before stop")
		}
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		if len(b.FollowTransitions) != 0 {
			acquisitions++
			cancel()
		}
		return nil
	}))
	if !errors.Is(err, context.Canceled) || acquisitions != 1 {
		t.Fatal("initial acquisition", err, acquisitions)
	}
	before := acquiredPathState(t, s, store)
	if before.FollowState != source.FollowFollowing || before.Checkpoint.Offset != 0 {
		t.Fatal("expected durable zero following", before)
	}
	err = s.Run(context.Background(), sinkFunc(func(context.Context, source.Batch) error { t.Fatal("strict zero wrote"); return nil }))
	var decision *ResumeDecisionError
	if !errors.As(err, &decision) || decision.Status != SelectionInsufficient {
		t.Fatal("strict zero", err)
	}
	if !reflect.DeepEqual(before, acquiredPathState(t, s, store)) {
		t.Fatal("strict refusal changed state")
	}
	s.config.ResumePolicy.AllowZeroCheckpoint = true
	ctx, cancelReplay := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelReplay()
	records := 0
	err = s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if len(b.Origins)+len(b.FollowTransitions) != 0 || len(b.Records) != 1 {
			t.Fatal("repeated registration/acquisition", b)
		}
		r := b.Records[0]
		if r.OriginID != before.Origin.ID || r.Start != 0 || r.End != 5 || string(r.Raw) != "seed\n" {
			t.Fatal("zero replay record", r)
		}
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		records++
		cancelReplay()
		return nil
	}))
	after := acquiredPathState(t, s, store)
	if !errors.Is(err, context.Canceled) || records != 1 || after.Checkpoint.Offset != 5 || after.Origin != before.Origin || after.FollowState != before.FollowState {
		t.Fatal("replay changed provenance or failed ack", err, after)
	}
	// After the positive acknowledgement, ordinary strict resume reads only append.
	s.config.ResumePolicy.AllowZeroCheckpoint = false
	appendNewTestLine(t, path, "added\n")
	ctx, cancelNext := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelNext()
	err = s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if len(b.Origins)+len(b.FollowTransitions) != 0 || len(b.Records) != 1 || b.Records[0].Start != 5 || string(b.Records[0].Raw) != "added\n" {
			t.Fatal("positive resume replayed", b)
		}
		if err := store.Commit(ctx, b); err != nil {
			return err
		}
		cancelNext()
		return nil
	}))
	if !errors.Is(err, context.Canceled) || acquiredPathState(t, s, store).Checkpoint.Offset != 11 || fileDescriptorCount(t, path) != 0 {
		t.Fatal("positive resume or cleanup", err)
	}
}

func TestZeroFollowingPreparationRejectsIncompleteOrDifferentEvidence(t *testing.T) {
	for _, kind := range []string{"nil checkpoint", "invalid anchor", "empty prefix", "unknown", "multiple", "prefix changed", "replaced", "missing"} {
		t.Run(kind, func(t *testing.T) {
			f, path := testRegularFile(t, "seed\n")
			state := ingestState(t, f, 0)
			state.FollowState = source.FollowFollowing
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			states := []source.OriginState{state}
			status := SelectionInsufficient
			var want error
			switch kind {
			case "nil checkpoint":
				states[0].Checkpoint = nil
			case "invalid anchor":
				states[0].Checkpoint.AnchorHash = "invalid"
			case "empty prefix":
				states[0].Origin.Fingerprint = (PrefixFingerprint{Digest: sha256.Sum256(nil)}).String()
			case "unknown":
				states[0].FollowState = source.FollowUnknown
				want = ErrUnknownFollowState
			case "multiple":
				other := state
				cp := *state.Checkpoint
				other.Origin.ID, cp.OriginID = "gen-2", "gen-2"
				other.Checkpoint = &cp
				states = append(states, other)
			case "prefix changed":
				if err := os.WriteFile(path, []byte("else\n"), 0600); err != nil {
					t.Fatal(err)
				}
				status = SelectionDifferent
			case "replaced":
				if err := rotateTo(path, ".1", "seed\n"); err != nil {
					t.Fatal(err)
				}
				status = SelectionDifferent
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				want = ErrCurrentMissing
			}
			reader := pathReaderFunc(func(context.Context, source.OriginPathQuery) (source.OriginPage, error) {
				return source.OriginPage{States: states}, nil
			})
			result, err := PrepareFollowResumeWithPolicy(context.Background(), fileSourceIdentity(), path, reader, testNormalizer, FollowResumeLimits{Origins: 2, Entries: 10}, ResumePolicy{AllowZeroCheckpoint: true})
			if result != (FollowResume{}) {
				t.Fatal("refusal returned partial owner", result)
			}
			if want != nil {
				if !errors.Is(err, want) {
					t.Fatal("lifecycle/current diagnostic", err, want)
				}
			} else {
				var decision *ResumeDecisionError
				if !errors.As(err, &decision) || decision.Status != status || errors.Is(err, ErrFollowResumeGap) {
					t.Fatal("zero refusal", err, status)
				}
			}
			if kind != "missing" && fileDescriptorCount(t, path) != 0 {
				t.Fatal("refusal leaked descriptor")
			}
		})
	}
}

func TestZeroFollowingPreparationAndTransferRecheckWithoutConsumption(t *testing.T) {
	for _, kind := range []string{"ready", "rewrite before transfer", "replaced observation", "cancel observation"} {
		t.Run(kind, func(t *testing.T) {
			f, path := testRegularFile(t, "seed\n")
			state := ingestState(t, f, 0)
			state.FollowState = source.FollowFollowing
			state.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			s, store := rotationSource(t, path)
			seedAcquisition(t, s, store, state)
			before := resumeStoredPage(t, s, store)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var captured *os.File
			result, err := prepareFollowResumeWithPolicy(ctx, s.config.Identity, path, store, testNormalizer, s.config.ResumeLimits, ResumePolicy{AllowZeroCheckpoint: true}, func(ctx context.Context, opened *OpenedFollowSet, path string) (FollowCurrent, error) {
				captured = opened.opened[0].file
				if kind == "replaced observation" {
					if err := rotateTo(path, ".1", "seed\n"); err != nil {
						return FollowCurrent{}, err
					}
				}
				current, err := opened.ObserveCurrent(ctx, path)
				if kind == "cancel observation" {
					cancel()
				}
				return current, err
			})
			if kind == "replaced observation" || kind == "cancel observation" {
				want := ErrPathChanged
				if kind == "cancel observation" {
					want = context.Canceled
				}
				if !errors.Is(err, want) || result != (FollowResume{}) {
					t.Fatal("observation refusal", result, err)
				}
				if err := captured.Close(); !errors.Is(err, fs.ErrClosed) {
					t.Fatal("failed preparation leaked", err)
				}
			} else {
				if err != nil || result.Status != FollowResumeReady || result.Current.Status != FollowCurrentKnown || result.Opened.Len() != 1 {
					t.Fatal("ready zero", result, err)
				}
				defer result.Opened.Close()
				g := result.Opened.opened[0]
				if g.ingestor.Position().Offset != 0 || g.ingestor.lines.offset != 0 || g.ingestor.pending != nil {
					t.Fatal("preparation consumed")
				}
				if kind == "rewrite before transfer" {
					if err := os.WriteFile(path, []byte("else\n"), 0600); err != nil {
						t.Fatal(err)
					}
					err := s.FollowOpened(ctx, result.Opened, result.Current, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("rewrite was consumed"); return nil }))
					var decision *ResumeDecisionError
					if !errors.As(err, &decision) || decision.Status != SelectionDifferent || result.Opened.Len() != 1 || g.ingestor.Position().Offset != 0 || g.ingestor.lines.offset != 0 {
						t.Fatal("rewrite lost owner or consumed", err)
					}
				}
				if err := result.Opened.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(before, resumeStoredPage(t, s, store)) || fileDescriptorCount(t, path) != 0 {
				t.Fatal("preparation changed state or leaked")
			}
		})
	}
}
