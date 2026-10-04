package file

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestFollowNewRefusalsPreserveOwnerAndDoNotWrite(t *testing.T) {
	for _, kind := range []string{"capacity", "stale", "open error", "open mismatch", "cancel after open", "source mismatch", "old rewritten", "old truncated"} {
		t.Run(kind, func(t *testing.T) {
			count := 1
			if kind == "capacity" {
				count = 2
			}
			set, paths, normalized := currentSetFixture(t, count)
			other, path := testRegularFile(t, "new\n")
			id, err := Inspect(other)
			if err != nil {
				t.Fatal(err)
			}
			if err := other.Close(); err != nil {
				t.Fatal(err)
			}
			s := transferSource(t, path)
			s.setPathStatus(PathMissing)
			current := FollowCurrent{Status: FollowCurrentNew, Current: id}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			boom := errors.New("synthetic open failure")
			want := error(boom)
			wantOpens := 0
			switch kind {
			case "capacity":
				want = ErrRotationCapacity
			case "stale":
				current.Current = Identity{}
				want = ErrPathChanged
			case "open error":
				wantOpens = 1
			case "open mismatch":
				want = ErrPathChanged
				wantOpens = 1
			case "cancel after open":
				want = context.Canceled
				wantOpens = 1
			case "source mismatch":
				set.opened[0].ingestor.identity.ID = "other"
				want = ErrInvalidOpenedFollowSet
			case "old rewritten":
				_, err = set.opened[0].file.WriteAt([]byte("edit\n"), 0)
				want = ErrCheckpointChanged
			case "old truncated":
				err = set.opened[0].file.Truncate(2)
				want = ErrFileTruncated
			}
			if err != nil {
				t.Fatal(err)
			}
			opens := 0
			var acquired *os.File
			err = s.followOpenedWithOpener(ctx, set, current, sinkFunc(func(context.Context, source.Batch) error { t.Fatal("write before transfer"); return nil }), waitForPoll, time.Now, func(ctx context.Context, path string) (*os.File, Identity, error) {
				opens++
				if kind == "open error" {
					return nil, Identity{}, boom
				}
				if kind == "open mismatch" {
					path = paths[0]
				}
				f, actual, err := OpenLog(ctx, path)
				acquired = f
				if kind == "cancel after open" {
					cancel()
				}
				return f, actual, err
			})
			if !errors.Is(err, want) || errors.Is(err, fs.ErrClosed) || opens != wantOpens {
				t.Fatal("rejection/open count", err, opens, wantOpens)
			}
			if acquired != nil && !errors.Is(acquired.Close(), fs.ErrClosed) {
				t.Fatal("temporary descriptor leaked")
			}
			assertCurrentSetUnchanged(t, set, count, normalized)
			if s.LastPathStatus() != PathMissing {
				t.Fatal("rejection reset status")
			}
		})
	}
}
