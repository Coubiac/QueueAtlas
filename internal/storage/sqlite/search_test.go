package sqlite

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/correlation"
	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
)

func eventSearchQuery(field SearchField, value string) SearchQuery {
	return SearchQuery{Instance: "trusted", Field: field, Value: value,
		From: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC), Limit: 100}
}

func seedSearchEvents(t *testing.T, s *Store, values []string, dates []time.Time) {
	t.Helper()
	when := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	batch := source.Batch{Source: source.Identity{ID: "search", Kind: "file", Name: "synthetic", TrustedHost: "trusted"}, Origins: []source.Origin{{ID: "search-origin", Path: "/synthetic/search", Fingerprint: "synthetic-search", FirstSeen: when}}}
	var offset int64
	for i, value := range values {
		at := dates[i]
		raw := []byte(fmt.Sprintf("synthetic event %d\n", i))
		end := offset + int64(len(raw))
		o := model.Observation{SourceID: "search", QueueID: "ABC123", Kind: model.KindDelivery, Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}, Fields: map[string]string{"to": value, "from": value}, Present: map[string]bool{"to": true, "from": true}}
		batch.Records = append(batch.Records, source.Record{OriginID: "search-origin", Start: offset, End: end, Raw: raw, ReadAt: when, Observation: o})
		offset = end
	}
	batch.Checkpoints = []source.Position{{OriginID: "search-origin", Offset: offset, AnchorHash: "synthetic"}}
	if err := s.Commit(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
}

func TestSearchEventsExactNativeValuesInstanceDatesAndNoQueue(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	for _, name := range []string{"11-filter-reinjection", "09-bounce", "06-noqueue-client-reject"} {
		storeCorrelationCorpus(t, s, name, name, "trusted", true, false)
	}
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "foreign", "foreign", true, false)
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "undated", "trusted", false, false)
	page, err := s.SearchEvents(ctx, eventSearchQuery(SearchSender, "alice@example.net"))
	if err != nil || len(page.Hits) != 2 || page.Next != nil {
		t.Fatal("sender native hits", err, len(page.Hits))
	}
	for _, hit := range page.Hits {
		if hit.Instance != "trusted" || hit.Ref.SourceID != "11-filter-reinjection" || hit.TimeQuality != model.TimeConfiguredYearAndZone || hit.Kind != model.KindMessage {
			t.Fatal("foreign/undated source or invented kind")
		}
	}
	page, err = s.SearchEvents(ctx, eventSearchQuery(SearchRecipient, "bob@example.org"))
	if err != nil || len(page.Hits) != 3 {
		t.Fatal("recipient facts including rejection", err, len(page.Hits))
	}
	var noqueue int
	for _, hit := range page.Hits {
		if hit.NoQueue {
			noqueue++
			if hit.QueueID != "" || hit.Kind != model.KindReject {
				t.Fatal("NOQUEUE converted into accepted queue")
			}
		}
	}
	if noqueue != 1 {
		t.Fatal("missing NOQUEUE")
	}
	page, err = s.SearchEvents(ctx, eventSearchQuery(SearchSender, ""))
	if err != nil || len(page.Hits) != 1 || page.Hits[0].QueueID != "09F1B2C3D4" {
		t.Fatal("explicit empty sender confused with absent", err)
	}
	page, err = s.SearchEvents(ctx, eventSearchQuery(SearchSender, "Alice@example.net"))
	if err != nil || len(page.Hits) != 0 {
		t.Fatal("search changed case", err)
	}
	query := eventSearchQuery(SearchSender, "alice@example.net")
	query.Instance = "not-present"
	page, err = s.SearchEvents(ctx, query)
	if err != nil || len(page.Hits) != 0 {
		t.Fatal("instance ignored", err)
	}
}

