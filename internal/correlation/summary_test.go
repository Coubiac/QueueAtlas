package correlation

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/model"
)

func TestSummariesCountRecipientsNotAttemptsOrExpirationReports(t *testing.T) {
	cases := []struct {
		name        string
		counts      RecipientCounts
		expirations int
	}{
		{"04-multi-recipient-mixed", RecipientCounts{Delivered: 1, Deferred: 1, Bounced: 1}, 0},
		{"08-multiple-retries", RecipientCounts{Sent: 1}, 0},
		{"10-expired", RecipientCounts{Deferred: 1}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := corpusFacts(t, tc.name, "a", "trusted", true)
			out, err := BuildSummaries(facts, 100)
			if err != nil || len(out.Queues) != 1 || out.Queues[0].Counts != tc.counts || out.Queues[0].ExpirationReports != tc.expirations {
				t.Fatalf("summary: %#v %v", out, err)
			}
			if tc.name == "08-multiple-retries" && len(out.Queues[0].Queue.Recipients[0].Attempts) != 3 {
				t.Fatal("lost retries")
			}
			rng := rand.New(rand.NewSource(95))
			for i := 0; i < 20; i++ {
				shuffled := append([]Fact(nil), facts...)
				rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
				got, err := BuildSummaries(shuffled, 100)
				if err != nil || !reflect.DeepEqual(got, out) {
					t.Fatalf("permutation%d: %v", i, err)
				}
			}
		})
	}
}

func TestSummariesNeverCertifyCoverageFromReceiptRemovalOrNrcpt(t *testing.T) {
	facts := corpusFacts(t, "08-multiple-retries", "a", "trusted", true)
	for i := range facts {
		facts[i].Observation.Timestamp.Quality = model.TimeExplicitOffset
	}
	out, err := BuildSummaries(facts, 100)
	if err != nil || len(out.Queues) != 1 || !reflect.DeepEqual(out.Queues[0].Reserves, []SummaryReserve{ReserveCoverageUnproven}) {
		t.Fatalf("complete-looking snapshot: %#v %v", out, err)
	}
	// Reported nrcpt is not a certificate of all addresses or alias expansion.
	facts[0].Observation.Fields["nrcpt"] = "999 (queue active)"
	got, err := BuildSummaries(facts, 100)
	if err != nil || !reflect.DeepEqual(got, out) {
		t.Fatalf("nrcpt manufactured coverage/recipients: %#v %v", got, err)
	}
	partial := corpusFacts(t, "17-partial-log", "a", "trusted", true)
	got, err = BuildSummaries(partial, 100)
	want := []SummaryReserve{ReserveCoverageUnproven, ReserveReceiptNotObserved, ReserveNonExplicitTime}
	if err != nil || !reflect.DeepEqual(got.Queues[0].Reserves, want) || got.Queues[0].Counts != (RecipientCounts{Deferred: 1}) {
		t.Fatalf("partial promoted: %#v %v", got, err)
	}
	got, err = BuildSummaries(facts[:1], 100)
	want = []SummaryReserve{ReserveCoverageUnproven, ReserveRemovalNotObserved, ReserveNoRecipientsObserved}
	if err != nil || !reflect.DeepEqual(got.Queues[0].Reserves, want) || got.Queues[0].Counts != (RecipientCounts{}) {
		t.Fatalf("no attempts: %#v %v", got, err)
	}
}

func TestSummariesExposeUnknownAddressTieAndUnprojectedFacts(t *testing.T) {
	facts := corpusFacts(t, "07-deferred-then-sent", "a", "trusted", true)
	date := *facts[2].Observation.Timestamp.Value
	facts[4].Observation.Timestamp.Value = &date
	out, err := BuildSummaries(facts, 100)
	want := []SummaryReserve{ReserveCoverageUnproven, ReserveNonExplicitTime, ReserveLatestOrderUncertain, ReserveUnknownResult}
	if err != nil || !reflect.DeepEqual(out.Queues[0].Reserves, want) || out.Queues[0].Counts != (RecipientCounts{Unknown: 1}) {
		t.Fatalf("tie hidden: %#v %v", out, err)
	}
	facts = corpusFacts(t, "04-multi-recipient-mixed", "a", "trusted", true)
	facts[3].Observation.Fields["to"] = ""
	delete(facts[4].Observation.Present, "to")
	out, err = BuildSummaries(facts, 100)
	want = []SummaryReserve{ReserveCoverageUnproven, ReserveRemovalNotObserved, ReserveNonExplicitTime, ReserveAddressUnspecified, ReserveUnknownResult, ReserveUnprojectedDeliveries}
	if err != nil || !reflect.DeepEqual(out.Queues[0].Reserves, want) || out.Queues[0].Counts != (RecipientCounts{Unknown: 1, Bounced: 1}) || !reflect.DeepEqual(out.Queues[0].Queue.UnprojectedDeliveries, []FactRef{facts[4].Ref}) {
		t.Fatalf("omitted/empty hidden: %#v %v", out, err)
	}
}

func TestSummariesPreserveIndependentCandidatesUnresolvedAndBounds(t *testing.T) {
	facts := corpusFacts(t, "10-expired", "live", "trusted", true)
	facts = append(facts, corpusFacts(t, "10-expired", "archive", "trusted", true)...)
	out, err := BuildSummaries(facts, 100)
	want := []SummaryReserve{ReserveCoverageUnproven, ReserveNonExplicitTime, ReserveCrossStreamUncertain}
	if err != nil || len(out.Queues) != 2 {
		t.Fatalf("origins: %#v %v", out, err)
	}
	for _, q := range out.Queues {
		if !reflect.DeepEqual(q.Reserves, want) || q.Counts != (RecipientCounts{Deferred: 1}) || q.ExpirationReports != 1 {
			t.Fatalf("origins merged: %#v", q)
		}
	}
	undated := corpusFacts(t, "10-expired", "a", "trusted", false)
	out, err = BuildSummaries(undated, 100)
	if err != nil || len(out.Queues) != 0 || len(out.Unresolved) != 1 || len(out.Unresolved[0].Facts) != len(undated) {
		t.Fatalf("unresolved lost: %#v %v", out, err)
	}
	noqueue := corpusFacts(t, "05-rcpt-reject", "a", "trusted", true)
	out, err = BuildSummaries(noqueue, 100)
	if err != nil || len(out.Other) != 2 || len(out.Queues) != 1 || out.Queues[0].Counts != (RecipientCounts{Delivered: 1}) {
		t.Fatalf("NOQUEUE merged: %#v %v", out, err)
	}
	for _, limit := range []int{0, 1, MaxPartitionFacts + 1} {
		if got, err := BuildSummaries(facts, limit); err != ErrPartitionLimit || !reflect.DeepEqual(got, SummaryPartition{}) {
			t.Fatalf("partial invalid snapshot: %#v %v", got, err)
		}
	}
}
