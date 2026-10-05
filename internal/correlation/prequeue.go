package correlation

import (
	"strings"

	"github.com/Coubiac/mailtrace/internal/model"
)

type PrequeueDisposition string

const (
	PrequeueRejected PrequeueDisposition = "rejected"
	PrequeueWarning  PrequeueDisposition = "warning"
)

// PrequeueAttempt is one smtpd NOQUEUE report, identified by physical provenance.
// ProcessID is metadata, never a session identity or a link to an accepted queue.
// Warning reports a warn_if_reject diagnostic, not an actual SMTP rejection.
type PrequeueAttempt struct {
	Ref          FactRef
	Instance     string
	ProcessID    string
	Timestamp    model.Timestamp
	NativeAction string
	Disposition  PrequeueDisposition
	Sender       Field
	Recipient    Field
	Protocol     Field
	Helo         Field
	Message      string
}

type PrequeuePartition struct {
	Attempts []PrequeueAttempt
	Queued   []QueueFacts // candidate index retained, no session/queue assignment
	Other    []FactRef
}

// BuildPrequeue keeps independent NOQUEUE reports, including undated and
// recipient-less reports. It uses PartitionFacts validation/bounds and sorts by
// provenance for display, without assuming date order or joining by PID/address.
func BuildPrequeue(facts []Fact, limit int) (PrequeuePartition, error) {
	partition, err := PartitionFacts(facts, limit)
	if err != nil {
		return PrequeuePartition{}, err
	}
	lookup := make(map[FactRef]Fact, len(facts))
	for _, f := range facts {
		lookup[f.Ref] = f
	}
	out := PrequeuePartition{Queued: partition.Queues}
	for _, ref := range partition.Other {
		f := lookup[ref]
		attempt, ok := prequeueFrom(f)
		if ok {
			out.Attempts = append(out.Attempts, attempt)
		} else {
			out.Other = append(out.Other, ref)
		}
	}
	return out, nil
}

func prequeueFrom(f Fact) (PrequeueAttempt, bool) {
	o := f.Observation
	if o.Kind != model.KindReject || o.Service != "smtpd" || !o.NoQueue || o.QueueID != "" || o.ParseError != "" || len(o.Message) > model.MaxLineBytes {
		return PrequeueAttempt{}, false
	}
	var action string
	var disposition PrequeueDisposition
	switch {
	case strings.HasPrefix(o.Message, "reject:"):
		action, disposition = "reject", PrequeueRejected
	case strings.HasPrefix(o.Message, "reject_warning:"):
		action, disposition = "reject_warning", PrequeueWarning
	default:
		return PrequeueAttempt{}, false
	}
	stamp := o.Timestamp
	if stamp.Value != nil {
		date := stamp.Value.UTC()
		stamp.Value = &date
	}
	return PrequeueAttempt{
		Ref: f.Ref, Instance: f.Instance, ProcessID: o.PID, Timestamp: stamp,
		NativeAction: action, Disposition: disposition, Sender: observedField(o, "from"),
		Recipient: observedField(o, "to"), Protocol: observedField(o, "proto"),
		Helo: observedField(o, "helo"), Message: o.Message,
	}, true
}
