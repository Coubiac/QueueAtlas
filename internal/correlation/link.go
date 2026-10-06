package correlation

import (
	"errors"
	"strings"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
)

const MaxSMTPBindings = 64

var ErrLinkOptions = errors.New("invalid queue link options")

// SMTPBinding is explicit configuration, never inferred from a logged hostname
// or loopback address. Relay must match the reported relay literally.
type SMTPBinding struct{ FromInstance, Relay, ToInstance string }
type LinkOptions struct {
	Window       time.Duration // positive, at most 24h
	SMTPBindings []SMTPBinding // at most 64, unique FromInstance/Relay pairs
}

type QueueLinkState string

const (
	LinkCandidate    QueueLinkState = "candidate"
	LinkCorroborated QueueLinkState = "corroborated"
)

type LinkReason string

const (
	LinkSourceUnresolved      LinkReason = "source_unresolved"
	LinkSourceAmbiguous       LinkReason = "source_ambiguous"
	LinkTargetInstanceUnknown LinkReason = "target_instance_unknown"
	LinkTargetAbsent          LinkReason = "target_absent"
	LinkTargetAmbiguous       LinkReason = "target_ambiguous"
	LinkEvidenceInsufficient  LinkReason = "evidence_insufficient"
	LinkSelfReference         LinkReason = "self_reference"
)

// QueueLink relates candidate anchors under their retained date hypotheses.
// Corroborated is not coverage, immutable global identity or recipient success.
// Bounce notifications remain a different kind from message forwarding.
type QueueLink struct {
	Hint               QueueHint
	State              QueueLinkState
	Reason             LinkReason
	From, To           *FactRef
	TargetInstance     Field
	Binding            *SMTPBinding
	Evidence           []FactRef
	HasNonExplicitTime bool
}

type QueueLinks struct {
	Hints       QueueHints
	Generations GenerationPartition
	Links       []QueueLink
}

// BuildQueueLinks keeps every hint, including failures to corroborate. It does
// not merge generations, journeys or their recipient statuses. A strictly later
// target receipt is required; this conservative time rule also prevents cycles.
func BuildQueueLinks(facts []Fact, limit int, opts LinkOptions) (QueueLinks, error) {
	bindings, err := linkBindings(opts)
	if err != nil {
		return QueueLinks{}, err
	}
	hints, err := BuildQueueHints(facts, limit)
	if err != nil {
		return QueueLinks{}, err
	}
	generations, err := BuildGenerations(facts, limit)
	if err != nil {
		return QueueLinks{}, err
	}
	lookup := make(map[FactRef]Fact, len(facts))
	for _, f := range facts {
		lookup[f.Ref] = f
	}
	byFact := make(map[FactRef]int)
	byKey := make(map[QueueKey][]int)
	for i, g := range generations.Generations {
		byKey[g.Key] = append(byKey[g.Key], i)
		for _, ref := range g.Facts {
			byFact[ref] = i
		}
	}
	unresolved := make(map[QueueKey]bool)
	for _, stream := range generations.Unresolved {
		unresolved[stream.Key] = true
	}
	out := QueueLinks{Hints: hints, Generations: generations}
	for _, hint := range hints.Hints {
		link := QueueLink{Hint: hint, State: LinkCandidate, Reason: LinkSourceUnresolved, Evidence: []FactRef{hint.Evidence}}
		index, found := byFact[hint.Evidence]
		if !found {
			out.Links = append(out.Links, link)
			continue
		}
		source := generations.Generations[index]
		anchor := source.First
		link.From = &anchor
		link.HasNonExplicitTime = source.HasNonExplicitTime
		if source.CrossStreamUncertain || unresolved[source.Key] {
			link.Reason = LinkSourceAmbiguous
			out.Links = append(out.Links, link)
			continue
		}
		link.TargetInstance = hint.TargetInstance
		if hint.Kind == HintSMTPQueue {
			binding, present := bindings[bindingKey{hint.From.Instance, hint.Relay.Value}]
			if !hint.Relay.Present || !present {
				link.Reason = LinkTargetInstanceUnknown
				out.Links = append(out.Links, link)
				continue
			}
			copy := binding
			link.Binding = &copy
			link.TargetInstance = Field{binding.ToInstance, true}
		}
		targetKey := QueueKey{link.TargetInstance.Value, hint.TargetQueueID}
		if targetKey == source.Key {
			link.Reason = LinkSelfReference
			out.Links = append(out.Links, link)
			continue
		}
		if unresolved[targetKey] {
			link.Reason = LinkTargetAmbiguous
			out.Links = append(out.Links, link)
			continue
		}
		var targets []QueueGeneration
		for _, i := range byKey[targetKey] {
			target := generations.Generations[i]
			if plausibleTarget(source, target, hint.Evidence, lookup, opts.Window) {
				targets = append(targets, target)
			}
		}
		switch {
		case len(byKey[targetKey]) == 0:
			link.Reason = LinkTargetAbsent
		case len(targets) == 0:
			link.Reason = LinkEvidenceInsufficient
		case len(targets) > 1 || targets[0].CrossStreamUncertain:
			link.Reason = LinkTargetAmbiguous
		default:
			target := targets[0]
			evidence, ok := corroboratingFacts(source, target, hint, lookup)
			if !ok {
				link.Reason = LinkEvidenceInsufficient
			} else {
				anchor := target.First
				link.To, link.State, link.Reason = &anchor, LinkCorroborated, ""
				link.HasNonExplicitTime = link.HasNonExplicitTime || target.HasNonExplicitTime
				link.Evidence = uniqueLinkEvidence(append(link.Evidence, evidence...))
			}
		}
		out.Links = append(out.Links, link)
	}
	return out, nil
}

