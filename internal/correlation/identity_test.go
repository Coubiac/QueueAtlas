package correlation

import (
	"encoding/hex"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func TestQueueInstancesRecycledIDKeysAndPermutations(t *testing.T) {
	facts := corpusFacts(t, "13-reused-queue-id", "source", "trusted", true)
	want, err := BuildQueueInstances(facts, 100)
	if err != nil || len(want.Instances) != 2 || len(want.Unresolved) != 0 {
		t.Fatalf("rebuild: %#v %v", want, err)
	}
	if digest, err := hex.DecodeString(want.Revision); err != nil || len(digest) != 32 {
		t.Fatal("revision is not a SHA256 digest")
	}
	for i, q := range want.Instances {
		if q.Key != (QueueInstanceKey{want.Revision, "trusted", "13A1B2C3D4", i}) || q.Observed.First != facts[4*i].Ref || len(q.Observed.Facts) != 4 {
			t.Fatalf("cycle%d key/evidence: %#v", i, q)
		}
	}
	rng := rand.New(rand.NewSource(103))
	for i := 0; i < 25; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := BuildQueueInstances(shuffled, 100)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("arrival order changed identity: %v", err)
		}
	}
	want.Instances[0].Observed.Facts[0] = FactRef{}
	*want.Instances[0].Observed.Removed = FactRef{}
	got, err := BuildQueueInstances(facts, 100)
	if err != nil || got.Instances[0].Observed.First != facts[0].Ref || *got.Instances[0].Observed.Removed != facts[3].Ref {
		t.Fatal("output borrowed input or another rebuild")
	}
}

func TestQueueInstancesOriginsInstancesAndUnresolvedRemainSeparate(t *testing.T) {
	facts := corpusFacts(t, "13-reused-queue-id", "live", "trusted", true)
	facts = append(facts, corpusFacts(t, "13-reused-queue-id", "archive", "trusted", true)...)
	facts = append(facts, corpusFacts(t, "13-reused-queue-id", "other", "other-instance", true)...)
	got, err := BuildQueueInstances(facts, 100)
	if err != nil || len(got.Instances) != 6 {
		t.Fatalf("collapsed origins/instances: %#v %v", got, err)
	}
	seen := make(map[QueueInstanceKey]bool)
	for _, q := range got.Instances {
		if seen[q.Key] || (q.Key.Instance == "trusted") != q.Observed.CrossStreamUncertain {
			t.Fatalf("identity collision or hidden uncertainty: %#v", q)
		}
		seen[q.Key] = true
	}
	if got.Instances[2].Key.Generation != 0 || got.Instances[5].Key.Generation != 3 {
		t.Fatal("ordinals do not restart for configured instance")
	}
	unknown := corpusFacts(t, "13-reused-queue-id", "undated", "trusted", false)
	noqueue := corpusFacts(t, "06-noqueue-client-reject", "reject", "trusted", true)
	got, err = BuildQueueInstances(append(unknown, noqueue...), 100)
	if err != nil || len(got.Instances) != 0 || len(got.Unresolved) != 1 || len(got.Unresolved[0].Facts) != 8 || len(got.Other) != 3 {
		t.Fatalf("unresolved/NOQUEUE invented an identity: %#v %v", got, err)
	}
}

func TestQueueInstancesLateImportInvalidatesOldOrdinals(t *testing.T) {
	facts := corpusFacts(t, "13-reused-queue-id", "source", "trusted", true)
	before, err := BuildQueueInstances(facts[4:], 100)
	if err != nil {
		t.Fatal(err)
	}
	after, err := BuildQueueInstances(facts, 100)
	if err != nil || before.Revision == after.Revision || before.Instances[0].Key.Generation != 0 || after.Instances[1].Key.Generation != 1 || before.Instances[0].Observed.First != after.Instances[1].Observed.First {
		t.Fatalf("late import silently reused old ordinal: %#v %#v %v", before, after, err)
	}
	for _, q := range after.Instances {
		if before.Instances[0].Key == q.Key {
			t.Fatal("old revision key aliased a new cycle")
		}
	}
}

