package correlation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/model"
)

func continuityFacts() []Fact {
	var facts []Fact
	for _, origin := range []string{"a", "b", "c", "d"} {
		for _, start := range []int64{0, 10} {
			facts = append(facts, Fact{
				Ref: FactRef{"source", origin, start, start + 10}, Instance: "configured",
				Observation: model.Observation{SourceID: "source", Kind: model.KindUnknown},
			})
		}
	}
	return facts
}

func TestContinuityClaimsCanonicalRevisionAndOwnership(t *testing.T) {
	facts := continuityFacts()
	claims := ContinuityClaims{instanceRevision(facts), []OriginBoundary{
		{facts[3].Ref, facts[4].Ref}, {facts[1].Ref, facts[2].Ref},
	}}
	want, err := CheckContinuityClaims(facts, 100, claims)
	if err != nil || len(want.Boundaries) != 2 || want.Boundaries[0].From != facts[1].Ref {
		t.Fatalf("canonical plan: %#v %v", want, err)
	}
	copyFacts := append([]Fact(nil), facts...)
	for i, j := 0, len(copyFacts)-1; i < j; i, j = i+1, j-1 {
		copyFacts[i], copyFacts[j] = copyFacts[j], copyFacts[i]
	}
	claims.Boundaries[0], claims.Boundaries[1] = claims.Boundaries[1], claims.Boundaries[0]
	got, err := CheckContinuityClaims(copyFacts, 100, claims)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("order changed plan: %#v %v", got, err)
	}
	got.Boundaries[0] = OriginBoundary{}
	if claims.Boundaries[0].From != facts[1].Ref || want.Boundaries[0].From != facts[1].Ref {
		t.Fatal("plan borrowed claims or another plan")
	}
	empty, err := CheckContinuityClaims(facts, 100, ContinuityClaims{InputRevision: claims.InputRevision})
	if err != nil || empty.Revision == want.Revision {
		t.Fatal("claims are missing from the revision")
	}
}

func TestContinuityClaimsRejectContradictoryGraphs(t *testing.T) {
	facts := continuityFacts()
	ab := OriginBoundary{facts[1].Ref, facts[2].Ref}
	bc := OriginBoundary{facts[3].Ref, facts[4].Ref}
	for name, boundaries := range map[string][]OriginBoundary{
		"duplicate": {ab, ab},
		"fork":      {ab, {facts[1].Ref, facts[4].Ref}},
		"join":      {ab, {facts[5].Ref, facts[2].Ref}},
		"self":      {{facts[1].Ref, facts[0].Ref}},
		"cycle":     {ab, bc, {facts[5].Ref, facts[0].Ref}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := CheckContinuityClaims(facts, 100, ContinuityClaims{instanceRevision(facts), boundaries})
			if !errors.Is(err, ErrContinuityClaims) || !reflect.DeepEqual(got, ContinuityPlan{}) {
				t.Fatalf("usable contradictory plan: %#v %v", got, err)
			}
		})
	}
}

func TestContinuityClaimsExactPhysicalBoundariesAndNamespace(t *testing.T) {
	for _, name := range []string{"interior-from", "interior-to", "wrong-end", "missing", "cross-source", "cross-instance", "nonzero-start", "nonqueue-tail"} {
		t.Run(name, func(t *testing.T) {
			facts := continuityFacts()
			b := OriginBoundary{facts[1].Ref, facts[2].Ref}
			switch name {
			case "interior-from":
				b.From = facts[0].Ref
			case "interior-to":
				b.To = facts[3].Ref
			case "wrong-end":
				b.From.End++
			case "missing":
				b.From.OriginID = "private@example.invalid"
			case "cross-source":
				for i := 2; i <= 3; i++ {
					facts[i].Ref.SourceID, facts[i].Observation.SourceID = "archive", "archive"
				}
				b.To = facts[2].Ref
			case "cross-instance":
				facts[2].Instance, facts[3].Instance = "foreign", "foreign"
			case "nonzero-start":
				facts[2].Ref.Start = 1
				b.To = facts[2].Ref
			case "nonqueue-tail":
				x := facts[1]
				x.Ref.Start, x.Ref.End = 20, 30
				x.Observation.NoQueue = true
				facts = append(facts, x)
			}
			got, err := CheckContinuityClaims(facts, 100, ContinuityClaims{instanceRevision(facts), []OriginBoundary{b}})
			if !errors.Is(err, ErrContinuityClaims) || !reflect.DeepEqual(got, ContinuityPlan{}) || strings.Contains(err.Error(), "private") {
				t.Fatalf("unsafe boundary/result: %#v %v", got, err)
			}
		})
	}
}

