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

func TestSearchDomainsExactCasePaginationAndNativeValues(t *testing.T) {
	s, _ := openTestStore(t)
	at := eventSearchQuery(SearchRecipientDomain, "").From.Add(time.Hour)
	seedSearchEvents(t, s, []string{"Alice@EXAMPLE.ORG", "bob@example.org", "third@sub.example.org", "bad@notexample.org", "@example.org", "user@[127.0.0.1]", "", "two@@example.org"}, []time.Time{at, at, at, at, at, at, at, at})
	storeCorrelationCorpus(t, s, "06-noqueue-client-reject", "noqueue", "trusted", true, false)
	storeCorrelationCorpus(t, s, "06-noqueue-client-reject", "foreign", "other", true, false)
	storeCorrelationCorpus(t, s, "06-noqueue-client-reject", "undated", "trusted", false, false)
	for _, field := range []SearchField{SearchSenderDomain, SearchRecipientDomain} {
		query := eventSearchQuery(field, "EXAMPLE.ORG")
		query.Limit = 1
		first, err := s.SearchEvents(context.Background(), query)
		if err != nil || len(first.Hits) != 1 || first.Next == nil {
			t.Fatal("domain first page", err)
		}
		query.Value = "example.org" // Equivalent normalized criterion accepts the cursor.
		query.After = first.Next
		second, err := s.SearchEvents(context.Background(), query)
		if err != nil || len(second.Hits) != 1 || first.Hits[0].Ref == second.Hits[0].Ref {
			t.Fatal("domain ties lost", err)
		}
		if field == SearchSenderDomain && second.Next != nil {
			t.Fatal("domain suffix or malformed address matched")
		}
		if field == SearchRecipientDomain {
			if second.Next == nil {
				t.Fatal("NOQUEUE missing")
			}
			query.After = second.Next
			third, err := s.SearchEvents(context.Background(), query)
			if err != nil || len(third.Hits) != 1 || third.Next != nil || !third.Hits[0].NoQueue || third.Hits[0].Ref.SourceID != "noqueue" {
				t.Fatal("foreign/undated or lost rejection", err)
			}
		}
		query.Field = SearchRecipient
		if page, err := s.SearchEvents(context.Background(), query); !errors.Is(err, ErrSearchCursor) || !reflect.DeepEqual(page, SearchPage{}) {
			t.Fatal("domain cursor crossed field", err)
		}
	}
	page, err := s.SearchEvents(context.Background(), eventSearchQuery(SearchRecipient, "Alice@EXAMPLE.ORG"))
	if err != nil || len(page.Hits) != 1 {
		t.Fatal("native address changed", err)
	}
	page, err = s.SearchEvents(context.Background(), eventSearchQuery(SearchRecipient, "Alice@example.org"))
	if err != nil || len(page.Hits) != 0 {
		t.Fatal("exact address normalized", err)
	}
	if count(t, s, "event_search_domains") != count(t, s, "events") {
		t.Fatal("domain rows lost")
	}
}

func TestSearchDomainsNormalizationSubsetAndInvalidQueries(t *testing.T) {
	max := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61)
	for input, want := range map[string]string{"EXAMPLE.ORG": "example.org", "localhost": "localhost", "XN--EXAMPLE-9D0B.ORG": "xn--example-9d0b.org", max: max} {
		got, ok := normalizeSearchDomain(input)
		if !ok || got != want {
			t.Fatal("accepted DNS subset changed", input)
		}
	}
	invalid := []string{"", "example.org.", ".example.org", "a..org", "-a.org", "a-.org", strings.Repeat("a", 64) + ".org", max + "d", "tést.org", "[127.0.0.1]", "example.org%", "example_org", "' OR 1=1--", " example.org", "example.org\x00", string([]byte{0xff})}
	s, _ := openTestStore(t)
	for _, input := range invalid {
		for _, field := range []SearchField{SearchSenderDomain, SearchRecipientDomain} {
			page, err := s.SearchEvents(context.Background(), eventSearchQuery(field, input))
			if !errors.Is(err, ErrSearchQuery) || !reflect.DeepEqual(page, SearchPage{}) {
				t.Fatal("invalid domain query accepted", err)
			}
		}
	}
	for _, input := range []any{nil, "", "postmaster", "@example.org", "u@@example.org", "<u@example.org>", "\"u v\"@example.org", "u@example.org.", "u@tést.org", "u@[127.0.0.1]", "u@exam%ple.org", "u@exam_ple.org", "u\n@example.org", "u" + string([]byte{0xff}) + "@example.org"} {
		if got := addressSearchDomain(input); got != nil {
			t.Fatal("ambiguous/absent address acquired domain")
		}
	}
	if got := addressSearchDomain("Élise@EXAMPLE.ORG"); got != "example.org" {
		t.Fatal("UTF8 local part lost")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if page, err := s.SearchEvents(ctx, eventSearchQuery(SearchSenderDomain, "example.org")); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(page, SearchPage{}) {
		t.Fatal("cancelled domain query", err)
	}
}

