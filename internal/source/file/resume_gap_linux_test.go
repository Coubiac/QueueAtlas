//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestRunGapRefusesPartialResumeThenRecoversRestoredArchive(t *testing.T) {
	for _, kind := range []string{"one retained missing", "first of two missing", "second of two missing"} {
		t.Run(kind, func(t *testing.T) {
			fixture := "two known"
			if kind == "one retained missing" {
				fixture = "new"
			}
			s, store := resumePreparationFixture(t, fixture)
			missingPath := s.config.Path + ".1"
			if kind == "second of two missing" {
				if err := os.Rename(s.config.Path, s.config.Path+".2"); err != nil {
					t.Fatal(err)
				}
				missingPath = s.config.Path + ".2"
				if err := os.WriteFile(s.config.Path, []byte("replacement\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			before := resumeStoredPage(t, s, store)
			for _, state := range before.States {
				path := s.config.Path + ".1"
				if state.Origin.ID == "gen-2" {
					path = s.config.Path
					if kind == "second of two missing" {
						path = s.config.Path + ".2"
					}
				}
				appendNewTestLine(t, path, "late\n")
			}
			savedPath := filepath.Join(t.TempDir(), "saved-archive.log")
			if err := os.Rename(missingPath, savedPath); err != nil {
				t.Fatal(err)
			}
			normalized, commits := 0, 0
			s.normalize = func(raw []byte) model.Observation { normalized++; return testNormalizer(raw) }
			err := s.Run(context.Background(), sinkFunc(func(context.Context, source.Batch) error { commits++; return nil }))
			var selection *ResumeDecisionError
			if !errors.Is(err, ErrFollowResumeGap) || !errors.As(err, &selection) || selection.Status != SelectionDifferent || errors.Is(err, ErrCurrentMissing) || normalized != 0 || commits != 0 {
				t.Fatal("missing archive started partial/current ingestion", err, normalized, commits)
			}
			if !reflect.DeepEqual(before, resumeStoredPage(t, s, store)) {
				t.Fatal("gap reset checkpoint or retired missing generation")
			}
			for _, path := range []string{s.config.Path, s.config.Path + ".1", s.config.Path + ".2", savedPath} {
				if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if fileDescriptorCount(t, path) != 0 {
					t.Fatal("gap leaked descriptor")
				}
			}
			if !s.running.TryLock() {
				t.Fatal("gap leaked guard")
			}
			s.running.Unlock()
			if err := os.Rename(savedPath, missingPath); err != nil {
				t.Fatal(err)
			}
			// Restore the tracked current as well; a third physical current would
			// correctly keep two following generations at capacity.
			if kind == "second of two missing" {
				if err := os.Remove(s.config.Path); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(s.config.Path+".2", s.config.Path); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var records []source.Record
			wantRecords := len(before.States)
			if fixture == "new" {
				wantRecords++
			}
			err = s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
				for _, r := range b.Records {
					if r.OriginID == "gen-1" || r.OriginID == "gen-2" {
						if r.Start != 4 || string(r.Raw) != "late\n" {
							t.Fatal("restored archive replayed or skipped", r)
						}
					} else if fixture != "new" || r.Start != 0 || string(r.Raw) != "new\n" {
						t.Fatal("unexpected new current", r)
					}
					records = append(records, r)
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				if len(records) == wantRecords {
					cancel()
				}
				return nil
			}))
			if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || len(records) != wantRecords {
				t.Fatal("restored archive failed to resume", err, records)
			}
			for _, state := range before.States {
				actual := retirementState(t, s, store, state.Origin.ID)
				if actual.Checkpoint.Offset != 9 || actual.FollowState != source.FollowFollowing || !reflect.DeepEqual(actual.Origin, state.Origin) {
					t.Fatal("restoration changed provenance or failed checkpoint", actual)
				}
			}
			if fileDescriptorCount(t, s.config.Path) != 0 || fileDescriptorCount(t, s.config.Path+".1") != 0 {
				t.Fatal("restored resume leaked files")
			}
		})
	}
}

func TestGapDifferentFromNoHistoryAndCurrentMissing(t *testing.T) {
	// No lifecycle history allows normal fresh startup. A verified retained file
	// with missing current
	// gives ErrCurrentMissing; an unavailable retained file gives the gap.
	s, store := resumePreparationFixture(t, "new")
	if err := os.Remove(s.config.Path); err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), store); !errors.Is(err, ErrCurrentMissing) || errors.Is(err, ErrFollowResumeGap) {
		t.Fatal("missing current mislabeled as gap", err)
	}
	if err := os.Remove(s.config.Path + ".1"); err != nil {
		t.Fatal(err)
	}
	err := s.Run(context.Background(), store)
	var decision *ResumeDecisionError
	if !errors.Is(err, ErrFollowResumeGap) || !errors.As(err, &decision) || decision.Status != SelectionAbsent {
		t.Fatal("empty directory did not diagnose persisted generation gap", err)
	}
	_, freshPath := testRegularFile(t, "fresh\n")
	fresh, freshStore := rotationSource(t, freshPath)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	records := 0
	err = fresh.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
		if err := freshStore.Commit(ctx, b); err != nil {
			return err
		}
		records += len(b.Records)
		if records == 1 {
			cancel()
		}
		return nil
	}))
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrFollowResumeGap) || records != 1 {
		t.Fatal("source without history blocked as gap", err, records)
	}
}
