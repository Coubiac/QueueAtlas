//go:build linux

package file

import (
	"context"
	"io"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestSelectResumeWithRealFileEvidence(t *testing.T) {
	for _, kind := range []string{"unique", "ambiguous", "insufficient"} {
		t.Run(kind, func(t *testing.T) {
			f, state := resumeFixture(t)
			if _, err := f.Seek(3, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			calls := 0
			reader := originReaderFunc(func(_ context.Context, q source.OriginQuery) (source.OriginPage, error) {
				calls++
				if q.SourceID != "source-1" || q.Device != state.Origin.Device || q.Inode != state.Origin.Inode {
					t.Fatalf("wrong namespace: %+v", q)
				}
				if calls == 1 {
					return source.OriginPage{States: []source.OriginState{state}, NextID: state.Origin.ID}, nil
				}
				other := state
				other.Origin.ID = "gen-2"
				position := *state.Checkpoint
				other.Checkpoint = &position
				other.Checkpoint.OriginID = other.Origin.ID
				switch kind {
				case "unique":
					other.Origin.Fingerprint = "sha256:1:0000000000000000000000000000000000000000000000000000000000000000"
				case "insufficient":
					other.Checkpoint = nil
				}
				return source.OriginPage{States: []source.OriginState{other}}, nil
			})
			got, err := SelectResume(context.Background(), f, "source-1", reader)
			if err != nil || string(got.Status) != kind || got.Examined != 2 {
				t.Fatalf("selection: %+v, %v", got, err)
			}
			if kind == "unique" && (got.Candidate == nil || got.Candidate.Origin.ID != state.Origin.ID) {
				t.Fatal("wrong candidate")
			}
			if pos, err := f.Seek(0, io.SeekCurrent); err != nil || pos != 3 {
				t.Fatalf("read position %d, %v", pos, err)
			}
		})
	}
}
