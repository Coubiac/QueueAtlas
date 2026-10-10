package httpapi

import (
	"context"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

// HTML-only view of selected fields in one already reconstructed generation.
// It does not extend the JSON API, infer message headers, inspect raw logs or
// choose a winner among simultaneous recipient results.
type webMessage struct {
	FirstAt, LastAt                    time.Time
	Senders, MessageIDs, SourceServers []NativeValue
	Recipients                         []webMessageRecipient
}

type webMessageRecipient struct {
	Address   NativeValue
	Status    correlation.DeliveryStatus
	Uncertain bool
	Latest    []webMessageAttempt
}

type webMessageAttempt struct {
	At       time.Time
	Delivery TimelineDelivery
	Delay    NativeField
	SMTPCode string
}

func buildWebMessage(ctx context.Context, queue correlation.ProjectedQueue, facts map[correlation.FactRef]correlation.Fact) (*webMessage, error) {
	out := &webMessage{}
	selected := make(map[correlation.FactRef]correlation.Fact)
	seen := make(map[string]map[string]bool)
	for _, ref := range queue.Summary.Queue.Generation.Facts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fact, ok := facts[ref]
		o := fact.Observation
		if !ok || fact.Instance != queue.Key.Instance || o.QueueID != queue.Key.QueueID || o.NoQueue {
			return nil, sqlite.ErrSearchStoredHit
		}
		selected[ref] = fact
		if o.Timestamp.Value == nil || !searchInstantFits(*o.Timestamp.Value) {
			return nil, sqlite.ErrSearchStoredHit
		}
		at := o.Timestamp.Value.UTC()
		if len(selected) == 1 || at.Before(out.FirstAt) {
			out.FirstAt = at
		}
		if len(selected) == 1 || at.After(out.LastAt) {
			out.LastAt = at
		}
		if o.ParseError != "" || o.Kind != model.KindMessage {
			continue
		}
		var field string
		var dest *[]NativeValue
		switch o.Service {
		case "qmgr":
			field, dest = "from", &out.Senders
		case "cleanup":
			field, dest = "message-id", &out.MessageIDs
		case "smtpd":
			field, dest = "client", &out.SourceServers
		default:
			continue
		}
		value, present := o.Field(field)
		if !present {
			continue
		}
		if seen[field] == nil {
			seen[field] = make(map[string]bool)
		}
		if seen[field][value] {
			continue
		}
		native, err := wireNative(value, 1024)
		if err != nil {
			return nil, err
		}
		seen[field][value] = true
		*dest = append(*dest, native)
	}
	for _, recipient := range queue.Summary.Queue.Recipients {
		address, err := wireNative(recipient.Address, 1024)
		if err != nil {
			return nil, err
		}
		item := webMessageRecipient{Address: address, Status: recipient.ObservedStatus, Uncertain: recipient.OrderUncertain}
		for _, ref := range recipient.Latest {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			fact, ok := selected[ref]
			if !ok {
				return nil, sqlite.ErrSearchStoredHit
			}
			event, err := timelineEvent(fact, false)
			if err != nil || event.Delivery == nil {
				return nil, sqlite.ErrSearchStoredHit
			}
			delay, present := fact.Observation.Field("delay")
			delayField, err := wireNativeField(delay, present, 1024)
			if err != nil {
				return nil, err
			}
			item.Latest = append(item.Latest, webMessageAttempt{At: event.At, Delivery: *event.Delivery, Delay: delayField, SMTPCode: observedSMTPCode(*event.Delivery)})
		}
		out.Recipients = append(out.Recipients, item)
	}
	return out, nil
}

// Display only a literal code at the beginning of a SMTP/LMTP reply. A DSN
// such as 2.0.0 is not a 250, and an embedded quote is not a reply prefix.
// This value never changes the recipient projection or implies mailbox delivery.
func observedSMTPCode(d TimelineDelivery) string {
	if (d.Scope != correlation.ScopeSMTPPeer && d.Scope != correlation.ScopeLMTPPeer) || !d.Reply.Present || d.Reply.Value == nil || d.Reply.Value.Encoding != "utf8" {
		return ""
	}
	v := d.Reply.Value.Value
	if len(v) < 3 || v[0] < '2' || v[0] > '5' || v[1] < '0' || v[1] > '5' || v[2] < '0' || v[2] > '9' || len(v) > 3 && v[3] != ' ' && v[3] != '-' {
		return ""
	}
	return v[:3]
}

func webDeliveryLabel(status correlation.DeliveryStatus) string {
	switch status {
	case correlation.DeliverySent:
		return "Transmis"
	case correlation.DeliveryDelivered:
		return "Remise locale rapportée"
	case correlation.DeliveryDeferred:
		return "Différé"
	case correlation.DeliveryBounced:
		return "Échec rapporté"
	default:
		return "Non déterminé"
	}
}
