//go:build linux

package file

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
)

func recoveryOpenFixture(t *testing.T, offset int64) (string, RecoveryOrigin) {
	t.Helper()
	f, path := testRegularFile(t, "seed\n")
	state := ingestState(t, f, offset)
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path, RecoveryOrigin{Status: RecoveryOriginUnknown, Examined: 1, State: &state}
}

func TestPrepareRecoveryCurrentOwnsProofWithoutSeekingOrMutation(t *testing.T) {
	for _, offset := range []int64{0, 5} {
		for _, following := range []bool{false, true} {
			path, selected := recoveryOpenFixture(t, offset)
			if following {
				selected.Status, selected.State.FollowState = RecoveryOriginFollowing, source.FollowFollowing
			}
			before := *selected.State.Checkpoint
			owned, err := prepareRecoveryCurrentWith(context.Background(), path, selected, func(ctx context.Context, path string) (*os.File, Identity, error) {
				// The proof must use the copy captured before opening.
				selected.State.Checkpoint.Offset = 999
				return OpenLog(ctx, path)
			}, ObservePath)
			if err != nil || owned == nil || *owned.origin.State.Checkpoint != before || owned.origin.Status != selected.Status {
				t.Fatal("proof/copy", owned, err)
			}
			f := owned.file
			if position, err := f.Seek(0, io.SeekCurrent); err != nil || position != 0 || fileDescriptorCount(t, path) != 1 {
				t.Fatal("proof consumed/sought/leaked", position, err)
			}
			if err := owned.Close(); err != nil || owned.Close() != nil || !errors.Is(f.Close(), fs.ErrClosed) || fileDescriptorCount(t, path) != 0 {
				t.Fatal("cleanup", err)
			}
		}
	}
}

func TestPrepareRecoveryCurrentProofAndObservationRefusalsCleanUp(t *testing.T) {
	for _, kind := range []string{"nil checkpoint", "invalid anchor", "empty prefix", "changed prefix", "truncated", "replaced", "missing", "observation error", "observation cancel", "observation missing", "observation replaced", "cleanup error"} {
		t.Run(kind, func(t *testing.T) {
			path, selected := recoveryOpenFixture(t, 5)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			boom := errors.New("synthetic recovery observation failure")
			var want error
			status := SelectionInsufficient
			switch kind {
			case "nil checkpoint":
				selected.State.Checkpoint = nil
			case "invalid anchor":
				selected.State.Checkpoint.AnchorHash = "invalid"
			case "empty prefix":
				selected.State.Origin.Fingerprint = (PrefixFingerprint{Digest: sha256.Sum256(nil)}).String()
			case "changed prefix":
				if err := os.WriteFile(path, []byte("else\n"), 0600); err != nil {
					t.Fatal(err)
				}
				status = SelectionDifferent
			case "truncated":
				if err := os.Truncate(path, 2); err != nil {
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
			case "observation error", "cleanup error":
				want = boom
			case "observation cancel":
				want = context.Canceled
			case "observation missing":
				want = ErrCurrentMissing
			case "observation replaced":
				want = ErrPathChanged
			}
			var captured *os.File
			result, err := prepareRecoveryCurrentWith(ctx, path, selected, func(ctx context.Context, path string) (*os.File, Identity, error) {
				f, id, err := OpenLog(ctx, path)
				captured = f
				return f, id, err
			}, func(ctx context.Context, f *os.File, path string) (PathObservation, error) {
				switch kind {
				case "observation error":
					return PathObservation{}, boom
				case "cleanup error":
					if err := f.Close(); err != nil {
						t.Fatal(err)
					}
					return PathObservation{}, boom
				case "observation missing":
					return PathObservation{Status: PathMissing}, nil
				case "observation replaced":
					return PathObservation{Status: PathReplaced}, nil
				}
				current, err := ObservePath(ctx, f, path)
				if kind == "observation cancel" {
					cancel()
				}
				return current, err
			})
			if result != nil || err == nil {
				t.Fatal("refusal returned owner", result, err)
			}
			if want != nil {
				if !errors.Is(err, want) {
					t.Fatal("lost cause", err)
				}
			} else {
				var decision *ResumeDecisionError
				if !errors.As(err, &decision) || decision.Status != status {
					t.Fatal("proof diagnostic", err)
				}
			}
			if kind == "missing" && !errors.Is(err, fs.ErrNotExist) || kind == "cleanup error" && !errors.Is(err, fs.ErrClosed) {
				t.Fatal("lost joined cause", err)
			}
			if captured != nil && !errors.Is(captured.Close(), fs.ErrClosed) {
				t.Fatal("descriptor leaked")
			}
			if kind != "missing" && fileDescriptorCount(t, path) != 0 {
				t.Fatal("descriptor count")
			}
		})
	}
}