func TestContinuityClaimsFreshnessAndLimits(t *testing.T) {
	facts := continuityFacts()
	claims := ContinuityClaims{instanceRevision(facts), []OriginBoundary{{facts[1].Ref, facts[2].Ref}}}
	facts[7].Observation.Raw = "late change in an unrelated unknown fact"
	got, err := CheckContinuityClaims(facts, 100, claims)
	if !errors.Is(err, ErrContinuityStale) || !reflect.DeepEqual(got, ContinuityPlan{}) {
		t.Fatalf("stale claims accepted: %#v %v", got, err)
	}
	claims.InputRevision = instanceRevision(facts)
	claims.Boundaries = make([]OriginBoundary, MaxContinuityBoundaries+1)
	if _, err := CheckContinuityClaims(facts, 100, claims); !errors.Is(err, ErrContinuityClaims) {
		t.Fatal("edge limit not enforced")
	}
	if _, err := CheckContinuityClaims(facts, 1, claims); !errors.Is(err, ErrPartitionLimit) {
		t.Fatal("fact limit not enforced")
	}
	badFacts := append(append([]Fact(nil), facts...), facts[0])
	if _, err := CheckContinuityClaims(badFacts, 100, claims); !errors.Is(err, ErrPartitionOverlap) {
		t.Fatal("physical overlap ignored")
	}
	empty, err := CheckContinuityClaims(nil, 1, ContinuityClaims{InputRevision: instanceRevision(nil)})
	if err != nil || empty.InputRevision == "" || len(empty.Boundaries) != 0 {
		t.Fatalf("empty snapshot: %#v %v", empty, err)
	}
}

func TestContinuityClaimsNeverInferOrMergeQueueGenerations(t *testing.T) {
	facts := corpusFacts(t, "13-reused-queue-id", "source", "trusted", true)
	other := corpusFacts(t, "13-reused-queue-id", "source", "trusted", true)
	for i := range other {
		other[i].Ref.OriginID = "another-origin"
	}
	facts = append(facts, other...)
	before, err := BuildQueueInstances(facts, 100)
	if err != nil || len(before.Instances) != 4 {
		t.Fatalf("fixture: %#v %v", before, err)
	}
	plan, err := CheckContinuityClaims(facts, 100, ContinuityClaims{InputRevision: before.Revision})
	if err != nil || len(plan.Boundaries) != 0 {
		t.Fatal("equal log content, Queue IDs or dates invented a claim")
	}
	after, err := BuildQueueInstances(facts, 100)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatal("checking claims changed generations")
	}
}

func TestContinuityClaimsAcceptBoundedChainsAndVersionChangedEdges(t *testing.T) {
	var facts []Fact
	for i := 0; i <= MaxContinuityBoundaries; i++ {
		facts = append(facts, Fact{
			Ref: FactRef{"source", fmt.Sprintf("origin-%03d", i), 0, 10}, Instance: "configured",
			Observation: model.Observation{SourceID: "source", Kind: model.KindUnknown},
		})
	}
	claims := ContinuityClaims{InputRevision: instanceRevision(facts)}
	for i := 0; i < MaxContinuityBoundaries; i++ {
		claims.Boundaries = append(claims.Boundaries, OriginBoundary{facts[i].Ref, facts[i+1].Ref})
	}
	full, err := CheckContinuityClaims(facts, MaxPartitionFacts, claims)
	if err != nil || len(full.Boundaries) != MaxContinuityBoundaries {
		t.Fatalf("full bounded chain: %#v %v", full, err)
	}
	// Removing one edge leaves two valid chains; disconnected chains do not
	// become an implicit succession or inherit the former claim revision.
	claims.Boundaries = append(claims.Boundaries[:100:100], claims.Boundaries[101:]...)
	split, err := CheckContinuityClaims(facts, MaxPartitionFacts, claims)
	if err != nil || split.Revision == full.Revision || split.InputRevision != full.InputRevision {
		t.Fatalf("changed edges reused plan revision: %#v %v", split, err)
	}
}