func TestSearchDomainsUseInstanceDomainTimeIndexes(t *testing.T) {
	s, _ := openTestStore(t)
	for _, test := range []struct {
		field SearchField
		index string
	}{{SearchSenderDomain, "search_sender_domain_time"}, {SearchRecipientDomain, "search_recipient_domain_time"}} {
		query := eventSearchQuery(test.field, "example.org")
		_, _, rev, err := eventSearchSelection(query)
		if err != nil {
			t.Fatal(err)
		}
		for _, cursor := range []*SearchCursor{nil, {QueryRevision: rev, TimeNS: query.From.UnixNano(), RowID: 1}} {
			query.After = cursor
			where, args, _, err := eventSearchSelection(query)
			if err != nil {
				t.Fatal(err)
			}
			args = append(args, query.Limit+1)
			rows, err := s.db.Query(`EXPLAIN QUERY PLAN SELECT e.id FROM `+eventSearchFrom(test.field)+` WHERE `+where+` ORDER BY `+eventSearchOrder(test.field)+` LIMIT ?`, args...)
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
			err = rows.Err()
			rows.Close()
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Join(plan, "\n")
			if !strings.Contains(text, test.index) || strings.Contains(text, "SCAN d") || strings.Contains(text, "TEMP B-TREE") {
				t.Fatal("domain search lost index/order", text)
			}
		}
	}
}

func seedV5DomainStore(t *testing.T) (*Store, string) {
	t.Helper()
	s, path := seedV4SearchStore(t)
	if _, err := s.db.Exec(schemaV5 + `INSERT INTO schema_migrations VALUES(5,5); PRAGMA user_version=5;`); err != nil {
		t.Fatal(err)
	}
	return s, path
}

func TestSearchDomainsMigrationBackfillsBatchesAndPreservesProjection(t *testing.T) {
	s, path := seedV5DomainStore(t)
	// Current writer creates 300 additional legacy facts for the fixture only.
	if _, err := s.db.Exec(schemaV6 + schemaV7); err != nil {
		t.Fatal(err)
	}
	values := make([]string, 300)
	dates := make([]time.Time, 300)
	for i := range values {
		values[i] = "u@EXAMPLE.NET"
		dates[i] = eventSearchQuery(SearchSenderDomain, "").From.Add(time.Hour)
	}
	seedSearchEvents(t, s, values, dates)
	if _, err := s.db.Exec(`DROP TABLE event_search_domains; DROP TABLE purged_records;`); err != nil {
		t.Fatal(err)
	}
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
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatal("migration changed projection", err)
	}
	after, err := s.CorrelationFacts(ctx, scope, 10)
	if err != nil || !reflect.DeepEqual(after, facts) {
		t.Fatal("migration changed facts", err)
	}
	cp, found, err := s.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !found || cp != testBatch().Checkpoints[0] {
		t.Fatal("checkpoint lost", err)
	}
	if count(t, s, "event_search_domains") != 301 || count(t, s, "schema_migrations") != schemaVersion {
		t.Fatal("backfill missed batch boundary")
	}
	query := eventSearchQuery(SearchRecipientDomain, "example.net")
	query.Limit = 200
	first, err := s.SearchEvents(ctx, query)
	if err != nil || len(first.Hits) != 200 || first.Next == nil {
		t.Fatal("backfilled first page", err)
	}
	query.After = first.Next
	second, err := s.SearchEvents(ctx, query)
	if err != nil || len(second.Hits) != 100 || second.Next != nil {
		t.Fatal("backfilled second page", err)
	}
	query = eventSearchQuery(SearchRecipientDomain, "example.org")
	query.Instance = "mx-a"
	page, err := s.SearchEvents(ctx, query)
	if err != nil || len(page.Hits) != 1 {
		t.Fatal("old address absent", err)
	}
	if _, err := s.db.Exec(`UPDATE events SET recipient='changed@example.org' WHERE instance='mx-a'`); err == nil {
		t.Fatal("immutable projection trigger weakened")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if count(t, s, "event_search_domains") != 301 || count(t, s, "schema_migrations") != schemaVersion {
		t.Fatal("reopen duplicated backfill")
	}
}

func TestSearchDomainsMigrationAndIngestionRollback(t *testing.T) {
	s, _ := seedV5DomainStore(t)
	defer s.Close()
	if _, err := s.db.Exec(`CREATE TRIGGER stop_v6 BEFORE INSERT ON schema_migrations WHEN NEW.version=6 BEGIN SELECT RAISE(ABORT,'synthetic v6 failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(context.Background(), s.db, 6); err == nil {
		t.Fatal("failed migration accepted")
	}
	var version, table int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 5 {
		t.Fatal("migration advanced version", err)
	}
	if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='event_search_domains'`).Scan(&table); err != nil || table != 0 {
		t.Fatal("partial derived table survived", err)
	}
	if count(t, s, "schema_migrations") != 5 || count(t, s, "events") != 1 {
		t.Fatal("rollback lost old facts/history")
	}
	current, _ := openTestStore(t)
	if _, err := current.db.Exec(`CREATE TRIGGER stop_domain BEFORE INSERT ON event_search_domains BEGIN SELECT RAISE(ABORT,'synthetic search failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := current.Commit(context.Background(), testBatch()); err == nil {
		t.Fatal("partial domain write committed")
	}
	if count(t, current, "events") != 0 || count(t, current, "raw_records") != 0 || count(t, current, "checkpoints") != 0 {
		t.Fatal("failed domain insert advanced ingestion")
	}
	if _, err := current.db.Exec(`DROP TRIGGER stop_domain`); err != nil {
		t.Fatal(err)
	}
	if err := current.Commit(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	if err := current.Commit(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	if count(t, current, "events") != 1 || count(t, current, "event_search_domains") != 1 {
		t.Fatal("retry duplicated domain rows")
	}
	if _, err := current.db.Exec(`DELETE FROM raw_records`); err != nil {
		t.Fatal(err)
	}
	if count(t, current, "event_search_domains") != 0 {
		t.Fatal("orphan search row after fact deletion")
	}
}
