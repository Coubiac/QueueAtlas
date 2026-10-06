package correlation

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/parser/postfix"
)

func TestQueueHintsKeepNativeEvidenceWithoutCreatingTargetOrEdge(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		kind         QueueHintKind
		evidence     int
		queues       int
		instance     Field
	}{
		{"11-filter-reinjection", "11F1B2C3D4", HintSMTPQueue, 3, 2, Field{}},
		{"12-unverified-reinjection", "12F1B2C3D4", HintSMTPQueue, 1, 1, Field{}},
		{"21-orig-to-alias", "21F1B2C3D4", HintLocalForward, 1, 2, Field{"trusted", true}},
		{"09-bounce", "09F1B2C3D4", HintBounceNotification, 2, 2, Field{"trusted", true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := corpusFacts(t, tc.name, "live", "trusted", true)
			out, err := BuildQueueHints(facts, 100)
			if err != nil || len(out.Hints) != 1 || len(out.Index.Queues) != tc.queues {
				t.Fatalf("hints: %#v %v", out, err)
			}
			h := out.Hints[0]
			if h.Evidence != facts[tc.evidence].Ref || h.From != (QueueKey{"trusted", facts[tc.evidence].Observation.QueueID}) || h.TargetQueueID != tc.target || h.Kind != tc.kind || h.TargetInstance != tc.instance || h.NativeProof == "" {
				t.Fatalf("native evidence: %#v", h)
			}
			rng := rand.New(rand.NewSource(100))
			for i := 0; i < 25; i++ {
				shuffled := append([]Fact(nil), facts...)
				rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
				got, err := BuildQueueHints(shuffled, 100)
				if err != nil || !reflect.DeepEqual(got, out) {
					t.Fatalf("permutation%d: %v", i, err)
				}
			}
			// Hint strings and references are values, not aliases into input maps.
			out.Hints[0].NativeProof = "changed"
			out.Hints[0].Relay.Value = "changed"
			out.Hints[0].Evidence = FactRef{}
			if facts[tc.evidence].Ref.SourceID == "" || facts[tc.evidence].Observation.Message == "changed" || facts[tc.evidence].Observation.Fields["relay"] == "changed" {
				t.Fatal("hint aliases immutable facts")
			}
		})
	}
}

func TestQueueHintsRejectUnsupportedHostileAndLegacyProofs(t *testing.T) {
	for _, tc := range []struct{ service, message string }{
		{"local", "to=<b@example.org>, status=sent (forwarded as <DEF456>)"},
		{"local", "to=<b@example.org>, status=sent (forwarded as DEF456 extra)"},
		{"local", "to=<b@example.org>, status=sent (forwarded as ../../secret)"},
		{"local", "to=<b@example.org>, status=sent (forwarded as " + strings.Repeat("A", 33) + ")"},
		{"local", "to=<b@example.org>, status=deferred (forwarded as DEF456)"},
		{"local", "to=<b@example.org>, status=<sent> (forwarded as DEF456)"},
		{"smtp", "to=<b@example.org>, status=sent (forwarded as DEF456)"},
		{"pipe", "to=<b@example.org>, status=sent (forwarded as DEF456)"},
		{"virtual", "to=<b@example.org>, status=sent (forwarded as DEF456)"},
		{"smtp", "to=<b@example.org>, status=sent (250 2.0.0 Ok: queued as DEF456; xstatus=sent)"},
		{"smtp", "to=<b@example.org>, status=sent (250 2.0.0 Ok: queued as DEF456 queued as ABC789)"},
		{"smtp", "to=<b@example.org>, status=sent (550 denied: queued as DEF456)"},
		{"smtp", "to=<b@example.org>, status=sent (<script>queued as DEF456</script>)"},
		{"bounce", "sender non-delivery notification: <DEF456>"},
		{"bounce", "sender non-delivery notification: DEF456 extra"},
	} {
		line := "Oct  3 12:00:00 mx postfix/" + tc.service + "[1]: ABC123: " + tc.message
		f := Fact{FactRef{"a", "synthetic", 0, int64(len(line))}, "trusted", postfix.Parse([]byte(line), postfix.Options{SourceID: "a"})}
		if out, err := BuildQueueHints([]Fact{f}, 10); err != nil || len(out.Hints) != 0 {
			t.Fatalf("unproved hint %s: %#v %v", tc.message, out, err)
		}
	}
	// Historically normalised replies/statuses do not replace native evidence,
	// including an unrelated status-looking suffix.
	for _, message := range []string{
		"to=<b@example.org>, status=sent (<forwarded as DEF456>)",
		"to=<b@example.org>, status=sent (<forwarded as DEF456>), xstatus=sent (forwarded as DEF456)",
		"to=<b@example.org>, status=<sent> (forwarded as DEF456)",
		"",
	} {
		f := corpusFacts(t, "21-orig-to-alias", "a", "trusted", true)[1]
		f.Observation.Message = message
		f.Observation.Fields["status"] = "sent"
		f.Observation.Fields["reply"] = "forwarded as DEF456"
		if got, ok := queueHintFrom(f); ok || !reflect.DeepEqual(got, QueueHint{}) {
			t.Fatalf("legacy manufactured hint: %#v", got)
		}
	}
}

func TestQueueHintsNeverGuessInstanceFromRelayHostOrTargetPresence(t *testing.T) {
	facts := corpusFacts(t, "11-filter-reinjection", "live", "trusted", true)
	facts = append(facts, corpusFacts(t, "11-filter-reinjection", "archive", "trusted", true)...)
	out, err := BuildQueueHints(facts, 100)
	if err != nil || len(out.Hints) != 2 {
		t.Fatalf("origins: %#v %v", out, err)
	}
	for _, h := range out.Hints {
		if h.TargetInstance.Present || h.Relay.Value != "127.0.0.1[127.0.0.1]:10024" {
			t.Fatalf("loopback resolved an instance: %#v", h)
		}
	}
	for i := range facts {
		facts[i].Observation.Host = "forged-host"
		facts[i].Observation.Timestamp.Value = nil
		facts[i].Observation.Timestamp.Quality = model.TimeWallOnly
	}
	got, err := BuildQueueHints(facts, 100)
	if err != nil || !reflect.DeepEqual(got.Hints, out.Hints) || len(got.Index.Queues[0].Streams[0].Untimed) == 0 {
		t.Fatalf("date/Host changed hint: %#v %v", got, err)
	}
	// Lack of a supported explicit phrase retains all facts without making hints.
	got, err = BuildQueueHints(corpusFacts(t, "02-outbound-auth", "a", "trusted", true), 100)
	if err != nil || len(got.Hints) != 0 || len(got.Index.Queues) != 1 {
		t.Fatalf("REMOTE02 became a local queue: %#v %v", got, err)
	}
}

func TestQueueHintsRejectInvalidSnapshotWithoutPartialIndex(t *testing.T) {
	facts := corpusFacts(t, "11-filter-reinjection", "a", "trusted", true)
	for _, tc := range []struct {
		facts []Fact
		limit int
		want  error
	}{
		{facts, 1, ErrPartitionLimit},
		{facts, MaxPartitionFacts + 1, ErrPartitionLimit},
		{append(facts, facts[0]), 100, ErrPartitionOverlap},
		{[]Fact{{Ref: FactRef{}, Observation: facts[0].Observation}}, 10, ErrPartitionFact},
	} {
		if got, err := BuildQueueHints(tc.facts, tc.limit); err != tc.want || !reflect.DeepEqual(got, QueueHints{}) {
			t.Fatalf("partial snapshot: %#v %v", got, err)
		}
	}
}
