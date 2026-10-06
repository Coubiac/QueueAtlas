package sqlite

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
)

func retentionTestQuery() RetentionQuery {
	return RetentionQuery{Instance: "trusted", Before: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC), Limit: 100}
}

func TestRetentionPreviewBoundedReadOnlyScopeAndUnknownDates(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "dated", "trusted", true, false)
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "undated", "trusted", false, false)
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "foreign", "other", true, false)
	scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: "trusted", QueueID: "13A1B2C3D4"}}}
	facts, err := s.CorrelationFacts(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	want, err := s.InstallProjection(ctx, scope, facts, 100, correlation.LinkOptions{Window: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, found, err := s.Checkpoint(ctx, "dated", "dated-13-reused-queue-id")
	if err != nil || !found {
		t.Fatal(err)
	}
	// Database-level read-only mode makes any accidental write fail.
	if _, err := s.db.Exec(`PRAGMA query_only=ON`); err != nil {
		t.Fatal(err)
	}
	query := retentionTestQuery()
	query.Limit = 2
	first, err := s.PreviewRetention(ctx, query)
	if err != nil || len(first.Candidates) != 2 || !first.More {
		t.Fatal("bounded retention preview", err)
	}
	again, err := s.PreviewRetention(ctx, query)
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatal("preview advanced or mutated", err)
	}
	query.Limit = MaxRetentionPreview
	all, err := s.PreviewRetention(ctx, query)
	if err != nil || len(all.Candidates) != 4 || all.More {
		t.Fatal("preview included undated/future/foreign", err)
	}
	for i, item := range all.Candidates {
		if item.Ref.SourceID != "dated" || !item.At.Before(query.Before) || item.At.Location() != time.UTC || item.TimeQuality != model.TimeConfiguredYearAndZone {
			t.Fatal("candidate lost date/physical scope")
		}
		if i > 0 && item.At.Before(all.Candidates[i-1].At) {
			t.Fatal("preview date order")
		}
	}
	current, exists, err := s.CurrentProjection(ctx, scope, 100)
	if err != nil || !exists || !reflect.DeepEqual(current, want) {
		t.Fatal("preview invalidated projection", err)
	}
	after, afterFound, err := s.Checkpoint(ctx, "dated", "dated-13-reused-queue-id")
	if err != nil || afterFound != found || after != checkpoint {
		t.Fatal("preview changed checkpoint", err)
	}
	if count(t, s, "events") != 24 || count(t, s, "raw_records") != 24 || count(t, s, "event_search_domains") != 24 || count(t, s, "projection_revision_facts") != 16 || count(t, s, "schema_migrations") != schemaVersion {
		t.Fatal("preview modified stored data")
	}
}

func TestRetentionPreviewRequiresCoveredCheckpointAndExclusiveCutoff(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	batch := testBatch()
	if err := s.Commit(ctx, batch); err != nil {
		t.Fatal(err)
	}
	query := retentionTestQuery()
	query.Instance = "mx-a"
	assertCount := func(want int) {
		t.Helper()
		got, err := s.PreviewRetention(ctx, query)
		if err != nil || len(got.Candidates) != want || got.More {
			t.Fatal("checkpoint eligibility", want, err)
		}
	}
	assertCount(1)
	query.Before = *batch.Records[0].Observation.Timestamp.Value
	assertCount(0)
	query.Before = query.Before.Add(time.Nanosecond)
	assertCount(1)
	if _, err := s.db.Exec(`DELETE FROM checkpoints`); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if _, err := s.db.Exec(`INSERT INTO checkpoints VALUES('mail','gen-1',0,'synthetic',0)`); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if _, err := s.db.Exec(`UPDATE checkpoints SET offset=?`, batch.Records[0].End-1); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if _, err := s.db.Exec(`UPDATE checkpoints SET offset=?,anchor_hash=''`, batch.Records[0].End); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if _, err := s.db.Exec(`UPDATE checkpoints SET anchor_hash='synthetic'`); err != nil {
		t.Fatal(err)
	}
	assertCount(1)
}

func TestRetentionPreviewRejectsInvalidQueryAndCancellation(t *testing.T) {
	s, _ := openTestStore(t)
	for _, change := range []func(*RetentionQuery){
		func(q *RetentionQuery) { q.Instance = "" }, func(q *RetentionQuery) { q.Instance = strings.Repeat("i", 1025) },
		func(q *RetentionQuery) { q.Instance = "x\x00" }, func(q *RetentionQuery) { q.Instance = "x\t" },
		func(q *RetentionQuery) { q.Before = time.Time{} }, func(q *RetentionQuery) { q.Before = time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC) },
		func(q *RetentionQuery) { q.Limit = 0 }, func(q *RetentionQuery) { q.Limit = MaxRetentionPreview + 1 },
	} {
		query := retentionTestQuery()
		change(&query)
		got, err := s.PreviewRetention(context.Background(), query)
		if !errors.Is(err, ErrRetentionQuery) || !reflect.DeepEqual(got, RetentionPreview{}) {
			t.Fatal("invalid preview query", err)
		}
	}
	query := retentionTestQuery()
	query.Instance = "' OR 1=1--%_"
	if got, err := s.PreviewRetention(context.Background(), query); err != nil || len(got.Candidates) != 0 {
		t.Fatal("instance interpreted as SQL", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := s.PreviewRetention(ctx, retentionTestQuery()); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, RetentionPreview{}) {
		t.Fatal("cancelled preview", err)
	}
}

func TestRetentionPreviewStoredErrorsAreFixedAndReturnNoPartialOutput(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	at := retentionTestQuery().Before.Add(-time.Hour)
	seedSearchEvents(t, s, []string{"one@example.org", "two@example.org"}, []time.Time{at, at})
	const marker = "synthetic-private-retention-value"
	if _, err := s.db.Exec(`UPDATE events SET time_quality=? WHERE id=2`, marker); err != nil {
		t.Fatal(err)
	}
	got, err := s.PreviewRetention(ctx, retentionTestQuery())
	if !errors.Is(err, ErrRetentionStoredCandidate) || !reflect.DeepEqual(got, RetentionPreview{}) || strings.Contains(err.Error(), marker) {
		t.Fatal("partial preview or leaked stored value", err)
	}
	if _, err := s.db.Exec(`UPDATE events SET time_quality='explicit_offset'; UPDATE checkpoints SET offset=?`, marker); err != nil {
		t.Fatal(err)
	}
	got, err = s.PreviewRetention(ctx, retentionTestQuery())
	if !errors.Is(err, ErrRetentionStoredCandidate) || !reflect.DeepEqual(got, RetentionPreview{}) || strings.Contains(err.Error(), marker) {
		t.Fatal("checkpoint conversion leaked value", err)
	}
}
