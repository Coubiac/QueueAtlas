// Package correlation derives revisable projections from immutable parsed facts.
package correlation

import (
	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/parser/postfix"
)

type DeliveryStatus string

const (
	DeliveryUnknown   DeliveryStatus = "unknown"
	DeliverySent      DeliveryStatus = "sent"
	DeliveryDelivered DeliveryStatus = "delivered"
	DeliveryDeferred  DeliveryStatus = "deferred"
	DeliveryBounced   DeliveryStatus = "bounced"
)

// DeliveryScope identifies the reporting transport, not a claim about later
// mailbox processing, recipient reads or the final result of a whole message.
type DeliveryScope string

const (
	ScopeSMTPPeer       DeliveryScope = "smtp_peer"
	ScopeLMTPPeer       DeliveryScope = "lmtp_peer"
	ScopePipeCommand    DeliveryScope = "pipe_command"
	ScopeLocalAgent     DeliveryScope = "local_agent"
	ScopeVirtualAgent   DeliveryScope = "virtual_agent"
	ScopeLocalMailbox   DeliveryScope = "local_mailbox"
	ScopeVirtualMailbox DeliveryScope = "virtual_mailbox"
)

// Field preserves absence separately from an explicitly logged empty value.
type Field struct {
	Value   string
	Present bool
}

// Delivery describes exactly one observation. Its caller retains the immutable
// event/provenance reference. It contains no generation, chronology or links;
// SourceID, declared Host, QueueID and Message-ID are not merged here.
type Delivery struct {
	Recipient         string
	OriginalRecipient Field
	NativeStatus      string
	Status            DeliveryStatus
	Scope             DeliveryScope
	Relay             Field
	DSN               Field
	Reply             Field
}

// DeliveryFrom accepts a recognised Postfix delivery observation only. It does
// not parse untrusted reply text for remote verdicts or queue identifiers. The
// input must come from the bounded parser (or the equivalent durable facts).
// Returned strings are immutable values, with no borrowed maps or time pointers.
func DeliveryFrom(o model.Observation) (Delivery, bool) {
	if o.Kind != model.KindDelivery || o.NoQueue || o.QueueID == "" || o.ParseError != "" {
		return Delivery{}, false
	}
	var scope DeliveryScope
	switch o.Service {
	case "smtp":
		scope = ScopeSMTPPeer
	case "lmtp":
		scope = ScopeLMTPPeer
	case "pipe":
		scope = ScopePipeCommand
	case "local":
		scope = ScopeLocalAgent
	case "virtual":
		scope = ScopeVirtualAgent
	default:
		return Delivery{}, false
	}
	to, hasTo := o.Field("to")
	native, hasStatus := o.Field("status")
	if !hasTo || !hasStatus {
		return Delivery{}, false
	}
	d := Delivery{
		Recipient: to, NativeStatus: native, Status: DeliveryUnknown, Scope: scope,
		OriginalRecipient: observedField(o, "orig_to"), Relay: observedField(o, "relay"),
		DSN: observedField(o, "dsn"), Reply: observedField(o, "reply"),
	}
	switch native {
	case "sent":
		d.Status = DeliverySent
		// local can forward or invoke a command; its service name alone is
		// insufficient. Limit mailbox reports to exact known local messages.
		// Check the first native status field too: old persisted parsers could
		// trim angle brackets from reply, but retained Message unchanged. Neither
		// a normalised field nor an unrelated suffix proves mailbox delivery.
		if d.Reply.Present && (d.Reply.Value == "delivered to maildir" || d.Reply.Value == "delivered to mailbox") &&
			postfix.HasExactStatusReply(o.Message, native, d.Reply.Value) {
			switch scope {
			case ScopeLocalAgent:
				d.Status, d.Scope = DeliveryDelivered, ScopeLocalMailbox
			case ScopeVirtualAgent:
				d.Status, d.Scope = DeliveryDelivered, ScopeVirtualMailbox
			}
		}
	case "deferred":
		d.Status = DeliveryDeferred
	case "bounced":
		d.Status = DeliveryBounced
	}
	return d, true
}

func observedField(o model.Observation, name string) Field {
	value, present := o.Field(name)
	if !present {
		value = ""
	}
	return Field{Value: value, Present: present}
}
