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

func TestRetentionIntegrationWALReaderKeepsSnapshotUntilNextRead(t *testing.T) {
	writer, path := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	if err := writer.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	retireRetentionOrigin(t, writer, b.Source, "gen-1")
	scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: "mx-a", QueueID: "ABC123"}}}
	facts, err := writer.CorrelationFacts(ctx, scope, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.InstallProjection(ctx, scope, facts, 10, correlation.LinkOptions{Window: time.Minute}); err != nil {
		t.Fatal(err)
	}
	reader, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	tx, err := reader.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	const inventory = `SELECT (SELECT count(*) FROM events),(SELECT count(*) FROM purged_records),
		(SELECT count(*) FROM projection_revisions),(SELECT count(*) FROM projection_scopes WHERE current_revision_id IS NOT NULL)`
	var before, old, after [4]int
	if err := tx.QueryRow(inventory).Scan(&before[0], &before[1], &before[2], &before[3]); err != nil {
		t.Fatal(err)
	}
	if before != [4]int{1, 0, 1, 1} {
		t.Fatal("snapshot fixture", before)
	}
	result, err := writer.PurgeRetention(ctx, RetentionQuery{Instance: "mx-a", Before: b.Records[0].ReadAt.Add(time.Hour), Limit: 1})
	if err != nil || result != (PurgeResult{Deleted: 1, InvalidatedRevisions: 1}) {
		t.Fatal("WAL writer blocked or incomplete", result, err)
	}
	if err := tx.QueryRow(inventory).Scan(&old[0], &old[1], &old[2], &old[3]); err != nil || old != before {
		t.Fatal("reader observed mixed purge snapshot", old, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := reader.db.QueryRow(inventory).Scan(&after[0], &after[1], &after[2], &after[3]); err != nil || after != [4]int{0, 1, 0, 0} {
		t.Fatal("new snapshot missed committed purge", after, err)
	}
	if _, found, err := reader.CurrentProjection(ctx, scope, 10); err != nil || found {
		t.Fatal("reader repaired invalidated projection implicitly", err)
	}
	cp, found, err := reader.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !found || cp != b.Checkpoints[0] {
		t.Fatal("purge changed durable read position", err)
	}
}

func TestRetentionIntegrationSearchRefreshUsesRemainingFullScopeAndRejectsOldFacts(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	dated := storeCorrelationCorpus(t, s, "13-reused-queue-id", "dated", "trusted", true, false)
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "undated", "trusted", false, false)
	for _, id := range []string{"dated", "undated"} {
		retireRetentionOrigin(t, s, source.Identity{ID: id, Kind: "file", Name: "synthetic", TrustedHost: "trusted"}, id+"-13-reused-queue-id")
	}
	scope := queueScope(dated)
	old, err := s.CorrelationFacts(ctx, scope, 30)
	if err != nil || len(old) != 16 {
		t.Fatal("full initial scope", err)
	}
	if _, err := s.InstallProjection(ctx, scope, old, 30, correlation.LinkOptions{Window: time.Minute}); err != nil {
		t.Fatal(err)
	}
	query := eventSearchQuery(SearchQueueID, "13A1B2C3D4")
	query.Instance = "trusted"
	query.Limit = 2
	query.Until = query.Until.Add(24 * time.Hour)
	page, err := s.SearchEvents(ctx, query)
	if err != nil || len(page.Hits) != 2 || page.Next == nil {
		t.Fatal("initial page", err)
	}
	cutoff := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	if result, err := s.PurgeRetention(ctx, RetentionQuery{Instance: "trusted", Before: cutoff, Limit: 256}); err != nil || result.Deleted != 4 {
		t.Fatal("actual purge", result, err)
	}
	if _, found, err := s.CurrentProjection(ctx, scope, 30); err != nil || found {
		t.Fatal("stale current survived", err)
	}
	if _, err := s.InstallProjection(ctx, scope, old, 30, correlation.LinkOptions{Window: time.Minute}); !errors.Is(err, ErrProjectionStale) {
		t.Fatal("old facts reinstalled after purge", err)
	}
	// Reusing an existing seek cursor is pagination, not a historical snapshot.
	query.After = page.Next
	page, err = s.SearchEvents(ctx, query)
	if err != nil || len(page.Hits) != 2 {
		t.Fatal("cursor after purge", err)
	}
	for _, hit := range page.Hits {
		if hit.At.Before(cutoff) {
			t.Fatal("search resurrected deleted hit")
		}
	}
	remaining, err := s.CorrelationFacts(ctx, scope, 30)
	if err != nil || len(remaining) != 12 {
		t.Fatal("page truncated full remaining scope", err)
	}
	unknown := 0
	for _, fact := range remaining {
		if fact.Observation.Timestamp.Value == nil {
			unknown++
		}
	}
	if unknown != 8 {
		t.Fatal("undated facts lost")
	}
	want, err := s.InstallProjection(ctx, scope, remaining, 30, correlation.LinkOptions{Window: time.Minute})
	if err != nil {
		t.Fatal("explicit rebuild", err)
	}
	got, found, err := s.CurrentProjection(ctx, scope, 30)
	if err != nil || !found || !reflect.DeepEqual(want, got) {
		t.Fatal("rebuilt current inconsistent", err)
	}
	if got.InputRevision == "" || count(t, s, "projection_revision_facts") != 12 || count(t, s, "purged_records") != 4 {
		t.Fatal("rebuilt manifest lost full membership/provenance trace")
	}
}

func TestRetentionIntegrationNonemptyCompletedImportRetryUsesCommitmentAndAnchorChecks(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	attached := prepareImportFixture(t, s, "same\n")
	last := importProgressBatch(attached, "same\n", source.ImportComplete)
	when := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	last.Records[0].Observation.Timestamp.Value = &when
	if err := s.Commit(ctx, last); err != nil {
		t.Fatal(err)
	}
	if result, err := s.PurgeRetention(ctx, RetentionQuery{Instance: last.Source.ID, Before: when.Add(time.Hour), Limit: 1}); err != nil || result.Deleted != 1 {
		t.Fatal("complete import purge", result, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	s = reopened
	defer s.Close()
	if err := s.Commit(ctx, last); err != nil {
		t.Fatal("nonempty lost-ACK retry after purge/reopen", err)
	}
	assertImportPosition(t, s, last.ImportChange.Target, 0)
	bad := last
	bad.Checkpoints = append([]source.Position(nil), last.Checkpoints...)
	bad.Checkpoints[0].AnchorHash = "synthetic-wrong-anchor"
	if err := s.Commit(ctx, bad); !errors.Is(err, ErrImportConflict) {
		t.Fatal("marker bypassed import anchor conflict", err)
	}
	bad = last
	bad.Records = append([]source.Record(nil), last.Records...)
	bad.Records[0].Raw = []byte("evil\n")
	if err := s.Commit(ctx, bad); !errors.Is(err, ErrPurgedRecordCollision) {
		t.Fatal("marker bypassed byte conflict", err)
	}
	assertImportPosition(t, s, last.ImportChange.Target, 0)
	if count(t, s, "purged_records") != 1 || count(t, s, "events") != 0 {
		t.Fatal("retry recreated facts or erased trace")
	}
}
