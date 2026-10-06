package sqlite

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/parser/postfix"
	"github.com/Coubiac/QueueAtlas/internal/parser/syslog"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

func storeCorrelationCorpus(t *testing.T, s *Store, name, sourceID, instance string, knownDate, reverse bool) []correlation.Fact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "postfix", name+".log"))
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)
	origin := sourceID + "-" + name
	batch := source.Batch{Source: source.Identity{ID: sourceID, Kind: "file", Name: "synthetic", TrustedHost: instance},
		Origins: []source.Origin{{ID: origin, Path: "/synthetic/" + name, Fingerprint: "synthetic-" + name, FirstSeen: when}}}
	opts := postfix.Options{SourceID: sourceID}
	if knownDate {
		opts.Time = syslog.TimeContext{Year: 2026, Location: time.UTC}
	}
	var facts []correlation.Fact
	var offset int64
	for _, line := range bytes.SplitAfter(raw, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		end := offset + int64(len(line))
		o := postfix.Parse(line, opts)
		facts = append(facts, correlation.Fact{Ref: correlation.FactRef{SourceID: sourceID, OriginID: origin, Start: offset, End: end}, Instance: instance, Observation: o})
		batch.Records = append(batch.Records, source.Record{OriginID: origin, Start: offset, End: end, Raw: line, ReadAt: when, Observation: o})
		offset = end
	}
	batch.Checkpoints = []source.Position{{OriginID: origin, Offset: offset, AnchorHash: "synthetic-tail"}}
	if reverse {
		for i, j := 0, len(batch.Records)-1; i < j; i, j = i+1, j-1 {
			batch.Records[i], batch.Records[j] = batch.Records[j], batch.Records[i]
		}
	}
	if err := s.Commit(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	return facts
}

func queueScope(facts []correlation.Fact) CorrelationScope {
	seen := make(map[correlation.QueueKey]bool)
	var scope CorrelationScope
	for _, f := range facts {
		if f.Observation.QueueID == "" {
			continue
		}
		key := correlation.QueueKey{Instance: f.Instance, QueueID: f.Observation.QueueID}
		if !seen[key] {
			scope.Queues = append(scope.Queues, key)
			seen[key] = true
		}
	}
	return scope
}

func TestCorrelationFactsPreserveStoredValuesAndIgnoreInsertionIDs(t *testing.T) {
	left, _ := openTestStore(t)
	right, _ := openTestStore(t)
	var want []correlation.Fact
	for _, name := range []string{"11-filter-reinjection", "09-bounce", "06-noqueue-client-reject"} {
		want = append(want, storeCorrelationCorpus(t, left, name, name, "trusted", true, false)...)
		storeCorrelationCorpus(t, right, name, name, "trusted", true, true)
	}
	// Same declared hosts/queue IDs on another configured instance are excluded.
	storeCorrelationCorpus(t, left, "11-filter-reinjection", "foreign", "other-instance", true, false)
	scope := queueScope(want)
	scope.UnqueuedInstances = []string{"trusted"}
	sort.Slice(want, func(i, j int) bool {
		a, b := want[i].Ref, want[j].Ref
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.OriginID != b.OriginID {
			return a.OriginID < b.OriginID
		}
		return a.Start < b.Start
	})
	got, err := left.CorrelationFacts(context.Background(), scope, 100)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted facts changed: %v", err)
	}
	other, err := right.CorrelationFacts(context.Background(), scope, 100)
	if err != nil || !reflect.DeepEqual(other, got) {
		t.Fatal("insertion IDs affected canonical facts")
	}
	opts := correlation.LinkOptions{Window: time.Minute, SMTPBindings: []correlation.SMTPBinding{{FromInstance: "trusted", Relay: "127.0.0.1[127.0.0.1]:10024", ToInstance: "trusted"}}}
	a, err := correlation.BuildProjection(got, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	b, err := correlation.BuildProjection(other, 100, opts)
	if err != nil || !reflect.DeepEqual(a, b) || len(a.Links) != 2 || len(a.Prequeue.Sessions) != 1 {
		t.Fatalf("SQLite projections differ: %v", err)
	}
	for _, link := range a.Links {
		if link.Observed.State != correlation.LinkCorroborated {
			t.Fatal("native proof lost during read")
		}
	}
	for i := range got {
		if got[i].Observation.Fields != nil && got[i].Observation.Timestamp.Value != nil {
			got[i].Observation.Fields["from"] = "changed"
			*got[i].Observation.Timestamp.Value = time.Time{}
			break
		}
	}
	fresh, err := left.CorrelationFacts(context.Background(), scope, 100)
	if err != nil || !reflect.DeepEqual(fresh, want) {
		t.Fatal("read output altered persisted observations")
	}
}

