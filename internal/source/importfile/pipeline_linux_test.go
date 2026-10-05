//go:build linux

package importfile

import (
	"context"
	"errors"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/source/file"
)

func TestImportPipelineFileSourceOverlapRetainsSeparateProvenance(t *testing.T) {
	s, db := pipelineStore(t)
	path := inputFixture(t, []byte(pipelinePayload))
	temp := t.TempDir()
	identity := source.Identity{ID: "archive", Kind: "import", Name: "synthetic archive", TrustedHost: "trusted-mx"}
	calls := 0
	normalize := pipelineNormalizer(0, &calls)
	archive, err := NewAttempt(AttemptConfig{Identity: identity, RunID: 101, Path: path, Prepare: PrepareOptions{Limits: unlimitedFixtureLimits(), TempDir: temp}}, s, normalize)
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.Run(attemptContext(t), s); err != nil {
		t.Fatal(err)
	}
	liveID := source.Identity{ID: "live", Kind: "file", Name: "synthetic live log", TrustedHost: "trusted-mx"}
	live, err := file.New(file.Config{Identity: liveID, Path: path}, s, normalize)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(attemptContext(t))
	defer cancel()
	committed := 0
	sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
		err := s.Commit(ctx, b)
		if err == nil {
			committed += len(b.Records)
			if committed == 4 {
				cancel()
			}
		}
		return err
	})
	if err := live.Run(ctx, sink); !errors.Is(err, context.Canceled) || committed != 4 {
		t.Fatal(err, committed)
	}
	facts := pipelineFacts(t, db)
	if len(facts) != 8 || calls != 8 {
		t.Fatal("cross-source text equality silently deduplicated observations", len(facts), calls)
	}
	for i := 0; i < 4; i++ {
		archiveFact, liveFact := facts[i], facts[i+4]
		if archiveFact.SourceID != "archive" || liveFact.SourceID != "live" || archiveFact.OriginID == liveFact.OriginID ||
			archiveFact.Raw != liveFact.Raw || archiveFact.Start != liveFact.Start || archiveFact.End != liveFact.End || archiveFact.Instance != liveFact.Instance {
			t.Fatal("overlap proof invented or provenance lost", archiveFact, liveFact)
		}
	}
	assertAttemptCleanup(t, temp)
}
