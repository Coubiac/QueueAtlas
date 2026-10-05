package correlation

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/parser/postfix"
)

func linkOptions() LinkOptions {
	return LinkOptions{Window: time.Minute, SMTPBindings: []SMTPBinding{{"trusted", "127.0.0.1[127.0.0.1]:10024", "trusted"}}}
}

func TestQueueLinksCorroborateBothEndsWithoutChangingRecipientResults(t *testing.T) {
	for _, name := range []string{"11-filter-reinjection", "21-orig-to-alias", "09-bounce"} {
		t.Run(name, func(t *testing.T) {
			facts := corpusFacts(t, name, "live", "trusted", true)
			before, err := BuildSummaries(facts, 100)
			if err != nil {
				t.Fatal(err)
			}
			opts := linkOptions()
			out, err := BuildQueueLinks(facts, 100, opts)
			if err != nil || len(out.Links) != 1 {
				t.Fatalf("links: %#v %v", out, err)
			}
			link := out.Links[0]
			if link.State != LinkCorroborated || link.Reason != "" || link.From == nil || link.To == nil || *link.From == *link.To || len(link.Evidence) < 3 || !link.HasNonExplicitTime {
				t.Fatalf("two-ended evidence: %#v", link)
			}
			if name == "11-filter-reinjection" && (link.Binding == nil || link.Hint.TargetInstance.Present || link.TargetInstance != (Field{"trusted", true})) {
				t.Fatalf("config replaced native hint: %#v", link)
			}
			rng := rand.New(rand.NewSource(101))
			for i := 0; i < 25; i++ {
				shuffled := append([]Fact(nil), facts...)
				rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
				got, err := BuildQueueLinks(shuffled, 100, opts)
				if err != nil || !reflect.DeepEqual(got, out) {
					t.Fatalf("permutation%d: %v", i, err)
				}
			}
			after, err := BuildSummaries(facts, 100)
			if err != nil || !reflect.DeepEqual(after, before) {
				t.Fatal("link changed recipient results or immutable facts")
			}
			*out.Links[0].From = FactRef{}
			out.Links[0].Evidence[0] = FactRef{}
			if out.Links[0].Binding != nil {
				out.Links[0].Binding.Relay = "changed"
			}
			if facts[0].Ref.SourceID != "live" || opts.SMTPBindings[0].Relay == "changed" {
				t.Fatal("output borrowed input references or configuration")
			}
		})
	}
}

func TestQueueLinksRemainCandidateWithoutMappingTargetOrMetadataProof(t *testing.T) {
	for _, tc := range []struct {
		name   string
		corpus string
		edit   func([]Fact, *LinkOptions)
		reason LinkReason
	}{
		{"no mapping even loopback", "11-filter-reinjection", func(_ []Fact, o *LinkOptions) { o.SMTPBindings = nil }, LinkTargetInstanceUnknown},
		{"no target", "12-unverified-reinjection", func([]Fact, *LinkOptions) {}, LinkTargetAbsent},
		{"wrong mapped instance", "11-filter-reinjection", func(_ []Fact, o *LinkOptions) { o.SMTPBindings[0].ToInstance = "other" }, LinkTargetAbsent},
		{"sender contradiction", "11-filter-reinjection", func(f []Fact, _ *LinkOptions) { f[6].Observation.Fields["from"] = "different@example.org" }, LinkEvidenceInsufficient},
		{"Message-ID contradiction", "11-filter-reinjection", func(f []Fact, _ *LinkOptions) { f[5].Observation.Fields["message-id"] = "different@example.org" }, LinkEvidenceInsufficient},
		{"Message-ID absent", "11-filter-reinjection", func(f []Fact, _ *LinkOptions) { delete(f[5].Observation.Present, "message-id") }, LinkEvidenceInsufficient},
		{"recipient contradiction", "11-filter-reinjection", func(f []Fact, _ *LinkOptions) { f[7].Observation.Fields["to"] = "other@example.org" }, LinkEvidenceInsufficient},
		{"source date unknown", "11-filter-reinjection", func(f []Fact, _ *LinkOptions) { f[0].Observation.Timestamp.Value = nil }, LinkSourceUnresolved},
		{"target date unknown", "11-filter-reinjection", func(f []Fact, _ *LinkOptions) { f[4].Observation.Timestamp.Value = nil }, LinkTargetAmbiguous},
		{"outside window", "11-filter-reinjection", func(_ []Fact, o *LinkOptions) { o.Window = time.Millisecond }, LinkEvidenceInsufficient},
		{"bounce sender not null", "09-bounce", func(f []Fact, _ *LinkOptions) { f[4].Observation.Fields["from"] = "other@example.org" }, LinkEvidenceInsufficient},
		{"bounce recipient not sender", "09-bounce", func(f []Fact, _ *LinkOptions) { f[5].Observation.Fields["to"] = "other@example.org" }, LinkEvidenceInsufficient},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := corpusFacts(t, tc.corpus, "live", "trusted", true)
			opts := linkOptions()
			tc.edit(facts, &opts)
			out, err := BuildQueueLinks(facts, 100, opts)
			if err != nil || len(out.Links) != 1 || out.Links[0].State != LinkCandidate || out.Links[0].To != nil || out.Links[0].Reason != tc.reason || !reflect.DeepEqual(out.Links[0].Evidence, []FactRef{out.Links[0].Hint.Evidence}) {
				t.Fatalf("unproved link: %#v %v", out, err)
			}
		})
	}
}