func TestCorrelationFactsKeepUndatedAndSeparateOriginsWithoutReparsing(t *testing.T) {
	s, _ := openTestStore(t)
	live := storeCorrelationCorpus(t, s, "11-filter-reinjection", "live", "trusted", true, false)
	archive := storeCorrelationCorpus(t, s, "11-filter-reinjection", "archive", "trusted", false, false)
	scope := queueScope(live)
	got, err := s.CorrelationFacts(context.Background(), scope, 100)
	if err != nil || len(got) != len(live)+len(archive) {
		t.Fatalf("origins collapsed: %v", err)
	}
	for _, f := range got {
		if f.Ref.SourceID == "archive" && (f.Observation.Timestamp.Value != nil || f.Observation.Timestamp.Year != 0 || f.Observation.Timestamp.Quality != "wall_only") {
			t.Fatal("reader invented a date context")
		}
	}
	projection, err := correlation.BuildProjection(got, 100, correlation.LinkOptions{Window: time.Minute})
	if err != nil || len(projection.Unresolved) != 2 || len(projection.Links) != 2 {
		t.Fatalf("uncertainty lost: %v", err)
	}
	for _, link := range projection.Links {
		if link.Observed.State != correlation.LinkCandidate || link.To != nil {
			t.Fatal("reader implied continuity")
		}
	}
	// SQL syntax and wildcard-looking IDs are literal parameters, not selectors.
	literal, err := s.CorrelationFacts(context.Background(), CorrelationScope{Queues: []correlation.QueueKey{{Instance: "trusted", QueueID: "' OR 1=1 --"}}}, 100)
	if err != nil || len(literal) != 0 {
		t.Fatal("scope value changed SQL selection")
	}
}

