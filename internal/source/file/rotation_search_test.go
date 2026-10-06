package file

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func rotationEntries(t *testing.T, count int) []os.DirEntry {
	t.Helper()
	directory := t.TempDir()
	for i := 0; i < count; i++ {
		if err := os.WriteFile(filepath.Join(directory, fmt.Sprintf("mail-%03d.log", i)), []byte("synthetic\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func rotationPages(entries []os.DirEntry) func(int) ([]os.DirEntry, error) {
	return func(n int) ([]os.DirEntry, error) {
		if len(entries) == 0 {
			return nil, io.EOF
		}
		end := min(n, len(entries))
		page := entries[:end]
		entries = entries[end:]
		return page, nil
	}
}

func TestRotationScanRequiresExhaustionAndUniqueEvidence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []ResumeStatus // empty status means skipped entry
		limit    int
		want     SelectionStatus
	}{
		{"empty", nil, 1, SelectionAbsent},
		{"skipped", []ResumeStatus{"", ""}, 2, SelectionAbsent},
		{"different", []ResumeStatus{ResumeDifferent}, 1, SelectionDifferent},
		{"unique_at_budget", []ResumeStatus{ResumeDifferent, ResumeMatch, ""}, 3, SelectionUnique},
		{"insufficient", []ResumeStatus{ResumeMatch, ResumeInsufficient}, 2, SelectionInsufficient},
		{"ambiguous", []ResumeStatus{ResumeMatch, ResumeMatch}, 2, SelectionAmbiguous},
		{"limit_discards_match", []ResumeStatus{ResumeMatch, "", ""}, 2, SelectionLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := rotationEntries(t, len(tc.statuses))
			index := 0
			result, err := scanRotation(context.Background(), "rotations", tc.limit, rotationPages(entries), func(os.DirEntry) (ResumeCheck, bool, error) {
				status := tc.statuses[index]
				index++
				return ResumeCheck{Status: status}, status != "", nil
			})
			if err != nil || result.Status != tc.want || result.Examined != min(tc.limit, len(entries)) || index != result.Examined {
				t.Fatalf("scan: %+v, %v, calls %d", result, err, index)
			}
			if tc.want == SelectionUnique {
				if result.Path != filepath.Join("rotations", entries[1].Name()) {
					t.Fatal("wrong unique path", result.Path)
				}
			} else if result.Path != "" {
				t.Fatal("unusable scan exposed selected path", result)
			}
		})
	}
}

func TestRotationScanBoundsPagesAndSkippedEntries(t *testing.T) {
	entries := rotationEntries(t, 34)
	read := rotationPages(entries)
	calls, verified := 0, 0
	result, err := scanRotation(context.Background(), "rotations", 33, func(n int) ([]os.DirEntry, error) {
		calls++
		if n < 1 || n > 32 {
			t.Fatal("unbounded directory read", n)
		}
		return read(n)
	}, func(os.DirEntry) (ResumeCheck, bool, error) {
		verified++
		return ResumeCheck{}, false, nil
	})
	if err != nil || result.Status != SelectionLimit || result.Path != "" || result.Examined != 33 || verified != 33 || calls != 2 {
		t.Fatal("skipped entries escaped budget", result, err, calls, verified)
	}
}

func TestRotationScanErrorsDiscardPartialSelection(t *testing.T) {
	entries := rotationEntries(t, 1)
	boom := errors.New("synthetic directory read failure")
	calls := 0
	result, err := scanRotation(context.Background(), "rotations", 2, func(int) ([]os.DirEntry, error) {
		calls++
		if calls == 1 {
			return entries, nil
		}
		return nil, boom
	}, func(os.DirEntry) (ResumeCheck, bool, error) { return ResumeCheck{Status: ResumeMatch}, true, nil })
	if !errors.Is(err, boom) || result != (RotationSelection{}) || calls != 2 {
		t.Fatal("directory error returned partial selection", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err = scanRotation(ctx, "rotations", 2, rotationPages(entries), func(os.DirEntry) (ResumeCheck, bool, error) {
		cancel()
		return ResumeCheck{Status: ResumeMatch}, true, nil
	})
	if !errors.Is(err, context.Canceled) || result != (RotationSelection{}) {
		t.Fatal("cancelled scan returned partial selection", result, err)
	}
	for _, read := range []func(int) ([]os.DirEntry, error){
		func(int) ([]os.DirEntry, error) { return nil, nil },
		func(int) ([]os.DirEntry, error) { return append(append(entries, entries...), entries...), nil },
	} {
		result, err = scanRotation(context.Background(), "rotations", 1, read, func(os.DirEntry) (ResumeCheck, bool, error) { return ResumeCheck{}, false, boom })
		if err == nil || result != (RotationSelection{}) {
			t.Fatal("invalid page returned selection", result, err)
		}
	}
}

func TestSelectRotationValidatesConfigurationAndDirectory(t *testing.T) {
	directory := t.TempDir()
	for _, tc := range []struct {
		path  string
		limit int
	}{{"", 1}, {directory, 0}, {directory, MaxRotationEntries + 1}} {
		result, err := SelectRotation(context.Background(), tc.path, source.OriginState{}, tc.limit)
		if err == nil || result != (RotationSelection{}) {
			t.Fatal("invalid arguments returned selection", result, err)
		}
	}
	_, path := testRegularFile(t, "synthetic\n")
	if result, err := SelectRotation(context.Background(), path, source.OriginState{}, 1); err == nil || result != (RotationSelection{}) {
		t.Fatal("non-directory returned selection", result, err)
	}
	if result, err := SelectRotation(context.Background(), directory, source.OriginState{}, 1); err != nil || result.Status != SelectionAbsent || result.Examined != 0 || result.Path != "" {
		t.Fatal("empty directory", result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := SelectRotation(ctx, directory, source.OriginState{}, 1); !errors.Is(err, context.Canceled) || result != (RotationSelection{}) {
		t.Fatal("cancelled directory search", result, err)
	}
}
