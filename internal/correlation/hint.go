package correlation

import (
	"strings"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/parser/postfix"
)

type QueueHintKind string

const (
	HintSMTPQueue          QueueHintKind = "smtp_queue_hint"
	HintLocalForward       QueueHintKind = "local_forward_hint"
	HintBounceNotification QueueHintKind = "bounce_notification_hint"
)

// QueueHint cites one explicit native report. It is not a graph edge, target
// generation or certificate that either queue exists. An SMTP peer's identifier
// has no trusted instance, including a peer whose logged relay is loopback.
type QueueHint struct {
	From           QueueKey
	Evidence       FactRef
	Kind           QueueHintKind
	TargetQueueID  string
	TargetInstance Field
	Relay          Field
	NativeProof    string
}

type QueueHints struct {
	Index Partition // all facts retained, even undated/unsupported hints
	Hints []QueueHint
}

// BuildQueueHints extracts bounded native hints without matching Queue IDs,
// Message-IDs or addresses to another generation. No delivery state is changed.
func BuildQueueHints(facts []Fact, limit int) (QueueHints, error) {
	index, err := PartitionFacts(facts, limit)
	if err != nil {
		return QueueHints{}, err
	}
	lookup := make(map[FactRef]Fact, len(facts))
	refs := make([]FactRef, 0, len(facts))
	for _, f := range facts {
		lookup[f.Ref] = f
		refs = append(refs, f.Ref)
	}
	sortRefs(refs)
	out := QueueHints{Index: index}
	for _, ref := range refs {
		if hint, ok := queueHintFrom(lookup[ref]); ok {
			out.Hints = append(out.Hints, hint)
		}
	}
	return out, nil
}

func queueHintFrom(f Fact) (QueueHint, bool) {
	o := f.Observation
	if !queuedFact(o) || len(o.Message) > model.MaxLineBytes {
		return QueueHint{}, false
	}
	hint := QueueHint{From: QueueKey{f.Instance, o.QueueID}, Evidence: f.Ref}
	if o.Kind == model.KindBounce && o.Service == "bounce" {
		id, present := o.Field("notification_queue_id")
		if !present || !postfix.IsQueueID(id) || o.Message != "sender non-delivery notification: "+id {
			return QueueHint{}, false
		}
		hint.Kind, hint.TargetQueueID = HintBounceNotification, id
		hint.TargetInstance = Field{f.Instance, true}
		hint.NativeProof = o.Message
		return hint, true
	}
	d, ok := DeliveryFrom(o)
	if !ok || d.Status != DeliverySent || !d.Reply.Present || !postfix.HasExactStatusReply(o.Message, d.NativeStatus, d.Reply.Value) {
		return QueueHint{}, false
	}
	var prefix string
	switch d.Scope {
	case ScopeLocalAgent:
		hint.Kind = HintLocalForward
		prefix = "forwarded as "
		hint.TargetInstance = Field{f.Instance, true}
	case ScopeSMTPPeer:
		hint.Kind = HintSMTPQueue
		// Deliberately limited native reply format. Other filter/peer formats
		// remain raw facts, never a permissive search inside hostile reply text.
		prefix = "250 2.0.0 Ok: queued as "
	default:
		return QueueHint{}, false
	}
	if !strings.HasPrefix(d.Reply.Value, prefix) {
		return QueueHint{}, false
	}
	id := strings.TrimPrefix(d.Reply.Value, prefix)
	if !postfix.IsQueueID(id) {
		return QueueHint{}, false
	}
	hint.TargetQueueID, hint.Relay, hint.NativeProof = id, d.Relay, d.Reply.Value
	return hint, true
}
