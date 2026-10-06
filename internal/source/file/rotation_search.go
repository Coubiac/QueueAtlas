package file

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

const MaxRotationEntries = 1000

type RotationSelection struct {
	Status   SelectionStatus
	Path     string // populated only for SelectionUnique; no descriptor is retained
	Examined int    // all directory entries, including skipped types and gzip
}

// SelectRotation searches one directory for a supplied origin/checkpoint in the
// caller's source namespace. limit (1..MaxRotationEntries) bounds all entries,
// not just regular files. Only an exhausted scan with one matching path and no
// insufficient candidate is usable. Multiple hard links are ambiguous paths.
// The strict VerifyCandidate policy applies, including insufficient zero anchors.
//
// Observed symlinks, nonregular files, .gz names and gzip signatures are skipped;
// no recursion or decompression is performed. Directory pages, metadata and
// bounded fingerprints are not an atomic snapshot. Errors discard selection.
// Context is checked between filesystem calls, not inside blocking syscalls.
// At most one candidate descriptor is open alongside the directory descriptor.
// Both are closed before return. No persisted state or ingestion is changed.
// Windows uses temporary metadata handles to pin identities before data open;
// fallback directory entries prove identity only when their IDs are loaded.
// A caller must reopen and reverify the selected path before using it.
func SelectRotation(ctx context.Context, directory string, state source.OriginState, limit int) (result RotationSelection, err error) {
	if err := ctx.Err(); err != nil {
		return RotationSelection{}, err
	}
	if directory == "" || limit < 1 || limit > MaxRotationEntries {
		return RotationSelection{}, errors.New("rotation directory and bounded entry limit are required")
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		return RotationSelection{}, err
	}
	before, err := statPath(directory)
	if err != nil {
		return RotationSelection{}, err
	}
	if !before.IsDir() {
		return RotationSelection{}, errors.New("rotation search path is not a directory")
	}
	if err := ctx.Err(); err != nil {
		return RotationSelection{}, err
	}
	dir, err := openReadOnly(directory)
	if err != nil {
		return RotationSelection{}, err
	}
	defer func() {
		err = errors.Join(err, dir.Close())
		if err != nil {
			result = RotationSelection{}
		}
	}()
	info, err := dir.Stat()
	if err != nil {
		return RotationSelection{}, err
	}
	if !info.IsDir() {
		return RotationSelection{}, errors.New("rotation search path is not a directory")
	}
	if !os.SameFile(before, info) {
		return RotationSelection{}, ErrPathChanged
	}
	return scanRotation(ctx, directory, limit, dir.ReadDir, func(entry os.DirEntry) (ResumeCheck, bool, error) {
		return checkRotationEntry(ctx, filepath.Join(directory, entry.Name()), entry, state)
	})
}

func checkRotationEntry(ctx context.Context, path string, entry os.DirEntry, state source.OriginState) (check ResumeCheck, eligible bool, err error) {
	if !entry.Type().IsRegular() || strings.EqualFold(filepath.Ext(entry.Name()), ".gz") {
		return ResumeCheck{}, false, nil
	}
	info, err := rotationEntryInfo(ctx, path, entry)
	if err != nil {
		return ResumeCheck{}, false, err
	}
	if !info.Mode().IsRegular() {
		return ResumeCheck{}, false, nil
	}
	f, id, err := OpenLog(ctx, path)
	if err != nil {
		return ResumeCheck{}, false, err
	}
	defer func() {
		err = errors.Join(err, f.Close())
		if err != nil {
			check, eligible = ResumeCheck{}, false
		}
	}()
	if !id.SameFile(Identity{info: info}) {
		return ResumeCheck{}, false, ErrPathChanged
	}
	var magic [2]byte
	n, err := f.ReadAt(magic[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return ResumeCheck{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return ResumeCheck{}, false, err
	}
	if n == len(magic) && magic == [2]byte{0x1f, 0x8b} {
		return ResumeCheck{}, false, nil
	}
	check, err = VerifyCandidate(f, state)
	return check, true, err
}

func scanRotation(ctx context.Context, directory string, limit int, read func(int) ([]os.DirEntry, error), verify func(os.DirEntry) (ResumeCheck, bool, error)) (RotationSelection, error) {
	result := RotationSelection{}
	matches, candidates, insufficient := 0, 0, false
	selected := ""
	for {
		if err := ctx.Err(); err != nil {
			return RotationSelection{}, err
		}
		// One extra entry distinguishes an exhausted scan at the exact budget.
		requested := min(32, limit-result.Examined+1)
		entries, err := read(requested)
		if err != nil && !errors.Is(err, io.EOF) {
			return RotationSelection{}, err
		}
		if len(entries) > requested || len(entries) == 0 && err == nil {
			return RotationSelection{}, errors.New("invalid rotation directory page")
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return RotationSelection{}, err
			}
			if result.Examined == limit {
				result.Status = SelectionLimit
				return result, nil
			}
			result.Examined++
			check, eligible, err := verify(entry)
			if err != nil {
				return RotationSelection{}, err
			}
			if eligible {
				candidates++
				switch check.Status {
				case ResumeMatch:
					matches++
					selected = filepath.Join(directory, entry.Name())
				case ResumeInsufficient:
					insufficient = true
				case ResumeDifferent:
				default:
					return RotationSelection{}, errors.New("invalid rotation candidate status")
				}
			}
		}
		if err := ctx.Err(); err != nil {
			return RotationSelection{}, err
		}
		if matches > 1 {
			result.Status = SelectionAmbiguous
			return result, nil
		}
		if errors.Is(err, io.EOF) {
			switch {
			case insufficient:
				result.Status = SelectionInsufficient
			case matches == 1:
				result.Status, result.Path = SelectionUnique, selected
			case candidates == 0:
				result.Status = SelectionAbsent
			default:
				result.Status = SelectionDifferent
			}
			return result, nil
		}
	}
}
