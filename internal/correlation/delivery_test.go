package correlation

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/parser/postfix"
)

func TestDeliveryTransportScopeAndHostileReply(t *testing.T) {
	for _, tc := range []struct {
		service, native, reply string
		status                 DeliveryStatus
		scope                  DeliveryScope
	}{
		{"smtp", "sent", "delivered to maildir", DeliverySent, ScopeSMTPPeer},
		{"smtp", "sent", "250 2.0.0 accepted; queued as REMOTE01", DeliverySent, ScopeSMTPPeer},
		{"lmtp", "sent", "250 2.0.0 Saved", DeliverySent, ScopeLMTPPeer},
		{"lmtp", "sent", "delivered to mailbox", DeliverySent, ScopeLMTPPeer},
		{"pipe", "sent", "delivered to maildir", DeliverySent, ScopePipeCommand},
		{"local", "sent", "delivered to maildir", DeliveryDelivered, ScopeLocalMailbox},
		{"local", "sent", "delivered to mailbox", DeliveryDelivered, ScopeLocalMailbox},
		{"local", "sent", "forwarded as ABC123", DeliverySent, ScopeLocalAgent},
		{"local", "sent", "delivered to command: archive", DeliverySent, ScopeLocalAgent},
		{"local", "sent", "250 <script>delivered to maildir</script>", DeliverySent, ScopeLocalAgent},
		{"local", "sent", "<delivered to maildir>", DeliverySent, ScopeLocalAgent},
		{"virtual", "sent", "<delivered to mailbox>", DeliverySent, ScopeVirtualAgent},
		{"virtual", "sent", "delivered to maildir", DeliveryDelivered, ScopeVirtualMailbox},
		{"virtual", "sent", "opaque success", DeliverySent, ScopeVirtualAgent},
		{"local", "deferred", "delivered to maildir", DeliveryDeferred, ScopeLocalAgent},
		{"smtp", "bounced", "550 user unknown (in reply to RCPT TO command)", DeliveryBounced, ScopeSMTPPeer},
		{"smtp", "SENT", "250 accepted", DeliveryUnknown, ScopeSMTPPeer},
		{"smtp", "expired", "returned to sender", DeliveryUnknown, ScopeSMTPPeer},
	} {
		t.Run(tc.service+"/"+tc.native+"/"+tc.reply, func(t *testing.T) {
			line := "Oct  3 12:00:01 mx-declared postfix/" + tc.service + "[1]: ABC123: to=<bob@example.org>, orig_to=<alias@example.org>, relay=remote.invalid, dsn=2.0.0, status=" + tc.native + " (" + tc.reply + ")\n"
			o := postfix.Parse([]byte(line), postfix.Options{SourceID: "source-a"})
			d, ok := DeliveryFrom(o)
			if !ok || d.Status != tc.status || d.Scope != tc.scope || d.NativeStatus != tc.native || d.Recipient != "bob@example.org" || d.OriginalRecipient != (Field{"alias@example.org", true}) || d.Reply != (Field{tc.reply, true}) || d.DSN != (Field{"2.0.0", true}) || d.Relay != (Field{"remote.invalid", true}) {
				t.Fatalf("projection: %#v, recognised=%v", d, ok)
			}
			before := d
			o.Fields["to"], o.Fields["reply"] = "mutated", "mutated"
			o.Present["dsn"] = false
			if d != before {
				t.Fatal("projection aliases parser maps")
			}
		})
	}
}

