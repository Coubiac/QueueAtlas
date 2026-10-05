package correlation

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
)

func TestRecipientsKeepMixedResultsAndEveryRetry(t *testing.T) {
	facts := corpusFacts(t, "04-multi-recipient-mixed", "source-a", "trusted-a", true)
	out, err := BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 1 {
		t.Fatalf("rebuild: %#v %v", out, err)
	}
	r := out.Queues[0].Recipients
	want := []DeliveryStatus{DeliveryDelivered, DeliveryBounced, DeliveryDeferred} // lexical one/three/two
	if len(r) != 3 {
		t.Fatalf("recipients: %#v", r)
	}
	for i, recipient := range r {
		if len(recipient.Attempts) != 1 || recipient.ObservedStatus != want[i] || recipient.OrderUncertain || len(recipient.Latest) != 1 {
			t.Fatalf("mixed recipient%d: %#v", i, recipient)
		}
	}
	facts = corpusFacts(t, "08-multiple-retries", "source-a", "trusted-a", true)
	out, err = BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 1 || len(out.Queues[0].Recipients) != 1 {
		t.Fatalf("retry: %#v %v", out, err)
	}
	r = out.Queues[0].Recipients
	if len(r[0].Attempts) != 3 || r[0].ObservedStatus != DeliverySent || len(r[0].Latest) != 1 || r[0].Latest[0] != facts[3].Ref {
		t.Fatalf("lost retry/latest: %#v", r)
	}
	for i, want := range []DeliveryStatus{DeliveryDeferred, DeliveryDeferred, DeliverySent} {
		if r[0].Attempts[i].Delivery.Status != want || r[0].Attempts[i].Delivery.DSN.Value == "" || !r[0].Attempts[i].Delivery.Reply.Present {
			t.Fatalf("attempt%d incomplete: %#v", i, r[0].Attempts[i])
		}
	}
	rng := rand.New(rand.NewSource(92))
	for i := 0; i < 25; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := BuildRecipients(shuffled, 100)
		if err != nil || !reflect.DeepEqual(got, out) {
			t.Fatalf("permutation%d: %v", i, err)
		}
	}
	// Output time/provenance/delivery values cannot mutate immutable inputs.
	*out.Queues[0].Recipients[0].Attempts[0].Timestamp.Value = out.Queues[0].Recipients[0].Attempts[2].Timestamp.Value.AddDate(1, 0, 0)
	out.Queues[0].Recipients[0].Attempts[0].Ref = FactRef{}
	if facts[1].Observation.Timestamp.Value.Year() != 2026 || facts[1].Ref.SourceID == "" {
		t.Fatal("attempt aliases facts")
	}
}

func TestRecipientsEqualDateConflictDoesNotInventLastVerdict(t *testing.T) {
	facts := corpusFacts(t, "07-deferred-then-sent", "source-a", "trusted-a", true)
	// Delivery indices2 and4, with qmgr retry3/removal5: offset cannot resolve a tie.
	date := *facts[2].Observation.Timestamp.Value
	facts[4].Observation.Timestamp.Value = &date
	out, err := BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 1 || len(out.Queues[0].Recipients) != 1 {
		t.Fatalf("rebuild: %#v %v", out, err)
	}
	r := out.Queues[0].Recipients[0]
	if r.ObservedStatus != DeliveryUnknown || !r.OrderUncertain || len(r.Latest) != 2 || len(r.Attempts) != 2 {
		t.Fatalf("tie invented verdict: %#v", r)
	}
	// A strictly later observation can replace that ambiguous latest result.
	facts = corpusFacts(t, "07-deferred-then-sent", "source-a", "trusted-a", true)
	out, err = BuildRecipients(facts, 100)
	if err != nil || out.Queues[0].Recipients[0].ObservedStatus != DeliverySent || out.Queues[0].Recipients[0].OrderUncertain {
		t.Fatalf("later result: %#v %v", out, err)
	}
}

func TestRecipientsNeverMergeGenerationsOriginsOrAliases(t *testing.T) {
	facts := corpusFacts(t, "13-reused-queue-id", "source-a", "trusted-a", true)
	facts = append(facts, corpusFacts(t, "13-reused-queue-id", "archive", "trusted-a", true)...)
	out, err := BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 4 {
		t.Fatalf("queue/source collapsed: %#v %v", out, err)
	}
	for _, q := range out.Queues {
		if len(q.Recipients) != 1 || len(q.Recipients[0].Attempts) != 1 || !q.Generation.CrossStreamUncertain {
			t.Fatalf("attempts collapsed: %#v", q)
		}
	}
	facts = corpusFacts(t, "04-multi-recipient-mixed", "source-a", "trusted-a", true)
	for i := range facts {
		if facts[i].Observation.Kind == model.KindDelivery {
			facts[i].Observation.Fields["orig_to"] = "alias@example.org"
			facts[i].Observation.Present["orig_to"] = true
		}
	}
	facts[3].Observation.Fields["to"] = "Bob@example.org"
	facts[4].Observation.Fields["to"] = "bob@example.org"
	facts[5].Observation.Fields["to"] = ""
	out, err = BuildRecipients(facts, 100)
	if err != nil || len(out.Queues[0].Recipients) != 3 {
		t.Fatalf("case/alias/empty merged: %#v %v", out, err)
	}
	if r := out.Queues[0].Recipients[0]; !r.AddressUnspecified || r.ObservedStatus != DeliveryUnknown || r.Attempts[0].Delivery.OriginalRecipient != (Field{"alias@example.org", true}) {
		t.Fatalf("empty/alias: %#v", r)
	}
}

func TestRecipientsDoNotTurnRemovedOrUnresolvedIntoSuccess(t *testing.T) {
	facts := corpusFacts(t, "17-partial-log", "source-a", "trusted-a", true)
	out, err := BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 1 || out.Queues[0].Generation.ReceiptObserved || out.Queues[0].Recipients[0].ObservedStatus != DeliveryDeferred {
		t.Fatalf("removed promoted partial: %#v %v", out, err)
	}
	facts = corpusFacts(t, "05-rcpt-reject", "source-a", "trusted-a", true)
	out, err = BuildRecipients(facts, 100)
	if err != nil || len(out.Other) != 2 || len(out.Queues[0].Recipients) != 1 {
		t.Fatalf("NOQUEUE assigned recipient: %#v %v", out, err)
	}
	facts = corpusFacts(t, "13-reused-queue-id", "source-a", "trusted-a", false)
	out, err = BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 0 || len(out.Unresolved) != 1 || len(out.Unresolved[0].Facts) != len(facts) {
		t.Fatalf("unresolved projected: %#v %v", out, err)
	}
	if got, err := BuildRecipients(facts, 1); err != ErrPartitionLimit || !reflect.DeepEqual(got, RecipientPartition{}) {
		t.Fatalf("partial on invalid input: %#v %v", got, err)
	}
}
