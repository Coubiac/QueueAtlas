package correlation

import (
	"sort"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
)

type RecipientAttempt struct {
	Ref       FactRef
	Delivery  Delivery
	Timestamp model.Timestamp
}

// RecipientResult is the latest observed result, not a final delivery verdict.
// Latest contains all attempts at the latest date. Conflicting results at that
// date stay unknown; offset tie breaking never decides which actually happened last.
type RecipientResult struct {
	Address            string
	Attempts           []RecipientAttempt
	Latest             []FactRef
	ObservedStatus     DeliveryStatus
	OrderUncertain     bool
	AddressUnspecified bool
}

type GenerationRecipients struct {
	Generation            QueueGeneration
	Recipients            []RecipientResult
	Expirations           []QueueExpiration
	UnprojectedDeliveries []FactRef
}

type RecipientPartition struct {
	Queues     []GenerationRecipients
	Unresolved []UnresolvedStream
	Other      []FactRef
}

type recipientState struct {
	Result     RecipientResult
	LatestTime time.Time
}

// BuildRecipients preserves every delivery attempt in each independent candidate
// generation. It shares the snapshot bounds/refusals of BuildGenerations, does
// not merge recipients by orig_to/case, and makes no global success conclusion.
func BuildRecipients(facts []Fact, limit int) (RecipientPartition, error) {
	partition, err := BuildGenerations(facts, limit)
	if err != nil {
		return RecipientPartition{}, err
	}
	lookup := make(map[FactRef]Fact, len(facts))
	for _, f := range facts {
		lookup[f.Ref] = f
	}
	out := RecipientPartition{Unresolved: partition.Unresolved, Other: partition.Other}
	for _, g := range partition.Generations {
		queue := GenerationRecipients{Generation: g}
		recipients := make(map[string]*recipientState)
		for _, ref := range g.Facts {
			o := lookup[ref].Observation
			if expiration, ok := queueExpirationFrom(ref, o); ok {
				queue.Expirations = append(queue.Expirations, expiration)
			}
			d, ok := DeliveryFrom(o)
			if !ok {
				if o.Kind == model.KindDelivery {
					queue.UnprojectedDeliveries = append(queue.UnprojectedDeliveries, ref)
				}
				continue
			}
			date := o.Timestamp.Value.UTC()
			stamp := o.Timestamp
			stamp.Value = &date
			state := recipients[d.Recipient]
			if state == nil {
				state = &recipientState{Result: RecipientResult{Address: d.Recipient, AddressUnspecified: d.Recipient == ""}}
				recipients[d.Recipient] = state
			}
			r := &state.Result
			if len(r.Attempts) == 0 || date.After(state.LatestTime) {
				state.LatestTime = date
				r.Latest = []FactRef{ref}
				r.ObservedStatus = d.Status
				r.OrderUncertain = false
			} else if date.Equal(state.LatestTime) {
				r.Latest = append(r.Latest, ref)
				if d.Status != r.ObservedStatus {
					r.ObservedStatus = DeliveryUnknown
					r.OrderUncertain = true
				}
			}
			r.Attempts = append(r.Attempts, RecipientAttempt{ref, d, stamp})
		}
		for _, state := range recipients {
			if state.Result.AddressUnspecified {
				state.Result.ObservedStatus = DeliveryUnknown
			}
			queue.Recipients = append(queue.Recipients, state.Result)
		}
		sort.Slice(queue.Recipients, func(i, j int) bool { return queue.Recipients[i].Address < queue.Recipients[j].Address })
		out.Queues = append(out.Queues, queue)
	}
	return out, nil
}