func TestQueueLinksNeverChooseBetweenOriginsOrRecycledTargetIDs(t *testing.T) {
	facts := corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	duplicateTarget := corpusFacts(t, "11-filter-reinjection", "archive", "trusted", true)[4:]
	facts = append(facts, duplicateTarget...)
	out, err := BuildQueueLinks(facts, 100, linkOptions())
	if err != nil || len(out.Links) != 1 || out.Links[0].Reason != LinkTargetAmbiguous || out.Links[0].To != nil {
		t.Fatalf("target origin chosen: %#v %v", out, err)
	}
	facts = append(corpusFacts(t, "11-filter-reinjection", "live", "trusted", true), corpusFacts(t, "11-filter-reinjection", "archive", "trusted", true)[:4]...)
	out, err = BuildQueueLinks(facts, 100, linkOptions())
	if err != nil || len(out.Links) != 2 || out.Links[0].Reason != LinkSourceAmbiguous || out.Links[1].Reason != LinkSourceAmbiguous {
		t.Fatalf("source origins merged: %#v %v", out, err)
	}
	facts = corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)[:4]
	line := strings.ReplaceAll(facts[3].Observation.Raw, "11F1B2C3D4", "13A1B2C3D4")
	o := postfix.Parse([]byte(line), postfix.Options{SourceID: "live"})
	o.Timestamp = facts[3].Observation.Timestamp
	date := facts[3].Observation.Timestamp.Value.Add(123 * time.Second)
	o.Timestamp.Value = &date
	facts[3].Observation = o
	facts = append(facts, corpusFacts(t, "13-reused-queue-id", "live", "trusted", true)...)
	opts := linkOptions()
	opts.Window = 24 * time.Hour
	out, err = BuildQueueLinks(facts, 100, opts)
	if err != nil || len(out.Links) != 1 || out.Links[0].State != LinkCandidate || out.Links[0].Reason != LinkTargetAmbiguous {
		t.Fatalf("recycled target chosen by matching metadata: %#v %v", out, err)
	}
}