func TestCorrelationFactsRefuseLimitsInvalidScopesAndCancellation(t *testing.T) {
	s, _ := openTestStore(t)
	facts := storeCorrelationCorpus(t, s, "11-filter-reinjection", "live", "trusted", true, false)
	scope := queueScope(facts)
	got, err := s.CorrelationFacts(context.Background(), scope, len(facts))
	if err != nil || len(got) != len(facts) {
		t.Fatal("exact row limit refused")
	}
	for _, limit := range []int{0, correlation.MaxPartitionFacts + 1, len(facts) - 1} {
		got, err := s.CorrelationFacts(context.Background(), scope, limit)
		if !errors.Is(err, correlation.ErrPartitionLimit) || got != nil {
			t.Fatalf("truncated snapshot escaped: %v", err)
		}
	}
	for _, bad := range []CorrelationScope{
		{}, {Queues: []correlation.QueueKey{{Instance: "", QueueID: "ABC"}}},
		{Queues: []correlation.QueueKey{{Instance: "trusted", QueueID: strings.Repeat("A", 33)}}},
		{Queues: []correlation.QueueKey{{Instance: "trusted", QueueID: "ABC"}, {Instance: "trusted", QueueID: "ABC"}}},
		{UnqueuedInstances: []string{"trusted", "trusted"}}, {UnqueuedInstances: []string{"bad\nvalue"}},
		{UnqueuedInstances: make([]string, MaxCorrelationScopeParts+1)},
	} {
		got, err := s.CorrelationFacts(context.Background(), bad, 100)
		if err != ErrCorrelationScope || got != nil {
			t.Fatalf("invalid scope escaped: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err = s.CorrelationFacts(ctx, scope, 100)
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Fatalf("cancel returned facts: %v", err)
	}
}

func TestCorrelationFactsKeepPersistedOutOfRangeDateUnknown(t *testing.T) {
	s, _ := openTestStore(t)
	batch := testBatch()
	batch.Source.TrustedHost = "" // persisted instance falls back to source ID
	raw := []byte("<22>1 2500-01-01T00:00:00Z declared-host postfix/qmgr 1 - - A1B2C3: from=<alice@example.org>, size=10, nrcpt=1 (queue active)\n")
	o := postfix.Parse(raw, postfix.Options{SourceID: "mail"})
	if o.Timestamp.Value == nil {
		t.Fatal("fixture did not parse explicit distant year")
	}
	batch.Records[0].Raw = raw
	batch.Records[0].End = int64(len(raw))
	batch.Records[0].Observation = o
	batch.Checkpoints[0].Offset = int64(len(raw))
	if err := s.Commit(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	got, err := s.CorrelationFacts(context.Background(), CorrelationScope{Queues: []correlation.QueueKey{{Instance: "mail", QueueID: "A1B2C3"}}}, 10)
	if err != nil || len(got) != 1 || got[0].Instance != "mail" {
		t.Fatalf("fallback instance/read: %v", err)
	}
	o.Timestamp.Value = nil // existing writer deliberately refuses wrapped UnixNano
	if !reflect.DeepEqual(got[0].Observation, o) {
		t.Fatal("stored date/metadata reinterpreted")
	}
	projection, err := correlation.BuildQueueInstances(got, 10)
	if err != nil || len(projection.Instances) != 0 || len(projection.Unresolved) != 1 {
		t.Fatal("out-of-range stored date gained an instance")
	}
}

func TestCorrelationFactsMalformedStoredFieldsExposeNoPartialSnapshot(t *testing.T) {
	for _, malformed := range []string{
		`null`, `[]`, `{"fields":{"status":1},"present":{}}`,
		`{"fields":{},"present":{},"unknown":"private-address"}`,
		`{"fields":{"from":null},"present":{"from":true}}`,
		`{"fields":{},"present":{"to":null}}`,
		`{"fields":{},"present":{},"fields":{"from":""}}`,
		`{"Fields":{},"present":{}}`, `{"fields":{}}`,
		`{"fields":{"from":"alice","from":""},"present":{"from":true}}`,
		`{"fields":{},"present":{"from":true,"from":false}}`,
	} {
		t.Run(malformed, func(t *testing.T) {
			s, _ := openTestStore(t)
			facts := storeCorrelationCorpus(t, s, "11-filter-reinjection", "live", "trusted", true, false)
			if _, err := s.db.Exec(`UPDATE events SET fields_json=? WHERE id=(SELECT max(id) FROM events)`, malformed); err != nil {
				t.Fatal(err)
			}
			got, err := s.CorrelationFacts(context.Background(), queueScope(facts), 100)
			if err != ErrCorrelationStoredFields || got != nil || strings.Contains(err.Error(), "private-address") {
				t.Fatalf("stored error leaked partial facts/content: %v", err)
			}
		})
	}
}

func TestCorrelationFactsStoredConversionErrorDoesNotExposeValue(t *testing.T) {
	s, _ := openTestStore(t)
	facts := storeCorrelationCorpus(t, s, "11-filter-reinjection", "live", "trusted", true, false)
	if _, err := s.db.Exec(`UPDATE events SET time_utc_ns=? WHERE id=(SELECT max(id) FROM events)`, "synthetic-private@example.org"); err != nil {
		t.Fatal(err)
	}
	got, err := s.CorrelationFacts(context.Background(), queueScope(facts), 100)
	if err != ErrCorrelationStoredFact || got != nil || strings.Contains(err.Error(), "synthetic-private@example.org") {
		t.Fatal("stored conversion leaked value or partial output")
	}
}
