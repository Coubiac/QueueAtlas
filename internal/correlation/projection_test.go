package correlation

import (
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func TestProjectionComposesEvidenceWithoutImprovingRecipientResults(t *testing.T) {
	var facts []Fact
	for _, name := range []string{"04-multi-recipient-mixed", "09-bounce", "10-expired", "11-filter-reinjection", "13-reused-queue-id", "06-noqueue-client-reject"} {
		facts = append(facts, corpusFacts(t, name, name, "trusted", true)...)
	}
	opts := linkOptions()
	want, err := BuildProjection(facts, 100, opts)
	if err != nil || len(want.Queues) != 8 || len(want.Links) != 3 || len(want.Prequeue.Sessions) != 1 || len(want.Prequeue.Prequeue.Attempts) != 1 {
		t.Fatalf("composition: %#v %v", want, err)
	}
	before, err := BuildSummaries(facts, 100)
	if err != nil {
		t.Fatal(err)
	}
	keys := make(map[QueueInstanceKey]bool)
	var summaries []GenerationSummary
	for _, q := range want.Queues {
		if q.Key.Revision != want.Revision || keys[q.Key] {
			t.Fatal("queue identity from another revision or duplicated")
		}
		keys[q.Key] = true
		summaries = append(summaries, q.Summary)
	}
	if !reflect.DeepEqual(summaries, before.Queues) || !reflect.DeepEqual(want.Unresolved, before.Unresolved) || !reflect.DeepEqual(want.Other, before.Other) {
		t.Fatal("composition changed recipient results, reserves or facts")
	}
	corroborated := 0
	for _, link := range want.Links {
		if link.From == nil || !keys[*link.From] || link.Observed.From == nil {
			t.Fatal("source endpoint not bound to this revision")
		}
		if link.Observed.State == LinkCorroborated {
			corroborated++
			if link.To == nil || !keys[*link.To] || *link.From == *link.To {
				t.Fatal("corroborated target absent, foreign or self")
			}
		} else if link.To != nil {
			t.Fatal("candidate was assigned a target identity")
		}
	}
	if corroborated != 2 {
		t.Fatal("unexpected corroborated link count")
	}
	assertProjectionFacts(t, want, facts)
	rng := rand.New(rand.NewSource(104))
	for i := 0; i < 25; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := BuildProjection(shuffled, 100, opts)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("arrival order changed composed projection: %v", err)
		}
	}
	// Endpoint pointers and options are copied, not aliases to queues/caller.
	want.Links[0].From.Revision = "changed"
	want.LinkOptions.SMTPBindings[0].ToInstance = "changed"
	want.Queues[0].Summary.Reserves[0] = "changed"
	got, err := BuildProjection(facts, 100, opts)
	if err != nil || got.Queues[0].Key.Revision != got.Revision || got.LinkOptions.SMTPBindings[0].ToInstance != "trusted" || got.Queues[0].Summary.Reserves[0] != ReserveCoverageUnproven {
		t.Fatal("projection borrowed caller or another rebuild")
	}
}

func TestProjectionRevisionIncludesConfigurationInCanonicalOrder(t *testing.T) {
	facts := corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	opts := linkOptions()
	opts.SMTPBindings = append(opts.SMTPBindings, SMTPBinding{"unused", "relay.invalid[192.0.2.1]:25", "other"})
	want, err := BuildProjection(facts, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*LinkOptions){
		func(o *LinkOptions) { o.Window += time.Second },
		func(o *LinkOptions) { o.SMTPBindings = nil },
		func(o *LinkOptions) { o.SMTPBindings[0].ToInstance = "other" },
		func(o *LinkOptions) { o.SMTPBindings[1].ToInstance = "changed-unused" },
	} {
		changed := LinkOptions{opts.Window, append([]SMTPBinding(nil), opts.SMTPBindings...)}
		change(&changed)
		got, err := BuildProjection(facts, 100, changed)
		if err != nil || got.Revision == want.Revision || got.InputRevision != want.InputRevision || got.Queues[0].Key == want.Queues[0].Key {
			t.Fatalf("changed options reused full revision: %v", err)
		}
	}
	reversed := LinkOptions{opts.Window, []SMTPBinding{opts.SMTPBindings[1], opts.SMTPBindings[0]}}
	got, err := BuildProjection(facts, 100, reversed)
	if err != nil || !reflect.DeepEqual(got, want) || reversed.SMTPBindings[0] != opts.SMTPBindings[1] {
		t.Fatal("binding order affected revision or caller was reordered")
	}
}

func TestProjectionRetainsAmbiguousAndUndatedFactsWithoutEndpoints(t *testing.T) {
	facts := corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	before, err := BuildProjection(facts, 100, linkOptions())
	if err != nil {
		t.Fatal(err)
	}
	archive := corpusFacts(t, "11-filter-reinjection", "archive", "trusted", false)
	facts = append(facts, archive...)
	got, err := BuildProjection(facts, 100, linkOptions())
	if err != nil || got.Revision == before.Revision || len(got.Unresolved) != 2 || len(got.Links) != 2 {
		t.Fatalf("new unknown provenance ignored: %#v %v", got, err)
	}
	for _, q := range got.Queues {
		if !q.Summary.Queue.Generation.CrossStreamUncertain || q.Key.Revision == before.Revision {
			t.Fatal("uncertainty or late revision hidden")
		}
	}
	for _, link := range got.Links {
		if link.Observed.State != LinkCandidate || link.To != nil || (link.Observed.From == nil) != (link.From == nil) {
			t.Fatal("unresolved/ambiguous facts gained a corroborated endpoint")
		}
	}
	assertProjectionFacts(t, got, facts)
}

func TestProjectionRefusalsExposeNoPartialRevision(t *testing.T) {
	base := corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	for _, tc := range []struct {
		facts []Fact
		limit int
		opts  LinkOptions
		err   error
	}{
		{base, 100, LinkOptions{}, ErrLinkOptions},
		{base, 0, linkOptions(), ErrPartitionLimit},
		{base, 1, linkOptions(), ErrPartitionLimit},
		{append(append([]Fact(nil), base...), base[0]), 100, linkOptions(), ErrPartitionOverlap},
	} {
		got, err := BuildProjection(tc.facts, tc.limit, tc.opts)
		if err != tc.err || !reflect.DeepEqual(got, Projection{}) {
			t.Fatalf("refusal leaked partial output/revision: %#v %v", got, err)
		}
	}
	empty, err := BuildProjection(nil, 1, linkOptions())
	if err != nil || len(empty.Queues) != 0 || len(empty.Links) != 0 || len(empty.Revision) != 64 || empty.Revision == empty.InputRevision {
		t.Fatalf("empty composed snapshot: %#v %v", empty, err)
	}
}

func assertProjectionFacts(t *testing.T, projection Projection, facts []Fact) {
	t.Helper()
	seen := make(map[FactRef]int)
	for _, q := range projection.Queues {
		for _, ref := range q.Summary.Queue.Generation.Facts {
			seen[ref]++
		}
	}
	for _, u := range projection.Unresolved {
		for _, ref := range u.Facts {
			seen[ref]++
		}
	}
	for _, ref := range projection.Other {
		seen[ref]++
	}
	if len(seen) != len(facts) {
		t.Fatal("composition lost immutable references")
	}
	for _, f := range facts {
		if seen[f.Ref] != 1 {
			t.Fatal("reference missing or multiply owned in primary partition")
		}
	}
}
