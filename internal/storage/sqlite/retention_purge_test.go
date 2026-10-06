package sqlite

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

func retireRetentionOrigin(t *testing.T, s *Store, identity source.Identity, origin string) {
	t.Helper()
	for _, transition := range []source.FollowTransition{
		{OriginID: origin, From: source.FollowUnknown, To: source.FollowFollowing},
		{OriginID: origin, From: source.FollowFollowing, To: source.FollowRetired},
	} {
		if err := s.Commit(context.Background(), source.Batch{Source: identity, FollowTransitions: []source.FollowTransition{transition}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRetentionPurgeBoundsInvalidatesAllAffectedRevisionsAndPreservesOtherScopes(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	dated := storeCorrelationCorpus(t, s, "13-reused-queue-id", "dated", "trusted", true, false)
	foreign := storeCorrelationCorpus(t, s, "13-reused-queue-id", "foreign", "other", true, false)
	for _, identity := range []source.Identity{
		{ID: "dated", Kind: "file", Name: "synthetic", TrustedHost: "trusted"},
		{ID: "foreign", Kind: "file", Name: "synthetic", TrustedHost: "other"},
	} {
		retireRetentionOrigin(t, s, identity, identity.ID+"-13-reused-queue-id")
	}
	scope := queueScope(dated)
	facts, err := s.CorrelationFacts(ctx, scope, 30)
	if err != nil {
		t.Fatal(err)
	}
	for i, window := range []time.Duration{time.Minute, 2 * time.Minute} {
		if i == 1 {
			storeCorrelationCorpus(t, s, "13-reused-queue-id", "undated", "trusted", false, false)
			retireRetentionOrigin(t, s, source.Identity{ID: "undated", Kind: "file", Name: "synthetic", TrustedHost: "trusted"}, "undated-13-reused-queue-id")
			facts, err = s.CorrelationFacts(ctx, scope, 30)
			if err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.InstallProjection(ctx, scope, facts, 30, correlation.LinkOptions{Window: window}); err != nil {
			t.Fatal(err)
		}
	}
	combined := queueScope(append(dated, foreign...))
	facts, err = s.CorrelationFacts(ctx, combined, 30)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallProjection(ctx, combined, facts, 30, correlation.LinkOptions{Window: time.Minute}); err != nil {
		t.Fatal(err)
	}
	foreignScope := queueScope(foreign)
	facts, err = s.CorrelationFacts(ctx, foreignScope, 30)
	if err != nil {
		t.Fatal(err)
	}
	wantForeign, err := s.InstallProjection(ctx, foreignScope, facts, 30, correlation.LinkOptions{Window: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	// InstallProjection replaces old revisions. Seed a permitted historical
	// manifest explicitly to verify retention handles every referencing revision.
	var currentID, historicalID int64
	if err := s.db.QueryRow(`SELECT id FROM projection_revisions WHERE fact_count=16`).Scan(&currentID); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow(`INSERT INTO projection_revisions(id,scope_id,format_version,revision,input_revision,link_window_ns,fact_count,created_at_ns)
		SELECT (SELECT max(id)+1 FROM projection_revisions),scope_id,format_version,printf('%064d',0),input_revision,link_window_ns,fact_count,created_at_ns
		FROM projection_revisions WHERE id=? RETURNING id`, currentID).Scan(&historicalID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO projection_revision_facts SELECT ?,raw_record_id FROM projection_revision_facts WHERE revision_id=?`, historicalID, currentID); err != nil {
		t.Fatal(err)
	}
	if count(t, s, "projection_revisions") != 4 {
		t.Fatal("fixture lacks distinct historical/current revisions")
	}
	cp, found, err := s.Checkpoint(ctx, "dated", "dated-13-reused-queue-id")
	if err != nil || !found {
		t.Fatal("missing checkpoint", err)
	}
	query := RetentionQuery{Instance: "trusted", Before: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC), Limit: 2}
	first, err := s.PurgeRetention(ctx, query)
	if err != nil || first != (PurgeResult{Deleted: 2, InvalidatedRevisions: 3, More: true}) {
		t.Fatal("first purge", first, err)
	}
	for _, affected := range []CorrelationScope{scope, combined} {
		if _, found, err := s.CurrentProjection(ctx, affected, 30); err != nil || found {
			t.Fatal("affected projection survived", err)
		}
	}
	got, found, err := s.CurrentProjection(ctx, foreignScope, 30)
	if err != nil || !found || !reflect.DeepEqual(got, wantForeign) {
		t.Fatal("unrelated projection changed", err)
	}
	second, err := s.PurgeRetention(ctx, query)
	if err != nil || second != (PurgeResult{Deleted: 2}) {
		t.Fatal("second purge", second, err)
	}
	third, err := s.PurgeRetention(ctx, query)
	if err != nil || third != (PurgeResult{}) {
		t.Fatal("empty purge", third, err)
	}
	if count(t, s, "raw_records") != 20 || count(t, s, "events") != 20 || count(t, s, "event_search_domains") != 20 || count(t, s, "purged_records") != 4 || count(t, s, "projection_revisions") != 1 {
		t.Fatal("purge exceeded cutoff/instance or missed cascade")
	}
	after, found, err := s.Checkpoint(ctx, "dated", "dated-13-reused-queue-id")
	if err != nil || !found || after != cp {
		t.Fatal("checkpoint changed", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if count(t, s, "purged_records") != 4 || count(t, s, "raw_records") != 20 {
		t.Fatal("reopen changed purge")
	}
}

func TestRetentionPurgeRequiresRetiredFileAndRevalidatesPreview(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	query := RetentionQuery{Instance: "mx-a", Before: b.Records[0].ReadAt.Add(time.Hour), Limit: 1}
	preview, err := s.PreviewRetention(ctx, query)
	if err != nil || len(preview.Candidates) != 1 {
		t.Fatal("preview", err)
	}
	for _, state := range []source.FollowState{source.FollowUnknown, source.FollowFollowing} {
		if state == source.FollowFollowing {
			if err := s.Commit(ctx, source.Batch{Source: b.Source, FollowTransitions: []source.FollowTransition{{OriginID: "gen-1", From: source.FollowUnknown, To: source.FollowFollowing}}}); err != nil {
				t.Fatal(err)
			}
		}
		if result, err := s.PurgeRetention(ctx, query); err != nil || result != (PurgeResult{}) {
			t.Fatal("live/unknown origin purged", state, result, err)
		}
	}
	if err := s.Commit(ctx, source.Batch{Source: b.Source, FollowTransitions: []source.FollowTransition{{OriginID: "gen-1", From: source.FollowFollowing, To: source.FollowRetired}}}); err != nil {
		t.Fatal(err)
	}
	// The previous preview does not authorize a purge after eligibility changes.
	if _, err := s.db.Exec(`UPDATE checkpoints SET anchor_hash=''`); err != nil {
		t.Fatal(err)
	}
	if result, err := s.PurgeRetention(ctx, query); err != nil || result != (PurgeResult{}) {
		t.Fatal("stale preview used", result, err)
	}
	if _, err := s.db.Exec(`UPDATE checkpoints SET anchor_hash='anchor'`); err != nil {
		t.Fatal(err)
	}
	query.Before = *b.Records[0].Observation.Timestamp.Value
	if result, err := s.PurgeRetention(ctx, query); err != nil || result != (PurgeResult{}) {
		t.Fatal("inclusive cutoff", result, err)
	}
	query.Before = query.Before.Add(time.Nanosecond)
	if result, err := s.PurgeRetention(ctx, query); err != nil || result.Deleted != 1 {
		t.Fatal("eligible origin not purged", result, err)
	}
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal("retired origin retry", err)
	}
	if count(t, s, "events") != 0 {
		t.Fatal("actual purge retry resurrected facts")
	}
}

func TestRetentionPurgeImportRequiresAllAttemptsCompleteAndPreservesManifests(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	attached := prepareImportFixture(t, s, "same\n")
	last := importProgressBatch(attached, "same\n", source.ImportRunning)
	when := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	last.Records[0].Observation.Timestamp.Value = &when
	if err := s.Commit(ctx, last); err != nil {
		t.Fatal(err)
	}
	query := RetentionQuery{Instance: last.Source.ID, Before: when.Add(time.Hour), Limit: 1}
	if result, err := s.PurgeRetention(ctx, query); err != nil || result != (PurgeResult{}) {
		t.Fatal("running import purged", result, err)
	}
	complete := importProgressBatch(last, "", source.ImportComplete)
	if err := s.Commit(ctx, complete); err != nil {
		t.Fatal(err)
	}
	// Another running attempt for the same content blocks eligibility.
	other := complete.ImportChange.Target
	other.ID, other.Status, other.CompletedAt = 999, source.ImportRunning, nil
	reimport := source.Batch{Source: last.Source, ImportChange: &source.ImportChange{Target: other}}
	if err := s.Commit(ctx, reimport); err != nil {
		t.Fatal(err)
	}
	if result, err := s.PurgeRetention(ctx, query); err != nil || result != (PurgeResult{}) {
		t.Fatal("shared running attempt ignored", result, err)
	}
	done := importProgressBatch(reimport, "", source.ImportComplete)
	if err := s.Commit(ctx, done); err != nil {
		t.Fatal(err)
	}
	result, err := s.PurgeRetention(ctx, query)
	if err != nil || result.Deleted != 1 {
		t.Fatal("complete import not purged", result, err)
	}
	assertImportPosition(t, s, complete.ImportChange.Target, 0)
	assertImportPosition(t, s, done.ImportChange.Target, 0)
	if count(t, s, "import_runs") != 2 || count(t, s, "purged_records") != 1 {
		t.Fatal("import manifests deleted")
	}
	if err := s.Commit(ctx, done); err != nil {
		t.Fatal("EOF retry after purge", err)
	}
}

func TestRetentionPurgeFailureRollsBackMarkersInvalidationAndFacts(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	retireRetentionOrigin(t, s, b.Source, "gen-1")
	scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: "mx-a", QueueID: "ABC123"}}}
	facts, err := s.CorrelationFacts(ctx, scope, 10)
	if err != nil {
		t.Fatal(err)
	}
	want, err := s.InstallProjection(ctx, scope, facts, 10, correlation.LinkOptions{Window: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`CREATE TRIGGER refuse_purge BEFORE DELETE ON raw_records BEGIN SELECT RAISE(ABORT,'synthetic final purge failure'); END;`); err != nil {
		t.Fatal(err)
	}
	query := RetentionQuery{Instance: "mx-a", Before: b.Records[0].ReadAt.Add(time.Hour), Limit: 1}
	if result, err := s.PurgeRetention(ctx, query); err == nil || result != (PurgeResult{}) {
		t.Fatal("partial result on failed transaction", result, err)
	}
	got, found, err := s.CurrentProjection(ctx, scope, 10)
	if err != nil || !found || !reflect.DeepEqual(want, got) {
		t.Fatal("failed purge invalidated projection", err)
	}
	if count(t, s, "purged_records") != 0 || count(t, s, "raw_records") != 1 || count(t, s, "event_search_domains") != 1 {
		t.Fatal("failed purge left marker/deleted facts")
	}
	if _, err := s.db.Exec(`DROP TRIGGER refuse_purge`); err != nil {
		t.Fatal(err)
	}
	if result, err := s.PurgeRetention(ctx, query); err != nil || result != (PurgeResult{Deleted: 1, InvalidatedRevisions: 1}) {
		t.Fatal("retry purge", result, err)
	}
}

func TestRetentionPurgeRejectsMalformedAndInvalidInputsWithoutPartialResult(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	retireRetentionOrigin(t, s, b.Source, "gen-1")
	query := RetentionQuery{Instance: "mx-a", Before: b.Records[0].ReadAt.Add(time.Hour), Limit: 1}
	invalid := query
	invalid.Limit = 257
	if result, err := s.PurgeRetention(ctx, invalid); !errors.Is(err, ErrRetentionQuery) || result != (PurgeResult{}) {
		t.Fatal("invalid query", result, err)
	}
	if _, err := s.db.Exec(`UPDATE events SET time_quality='synthetic-private-quality'`); err != nil {
		t.Fatal(err)
	}
	if result, err := s.PurgeRetention(ctx, query); !errors.Is(err, ErrRetentionStoredCandidate) || err.Error() != ErrRetentionStoredCandidate.Error() || result != (PurgeResult{}) {
		t.Fatal("malformed candidate", result, err)
	}
	if count(t, s, "purged_records") != 0 || count(t, s, "events") != 1 {
		t.Fatal("malformed candidate changed DB")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if result, err := s.PurgeRetention(ctx, query); !errors.Is(err, context.Canceled) || result != (PurgeResult{}) {
		t.Fatal("cancelled purge", result, err)
	}
}
