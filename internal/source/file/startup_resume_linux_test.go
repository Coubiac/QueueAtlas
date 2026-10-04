//go:build linux

package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestRunResumesWholeFollowingSetAndNewCurrent(t *testing.T) {
	for _, kind := range []string{"one known", "two known", "new", "empty new"} {
		t.Run(kind, func(t *testing.T) {
			s, store := resumePreparationFixture(t, kind)
			before := resumeStoredPage(t, s, store)
			if kind == "empty new" {
				if err := os.Truncate(s.config.Path, 0); err != nil {
					t.Fatal(err)
				}
			}
			for _, state := range before.States {
				path := s.config.Path + ".1"
				if state.Origin.ID == "gen-2" {
					path = s.config.Path
				}
				appendNewTestLine(t, path, "late\n")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			records, registrations, acquisitions := 0, 0, 0
			var newID string
			wantRecords := len(before.States)
			if kind == "new" || kind == "empty new" {
				wantRecords++
			}
			err := s.Run(ctx, sinkFunc(func(ctx context.Context, b source.Batch) error {
				for _, origin := range b.Origins {
					newID = origin.ID
					registrations++
				}
				for _, transition := range b.FollowTransitions {
					if transition.OriginID != newID || transition.To != source.FollowFollowing {
						t.Fatal("existing file reacquired or wrong transition", transition)
					}
					acquisitions++
				}
				for _, record := range b.Records {
					records++
					if record.OriginID == newID {
						want := "new\n"
						if kind == "empty new" {
							want = "added\n"
						}
						if acquisitions != 1 || record.Start != 0 || string(record.Raw) != want {
							t.Fatal("new current consumed before acquisition", record)
						}
					} else {
						if record.Start != 4 || string(record.Raw) != "late\n" {
							t.Fatal("existing prefix replayed", record)
						}
						if kind == "empty new" {
							if registrations != 0 || acquisitions != 0 {
								t.Fatal("empty new registered before content")
							}
							appendNewTestLine(t, s.config.Path, "added\n")
						}
					}
				}
				if err := store.Commit(ctx, b); err != nil {
					return err
				}
				if records == wantRecords {
					cancel()
				}
				return nil
			}))
			if !errors.Is(err, context.Canceled) || errors.Is(err, fs.ErrClosed) || records != wantRecords {
				t.Fatal("resumed Run", err, records)
			}
			wantRegistrations := 0
			if kind == "new" || kind == "empty new" {
				wantRegistrations = 1
			}
			if registrations != wantRegistrations || acquisitions != wantRegistrations {
				t.Fatal("repeated registration/acquisition", registrations, acquisitions)
			}
			for _, state := range before.States {
				actual := retirementState(t, s, store, state.Origin.ID)
				if actual.Checkpoint.Offset != 9 || actual.FollowState != source.FollowFollowing {
					t.Fatal("resumed durable state", actual)
				}
			}
			if fileDescriptorCount(t, s.config.Path) != 0 || fileDescriptorCount(t, s.config.Path+".1") != 0 {
				t.Fatal("Run leaked resumed files")
			}
			if !s.running.TryLock() {
				t.Fatal("Run guard leaked")
			}
			s.running.Unlock()
		})
	}
}

func TestRunResumeBlockersDoNotFallBackToCurrent(t *testing.T) {
	for _, kind := range []string{"unknown with explicit zero", "following zero with explicit zero", "missing", "capacity", "origin limit", "entry limit", "changed"} {
		t.Run(kind, func(t *testing.T) {
			fixture := "new"
			if kind == "capacity" || kind == "origin limit" {
				fixture = "two known"
			}
			if kind == "following zero with explicit zero" {
				fixture = "zero checkpoint"
			}
			s, store := resumePreparationFixture(t, fixture)
			s.setPathStatus(PathSame)
			var want error
			var status SelectionStatus
			switch kind {
			case "unknown with explicit zero":
				f, _, err := OpenLog(context.Background(), s.config.Path)
				if err != nil {
					t.Fatal(err)
				}
				state := ingestState(t, f, 0)
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				state.Origin.ID, state.Checkpoint.OriginID = "legacy", "legacy"
				state.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
				seedAcquisition(t, s, store, state)
				s.config.ResumePolicy.AllowZeroCheckpoint = true
				want = ErrUnknownFollowState
			case "following zero with explicit zero":
				s.config.ResumePolicy.AllowZeroCheckpoint = true
				status = SelectionInsufficient
			case "missing":
				if err := os.Remove(s.config.Path); err != nil {
					t.Fatal(err)
				}
				want = ErrCurrentMissing
			case "capacity":
				if err := rotateTo(s.config.Path, ".2", "third\n"); err != nil {
					t.Fatal(err)
				}
				want = ErrRotationCapacity
			case "origin limit":
				s.config.ResumeLimits.Origins = 1
				status = SelectionLimit
			case "entry limit":
				s.config.ResumeLimits.Entries = 1
				status = SelectionLimit
			case "changed":
				if err := os.WriteFile(s.config.Path+".1", []byte("bad\n"), 0600); err != nil {
					t.Fatal(err)
				}
				status = SelectionDifferent
			}
			before := resumeStoredPage(t, s, store)
			err := s.Run(context.Background(), sinkFunc(func(context.Context, source.Batch) error { t.Fatal("blocked restart fell back and wrote"); return nil }))
			if status != "" {
				var decision *ResumeDecisionError
				if !errors.As(err, &decision) || decision.Status != status {
					t.Fatal("Run resume diagnostic", err, status)
				}
			} else if !errors.Is(err, want) {
				t.Fatal("Run lifecycle/current diagnostic", err, want)
			}
			if !reflect.DeepEqual(before, resumeStoredPage(t, s, store)) || s.LastPathStatus() != "" {
				t.Fatal("blocked Run wrote state or retained old status")
			}
			for _, path := range []string{s.config.Path, s.config.Path + ".1", s.config.Path + ".2"} {
				if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
					continue
				}
				if fileDescriptorCount(t, path) != 0 {
					t.Fatal("blocked Run leaked file")
				}
			}
			if !s.running.TryLock() {
				t.Fatal("blocked Run guard leaked")
			}
			s.running.Unlock()
		})
	}
}

func TestRunPreparedResumeCleansRejectionBeforeTransfer(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		s, store := resumePreparationFixture(t, "two known")
		resume, err := PrepareFollowResume(context.Background(), s.config.Identity, s.config.Path, store, s.normalize, s.config.ResumeLimits)
		if err != nil {
			t.Fatal(err)
		}
		generations := append([]*openedGeneration(nil), resume.Opened.opened...)
		ctx, cancel := context.WithCancel(context.Background())
		want := ErrPathChanged
		if canceled {
			cancel()
			want = context.Canceled
		} else {
			if err := rotateTo(s.config.Path, ".2", "replacement\n"); err != nil {
				t.Fatal(err)
			}
		}
		s.running.Lock()
		err = s.runPreparedResume(ctx, resume, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("rejected application committed"); return nil }))
		s.running.Unlock()
		cancel()
		if !errors.Is(err, want) || errors.Is(err, fs.ErrClosed) || resume.Opened.Len() != 0 || resume.Opened.Close() != nil {
			t.Fatal("Run application cleanup", err)
		}
		for _, g := range generations {
			if !errors.Is(g.file.Close(), fs.ErrClosed) {
				t.Fatal("Run validation rejection leaked descriptor")
			}
		}
	}
}
