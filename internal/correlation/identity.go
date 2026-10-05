package correlation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"sort"
	"strconv"
)

// QueueInstanceKey identifies one candidate within one immutable input revision.
// Generation is a zero-based ordinal for Instance/QueueID, not a chronology
// across origins. Never use the ordinal without Revision or as continuity proof.
type QueueInstanceKey struct {
	Revision          string
	Instance, QueueID string
	Generation        int
}

type QueueInstance struct {
	Key      QueueInstanceKey
	Observed QueueGeneration
}

type InstancePartition struct {
	Revision   string
	Instances  []QueueInstance
	Unresolved []UnresolvedStream
	Other      []FactRef
}

// BuildQueueInstances assigns deterministic revision-scoped candidate keys.
// Reordering unchanged facts preserves keys. A late import or changed observation
// invalidates the entire revision, including formerly unchanged ordinals. The
// digest versions inputs; it is neither content deduplication nor log coverage.
func BuildQueueInstances(facts []Fact, limit int) (InstancePartition, error) {
	partition, err := BuildGenerations(facts, limit)
	if err != nil {
		return InstancePartition{}, err
	}
	revision := instanceRevision(facts)
	out := InstancePartition{Revision: revision, Unresolved: partition.Unresolved, Other: partition.Other}
	ordinals := make(map[QueueKey]int)
	// BuildGenerations orders queue keys, then source/origin and the cycles
	// inside that origin. None of these display orders proves cross-origin time.
	for _, g := range partition.Generations {
		key := QueueInstanceKey{revision, g.Key.Instance, g.Key.QueueID, ordinals[g.Key]}
		ordinals[g.Key]++
		out.Instances = append(out.Instances, QueueInstance{key, g})
	}
	return out, nil
}

// Length framing preserves arbitrary log bytes and field boundaries. JSON would
// replace invalid UTF-8, potentially aliasing distinct immutable observations.
// Sort map keys and physical refs; never include insertion IDs or arrival order.
func instanceRevision(facts []Fact) string {
	ordered := append([]Fact(nil), facts...)
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i].Ref, ordered[j].Ref
		if a.SourceID != b.SourceID {
			return a.SourceID < b.SourceID
		}
		if a.OriginID != b.OriginID {
			return a.OriginID < b.OriginID
		}
		return a.Start < b.Start
	})
	h := sha256.New()
	frame := revisionFramer{h}
	frame.text("queue-instances-v1")
	frame.number(int64(len(ordered)))
	for _, f := range ordered {
		r, o := f.Ref, f.Observation
		frame.text(r.SourceID)
		frame.text(r.OriginID)
		frame.number(r.Start)
		frame.number(r.End)
		frame.text(f.Instance)
		frame.text(o.Raw)
		frame.text(o.SourceID)
		frame.text(o.Timestamp.Raw)
		frame.text(string(o.Timestamp.Quality))
		frame.number(int64(o.Timestamp.Year))
		frame.text(o.Timestamp.Zone)
		frame.flag(o.Timestamp.Value != nil)
		if o.Timestamp.Value != nil {
			// UTC eliminates the mutable time.Location representation while
			// retaining the instant, nanoseconds and years outside UnixNano.
			frame.text(o.Timestamp.Value.UTC().Format("2006-01-02T15:04:05.999999999Z"))
		}
		frame.text(o.Host)
		frame.text(o.Program)
		frame.text(o.Service)
		frame.text(o.PID)
		frame.text(o.QueueID)
		frame.flag(o.NoQueue)
		frame.text(string(o.Kind))
		frame.text(o.Message)
		var keys []string
		for k := range o.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		frame.number(int64(len(keys)))
		for _, k := range keys {
			frame.text(k)
			frame.text(o.Fields[k])
		}
		keys = nil
		for k := range o.Present {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		frame.number(int64(len(keys)))
		for _, k := range keys {
			frame.text(k)
			frame.flag(o.Present[k])
		}
		frame.text(o.ParseError)
	}
	return hex.EncodeToString(h.Sum(nil))
}

type revisionFramer struct{ hash.Hash }

func (f revisionFramer) text(v string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(v)))
	f.Write(size[:])
	f.Write([]byte(v))
}

func (f revisionFramer) number(v int64) { f.text(strconv.FormatInt(v, 10)) }
func (f revisionFramer) flag(v bool)    { f.text(strconv.FormatBool(v)) }
