package sqlite

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
)

func TestSearchReconstructionReadsCompleteQueueScopeAcrossCriteria(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "dated", "trusted", true, false)
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "undated", "trusted", false, false)
	storeCorrelationCorpus(t, s, "13-reused-queue-id", "foreign", "other", true, false)
	for _, query := range []SearchQuery{
		eventSearchQuery(SearchQueueID, "13A1B2C3D4"),
		eventSearchQuery(SearchMessageID, "first-13@example.org"),
		eventSearchQuery(SearchSender, "alice@example.org"),
		eventSearchQuery(SearchRecipient, "first@example.org"),
		eventSearchQuery(SearchSenderDomain, "EXAMPLE.ORG"),
		eventSearchQuery(SearchRecipientDomain, "EXAMPLE.ORG"),
	} {
		t.Run(string(query.Field), func(t *testing.T) {
			query.Limit = 1
			page, err := s.SearchEvents(ctx, query)
			if err != nil || len(page.Hits) != 1 {
				t.Fatal("search candidate", err)
			}
			hit := page.Hits[0]
			if hit.Ref.SourceID != "dated" || hit.NoQueue || hit.QueueID != "13A1B2C3D4" {
				t.Fatal("search changed candidate identity")
			}
			// Caller explicitly chooses this queue scope. Neither the one-row
			// page nor its date/field filter is a complete correlation input.
			scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: hit.Instance, QueueID: hit.QueueID}}}
			facts, err := s.CorrelationFacts(ctx, scope, 100)
			if err != nil || len(facts) != 16 {
				t.Fatal("page/window lost recycled or undated facts", err, len(facts))
			}
			var undated, afterWindow int
			for _, fact := range facts {
				if fact.Instance != "trusted" || fact.Ref.SourceID == "foreign" {
					t.Fatal("scope included foreign instance")
				}
				if fact.Observation.Timestamp.Value == nil {
					undated++
				} else if !fact.Observation.Timestamp.Value.Before(query.Until) {
					afterWindow++
				}
			}
			if undated != 8 || afterWindow != 4 {
				t.Fatal("full scope inherited search date filter", undated, afterWindow)
			}
			projection, err := s.InstallProjection(ctx, scope, facts, 100, correlation.LinkOptions{Window: time.Minute})
			if err != nil || len(projection.Queues) != 2 || len(projection.Unresolved) != 1 {
				t.Fatal("reconstruction merged cycles or assigned undated stream", err)
			}
			var sent, delivered int
			for _, queue := range projection.Queues {
				if queue.Key.Revision != projection.Revision || !slices.Contains(queue.Summary.Reserves, correlation.ReserveCoverageUnproven) {
					t.Fatal("revision or coverage reserve lost")
				}
				sent += queue.Summary.Counts.Sent
				delivered += queue.Summary.Counts.Delivered
			}
			if sent != 1 || delivered != 1 {
				t.Fatal("transport/local results promoted or merged", sent, delivered)
			}
			if rev, n := installedRevision(t, s, scope); rev != projection.Revision || n != 16 {
				t.Fatal("manifest lost facts excluded by search")
			}
			current, found, err := s.CurrentProjection(ctx, scope, 100)
			if err != nil || !found || !reflect.DeepEqual(current, projection) {
				t.Fatal("current projection lost complete reconstruction", err)
			}
		})
	}
}