func TestSearchEventsSeekPaginationTiesBoundariesAndCursorScope(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	query := eventSearchQuery(SearchRecipient, "same@example.org")
	query.Until = query.From.Add(4 * time.Second)
	query.Limit = 2
	dates := []time.Time{query.From, query.From.Add(time.Second), query.From.Add(time.Second), query.From.Add(time.Second), query.From.Add(2 * time.Second), query.From.Add(3 * time.Second), query.Until}
	values := make([]string, len(dates))
	for i := range values {
		values[i] = query.Value
	}
	seedSearchEvents(t, s, values, dates)
	first, err := s.SearchEvents(ctx, query)
	if err != nil || len(first.Hits) != 2 || first.Next == nil || !first.Hits[0].At.Equal(query.From) {
		t.Fatal("first page/boundary", err)
	}
	var hits []SearchHit
	page := first
	for pages := 0; pages < 5; pages++ {
		hits = append(hits, page.Hits...)
		if page.Next == nil {
			break
		}
		query.After = page.Next
		page, err = s.SearchEvents(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(hits) != 6 || page.Next != nil {
		t.Fatal("pagination lost tied facts or included Until", len(hits))
	}
	seen := make(map[correlation.FactRef]bool)
	for i, hit := range hits {
		if seen[hit.Ref] || (i > 0 && hit.At.Before(hits[i-1].At)) {
			t.Fatal("duplicate or unstable order")
		}
		seen[hit.Ref] = true
	}
	query.After = first.Next
	query.Limit = 3
	changedSize, err := s.SearchEvents(ctx, query)
	if err != nil || len(changedSize.Hits) != 3 || changedSize.Hits[0].Ref != hits[2].Ref {
		t.Fatal("page size altered cursor semantics", err)
	}
	for _, change := range []func(*SearchQuery){func(q *SearchQuery) { q.Instance = "foreign" }, func(q *SearchQuery) { q.Value = "different" }, func(q *SearchQuery) { q.Field = SearchSender }, func(q *SearchQuery) { q.Until = q.Until.Add(time.Second) }, func(q *SearchQuery) { q.From = q.From.Add(-time.Second) }, func(q *SearchQuery) { c := *q.After; c.TimeNS = q.Until.UnixNano(); q.After = &c }} {
		other := query
		change(&other)
		if got, err := s.SearchEvents(ctx, other); !errors.Is(err, ErrSearchCursor) || !reflect.DeepEqual(got, SearchPage{}) {
			t.Fatal("foreign cursor accepted", err)
		}
	}
	// Equivalent UTC instants have the same query revision despite location.
	query.From = query.From.In(time.FixedZone("other", 3600))
	query.Until = query.Until.In(time.FixedZone("other", 3600))
	if _, err := s.SearchEvents(ctx, query); err != nil {
		t.Fatal("time location changed search identity", err)
	}
}

func TestSearchEventsTreatsWildcardsSQLAndBinaryValuesLiterally(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	query := eventSearchQuery(SearchRecipient, "' OR 1=1 --%_@example.org")
	binaryValue := "binary" + string([]byte{0xff}) + "@example.org"
	values := []string{query.Value, "anything@example.org", binaryValue}
	seedSearchEvents(t, s, values, []time.Time{query.From, query.From, query.From})
	for _, value := range []string{query.Value, binaryValue} {
		query.Value = value
		page, err := s.SearchEvents(ctx, query)
		if err != nil || len(page.Hits) != 1 || page.Next != nil {
			t.Fatal("literal value became pattern or bytes changed", err)
		}
	}
	query.Value = "%@example.org"
	if page, err := s.SearchEvents(ctx, query); err != nil || len(page.Hits) != 0 {
		t.Fatal("wildcard expanded", err)
	}
}

func TestSearchEventsBoundsValidationAndCancellation(t *testing.T) {
	s, _ := openTestStore(t)
	query := eventSearchQuery(SearchRecipient, "recipient@example.org")
	for _, change := range []func(*SearchQuery){
		func(q *SearchQuery) { q.Instance = "" }, func(q *SearchQuery) { q.Instance = "bad\ninstance" }, func(q *SearchQuery) { q.Value = "bad\x00value" },
		func(q *SearchQuery) { q.Value = strings.Repeat("v", 1025) }, func(q *SearchQuery) { q.Instance = strings.Repeat("i", 1025) }, func(q *SearchQuery) { q.Field = "sender OR 1=1" },
		func(q *SearchQuery) { q.Limit = 0 }, func(q *SearchQuery) { q.Limit = MaxSearchResults + 1 }, func(q *SearchQuery) { q.From = time.Time{} },
		func(q *SearchQuery) { q.Until = q.From }, func(q *SearchQuery) { q.Until = q.From.Add(-time.Second) }, func(q *SearchQuery) { q.Until = q.From.Add(MaxSearchWindow + time.Nanosecond) },
		func(q *SearchQuery) {
			q.From = time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC)
			q.Until = q.From.Add(time.Hour)
		},
	} {
		bad := query
		change(&bad)
		if got, err := s.SearchEvents(context.Background(), bad); !errors.Is(err, ErrSearchQuery) || !reflect.DeepEqual(got, SearchPage{}) {
			t.Fatal("invalid search query", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := s.SearchEvents(ctx, query); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, SearchPage{}) {
		t.Fatal("canceled read", err)
	}
	values := make([]string, MaxSearchResults+1)
	dates := make([]time.Time, len(values))
	for i := range values {
		values[i] = query.Value
		dates[i] = query.From
	}
	seedSearchEvents(t, s, values, dates)
	query.Limit = MaxSearchResults
	query.Until = query.From.Add(MaxSearchWindow)
	page, err := s.SearchEvents(context.Background(), query)
	if err != nil || len(page.Hits) != MaxSearchResults || page.Next == nil {
		t.Fatal("exact bounds or limit+1", err)
	}
}

func TestSearchEventsMalformedRowDiscardsPageAndSanitizesConversion(t *testing.T) {
	for _, statement := range []string{
		`PRAGMA ignore_check_constraints=ON; UPDATE events SET no_queue='synthetic-private@example.org' WHERE id=(SELECT max(id) FROM events); PRAGMA ignore_check_constraints=OFF`,
		`PRAGMA ignore_check_constraints=ON; UPDATE raw_records SET end_offset=start_offset WHERE id=(SELECT max(id) FROM raw_records); PRAGMA ignore_check_constraints=OFF`,
	} {
		s, _ := openTestStore(t)
		query := eventSearchQuery(SearchRecipient, "recipient@example.org")
		seedSearchEvents(t, s, []string{query.Value, query.Value}, []time.Time{query.From, query.From})
		if _, err := s.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
		page, err := s.SearchEvents(context.Background(), query)
		if !errors.Is(err, ErrSearchStoredHit) || !reflect.DeepEqual(page, SearchPage{}) || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatal("partial/PII escaped", err)
		}
	}
}

func TestSearchEventsUsesExistingFieldTimeIndexes(t *testing.T) {
	s, _ := openTestStore(t)
	for _, test := range []struct {
		field SearchField
		index string
	}{{SearchSender, "events_sender"}, {SearchRecipient, "events_recipient"}} {
		query := eventSearchQuery(test.field, "value@example.org")
		_, _, revision, err := eventSearchSelection(query)
		if err != nil {
			t.Fatal(err)
		}
		query.After = &SearchCursor{QueryRevision: revision, TimeNS: query.From.Add(time.Hour).UnixNano(), RowID: 10}
		where, args, _, err := eventSearchSelection(query)
		if err != nil {
			t.Fatal(err)
		}
		args = append(args, query.Limit+1)
		rows, err := s.db.Query(`EXPLAIN QUERY PLAN SELECT e.id FROM events e JOIN raw_records r ON r.id=e.raw_record_id WHERE `+where+` ORDER BY e.time_utc_ns,e.id LIMIT ?`, args...)
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		text := strings.Join(plan, "\n")
		if !strings.Contains(text, "USING INDEX "+test.index) || strings.Contains(text, "SCAN e") || strings.Contains(text, "TEMP B-TREE") {
			t.Fatal("unindexed or sorted search", text)
		}
	}
}