func TestQueueLinksRequireLaterTargetReceiptAndRejectSelfReference(t *testing.T) {
	facts := corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	for i := 4; i < len(facts); i++ {
		date := facts[i].Observation.Timestamp.Value.Add(-2 * time.Second)
		facts[i].Observation.Timestamp.Value = &date
	}
	out, err := BuildQueueLinks(facts, 100, linkOptions())
	if err != nil || out.Links[0].Reason != LinkEvidenceInsufficient || out.Links[0].State != LinkCandidate {
		t.Fatalf("equal receipt or backwards relation: %#v %v", out, err)
	}
	facts = corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	facts[3].Observation.Message = strings.ReplaceAll(facts[3].Observation.Message, "11F1B2C3D4", "11A1B2C3D4")
	facts[3].Observation.Fields["reply"] = "250 2.0.0 Ok: queued as 11A1B2C3D4"
	out, err = BuildQueueLinks(facts, 100, linkOptions())
	if err != nil || out.Links[0].Reason != LinkSelfReference || out.Links[0].To != nil {
		t.Fatalf("self reference: %#v %v", out, err)
	}
	// Date qualities remain reported under their hypotheses, not silently
	// converted into explicit clock proof.
	facts = corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	for i := range facts {
		facts[i].Observation.Timestamp.Quality = model.TimeExplicitOffset
	}
	out, err = BuildQueueLinks(facts, 100, linkOptions())
	if err != nil || out.Links[0].State != LinkCorroborated || out.Links[0].HasNonExplicitTime {
		t.Fatalf("date quality: %#v %v", out, err)
	}
}

func TestQueueLinksRepeatedMetadataDoesNotMultiplyPositiveProofs(t *testing.T) {
	facts := corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	end := facts[len(facts)-1].Ref.End
	for i := 0; i < 700; i++ {
		extra := facts[2]
		extra.Ref.Start = end
		extra.Ref.End = end + 128
		end = extra.Ref.End
		facts = append(facts, extra)
	}
	out, err := BuildQueueLinks(facts, 1000, linkOptions())
	if err != nil || len(out.Links) != 1 {
		t.Fatalf("rebuild repeated metadata: %v", err)
	}
	if out.Links[0].State != LinkCorroborated || len(out.Links[0].Evidence) > 8 || len(out.Generations.Generations[0].Facts) != 704 {
		t.Fatalf("repeated metadata expanded positive proofs: evidence=%d, err=%v", len(out.Links[0].Evidence), err)
	}
	// Limiting positive references must still detect the final contradictory
	// report; never stop validating after the first metadata observation.
	facts[len(facts)-1].Observation.Fields = map[string]string{"from": "other@example.org"}
	out, err = BuildQueueLinks(facts, 1000, linkOptions())
	if err != nil || len(out.Links) != 1 || out.Links[0].State != LinkCandidate || out.Links[0].Reason != LinkEvidenceInsufficient {
		t.Fatalf("late contradiction ignored: %#v %v", out, err)
	}
}

func TestQueueLinksRejectInvalidOptionsAndSnapshotsWithoutPartialOutput(t *testing.T) {
	facts := corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	for _, opts := range []LinkOptions{
		{}, {Window: -time.Second}, {Window: 24*time.Hour + time.Second},
		{Window: time.Minute, SMTPBindings: make([]SMTPBinding, MaxSMTPBindings+1)},
		{Window: time.Minute, SMTPBindings: []SMTPBinding{{"trusted", "relay", "target"}, {"trusted", "relay", "other"}}},
		{Window: time.Minute, SMTPBindings: []SMTPBinding{{"", "relay", "target"}}},
		{Window: time.Minute, SMTPBindings: []SMTPBinding{{"trusted", "relay\nforged", "target"}}},
		{Window: time.Minute, SMTPBindings: []SMTPBinding{{"trusted", strings.Repeat("x", 1025), "target"}}},
	} {
		if got, err := BuildQueueLinks(facts, 100, opts); err != ErrLinkOptions || !reflect.DeepEqual(got, QueueLinks{}) {
			t.Fatalf("partial invalid rules: %#v %v", got, err)
		}
	}
	for _, tc := range []struct {
		facts []Fact
		limit int
		want  error
	}{
		{facts, 1, ErrPartitionLimit}, {facts, 0, ErrPartitionLimit},
		{append(facts, facts[0]), 100, ErrPartitionOverlap},
	} {
		if got, err := BuildQueueLinks(tc.facts, tc.limit, linkOptions()); err != tc.want || !reflect.DeepEqual(got, QueueLinks{}) {
			t.Fatalf("partial invalid facts: %#v %v", got, err)
		}
	}
}
