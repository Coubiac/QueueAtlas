package sqlite

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
)

func TestSearchIdentifiersKeepReusedQueuesAndRepeatedMessageIDs(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "reused", "trusted", true, false)
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "foreign", "other", true, false)
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "filter", "trusted", true, false)
	query := eventSearchQuery(SearchQueueID, "13A1B2C3D4")
	query.Until = query.From.Add(48 * time.Hour)
	query.Limit = 2
	var hits []SearchHit
	for i := 0; i < 6; i++ {
		page, err := s.SearchEvents(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		hits = append(hits, page.Hits...)
		if page.Next == nil {
			break
		}
		query.After = page.Next
	}
	if len(hits) != 8 {
		t.Fatal("recycled queue generations were merged or foreign instance included", len(hits))
	}
	refs := make(map[correlation.FactRef]bool)
	for _, hit := range hits {
		if hit.Ref.SourceID != "reused" || hit.QueueID != query.Value || refs[hit.Ref] {
			t.Fatal("lost physical identity")
		}
		refs[hit.Ref] = true
	}
	// The existing parser strips angle delimiters before persisting this field.
	query = eventSearchQuery(SearchMessageID, "filter-11@example.net")
	query.Limit = 1
	first, err := s.SearchEvents(ctx, query)
	if err != nil || len(first.Hits) != 1 || first.Next == nil {
		t.Fatal("repeated Message-ID first page", err)
	}
	query.After = first.Next
	second, err := s.SearchEvents(ctx, query)
	if err != nil || len(second.Hits) != 1 || second.Next != nil || first.Hits[0].QueueID == second.Hits[0].QueueID {
		t.Fatal("Message-ID became a unique identity", err)
	}
	query.After = nil
	query.Value = "<filter-11@example.net>"
	if page, err := s.SearchEvents(ctx, query); err != nil || len(page.Hits) != 0 {
		t.Fatal("Message-ID angle brackets normalized", err)
	}
	query.Value = "FILTER-11@example.net"
	if page, err := s.SearchEvents(ctx, query); err != nil || len(page.Hits) != 0 {
		t.Fatal("Message-ID case normalized", err)
	}
}

func TestSearchIdentifiersLiteralValuesBoundsAndCursorKinds(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	batch := testBatch()
	batch.Records[0].Observation.QueueID = "' OR 1=1--%_"
	batch.Records[0].Observation.Fields["message-id"] = "<literal%_" + string([]byte{0xff}) + "@example.org>"
	if err := s.Commit(ctx, batch); err != nil {
		t.Fatal(err)
	}
	for _, query := range []SearchQuery{
		{Instance: "mx-a", Field: SearchQueueID, Value: batch.Records[0].Observation.QueueID, From: eventSearchQuery(SearchSender, "").From, Until: eventSearchQuery(SearchSender, "").Until, Limit: 1},
		{Instance: "mx-a", Field: SearchMessageID, Value: batch.Records[0].Observation.Fields["message-id"], From: eventSearchQuery(SearchSender, "").From, Until: eventSearchQuery(SearchSender, "").Until, Limit: 1},
	} {
		page, err := s.SearchEvents(ctx, query)
		if err != nil || len(page.Hits) != 1 || page.Next != nil {
			t.Fatal("identifier interpreted as SQL/pattern or bytes changed", err)
		}
		_, _, revision, err := eventSearchSelection(query)
		if err != nil {
			t.Fatal(err)
		}
		query.After = &SearchCursor{QueryRevision: revision, TimeNS: page.Hits[0].At.UnixNano(), RowID: 1}
		query.Field = SearchSender
		if got, err := s.SearchEvents(ctx, query); !errors.Is(err, ErrSearchCursor) || !reflect.DeepEqual(got, SearchPage{}) {
			t.Fatal("identifier cursor crossed search field", err)
		}
	}
	for _, query := range []SearchQuery{
		eventSearchQuery(SearchQueueID, ""), eventSearchQuery(SearchMessageID, ""),
		eventSearchQuery(SearchQueueID, strings.Repeat("q", 33)), eventSearchQuery(SearchMessageID, strings.Repeat("m", 1025)),
	} {
		if page, err := s.SearchEvents(ctx, query); !errors.Is(err, ErrSearchQuery) || !reflect.DeepEqual(page, SearchPage{}) {
			t.Fatal("invalid identifier bounds", err)
		}
	}
	for _, query := range []SearchQuery{eventSearchQuery(SearchQueueID, strings.Repeat("q", 32)), eventSearchQuery(SearchMessageID, strings.Repeat("m", 1024))} {
		if _, err := s.SearchEvents(ctx, query); err != nil {
			t.Fatal("exact identifier bound rejected", err)
		}
	}
}

