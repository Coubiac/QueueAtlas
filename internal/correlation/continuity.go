package correlation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
)

const MaxContinuityBoundaries = 256

var (
	ErrContinuityClaims = errors.New("invalid continuity claims")
	ErrContinuityStale  = errors.New("continuity claims refer to another input revision")
)

// OriginBoundary claims that From ends a sealed origin and To starts its direct
// successor at byte zero, without omitted bytes or intervening origins. This is
// a trusted caller assertion, NOT evidence extracted from dates or log content.
// Its producer must establish physical succession and sealing independently.
type OriginBoundary struct {
	From, To FactRef
}

type ContinuityClaims struct {
	InputRevision string
	Boundaries    []OriginBoundary
}

// ContinuityPlan describes internally consistent claims for one input snapshot.
// Revision versions facts AND claims; it neither authenticates their producer nor
// proves coverage. Checking a plan does not merge queues or change projections.
type ContinuityPlan struct {
	Revision, InputRevision string
	Boundaries              []OriginBoundary
}

// CheckContinuityClaims checks provenance, freshness and a nonbranching acyclic
// origin graph. All facts (including unknown/NOQUEUE) participate in boundary
// checks. Snapshot extremities cannot prove that the real origin is sealed or
// fully ingested: that remains the responsibility of the trusted claim producer.
// Facts must remain immutable throughout the call. Errors return no partial plan.
func CheckContinuityClaims(facts []Fact, limit int, claims ContinuityClaims) (ContinuityPlan, error) {
	if _, err := PartitionFacts(facts, limit); err != nil {
		return ContinuityPlan{}, err
	}
	if len(claims.Boundaries) > MaxContinuityBoundaries {
		return ContinuityPlan{}, ErrContinuityClaims
	}
	inputRevision := instanceRevision(facts)
	if claims.InputRevision != inputRevision {
		return ContinuityPlan{}, ErrContinuityStale
	}
	type extremities struct {
		first, last FactRef
		instance    string
	}
	origins := make(map[streamKey]extremities)
	for _, f := range facts {
		key := streamKey{f.Ref.SourceID, f.Ref.OriginID}
		x, found := origins[key]
		if !found {
			x = extremities{f.Ref, f.Ref, f.Instance}
		} else {
			if f.Ref.Start < x.first.Start {
				x.first = f.Ref
			}
			if f.Ref.Start > x.last.Start {
				x.last = f.Ref
			}
		}
		origins[key] = x
	}
	boundaries := append([]OriginBoundary(nil), claims.Boundaries...)
	successors := make(map[streamKey]streamKey)
	predecessors := make(map[streamKey]streamKey)
	for _, b := range boundaries {
		from, to := streamKey{b.From.SourceID, b.From.OriginID}, streamKey{b.To.SourceID, b.To.OriginID}
		a, haveA := origins[from]
		z, haveZ := origins[to]
		_, hasNext := successors[from]
		_, hasPrevious := predecessors[to]
		if !haveA || !haveZ || from == to || from.SourceID != to.SourceID ||
			a.instance != z.instance || b.From != a.last || b.To != z.first ||
			b.To.Start != 0 || hasNext || hasPrevious {
			return ContinuityPlan{}, ErrContinuityClaims
		}
		successors[from], predecessors[to] = to, from
	}
	// At most 256 edges; iterative traversal avoids an input-controlled recursion.
	done := make(map[streamKey]bool)
	for start := range successors {
		path := make(map[streamKey]bool)
		for current := start; !done[current]; {
			if path[current] {
				return ContinuityPlan{}, ErrContinuityClaims
			}
			path[current] = true
			next, exists := successors[current]
			if !exists {
				break
			}
			current = next
		}
		for key := range path {
			done[key] = true
		}
	}
	sort.Slice(boundaries, func(i, j int) bool {
		a, b := boundaries[i].From, boundaries[j].From
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		return a.OriginID < b.OriginID
	})
	h := sha256.New()
	frame := revisionFramer{h}
	frame.text("continuity-claims-v1")
	frame.text(inputRevision)
	frame.number(int64(len(boundaries)))
	for _, b := range boundaries {
		for _, ref := range []FactRef{b.From, b.To} {
			frame.text(ref.SourceID)
			frame.text(ref.OriginID)
			frame.number(ref.Start)
			frame.number(ref.End)
		}
	}
	return ContinuityPlan{hex.EncodeToString(h.Sum(nil)), inputRevision, boundaries}, nil
}
