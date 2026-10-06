package correlation

import (
	"bytes"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/parser/postfix"
	"github.com/Coubiac/QueueAtlas/internal/parser/syslog"
)

func corpusFacts(t *testing.T, name, sourceID, instance string, knownDate bool) []Fact {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "postfix", name+".log"))
	if err != nil {
		t.Fatal(err)
	}
	opts := postfix.Options{SourceID: sourceID}
	if knownDate {
		opts.Time = syslog.TimeContext{Year: 2026, Location: time.UTC}
	}
	var facts []Fact
	var offset int64
	for _, line := range bytes.SplitAfter(raw, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		end := offset + int64(len(line))
		facts = append(facts, Fact{FactRef{sourceID, name, offset, end}, instance, postfix.Parse(line, opts)})
		offset = end
	}
	return facts
}

func TestPartitionFactsDeterministicDatesAndProvenance(t *testing.T) {
	facts := corpusFacts(t, "16-out-of-order", "source-a", "trusted-a", true)
	want, err := PartitionFacts(facts, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(want.Queues) != 1 || len(want.Queues[0].Streams) != 1 || len(want.Other) != 0 || want.Queues[0].CrossStreamUncertain {
		t.Fatalf("partition: %#v", want)
	}
	s := want.Queues[0].Streams[0]
	if len(s.Timed) != 4 || len(s.Untimed) != 0 || s.Timed[0] != facts[1].Ref || s.Timed[1] != facts[2].Ref || s.Timed[2] != facts[0].Ref || s.Timed[3] != facts[3].Ref {
		t.Fatalf("dates: %#v", s)
	}
	rng := rand.New(rand.NewSource(90))
	for i := 0; i < 25; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := PartitionFacts(shuffled, 100)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("import permutation %d: %#v %v", i, got, err)
		}
	}
	// Equal instants use offsets only as a stable display tie break.
	tied := append([]Fact(nil), facts...)
	instant := *facts[0].Observation.Timestamp.Value
	for i := range tied {
		tied[i].Observation.Timestamp.Value = &instant
	}
	got, err := PartitionFacts(tied, 100)
	if err != nil {
		t.Fatal(err)
	}
	for i, ref := range got.Queues[0].Streams[0].Timed {
		if ref != facts[i].Ref {
			t.Fatal("equal timestamps ordered by arrival instead of provenance")
		}
	}
	// Output stores references by value, no borrowed slices/maps/timestamps.
	facts[1].Ref.Start = 900
	*facts[1].Observation.Timestamp.Value = time.Time{}
	if s.Timed[0].Start == 900 || !reflect.DeepEqual(want.Queues[0].Streams[0], s) {
		t.Fatal("output aliases input")
	}
}

func TestPartitionFactsNeverMergesOriginsHostsOrMessageIDs(t *testing.T) {
	facts := append(corpusFacts(t, "14-same-id-host-a", "source-a", "trusted-a", true), corpusFacts(t, "14-same-id-host-b", "source-b", "trusted-b", true)...)
	facts = append(facts, corpusFacts(t, "15-duplicate-message-id", "source-c", "trusted-a", true)...)
	// Text/offsets identical under another origin of the SAME trusted instance.
	copyFacts := corpusFacts(t, "14-same-id-host-a", "archive", "trusted-a", true)
	facts = append(facts, copyFacts...)
	out, err := PartitionFacts(facts, 100)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, q := range out.Queues {
		for _, s := range q.Streams {
			count += len(s.Timed) + len(s.Untimed)
		}
		if q.Key.QueueID == facts[0].Observation.QueueID && q.Key.Instance == "trusted-a" {
			if len(q.Streams) != 2 || !q.CrossStreamUncertain {
				t.Fatalf("unproven overlap hidden: %#v", q)
			}
		}
	}
	if count != len(facts) || len(out.Queues) != 4 {
		t.Fatalf("facts/queues: %d/%d, want %d/4", count, len(out.Queues), len(facts))
	}
	// Changing all declared hosts cannot alter configured instance boundaries.
	for i := range facts {
		facts[i].Observation.Host = "same-forged-host"
		if facts[i].Observation.Fields == nil {
			facts[i].Observation.Fields = make(map[string]string)
		}
		facts[i].Observation.Fields["message-id"] = "same-forged-message-id"
	}
	got, err := PartitionFacts(facts, 100)
	if err != nil || !reflect.DeepEqual(got, out) {
		t.Fatalf("weak attributes affect identity: %v", err)
	}
}