func TestDeliveryRequiresObservedFieldsAndDeliveryKind(t *testing.T) {
	base := model.Observation{QueueID: "ABC123", Service: "smtp", Kind: model.KindDelivery,
		Fields:  map[string]string{"to": "", "status": "sent", "reply": "ignored absent", "orig_to": "ignored absent"},
		Present: map[string]bool{"to": true, "status": true, "dsn": true}}
	d, ok := DeliveryFrom(base)
	if !ok || d.Recipient != "" || d.DSN != (Field{"", true}) || d.Reply != (Field{}) || d.OriginalRecipient != (Field{}) || d.Status != DeliverySent {
		t.Fatalf("presence/empty: %#v, %v", d, ok)
	}
	for _, change := range []func(*model.Observation){
		func(o *model.Observation) { o.NoQueue = true },
		func(o *model.Observation) { o.QueueID = "" },
		func(o *model.Observation) { o.ParseError = "malformed" },
		func(o *model.Observation) { o.Service = "qmgr" },
		func(o *model.Observation) { o.Kind = model.KindUnknown },
		func(o *model.Observation) { o.Kind = model.KindRemoved },
		func(o *model.Observation) { o.Kind = model.KindBounce },
		func(o *model.Observation) { o.Kind = model.KindReject },
		func(o *model.Observation) { o.Present = map[string]bool{"status": true} },
		func(o *model.Observation) { o.Present = map[string]bool{"to": true} },
	} {
		o := base
		change(&o)
		if got, ok := DeliveryFrom(o); ok || got != (Delivery{}) {
			t.Fatalf("non-delivery accepted: %#v, %v", got, ok)
		}
	}
}

func TestDeliveryLegacyNormalisedReplyDoesNotProveMailbox(t *testing.T) {
	for _, service := range []string{"local", "virtual"} {
		for _, reply := range []string{"delivered to maildir", "delivered to mailbox"} {
			line := "Oct  3 12:00:01 mx postfix/" + service + "[1]: ABC123: to=<bob@example.org>, status=sent (<" + reply + ">)"
			o := postfix.Parse([]byte(line), postfix.Options{})
			// Reproduce a pre-lot89 durable field; original Message retained.
			o.Fields["reply"] = reply
			d, ok := DeliveryFrom(o)
			if !ok || d.Status != DeliverySent || d.Scope == ScopeLocalMailbox || d.Scope == ScopeVirtualMailbox {
				t.Fatalf("legacy reply promoted: %#v, %v", d, ok)
			}
			o.Message += ", xstatus=sent (" + reply + ")"
			if d, ok := DeliveryFrom(o); !ok || d.Status != DeliverySent {
				t.Fatalf("ignored status suffix promoted: %#v, %v", d, ok)
			}
			o.Message = ""
			if d, ok := DeliveryFrom(o); !ok || d.Status != DeliverySent {
				t.Fatalf("missing native text promoted: %#v, %v", d, ok)
			}
		}
	}
}

func TestDeliveryCorpusPreservesEveryAttempt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []DeliveryStatus
	}{
		{"01-inbound-local", []DeliveryStatus{DeliveryDelivered}},
		{"02-outbound-auth", []DeliveryStatus{DeliverySent}},
		{"04-multi-recipient-mixed", []DeliveryStatus{DeliveryDelivered, DeliveryDeferred, DeliveryBounced}},
		{"07-deferred-then-sent", []DeliveryStatus{DeliveryDeferred, DeliverySent}},
		{"08-multiple-retries", []DeliveryStatus{DeliveryDeferred, DeliveryDeferred, DeliverySent}},
		{"09-bounce", []DeliveryStatus{DeliveryBounced, DeliveryDelivered}},
		{"10-expired", []DeliveryStatus{DeliveryDeferred}},
		{"18-lmtp", []DeliveryStatus{DeliverySent}},
		{"19-virtual", []DeliveryStatus{DeliveryDelivered}},
		{"20-pipe", []DeliveryStatus{DeliverySent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "postfix", tc.name+".log"))
			if err != nil {
				t.Fatal(err)
			}
			var got []DeliveryStatus
			for _, line := range bytes.SplitAfter(raw, []byte("\n")) {
				o := postfix.Parse(line, postfix.Options{})
				if d, ok := DeliveryFrom(o); ok {
					got = append(got, d.Status)
				}
			}
			if !reflect.DeepEqual(got, tc.statuses) {
				t.Fatalf("attempts: got %v want %v", got, tc.statuses)
			}
		})
	}
}
