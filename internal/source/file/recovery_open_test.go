package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestPrepareRecoveryCurrentValidatesBeforeOpening(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-created", "mail.log")
	for _, kind := range []string{"nil state", "relative", "path", "ID", "budget", "status", "state", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			selected := RecoveryOrigin{Status: RecoveryOriginUnknown, Examined: 1, State: &source.OriginState{Origin: source.Origin{ID: "target", Path: path}}}
			inputPath := path
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "nil state":
				selected.State = nil
			case "relative":
				inputPath = "mail.log"
			case "path":
				selected.State.Origin.Path = "other"
			case "ID":
				selected.State.Origin.ID = ""
			case "budget":
				selected.Examined = 0
			case "status":
				selected.Status = RecoveryOriginConflict
			case "state":
				selected.State.FollowState = source.FollowRetired
			case "canceled":
				cancel()
			}
			result, err := prepareRecoveryCurrentWith(ctx, inputPath, selected, func(context.Context, string) (*os.File, Identity, error) {
				t.Fatal("invalid plan opened a journal")
				return nil, Identity{}, nil
			}, ObservePath)
			want := ErrInvalidRecoveryOrigin
			if kind == "canceled" {
				want = context.Canceled
			}
			if result != nil || !errors.Is(err, want) {
				t.Fatal(result, err)
			}
		})
	}
}

func TestPrepareRecoveryCurrentPreservesOpenFailureAndClosesOnCancellation(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		f, path := testRegularFile(t, "seed\n")
		state := ingestState(t, f, 0)
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		selected := RecoveryOrigin{Status: RecoveryOriginUnknown, Examined: 1, State: &state}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		boom := errors.New("synthetic recovery open failure")
		want := error(boom)
		var acquired *os.File
		result, err := prepareRecoveryCurrentWith(ctx, path, selected, func(ctx context.Context, path string) (*os.File, Identity, error) {
			if !canceled {
				return nil, Identity{}, boom
			}
			f, id, err := OpenLog(ctx, path)
			acquired = f
			cancel()
			want = context.Canceled
			return f, id, err
		}, func(context.Context, *os.File, string) (PathObservation, error) {
			t.Fatal("failed open observed path")
			return PathObservation{}, nil
		})
		if result != nil || !errors.Is(err, want) || errors.Is(err, fs.ErrClosed) {
			t.Fatal(result, err)
		}
		if acquired != nil && !errors.Is(acquired.Close(), fs.ErrClosed) {
			t.Fatal("temporary file leaked")
		}
	}
	var empty *openedRecovery
	if empty.Close() != nil || (&openedRecovery{}).Close() != nil {
		t.Fatal("nil/zero owner close")
	}
}