func TestPartitionFactsRetainsUntimedNoQueueAndUnknown(t *testing.T) {
	facts := corpusFacts(t, "05-rcpt-reject", "source-a", "trusted-a", false)
	facts = append(facts, corpusFacts(t, "26-malformed-and-unknown", "source-a", "trusted-a", false)...)
	out, err := PartitionFacts(facts, 100)
	if err != nil {
		t.Fatal(err)
	}
	count := len(out.Other)
	for _, q := range out.Queues {
		for _, s := range q.Streams {
			if len(s.Timed) != 0 {
				t.Fatal("unknown year produced a timeline")
			}
			count += len(s.Untimed)
		}
	}
	if count != len(facts) || len(out.Other) == 0 || len(out.Queues) != 2 {
		t.Fatalf("lost facts/NOQUEUE: %#v", out)
	}
	// Even an inconsistent injected Value must not invent an instant for wall_only.
	instant := time.Now()
	for i := range facts {
		facts[i].Observation.Timestamp.Value = &instant
		facts[i].Observation.Timestamp.Quality = model.TimeWallOnly
	}
	got, err := PartitionFacts(facts, 100)
	if err != nil || !reflect.DeepEqual(got, out) {
		t.Fatalf("unknown quality promoted: %v", err)
	}
}

func TestPartitionFactsRejectsInvalidOrOverlappingSnapshot(t *testing.T) {
	base := corpusFacts(t, "01-inbound-local", "source-a", "trusted-a", true)
	for _, tc := range []struct {
		name  string
		facts []Fact
		limit int
		err   error
	}{
		{"zero limit", base, 0, ErrPartitionLimit},
		{"hard limit", base, MaxPartitionFacts + 1, ErrPartitionLimit},
		{"count limit", base, 1, ErrPartitionLimit},
		{"duplicate", append(append([]Fact(nil), base...), base[0]), 100, ErrPartitionOverlap},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PartitionFacts(tc.facts, tc.limit)
			if !errors.Is(err, tc.err) || !reflect.DeepEqual(got, Partition{}) {
				t.Fatalf("partial/refusal: %#v %v", got, err)
			}
		})
	}
	for _, change := range []func(*Fact){
		func(f *Fact) { f.Ref.SourceID = "" }, func(f *Fact) { f.Ref.OriginID = "" }, func(f *Fact) { f.Instance = "" },
		func(f *Fact) { f.Ref.Start = -1 }, func(f *Fact) { f.Ref.End = f.Ref.Start }, func(f *Fact) { f.Observation.SourceID = "foreign" },
	} {
		facts := append([]Fact(nil), base...)
		change(&facts[0])
		if got, err := PartitionFacts(facts, 100); err != ErrPartitionFact || !reflect.DeepEqual(got, Partition{}) {
			t.Fatalf("invalid fact accepted: %#v %v", got, err)
		}
	}
	inconsistent := append([]Fact(nil), base...)
	inconsistent[1].Instance = "other-instance"
	if got, err := PartitionFacts(inconsistent, 100); err != ErrPartitionFact || !reflect.DeepEqual(got, Partition{}) {
		t.Fatalf("one origin attributed to multiple configured instances: %#v %v", got, err)
	}
	base[1].Ref.Start = base[0].Ref.End - 1
	if got, err := PartitionFacts(base, 100); err != ErrPartitionOverlap || !reflect.DeepEqual(got, Partition{}) {
		t.Fatalf("overlap accepted: %#v %v", got, err)
	}
	if got, err := PartitionFacts(nil, 1); err != nil || !reflect.DeepEqual(got, Partition{}) {
		t.Fatalf("empty: %#v %v", got, err)
	}
}
