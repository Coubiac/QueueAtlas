package correlation

import (
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/parser/postfix"
	"github.com/Coubiac/mailtrace/internal/parser/syslog"
)

func TestGenerationsRecycledQueueIDAndImportOrder(t *testing.T) {
	facts := corpusFacts(t, "13-reused-queue-id", "source-a", "trusted-a", true)
	want, err := BuildGenerations(facts, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(want.Generations) != 2 || len(want.Unresolved) != 0 || len(want.Other) != 0 {
		t.Fatalf("generations: %#v", want)
	}
	for i, g := range want.Generations {
		if len(g.Facts) != 4 || g.First != facts[i*4].Ref || g.Removed == nil || *g.Removed != facts[i*4+3].Ref || !g.ReceiptObserved || !g.HasNonExplicitTime || g.CrossStreamUncertain {
			t.Fatalf("generation%d: %#v", i, g)
		}
	}
	rng := rand.New(rand.NewSource(91))
	for i := 0; i < 25; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := BuildGenerations(shuffled, 100)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("permutation%d: %#v %v", i, got, err)
		}
	}
	// Neither Removed nor the fact lists borrow mutable caller references.
	*want.Generations[0].Removed = FactRef{}
	want.Generations[0].Facts[0] = FactRef{}
	if facts[0].Ref.SourceID == "" || facts[3].Ref.SourceID == "" {
		t.Fatal("output mutated facts")
	}
}

func TestGenerationsRetainPartialRetriesAndSeparateStreams(t *testing.T) {
	for _, tc := range []struct {
		name             string
		count            int
		receipt, removed bool
	}{
		{"16-out-of-order", 4, true, true}, {"17-partial-log", 2, false, true},
		{"08-multiple-retries", 5, true, true}, {"18-lmtp", 2, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := corpusFacts(t, tc.name, "source-a", "trusted-a", true)
			got, err := BuildGenerations(facts, 100)
			if err != nil || len(got.Generations) != 1 || len(got.Unresolved) != 0 {
				t.Fatalf("rebuild: %#v %v", got, err)
			}
			g := got.Generations[0]
			if len(g.Facts) != tc.count || g.ReceiptObserved != tc.receipt || (g.Removed != nil) != tc.removed {
				t.Fatalf("partial/retries lost: %#v", g)
			}
		})
	}
	facts := corpusFacts(t, "13-reused-queue-id", "source-a", "trusted-a", true)
	facts = append(facts, corpusFacts(t, "13-reused-queue-id", "archive", "trusted-a", true)...)
	out, err := BuildGenerations(facts, 100)
	if err != nil || len(out.Generations) != 4 {
		t.Fatalf("streams collapsed: %#v %v", out, err)
	}
	for _, g := range out.Generations {
		if !g.CrossStreamUncertain {
			t.Fatal("unproven continuity hidden")
		}
	}
}

func TestGenerationsUnprovenBoundariesAreFullyUnresolved(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]Fact) []Fact
		reason GenerationReason
	}{
		{"undated", func(f []Fact) []Fact { f[6].Observation.Timestamp.Value = nil; return f }, GenerationUndated},
		{"equal boundary dates", func(f []Fact) []Fact {
			date := *f[3].Observation.Timestamp.Value
			f[4].Observation.Timestamp.Value = &date
			return f
		}, GenerationBoundaryUnproven},
		{"delivery without new receipt", func(f []Fact) []Fact { return append(append([]Fact(nil), f[:4]...), f[6:]...) }, GenerationBoundaryUnproven},
		{"different message IDs without removal", func(f []Fact) []Fact { return append(append([]Fact(nil), f[:3]...), f[4:]...) }, GenerationConflictingIDs},
		{"duplicate removal", func(f []Fact) []Fact {
			last := f[3]
			last.Ref.Start = f[7].Ref.End
			last.Ref.End = last.Ref.Start + 10
			last.Observation.Timestamp.Value = f[7].Observation.Timestamp.Value
			return append(f, last)
		}, GenerationBoundaryUnproven},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := tc.change(corpusFacts(t, "13-reused-queue-id", "source-a", "trusted-a", true))
			got, err := BuildGenerations(facts, 100)
			if err != nil || len(got.Generations) != 0 || len(got.Unresolved) != 1 || got.Unresolved[0].Reason != tc.reason || len(got.Unresolved[0].Facts) != len(facts) {
				t.Fatalf("unproven boundary promoted/lost: %#v %v", got, err)
			}
		})
	}
}