func TestQueueInstancesRevisionPreservesBytesMetadataAndFraming(t *testing.T) {
	revision := func(facts []Fact) string {
		t.Helper()
		got, err := BuildQueueInstances(facts, 100)
		if err != nil {
			t.Fatal(err)
		}
		return got.Revision
	}
	base := corpusFacts(t, "08-multiple-retries", "source", "trusted", true)
	want := revision(base)
	for _, change := range []func([]Fact){
		func(f []Fact) { f[1].Observation.Raw += "extra" },
		func(f []Fact) { f[1].Observation.Message += "extra" },
		func(f []Fact) { f[1].Observation.Fields = map[string]string{"status": "sent"} },
		func(f []Fact) { f[1].Observation.Present = map[string]bool{"status": false} },
		func(f []Fact) { f[1].Observation.Timestamp.Zone = "other" },
		func(f []Fact) { f[1].Observation.Timestamp.Year++ },
		func(f []Fact) { f[1].Observation.Timestamp.Value = nil },
		func(f []Fact) { f[1].Observation.Host = "untrusted-other" },
	} {
		copy := append([]Fact(nil), base...)
		change(copy)
		if revision(copy) == want {
			t.Fatal("changed immutable observation reused the revision")
		}
	}
	// Equal instants in different Go locations are the same stored UTC value.
	copy := append([]Fact(nil), base...)
	date := copy[1].Observation.Timestamp.Value.In(time.FixedZone("alias", 7200))
	copy[1].Observation.Timestamp.Value = &date
	if revision(copy) != want {
		t.Fatal("Go location affected UTC identity")
	}
	left, right := append([]Fact(nil), base...), append([]Fact(nil), base...)
	left[0].Observation.Raw, right[0].Observation.Raw = string([]byte{0xff}), string([]byte{0xfe})
	if revision(left) == revision(right) {
		t.Fatal("invalid UTF8 bytes were canonicalized together")
	}
	// Concatenating adjacent values would alias these different field maps.
	left[0].Observation.Fields = map[string]string{"a": "bc"}
	right[0].Observation.Raw = left[0].Observation.Raw
	right[0].Observation.Fields = map[string]string{"ab": "c"}
	if revision(left) == revision(right) {
		t.Fatal("field boundaries were lost")
	}
	left[0].Observation.Fields = map[string]string{"z": "last", "a": "first"}
	right[0].Observation.Fields = map[string]string{"a": "first", "z": "last"}
	left[0].Observation.Present = map[string]bool{"z": false, "a": true}
	right[0].Observation.Present = map[string]bool{"a": true, "z": false}
	if revision(left) != revision(right) {
		t.Fatal("map insertion order affected identity")
	}
}

func TestQueueInstancesRefuseInvalidSnapshotsWithoutRevision(t *testing.T) {
	base := corpusFacts(t, "08-multiple-retries", "source", "trusted", true)
	bad := append([]Fact(nil), base...)
	bad[0].Ref.SourceID = ""
	for _, tc := range []struct {
		facts []Fact
		limit int
		err   error
	}{
		{base, 0, ErrPartitionLimit}, {base, MaxPartitionFacts + 1, ErrPartitionLimit},
		{base, 1, ErrPartitionLimit}, {bad, 100, ErrPartitionFact},
		{append(append([]Fact(nil), base...), base[0]), 100, ErrPartitionOverlap},
	} {
		got, err := BuildQueueInstances(tc.facts, tc.limit)
		if err != tc.err || !reflect.DeepEqual(got, InstancePartition{}) {
			t.Fatalf("partial identity on refusal: %#v %v", got, err)
		}
	}
	empty, err := BuildQueueInstances(nil, 1)
	if err != nil || len(empty.Instances) != 0 || len(empty.Revision) != 64 {
		t.Fatalf("empty input version: %#v %v", empty, err)
	}
}
