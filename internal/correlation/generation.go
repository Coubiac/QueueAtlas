package correlation

import (
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
)

type GenerationReason string

const (
	GenerationUndated          GenerationReason = "undated"
	GenerationBoundaryUnproven GenerationReason = "boundary_unproven"
	GenerationConflictingIDs   GenerationReason = "conflicting_message_ids"
)

// QueueGeneration is scoped to one source/origin. First is its revisable anchor,
// not a global queue identity or SQLite insertion ID. Removed only reports the
// removal event: it proves neither successful delivery nor complete log coverage.
type QueueGeneration struct {
	Key                  QueueKey
	First                FactRef
	Facts                []FactRef
	Removed              *FactRef
	ReceiptObserved      bool
	HasNonExplicitTime   bool
	CrossStreamUncertain bool
}

type UnresolvedStream struct {
	Key                QueueKey
	SourceID, OriginID string
	Reason             GenerationReason
	Facts              []FactRef
}

type GenerationPartition struct {
	Generations []QueueGeneration
	Unresolved  []UnresolvedStream
	Other       []FactRef
}

// BuildGenerations rebuilds a bounded snapshot under its preserved date
// hypotheses. Origins remain independent. An uncertain boundary makes the entire
// affected queue stream unresolved; no partial confident generation is exposed.
func BuildGenerations(facts []Fact, limit int) (GenerationPartition, error) {
	partition, err := PartitionFacts(facts, limit)
	if err != nil {
		return GenerationPartition{}, err
	}
	lookup := make(map[FactRef]Fact, len(facts))
	for _, f := range facts {
		lookup[f.Ref] = f
	}
	out := GenerationPartition{Other: partition.Other}
	for _, queue := range partition.Queues {
		for _, stream := range queue.Streams {
			var nodes []QueueGeneration
			var reason GenerationReason
			if len(stream.Untimed) > 0 {
				reason = GenerationUndated
			} else {
				nodes, reason = splitGenerations(queue.Key, stream.Timed, lookup, queue.CrossStreamUncertain)
			}
			if reason != "" {
				refs := append([]FactRef(nil), stream.Timed...)
				refs = append(refs, stream.Untimed...)
				sortRefs(refs)
				out.Unresolved = append(out.Unresolved, UnresolvedStream{queue.Key, stream.SourceID, stream.OriginID, reason, refs})
			} else {
				out.Generations = append(out.Generations, nodes...)
			}
		}
	}
	return out, nil
}

func splitGenerations(key QueueKey, refs []FactRef, lookup map[FactRef]Fact, crossStream bool) ([]QueueGeneration, GenerationReason) {
	if physicalBoundaryConflict(refs, lookup) {
		return nil, GenerationBoundaryUnproven
	}
	var nodes []QueueGeneration
	var current QueueGeneration
	var closedAt time.Time
	var messageID string
	var hasMessageID bool
	for _, ref := range refs {
		f := lookup[ref]
		o := f.Observation
		if current.Removed != nil {
			// Date equality is not enough, even when file offsets would give a
			// deterministic display order. Require a new observed receipt.
			if !observedReceipt(o) || !o.Timestamp.Value.After(closedAt) {
				return nil, GenerationBoundaryUnproven
			}
			nodes = append(nodes, current)
			current = QueueGeneration{}
			hasMessageID = false
		}
		if len(current.Facts) == 0 {
			current = QueueGeneration{Key: key, First: ref, ReceiptObserved: observedReceipt(o), CrossStreamUncertain: crossStream}
		}
		if o.Service == "cleanup" {
			if id, present := o.Field("message-id"); present {
				if hasMessageID && id != messageID {
					return nil, GenerationConflictingIDs
				}
				messageID, hasMessageID = id, true
			}
		}
		current.Facts = append(current.Facts, ref)
		if o.Timestamp.Quality != model.TimeExplicitOffset {
			current.HasNonExplicitTime = true
		}
		if o.Kind == model.KindRemoved {
			removed := ref
			current.Removed = &removed
			closedAt = o.Timestamp.Value.UTC()
		}
	}
	if len(current.Facts) > 0 {
		nodes = append(nodes, current)
	}
	return nodes, ""
}

// Sorting date hypotheses must never hide a contradictory physical removal.
// Within a cycle, out-of-order timestamps remain permitted; across a removal,
// earlier physical facts cannot be later-dated and later facts must be strictly
// later-dated. This is a conservative boundary test, not a clock certificate.
func physicalBoundaryConflict(refs []FactRef, lookup map[FactRef]Fact) bool {
	physical := append([]FactRef(nil), refs...)
	sortRefs(physical)
	var latest time.Time
	var haveLatest bool
	var removedAt time.Time
	var haveRemoval bool
	for _, ref := range physical {
		o := lookup[ref].Observation
		date := o.Timestamp.Value.UTC()
		if haveRemoval && !date.After(removedAt) {
			return true
		}
		if o.Kind == model.KindRemoved {
			if haveLatest && latest.After(date) {
				return true
			}
			removedAt, haveRemoval = date, true
		}
		if !haveLatest || date.After(latest) {
			latest, haveLatest = date, true
		}
	}
	return false
}

func observedReceipt(o model.Observation) bool {
	if o.Kind != model.KindMessage {
		return false
	}
	switch o.Service {
	case "cleanup":
		_, present := o.Field("message-id")
		return present
	case "smtpd":
		_, present := o.Field("client")
		return present
	case "pickup":
		_, present := o.Field("uid")
		return present
	case "qmgr":
		_, from := o.Field("from")
		_, size := o.Field("size")
		_, recipients := o.Field("nrcpt")
		_, status := o.Field("status")
		return from && size && recipients && !status
	}
	return false
}