func TestGenerationsExplicitDatesNoQueueAndInvalidSnapshot(t *testing.T) {
	facts := corpusFacts(t, "05-rcpt-reject", "source-a", "trusted-a", true)
	for i := range facts {
		facts[i].Observation.Timestamp.Quality = model.TimeExplicitOffset
	}
	out, err := BuildGenerations(facts, 100)
	if err != nil || len(out.Generations) != 1 || len(out.Other) != 2 || out.Generations[0].HasNonExplicitTime {
		t.Fatalf("NOQUEUE/time: %#v %v", out, err)
	}
	for _, input := range [][]Fact{append(append([]Fact(nil), facts...), facts[0])} {
		if got, err := BuildGenerations(input, 100); err != ErrPartitionOverlap || !reflect.DeepEqual(got, GenerationPartition{}) {
			t.Fatalf("invalid snapshot produced generations: %#v %v", got, err)
		}
	}
	// Each supported receipt marker permits a new candidate only AFTER removed.
	for _, body := range []string{"client=sender.invalid[192.0.2.1]", "uid=1001 from=<alice@example.org>", "message-id=<new@example.org>", "from=<alice@example.org>, size=10, nrcpt=1 (queue active)"} {
		service := map[string]string{"client=sender.invalid[192.0.2.1]": "smtpd", "uid=1001 from=<alice@example.org>": "pickup", "message-id=<new@example.org>": "cleanup", "from=<alice@example.org>, size=10, nrcpt=1 (queue active)": "qmgr"}[body]
		raw := "Oct  5 12:00:00 mx postfix/" + service + "[1]: 13A1B2C3D4: " + body + "\n"
		base := corpusFacts(t, "13-reused-queue-id", "source-a", "trusted-a", true)[:4]
		start := base[3].Ref.End
		base = append(base, Fact{FactRef{"source-a", "13-reused-queue-id", start, start + int64(len(raw))}, "trusted-a", postfix.Parse([]byte(raw), postfix.Options{SourceID: "source-a", Time: syslog.TimeContext{Year: 2026, Location: time.UTC}})})
		got, err := BuildGenerations(base, 100)
		if err != nil || len(got.Generations) != 2 || !got.Generations[1].ReceiptObserved {
			t.Fatalf("receipt %s: %#v %v", service, got, err)
		}
	}
}

func TestGenerationsPhysicalRemovalCannotBeHiddenByClockOrder(t *testing.T) {
	for _, body := range []string{
		"postfix/qmgr[1]: 13A1B2C3D4: from=<alice@example.org>, size=10, nrcpt=1 (queue active)",
		"postfix/smtp[1]: 13A1B2C3D4: to=<bob@example.org>, status=sent (250 accepted)",
	} {
		facts := corpusFacts(t, "13-reused-queue-id", "source-a", "trusted-a", true)[:4]
		raw := "Oct  3 12:12:00 mx " + body + "\n"
		start := facts[3].Ref.End
		facts = append(facts, Fact{FactRef{"source-a", "13-reused-queue-id", start, start + int64(len(raw))}, "trusted-a", postfix.Parse([]byte(raw), postfix.Options{SourceID: "source-a", Time: syslog.TimeContext{Year: 2026, Location: time.UTC}})})
		got, err := BuildGenerations(facts, 100)
		if err != nil || len(got.Generations) != 0 || len(got.Unresolved) != 1 || got.Unresolved[0].Reason != GenerationBoundaryUnproven || len(got.Unresolved[0].Facts) != 5 {
			t.Fatalf("physical removal hidden by time ordering: %#v %v", got, err)
		}
	}
	// The opposite contradiction: a physically earlier fact dated after removal.
	facts := corpusFacts(t, "13-reused-queue-id", "source-a", "trusted-a", true)[:4]
	date := facts[3].Observation.Timestamp.Value.Add(time.Second)
	facts[2].Observation.Timestamp.Value = &date
	got, err := BuildGenerations(facts, 100)
	if err != nil || len(got.Generations) != 0 || len(got.Unresolved) != 1 || len(got.Unresolved[0].Facts) != 4 {
		t.Fatalf("future fact crossed removal: %#v %v", got, err)
	}
}
