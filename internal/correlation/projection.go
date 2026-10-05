package correlation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
)

var ErrProjectionInvariant = errors.New("inconsistent correlation projection anchors")

type ProjectedQueue struct {
	Key     QueueInstanceKey
	Summary GenerationSummary
}

type ProjectedLink struct {
	Observed QueueLink
	From, To *QueueInstanceKey // copied keys; nil when no candidate was assigned
}

// Projection is one coherent, revisable reconstruction of a bounded snapshot.
// No global journey/status or coverage certificate is implied by composition.
type Projection struct {
	Revision      string
	InputRevision string // candidate-instance input revision, excluding link options
	LinkOptions   LinkOptions
	Queues        []ProjectedQueue
	Links         []ProjectedLink
	Prequeue      PrequeueSessions
	Unresolved    []UnresolvedStream
	Other         []FactRef
}

// BuildProjection binds summaries and link endpoints to the same full revision,
// including explicit link configuration. Neither links nor NOQUEUE sessions
// improve recipient results; unresolved facts remain unassigned. On error there
// is no partial projection or revision. Caller observations are never mutated.
func BuildProjection(facts []Fact, limit int, opts LinkOptions) (Projection, error) {
	links, err := BuildQueueLinks(facts, limit, opts)
	if err != nil {
		return Projection{}, err
	}
	instances, err := BuildQueueInstances(facts, limit)
	if err != nil {
		return Projection{}, err
	}
	summaries, err := BuildSummaries(facts, limit)
	if err != nil {
		return Projection{}, err
	}
	prequeue, err := BuildPrequeueSessions(facts, limit)
	if err != nil {
		return Projection{}, err
	}
	options := LinkOptions{Window: opts.Window, SMTPBindings: append([]SMTPBinding(nil), opts.SMTPBindings...)}
	sort.Slice(options.SMTPBindings, func(i, j int) bool {
		a, b := options.SMTPBindings[i], options.SMTPBindings[j]
		if a.FromInstance != b.FromInstance {
			return a.FromInstance < b.FromInstance
		}
		return a.Relay < b.Relay
	})
	revision := projectionRevision(instances.Revision, options)
	out := Projection{Revision: revision, InputRevision: instances.Revision, LinkOptions: options,
		Prequeue: prequeue, Unresolved: summaries.Unresolved, Other: summaries.Other}
	byAnchor := make(map[FactRef]QueueInstanceKey, len(instances.Instances))
	for _, q := range instances.Instances {
		key := q.Key
		key.Revision = revision
		byAnchor[q.Observed.First] = key
	}
	for _, summary := range summaries.Queues {
		key, found := byAnchor[summary.Queue.Generation.First]
		if !found {
			return Projection{}, ErrProjectionInvariant
		}
		out.Queues = append(out.Queues, ProjectedQueue{key, summary})
	}
	for _, link := range links.Links {
		projected := ProjectedLink{Observed: link}
		for _, endpoint := range []struct {
			anchor *FactRef
			key    **QueueInstanceKey
		}{{link.From, &projected.From}, {link.To, &projected.To}} {
			if endpoint.anchor == nil {
				continue
			}
			key, found := byAnchor[*endpoint.anchor]
			if !found {
				return Projection{}, ErrProjectionInvariant
			}
			*endpoint.key = &key
		}
		out.Links = append(out.Links, projected)
	}
	return out, nil
}

func projectionRevision(input string, opts LinkOptions) string {
	h := sha256.New()
	frame := revisionFramer{h}
	frame.text("correlation-projection-v1")
	frame.text(input)
	frame.number(int64(opts.Window))
	frame.number(int64(len(opts.SMTPBindings)))
	for _, b := range opts.SMTPBindings {
		frame.text(b.FromInstance)
		frame.text(b.Relay)
		frame.text(b.ToInstance)
	}
	return hex.EncodeToString(h.Sum(nil))
}
