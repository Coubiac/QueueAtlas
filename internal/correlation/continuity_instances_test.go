package correlation

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

// One observed queue cycle split across two distinct physical origins. Their
// lexical display order opposes the explicit claimed succession deliberately.
func splitContinuityCycle(t *testing.T) ([]Fact, ContinuityClaims) {
	t.Helper()
	facts := corpusFacts(t, "13-reused-queue-id", "source", "trusted", true)[:4]
	base := facts[2].Ref.Start
	for i := range facts {
		facts[i].Ref.OriginID = "z-old"
		if i >= 2 {
			facts[i].Ref.OriginID = "a-new"
			facts[i].Ref.Start -= base
			facts[i].Ref.End -= base
		}
	}
	return facts, ContinuityClaims{instanceRevision(facts), []OriginBoundary{{facts[1].Ref, facts[2].Ref}}}
}

func TestContinuityInstancesBindKeysWithoutMergingOrClearingReserves(t *testing.T) {
	facts, claims := splitContinuityCycle(t)
	ordinary, err := BuildQueueInstances(facts, 100)
	if err != nil || len(ordinary.Instances) != 2 {
		t.Fatalf("ordinary candidates: %#v %v", ordinary, err)
	}
	got, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if err != nil || len(got.Partition.Instances) != 2 || len(got.Plan.Boundaries) != 1 ||
		got.Partition.Revision == ordinary.Revision || got.Partition.Revision == got.Plan.Revision ||
		got.Plan.InputRevision != ordinary.Revision {
		t.Fatalf("context binding: %#v %v", got, err)
	}
	for i, q := range got.Partition.Instances {
		if q.Key.Revision != got.Partition.Revision || q.Key == ordinary.Instances[i].Key ||
			q.Key.Generation != ordinary.Instances[i].Key.Generation ||
			!reflect.DeepEqual(q.Observed, ordinary.Instances[i].Observed) || !q.Observed.CrossStreamUncertain {
			t.Fatalf("claim merged or strengthened candidate: %#v", q)
		}
	}
	// A later ordinary reconstruction must not inherit this explicit context.
	again, err := BuildQueueInstances(facts, 100)
	if err != nil || !reflect.DeepEqual(again, ordinary) {
		t.Fatal("explicit context changed the default path or input facts")
	}
}

func TestContinuityInstancesClaimChangesInvalidateAllOldKeys(t *testing.T) {
	facts, claims := splitContinuityCycle(t)
	before, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if err != nil {
		t.Fatal(err)
	}
	claims.Boundaries = nil
	after, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if err != nil || after.Partition.Revision == before.Partition.Revision ||
		after.Plan.InputRevision != before.Plan.InputRevision || len(after.Partition.Instances) != len(before.Partition.Instances) {
		t.Fatalf("changed context reused revision: %#v %v", after, err)
	}
	for i, q := range after.Partition.Instances {
		if q.Key == before.Partition.Instances[i].Key || !reflect.DeepEqual(q.Observed, before.Partition.Instances[i].Observed) {
			t.Fatal("old key survived a claim change or native evidence changed")
		}
	}
}

func TestContinuityInstancesLateFactNeedsReattestationAndNewKeys(t *testing.T) {
	facts, claims := splitContinuityCycle(t)
	before, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if err != nil {
		t.Fatal(err)
	}
	// A late unrelated unknown fact changes the entire input snapshot, even
	// though the claimed endpoints and every queue generation stay unchanged.
	facts = append(facts, continuityFacts()[0])
	stale, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if !errors.Is(err, ErrContinuityStale) || !reflect.DeepEqual(stale, ContinuityInstances{}) {
		t.Fatal("late fact retained the old attestation context")
	}
	plain, err := BuildQueueInstances(facts, 100)
	if err != nil {
		t.Fatal(err)
	}
	claims.InputRevision = plain.Revision // explicit caller reattestation
	after, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if err != nil || after.Plan.InputRevision == before.Plan.InputRevision ||
		after.Partition.Revision == before.Partition.Revision || len(after.Partition.Other) != 1 {
		t.Fatalf("reattested late snapshot: %#v %v", after, err)
	}
	for i, q := range after.Partition.Instances {
		if q.Key == before.Partition.Instances[i].Key || !reflect.DeepEqual(q.Observed, before.Partition.Instances[i].Observed) {
			t.Fatal("late fact silently reused a contextual key or changed native evidence")
		}
	}
}

func TestContinuityInstancesOrderAndOutputOwnership(t *testing.T) {
	facts, claims := splitContinuityCycle(t)
	// Add a disconnected, unresolved origin to exercise preserved non-queue and
	// undated outputs while permutations vary physical input arrival order.
	facts = append(facts, corpusFacts(t, "13-reused-queue-id", "undated", "trusted", false)...)
	facts = append(facts, corpusFacts(t, "06-noqueue-client-reject", "reject", "trusted", true)...)
	claims.InputRevision = instanceRevision(facts)
	want, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if err != nil || len(want.Partition.Unresolved) != 1 || len(want.Partition.Other) != 3 {
		t.Fatalf("preserved unresolved/other: %#v %v", want, err)
	}
	rng := rand.New(rand.NewSource(121))
	for i := 0; i < 10; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := BuildQueueInstancesWithContinuity(shuffled, 100, claims)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("arrival order changed context: %#v %v", got, err)
		}
	}
	changed, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if err != nil {
		t.Fatal(err)
	}
	changed.Plan.Boundaries[0] = OriginBoundary{}
	changed.Partition.Instances[0].Observed.Facts[0] = FactRef{}
	*changed.Partition.Instances[0].Observed.Removed = FactRef{}
	changed.Partition.Unresolved[0].Facts[0] = FactRef{}
	changed.Partition.Other[0] = FactRef{}
	again, err := BuildQueueInstancesWithContinuity(facts, 100, claims)
	if err != nil || !reflect.DeepEqual(again, want) || claims.Boundaries[0].From != facts[1].Ref {
		t.Fatal("result borrowed caller data or another reconstruction")
	}
}

func TestContinuityInstancesRefuseStaleInvalidAndOversizedInputs(t *testing.T) {
	for _, name := range []string{"stale", "boundary", "limit", "provenance", "overlap"} {
		t.Run(name, func(t *testing.T) {
			facts, claims := splitContinuityCycle(t)
			limit, wantErr := 100, ErrContinuityStale
			switch name {
			case "stale":
				facts[0].Observation.Raw += "late edit"
			case "boundary":
				claims.Boundaries[0].To.End++
				wantErr = ErrContinuityClaims
			case "limit":
				limit, wantErr = 1, ErrPartitionLimit
			case "provenance":
				facts[0].Observation.SourceID = "foreign"
				wantErr = ErrPartitionFact
			case "overlap":
				facts = append(facts, facts[0])
				wantErr = ErrPartitionOverlap
			}
			got, err := BuildQueueInstancesWithContinuity(facts, limit, claims)
			if !errors.Is(err, wantErr) || !reflect.DeepEqual(got, ContinuityInstances{}) {
				t.Fatalf("usable result after refusal: %#v %v", got, err)
			}
		})
	}
	got, err := BuildQueueInstancesWithContinuity(nil, 1, ContinuityClaims{InputRevision: instanceRevision(nil)})
	ordinary, plainErr := BuildQueueInstances(nil, 1)
	if err != nil || plainErr != nil || len(got.Partition.Instances) != 0 ||
		got.Partition.Revision == "" || got.Partition.Revision == ordinary.Revision || got.Plan.InputRevision != ordinary.Revision {
		t.Fatalf("empty explicit context: %#v %v", got, err)
	}
}