func TestSearchIdentifiersUseQueueAndMessageTimeIndexes(t *testing.T) {
	s, _ := openTestStore(t)
	for _, test := range []struct {
		field SearchField
		index string
	}{{SearchQueueID, "events_queue"}, {SearchMessageID, "events_message_id_time"}} {
		query := eventSearchQuery(test.field, "EXACT")
		_, _, revision, err := eventSearchSelection(query)
		if err != nil {
			t.Fatal(err)
		}
		for _, cursor := range []*SearchCursor{nil, {QueryRevision: revision, TimeNS: query.From.Add(time.Hour).UnixNano(), RowID: 10}} {
			query.After = cursor
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
				t.Fatal("identifier search lost index/time ordering", text)
			}
		}
	}
}

func seedV4SearchStore(t *testing.T) (*Store, string) {
	t.Helper()
	db, path := seedV3ProjectionStore(t)
	if _, err := db.Exec(schemaV4 + `INSERT INTO schema_migrations VALUES(4,4); PRAGMA user_version=4;`); err != nil {
		t.Fatal(err)
	}
	return &Store{db: db}, path
}

func TestSearchMigrationV4PreservesManifestFactsAndCheckpoint(t *testing.T) {
	s, path := seedV4SearchStore(t)
	ctx := context.Background()
	scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: "mx-a", QueueID: "ABC123"}}}
	facts, err := s.CorrelationFacts(ctx, scope, 10)
	if err != nil {
		t.Fatal(err)
	}
	want, err := s.InstallProjection(ctx, scope, facts, 10, correlation.LinkOptions{Window: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, found, err := s.CurrentProjection(ctx, scope, 10)
	if err != nil || !found || !reflect.DeepEqual(got, want) || count(t, s, "schema_migrations") != 6 {
		t.Fatal("index migration changed projection", err)
	}
	after, err := s.CorrelationFacts(ctx, scope, 10)
	if err != nil || !reflect.DeepEqual(after, facts) {
		t.Fatal("index migration changed facts", err)
	}
	position, found, err := s.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !found || position != testBatch().Checkpoints[0] {
		t.Fatal("index migration changed checkpoint", err)
	}
	query := eventSearchQuery(SearchMessageID, "<id@example.org>")
	query.Instance = "mx-a"
	page, err := s.SearchEvents(ctx, query)
	if err != nil || len(page.Hits) != 1 {
		t.Fatal("old Message-ID missing after migration", err)
	}
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 6 {
		t.Fatal("current schema version not recorded", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if count(t, s, "schema_migrations") != 6 {
		t.Fatal("reopen migrated again")
	}
}

func TestSearchMigrationFailureRollsBackIndexAndHistory(t *testing.T) {
	s, _ := seedV4SearchStore(t)
	defer s.Close()
	if _, err := s.db.Exec(`CREATE TRIGGER stop_v5 BEFORE INSERT ON schema_migrations WHEN NEW.version=5 BEGIN SELECT RAISE(ABORT,'synthetic v5 failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(context.Background(), s.db, 5); err == nil {
		t.Fatal("failed migration accepted")
	}
	var version, index int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 4 {
		t.Fatal("failed migration changed version", err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='events_message_id_time'`).Scan(&index); err != nil || index != 0 {
		t.Fatal("partial index survived", err)
	}
	if count(t, s, "schema_migrations") != 4 || count(t, s, "events") != 1 || count(t, s, "raw_records") != 1 {
		t.Fatal("failed migration changed durable data")
	}
}