func TestSearchReconstructionNoQueueUsesExplicitUnqueuedScope(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	storeCorrelationCorpus(t, s, "06-noqueue-client-reject", "dated", "trusted", true, false)
	storeCorrelationCorpus(t, s, "06-noqueue-client-reject", "undated", "trusted", false, false)
	storeCorrelationCorpus(t, s, "06-noqueue-client-reject", "foreign", "other", true, false)
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "queued", "trusted", true, false)
	query := eventSearchQuery(SearchRecipientDomain, "example.org")
	query.Limit = 1
	page, err := s.SearchEvents(ctx, query)
	if err != nil || len(page.Hits) != 1 || !page.Hits[0].NoQueue || page.Hits[0].QueueID != "" {
		t.Fatal("NOQUEUE search hit", err)
	}
	// This is an explicit instance-wide choice, not a session inferred from a
	// recipient, PID, one result, or search window. It can exceed the read limit.
	scope := CorrelationScope{UnqueuedInstances: []string{page.Hits[0].Instance}}
	facts, err := s.CorrelationFacts(ctx, scope, 100)
	if err != nil || len(facts) != 6 {
		t.Fatal("unqueued complete scope", err, len(facts))
	}
	for _, fact := range facts {
		if fact.Instance != "trusted" || fact.Observation.QueueID != "" || fact.Ref.SourceID == "foreign" || fact.Ref.SourceID == "queued" {
			t.Fatal("NOQUEUE became accepted queue or crossed instance")
		}
	}
	projection, err := s.InstallProjection(ctx, scope, facts, 100, correlation.LinkOptions{Window: time.Minute})
	if err != nil || len(projection.Queues) != 0 || len(projection.Links) != 0 || len(projection.Prequeue.Prequeue.Attempts) != 2 {
		t.Fatal("NOQUEUE reconstructed as accepted delivery", err)
	}
	if len(projection.Prequeue.Sessions) != 1 || !projection.Prequeue.Sessions[0].CoverageUnproven || len(projection.Prequeue.Unassigned) != 1 || projection.Prequeue.Unassigned[0].Reason != correlation.SessionUndated {
		t.Fatal("session boundaries/date reserve lost")
	}
	if _, n := installedRevision(t, s, scope); n != 6 {
		t.Fatal("NOQUEUE manifest lost boundary/undated facts")
	}
	current, found, err := s.CurrentProjection(ctx, scope, 100)
	if err != nil || !found || !reflect.DeepEqual(current, projection) {
		t.Fatal("NOQUEUE current projection", err)
	}
}

func TestSearchReconstructionLateImportRequiresCompleteRefresh(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "initial", "trusted", true, false)
	query := eventSearchQuery(SearchMessageID, "filter-11@example.net")
	query.Limit = 1
	page, err := s.SearchEvents(ctx, query)
	if err != nil || len(page.Hits) != 1 || page.Next == nil || page.Hits[0].QueueID != "11A1B2C3D4" {
		t.Fatal("repeated Message-ID first candidate", err)
	}
	hit := page.Hits[0]
	// Selecting one queue is distinct from selecting every Message-ID match.
	// Linked/reinjected queues are not traversed implicitly.
	scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: hit.Instance, QueueID: hit.QueueID}}}
	facts, err := s.CorrelationFacts(ctx, scope, 100)
	if err != nil || len(facts) != 4 {
		t.Fatal("selected queue expanded from Message-ID", err)
	}
	opts := correlation.LinkOptions{Window: time.Minute}
	initial, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "late", "trusted", true, false)
	if got, found, err := s.CurrentProjection(ctx, scope, 100); !errors.Is(err, ErrProjectionStale) || found || !reflect.DeepEqual(got, correlation.Projection{}) {
		t.Fatal("late import silently reused search-selected revision", err)
	}
	if got, err := s.InstallProjection(ctx, scope, facts, 100, opts); !errors.Is(err, ErrProjectionStale) || !reflect.DeepEqual(got, correlation.Projection{}) {
		t.Fatal("stale input installed", err)
	}
	if got, err := s.CorrelationFacts(ctx, scope, 4); !errors.Is(err, correlation.ErrPartitionLimit) || got != nil {
		t.Fatal("search size truncated correlation input", err)
	}
	if rev, n := installedRevision(t, s, scope); rev != initial.Revision || n != 4 {
		t.Fatal("failed refresh replaced old manifest")
	}
	facts, err = s.CorrelationFacts(ctx, scope, 100)
	if err != nil || len(facts) != 8 {
		t.Fatal("full refresh lost late origin", err)
	}
	for _, fact := range facts {
		if fact.Observation.QueueID != "11A1B2C3D4" {
			t.Fatal("refresh expanded to another queue")
		}
	}
	updated, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil || updated.Revision == initial.Revision || len(updated.Queues) != 2 {
		t.Fatal("refresh merged origins or failed to version", err)
	}
	current, found, err := s.CurrentProjection(ctx, scope, 100)
	if err != nil || !found || !reflect.DeepEqual(current, updated) {
		t.Fatal("refreshed current projection", err)
	}
}
