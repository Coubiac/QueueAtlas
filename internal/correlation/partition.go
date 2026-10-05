package correlation

import (
	"errors"
	"sort"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
)

const MaxPartitionFacts = 4096

var (
	ErrPartitionLimit   = errors.New("invalid or exceeded correlation fact limit")
	ErrPartitionFact    = errors.New("invalid correlation fact provenance")
	ErrPartitionOverlap = errors.New("duplicate or overlapping correlation fact provenance")
)

// FactRef is a stable identity independent of SQLite insertion IDs or import
// order. Different source/origin references are never deduplicated by text.
type FactRef struct {
	SourceID string
	OriginID string
	Start    int64
	End      int64
}

// Fact.Instance is the configured trusted instance persisted with the event,
// never the declared syslog Host. Observations remain owned by the caller.
type Fact struct {
	Ref         FactRef
	Instance    string
	Observation model.Observation
}

type QueueKey struct{ Instance, QueueID string }

// QueueStream preserves one source/origin boundary. Timed is ordered by its
// preserved date hypotheses, then physical offset; Untimed by physical offset.
// Neither tie breaking nor a cross-stream display order proves chronology.
type QueueStream struct {
	SourceID, OriginID string
	Timed              []FactRef
	Untimed            []FactRef
}

// QueueFacts is an index of candidate facts, NOT one queue generation/journey.
// Multiple streams have unproven continuity/overlap, even on a trusted instance.
type QueueFacts struct {
	Key                  QueueKey
	Streams              []QueueStream
	CrossStreamUncertain bool
}

type Partition struct {
	Queues []QueueFacts
	Other  []FactRef // NOQUEUE, unrecognised/malformed and non-queue facts retained
}

type streamKey struct{ SourceID, OriginID string }
type datedRef struct {
	Ref  FactRef
	Date *time.Time
}

// PartitionFacts indexes a bounded snapshot without mutating observations or
// inventing generation/link/state conclusions. It returns no partial result on
// invalid input. The caller must supply all relevant facts for its reconstruction
// scope: absence from this snapshot is not evidence of absence from the logs.
func PartitionFacts(facts []Fact, limit int) (Partition, error) {
	if limit < 1 || limit > MaxPartitionFacts || len(facts) > limit {
		return Partition{}, ErrPartitionLimit
	}
	byOrigin := make(map[streamKey][]FactRef)
	instances := make(map[streamKey]string)
	for _, f := range facts {
		r := f.Ref
		if r.SourceID == "" || r.OriginID == "" || r.Start < 0 || r.End <= r.Start ||
			f.Instance == "" || f.Observation.SourceID != r.SourceID {
			return Partition{}, ErrPartitionFact
		}
		key := streamKey{r.SourceID, r.OriginID}
		if previous, found := instances[key]; found && previous != f.Instance {
			return Partition{}, ErrPartitionFact
		}
		instances[key] = f.Instance
		byOrigin[key] = append(byOrigin[key], r)
	}
	// Validate physical provenance across every kind, not only queued facts.
	for _, refs := range byOrigin {
		sort.Slice(refs, func(i, j int) bool { return refs[i].Start < refs[j].Start })
		for i := 1; i < len(refs); i++ {
			if refs[i-1].End > refs[i].Start {
				return Partition{}, ErrPartitionOverlap
			}
		}
	}
	groups := make(map[QueueKey]map[streamKey][]datedRef)
	var out Partition
	for _, f := range facts {
		o := f.Observation
		if !queuedFact(o) {
			out.Other = append(out.Other, f.Ref)
			continue
		}
		key := QueueKey{f.Instance, o.QueueID}
		if groups[key] == nil {
			groups[key] = make(map[streamKey][]datedRef)
		}
		stream := streamKey{f.Ref.SourceID, f.Ref.OriginID}
		var date *time.Time
		if usableDate(o.Timestamp) {
			copy := o.Timestamp.Value.UTC()
			date = &copy
		}
		groups[key][stream] = append(groups[key][stream], datedRef{f.Ref, date})
	}
	sortRefs(out.Other)
	for key, streams := range groups {
		queue := QueueFacts{Key: key, CrossStreamUncertain: len(streams) > 1}
		for origin, refs := range streams {
			sort.Slice(refs, func(i, j int) bool {
				a, b := refs[i], refs[j]
				if a.Date != nil && b.Date != nil && !a.Date.Equal(*b.Date) {
					return a.Date.Before(*b.Date)
				}
				if (a.Date == nil) != (b.Date == nil) {
					return a.Date != nil
				}
				return a.Ref.Start < b.Ref.Start
			})
			stream := QueueStream{SourceID: origin.SourceID, OriginID: origin.OriginID}
			for _, f := range refs {
				if f.Date == nil {
					stream.Untimed = append(stream.Untimed, f.Ref)
				} else {
					stream.Timed = append(stream.Timed, f.Ref)
				}
			}
			queue.Streams = append(queue.Streams, stream)
		}
		sort.Slice(queue.Streams, func(i, j int) bool {
			a, b := queue.Streams[i], queue.Streams[j]
			if a.SourceID != b.SourceID {
				return a.SourceID < b.SourceID
			}
			return a.OriginID < b.OriginID
		})
		out.Queues = append(out.Queues, queue)
	}
	sort.Slice(out.Queues, func(i, j int) bool {
		a, b := out.Queues[i].Key, out.Queues[j].Key
		if a.Instance != b.Instance {
			return a.Instance < b.Instance
		}
		return a.QueueID < b.QueueID
	})
	return out, nil
}

func queuedFact(o model.Observation) bool {
	if o.QueueID == "" || o.NoQueue || o.ParseError != "" {
		return false
	}
	switch o.Kind {
	case model.KindMessage, model.KindDelivery, model.KindRemoved, model.KindBounce:
		return true
	}
	return false
}

func usableDate(ts model.Timestamp) bool {
	if ts.Value == nil {
		return false
	}
	switch ts.Quality {
	case model.TimeConfiguredYearAndZone, model.TimeInferredYearAndZone, model.TimeExplicitOffset:
		return true
	}
	return false
}

func sortRefs(refs []FactRef) {
	sort.Slice(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.OriginID != b.OriginID {
			return a.OriginID < b.OriginID
		}
		return a.Start < b.Start
	})
}
