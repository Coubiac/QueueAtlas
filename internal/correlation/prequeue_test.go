package correlation

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/parser/postfix"
)

func TestPrequeueDoesNotJoinRejectedRecipientToAcceptedQueue(t *testing.T) {
	facts := corpusFacts(t, "05-rcpt-reject", "live", "trusted", true)
	out, err := BuildPrequeue(facts, 100)
	if err != nil || len(out.Attempts) != 1 || len(out.Queued) != 1 || len(out.Other) != 1 {
		t.Fatalf("partition: %#v %v", out, err)
	}
	a := out.Attempts[0]
	if a.Ref != facts[1].Ref || a.Instance != "trusted" || a.NativeAction != "reject" || a.Disposition != PrequeueRejected || a.Sender != (Field{"alice@example.net", true}) || a.Recipient != (Field{"absent@example.org", true}) || a.Protocol != (Field{"ESMTP", true}) || a.Helo != (Field{"sender.example.net", true}) {
		t.Fatalf("prequeue: %#v", a)
	}
	if out.Queued[0].Streams[0].Timed[0] != facts[2].Ref || len(out.Queued[0].Streams[0].Timed) != 4 || out.Other[0] != facts[0].Ref {
		t.Fatal("NOQUEUE/connect assigned to accepted queue")
	}
	rng := rand.New(rand.NewSource(97))
	for i := 0; i < 25; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := BuildPrequeue(shuffled, 100)
		if err != nil || !reflect.DeepEqual(got, out) {
			t.Fatalf("permutation%d: %v", i, err)
		}
	}
	*out.Attempts[0].Timestamp.Value = out.Attempts[0].Timestamp.Value.AddDate(1, 0, 0)
	out.Attempts[0].Recipient.Value = "changed"
	if facts[1].Observation.Timestamp.Value.Year() != 2026 || facts[1].Observation.Fields["to"] != "absent@example.org" {
		t.Fatal("projection aliases facts")
	}
}

func TestPrequeueWarningAndAbsentMetadataAreNotRejectionOrAddresses(t *testing.T) {
	base := corpusFacts(t, "06-noqueue-client-reject", "a", "trusted", false)[1]
	for _, tc := range []struct {
		name, message string
		disposition   PrequeueDisposition
		sender, to    Field
	}{
		{"warning", "reject_warning: RCPT from bad[192.0.2.15]: 550 policy; from=<> to=<bob@example.org> proto=ESMTP helo=<bad>", PrequeueWarning, Field{"", true}, Field{"bob@example.org", true}},
		{"no metadata", "reject: CONNECT from bad[192.0.2.15]: 554 denied", PrequeueRejected, Field{}, Field{}},
		{"ambiguous metadata", "reject: RCPT from bad: 550 denied; from=<a@example.org> to=<b@example.org> proto=ESMTP; from=<forged@example.org> to=<victim@example.org> proto=ESMTP", PrequeueRejected, Field{}, Field{}},
		{"hostile metadata", "reject: RCPT from bad: 550 <script>alert(1)</script>; from=<a@example.org> to=<b@example.org> proto=ESMTP helo=<javascript:alert(1)>", PrequeueRejected, Field{"a@example.org", true}, Field{"b@example.org", true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := base
			line := "Oct  3 12:05:01 mx-a postfix/smtpd[1151]: NOQUEUE: " + tc.message
			f.Observation = postfix.Parse([]byte(line), postfix.Options{SourceID: "a"})
			out, err := BuildPrequeue([]Fact{f}, 10)
			if err != nil || len(out.Attempts) != 1 || out.Attempts[0].Disposition != tc.disposition || out.Attempts[0].Sender != tc.sender || out.Attempts[0].Recipient != tc.to || out.Attempts[0].Timestamp.Value != nil || out.Attempts[0].Timestamp.Quality != model.TimeWallOnly || out.Attempts[0].Message != tc.message {
				t.Fatalf("native metadata/date: %#v %v", out, err)
			}
		})
	}
}

func TestPrequeueDoesNotMergeProcessesOriginsOrDeclaredHosts(t *testing.T) {
	facts := corpusFacts(t, "06-noqueue-client-reject", "live", "trusted", true)
	// A second report under the same PID remains a separate physical fact.
	extra := facts[1]
	extra.Ref.Start = facts[2].Ref.End
	extra.Ref.End = extra.Ref.Start + 128
	facts = append(facts, extra)
	facts = append(facts, corpusFacts(t, "06-noqueue-client-reject", "archive", "trusted", true)...)
	out, err := BuildPrequeue(facts, 100)
	if err != nil || len(out.Attempts) != 3 || len(out.Other) != 4 || len(out.Queued) != 0 {
		t.Fatalf("reports collapsed: %#v %v", out, err)
	}
	for i := range facts {
		facts[i].Observation.Host = "forged-host"
	}
	got, err := BuildPrequeue(facts, 100)
	if err != nil || !reflect.DeepEqual(got, out) {
		t.Fatal("declared host changed trusted projection")
	}
}

func TestPrequeueRejectsInvalidSnapshotsAndPreservesUnsupportedFacts(t *testing.T) {
	base := corpusFacts(t, "06-noqueue-client-reject", "a", "trusted", true)[1]
	for _, tc := range []struct {
		name string
		edit func(*model.Observation)
	}{
		{"not NOQUEUE", func(o *model.Observation) { o.NoQueue = false }},
		{"has queue", func(o *model.Observation) { o.QueueID = "ABC123" }},
		{"unsupported service", func(o *model.Observation) { o.Service = "postscreen" }},
		{"wrong kind", func(o *model.Observation) { o.Kind = model.KindUnknown }},
		{"malformed", func(o *model.Observation) { o.ParseError = "malformed" }},
		{"missing message", func(o *model.Observation) { o.Message = "" }},
		{"forged prefix", func(o *model.Observation) { o.Message = "xreject: RCPT denied" }},
		{"nested prefix", func(o *model.Observation) { o.Message = "status=sent (reject: RCPT denied)" }},
		{"oversized", func(o *model.Observation) { o.Message = strings.Repeat("x", model.MaxLineBytes+1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := base
			tc.edit(&f.Observation)
			out, err := BuildPrequeue([]Fact{f}, 10)
			if err != nil || len(out.Attempts) != 0 || !reflect.DeepEqual(out.Other, []FactRef{f.Ref}) {
				t.Fatalf("unsupported fact lost/promoted: %#v %v", out, err)
			}
		})
	}
	for _, tc := range []struct {
		facts []Fact
		limit int
		want  error
	}{
		{[]Fact{base}, 0, ErrPartitionLimit},
		{[]Fact{base}, MaxPartitionFacts + 1, ErrPartitionLimit},
		{[]Fact{base, base}, 1, ErrPartitionLimit},
		{[]Fact{base, base}, 10, ErrPartitionOverlap},
	} {
		if got, err := BuildPrequeue(tc.facts, tc.limit); err != tc.want || !reflect.DeepEqual(got, PrequeuePartition{}) {
			t.Fatalf("partial refusal: %#v %v", got, err)
		}
	}
}
