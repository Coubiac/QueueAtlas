package correlation

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
)

func TestExpirationsRequireExplicitNativeQueueProof(t *testing.T) {
	facts := corpusFacts(t, "10-expired", "source-a", "trusted-a", true)
	out, err := BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 1 {
		t.Fatalf("rebuild: %#v %v", out, err)
	}
	q := out.Queues[0]
	if len(q.Expirations) != 1 || q.Expirations[0].Ref != facts[2].Ref || q.Expirations[0].NativeStatus != "expired" || !reflect.DeepEqual(q.Expirations[0].Timestamp, facts[2].Observation.Timestamp) {
		t.Fatalf("expiration evidence: %#v", q)
	}
	if len(q.Recipients) != 1 || len(q.Recipients[0].Attempts) != 1 || q.Recipients[0].ObservedStatus != DeliveryDeferred || len(q.Generation.Facts) != 5 {
		t.Fatalf("expiration invented attempt/verdict: %#v", q)
	}
	rng := rand.New(rand.NewSource(94))
	for i := 0; i < 25; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := BuildRecipients(shuffled, 100)
		if err != nil || !reflect.DeepEqual(got, out) {
			t.Fatalf("permutation%d: %v", i, err)
		}
	}
	*q.Expirations[0].Timestamp.Value = q.Expirations[0].Timestamp.Value.AddDate(1, 0, 0)
	q.Expirations[0].Ref = FactRef{}
	if facts[2].Observation.Timestamp.Value.Year() != 2026 || facts[2].Ref.SourceID == "" {
		t.Fatal("expiration aliases immutable facts")
	}
}

func TestExpirationsNeverUseRemovalReplyOrLegacyNormalisation(t *testing.T) {
	for _, name := range []string{"17-partial-log", "08-multiple-retries", "04-multi-recipient-mixed"} {
		out, err := BuildRecipients(corpusFacts(t, name, "a", "trusted", true), 100)
		if err != nil || len(out.Queues) != 1 || len(out.Queues[0].Expirations) != 0 {
			t.Fatalf("inferred expiry %s: %#v %v", name, out, err)
		}
	}
	base := corpusFacts(t, "10-expired", "a", "trusted", true)[2].Observation
	cases := []struct {
		name string
		edit func(*model.Observation)
	}{
		{"missing native", func(o *model.Observation) { o.Message = "" }},
		{"legacy angle", func(o *model.Observation) {
			o.Message = "from=<alice@example.org>, status=<expired>, returned to sender"
		}},
		{"later status", func(o *model.Observation) { o.Message = "status=active, status=expired" }},
		{"xstatus", func(o *model.Observation) { o.Message = "xstatus=expired" }},
		{"remote text", func(o *model.Observation) { o.Message = "status=sent (remote status=expired)" }},
		{"oversized", func(o *model.Observation) { o.Message = strings.Repeat("x", model.MaxLineBytes+1) }},
		{"absent field", func(o *model.Observation) { o.Present = nil }},
		{"uppercase", func(o *model.Observation) {
			o.Fields = map[string]string{"status": "EXPIRED"}
			o.Message = "status=EXPIRED"
		}},
		{"smtp peer", func(o *model.Observation) { o.Service = "smtp" }},
		{"bounce", func(o *model.Observation) { o.Kind = model.KindBounce }},
		{"removal", func(o *model.Observation) { o.Kind = model.KindRemoved }},
		{"NOQUEUE", func(o *model.Observation) { o.NoQueue = true }},
		{"no ID", func(o *model.Observation) { o.QueueID = "" }},
		{"malformed", func(o *model.Observation) { o.ParseError = "malformed" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := base
			tc.edit(&o)
			if got, ok := queueExpirationFrom(FactRef{}, o); ok || !reflect.DeepEqual(got, QueueExpiration{}) {
				t.Fatalf("unproved expiration: %#v", got)
			}
		})
	}
}

func TestExpirationsKeepEveryReportAndIndependentOrigin(t *testing.T) {
	facts := corpusFacts(t, "10-expired", "live", "trusted", true)
	// Add a second report before bounce/removal, at the same date. Neither text
	// equality nor date equality removes an independently located observation.
	extra := facts[2]
	extra.Ref.Start = extra.Ref.End
	extra.Ref.End += 128
	for i := 3; i < len(facts); i++ {
		facts[i].Ref.Start += 128
		facts[i].Ref.End += 128
	}
	facts = append(facts, extra)
	facts = append(facts, corpusFacts(t, "10-expired", "archive", "trusted", true)...)
	out, err := BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 2 {
		t.Fatalf("origins: %#v %v", out, err)
	}
	if len(out.Queues[0].Expirations) != 1 || len(out.Queues[1].Expirations) != 2 {
		t.Fatalf("reports collapsed: %#v", out)
	}
	for _, q := range out.Queues {
		if !q.Generation.CrossStreamUncertain || q.Recipients[0].ObservedStatus != DeliveryDeferred {
			t.Fatalf("scope/verdict: %#v", q)
		}
	}
}

func TestExpirationsDoNotPromoteUnresolvedOrInvalidSnapshot(t *testing.T) {
	facts := corpusFacts(t, "10-expired", "a", "trusted", false)
	out, err := BuildRecipients(facts, 100)
	if err != nil || len(out.Queues) != 0 || len(out.Unresolved) != 1 || len(out.Unresolved[0].Facts) != len(facts) {
		t.Fatalf("undated certainty: %#v %v", out, err)
	}
	if got, err := BuildRecipients(facts, 1); err != ErrPartitionLimit || !reflect.DeepEqual(got, RecipientPartition{}) {
		t.Fatalf("invalid snapshot: %#v %v", got, err)
	}
}