type bindingKey struct{ Instance, Relay string }

func linkBindings(opts LinkOptions) (map[bindingKey]SMTPBinding, error) {
	if opts.Window <= 0 || opts.Window > 24*time.Hour || len(opts.SMTPBindings) > MaxSMTPBindings {
		return nil, ErrLinkOptions
	}
	out := make(map[bindingKey]SMTPBinding, len(opts.SMTPBindings))
	for _, binding := range opts.SMTPBindings {
		for _, value := range []string{binding.FromInstance, binding.Relay, binding.ToInstance} {
			if value == "" || len(value) > 1024 || strings.ContainsAny(value, "\x00\r\n\t") {
				return nil, ErrLinkOptions
			}
		}
		key := bindingKey{binding.FromInstance, binding.Relay}
		if _, duplicate := out[key]; duplicate {
			return nil, ErrLinkOptions
		}
		out[key] = binding
	}
	return out, nil
}

func plausibleTarget(source, target QueueGeneration, hint FactRef, lookup map[FactRef]Fact, window time.Duration) bool {
	if !source.ReceiptObserved || !target.ReceiptObserved {
		return false
	}
	sourceDate := lookup[source.First].Observation.Timestamp.Value
	targetDate := lookup[target.First].Observation.Timestamp.Value
	hintDate := lookup[hint].Observation.Timestamp.Value
	if !targetDate.After(*sourceDate) {
		return false
	}
	// Compare without subtracting two potentially distant dates into a duration.
	return !targetDate.Before(hintDate.Add(-window)) && !targetDate.After(hintDate.Add(window))
}

type fieldEvidence struct {
	Field Field
	Refs  []FactRef
	Valid bool
}

func generationField(g QueueGeneration, service, name string, lookup map[FactRef]Fact) fieldEvidence {
	out := fieldEvidence{Valid: true}
	for _, ref := range g.Facts {
		o := lookup[ref].Observation
		if o.Kind != model.KindMessage || o.Service != service {
			continue
		}
		value, present := o.Field(name)
		if !present {
			continue
		}
		if out.Field.Present && out.Field.Value != value {
			out.Valid = false
		}
		// Check every repetition for contradictions, but cite only the first
		// positive field proof. All originals remain in the generation facts;
		// repeated metadata must not multiply proof slices for every link.
		if !out.Field.Present {
			out.Refs = append(out.Refs, ref)
		}
		out.Field = Field{value, true}
	}
	return out
}

func corroboratingFacts(source, target QueueGeneration, hint QueueHint, lookup map[FactRef]Fact) ([]FactRef, bool) {
	sender := generationField(source, "qmgr", "from", lookup)
	targetSender := generationField(target, "qmgr", "from", lookup)
	if !sender.Valid || !targetSender.Valid || !sender.Field.Present || !targetSender.Field.Present {
		return nil, false
	}
	evidence := append([]FactRef{source.First, target.First}, sender.Refs...)
	evidence = append(evidence, targetSender.Refs...)
	if hint.Kind == HintBounceNotification {
		if sender.Field.Value == "" || targetSender.Field.Value != "" {
			return nil, false
		}
		ref, found := recipientEvidence(target, sender.Field.Value, lookup)
		return append(evidence, ref), found
	}
	if sender.Field.Value != targetSender.Field.Value {
		return nil, false
	}
	sourceID := generationField(source, "cleanup", "message-id", lookup)
	targetID := generationField(target, "cleanup", "message-id", lookup)
	if !sourceID.Valid || !targetID.Valid || (sourceID.Field.Present && targetID.Field.Present && sourceID.Field.Value != targetID.Field.Value) {
		return nil, false
	}
	if hint.Kind == HintSMTPQueue {
		if !sourceID.Field.Present || sourceID.Field.Value == "" || !targetID.Field.Present {
			return nil, false
		}
		delivery, ok := DeliveryFrom(lookup[hint.Evidence].Observation)
		if !ok || delivery.Recipient == "" {
			return nil, false
		}
		ref, found := recipientEvidence(target, delivery.Recipient, lookup)
		if !found {
			return nil, false
		}
		evidence = append(evidence, ref)
	}
	evidence = append(evidence, sourceID.Refs...)
	evidence = append(evidence, targetID.Refs...)
	return evidence, true
}

func recipientEvidence(g QueueGeneration, address string, lookup map[FactRef]Fact) (FactRef, bool) {
	for _, ref := range g.Facts {
		if d, ok := DeliveryFrom(lookup[ref].Observation); ok && d.Recipient == address && d.Status != DeliveryUnknown {
			return ref, true
		}
	}
	return FactRef{}, false
}

func uniqueLinkEvidence(refs []FactRef) []FactRef {
	seen := make(map[FactRef]bool, len(refs))
	var out []FactRef
	for _, ref := range refs {
		if !seen[ref] {
			seen[ref] = true
			out = append(out, ref)
		}
	}
	sortRefs(out)
	return out
}
