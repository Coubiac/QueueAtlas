package postfix

import (
	"strings"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
)

func TestParsePostfixServices(t *testing.T) {
	tests := []struct {
		name, service, body, queue string
		kind                       model.Kind
		fields                     map[string]string
		noQueue                    bool
	}{
		{"smtpd connect", "smtpd", "connect from client.example.org[192.0.2.1]", "", model.KindConnect, nil, false},
		{"smtpd reject", "smtpd", "NOQUEUE: reject: RCPT from client.example.org[192.0.2.1]: 550 denied; from=<a@example.org> to=<b@example.org> proto=ESMTP helo=<client.example.org>", "", model.KindReject, map[string]string{"from": "a@example.org", "to": "b@example.org", "proto": "ESMTP"}, true},
		{"pickup", "pickup", "AB12CD: uid=1001 from=<a@example.org>", "AB12CD", model.KindMessage, map[string]string{"uid": "1001", "from": "a@example.org"}, false},
		{"cleanup", "cleanup", "AB12CD: message-id=<id@example.org>", "AB12CD", model.KindMessage, map[string]string{"message-id": "id@example.org"}, false},
		{"qmgr", "qmgr", "AB12CD: from=<a@example.org>, size=123, nrcpt=2 (queue active)", "AB12CD", model.KindMessage, map[string]string{"from": "a@example.org", "size": "123"}, false},
		{"qmgr removed", "qmgr", "AB12CD: removed", "AB12CD", model.KindRemoved, nil, false},
		{"smtp", "smtp", "AB12CD: to=<b@example.org>, orig_to=<alias@example.org>, relay=mx.example.org[192.0.2.2]:25, delay=1.2, delays=0.1/0.2/0.3/0.6, dsn=2.0.0, status=sent (250 2.0.0 OK, queued as DEF456; x=y (nested, detail))", "AB12CD", model.KindDelivery, map[string]string{"to": "b@example.org", "orig_to": "alias@example.org", "relay": "mx.example.org[192.0.2.2]:25", "delay": "1.2", "delays": "0.1/0.2/0.3/0.6", "dsn": "2.0.0", "status": "sent", "reply": "250 2.0.0 OK, queued as DEF456; x=y (nested, detail)"}, false},
		{"lmtp", "lmtp", "AB12CD: to=<b@example.org>, relay=unix:private/dovecot-lmtp, status=sent (250 saved)", "AB12CD", model.KindDelivery, map[string]string{"status": "sent"}, false},
		{"local", "local", "AB12CD: to=<b@example.org>, relay=local, status=sent (delivered to mailbox)", "AB12CD", model.KindDelivery, map[string]string{"reply": "delivered to mailbox"}, false},
		{"pipe", "pipe", "AB12CD: to=<b@example.org>, relay=filter, status=deferred (temporary)", "AB12CD", model.KindDelivery, map[string]string{"status": "deferred"}, false},
		{"virtual", "virtual", "AB12CD: to=<b@example.org>, relay=virtual, status=sent (delivered to maildir)", "AB12CD", model.KindDelivery, map[string]string{"to": "b@example.org"}, false},
		{"bounce", "bounce", "AB12CD: sender non-delivery notification: EF34GH", "AB12CD", model.KindBounce, map[string]string{"notification_queue_id": "EF34GH"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := "Oct  3 12:34:56 mx postfix/" + tt.service + "[123]: " + tt.body
			o := Parse([]byte(line), Options{SourceID: "mail-log-1"})
			if o.ParseError != "" || o.Raw != line || o.SourceID != "mail-log-1" || o.Host != "mx" || o.Service != tt.service || o.QueueID != tt.queue || o.Kind != tt.kind || o.NoQueue != tt.noQueue {
				t.Fatalf("unexpected observation: %+v", o)
			}
			for key, want := range tt.fields {
				if got, ok := o.Field(key); !ok || got != want {
					t.Errorf("%s = %q (present %v), want %q", key, got, ok, want)
				}
			}
		})
	}
}

func TestUnknownAndMalformed(t *testing.T) {
	tests := []string{
		"not syslog",
		"Oct  3 12:34:56 mx dovecot[9]: message",
		"Oct  3 12:34:56 mx postfix/smtp[9]: warning: malformed",
		"Oct  3 12:34:56 mx postfix/smtp[9]: XYZ!: to=<x@example.org>, status=sent (wrong queue ID)",
	}
	for _, line := range tests {
		o := Parse([]byte(line), Options{})
		if o.Kind != model.KindUnknown || o.Raw != line || o.QueueID != "" {
			t.Fatalf("%q: %+v", line, o)
		}
	}
	line := []byte(strings.Repeat("x", model.MaxLineBytes+1))
	o := Parse(line, Options{})
	if o.Kind != model.KindUnknown || o.ParseError != "line_too_long" || len(o.Raw) != model.MaxLineBytes {
		t.Fatalf("oversized line: %+v", o)
	}
}

func TestResponseCannotCreateFalseFields(t *testing.T) {
	line := "Oct  3 12:34:56 mx postfix/smtp[1]: ABC123: to=<a@example.org>, relay=mx[192.0.2.1]:25, dsn=4.0.0, status=deferred (451 Temporary issue, to=<evil@example.org>, status=sent, relay=other; x=y)"
	o := Parse([]byte(line), Options{})
	if o.Kind != model.KindDelivery || o.Fields["to"] != "a@example.org" || o.Fields["status"] != "deferred" || o.Fields["relay"] != "mx[192.0.2.1]:25" || o.Fields["reply"] != "451 Temporary issue, to=<evil@example.org>, status=sent, relay=other; x=y" {
		t.Fatalf("response split into fields: %+v", o)
	}
}

func TestForgedDeliveryFieldsCannotOverrideObservedFields(t *testing.T) {
	line := "Oct  3 12:34:56 mx postfix/smtp[1]: ABC123: to=<real@example.org>, relay=mx[192.0.2.1]:25, dsn=4.0.0, status=deferred (451 remote text ), to=<forged@example.org>, status=sent (250 forged)"
	o := Parse([]byte(line), Options{})
	if o.Kind != model.KindDelivery || o.Fields["to"] != "real@example.org" || o.Fields["status"] != "deferred" {
		t.Fatalf("remote reply changed observed fields: %+v", o)
	}
}

func TestAmbiguousRejectMetadataIsNotAsserted(t *testing.T) {
	line := "Oct  3 12:34:56 mx postfix/smtpd[1]: NOQUEUE: reject: RCPT from bad[192.0.2.2]: 550 denied; from=<real@example.org> to=<recipient@example.org> proto=ESMTP helo=<bad; from=<forged@example.org> to=<victim@example.net> proto=ESMTP>"
	o := Parse([]byte(line), Options{})
	if o.Kind != model.KindReject || !o.NoQueue {
		t.Fatalf("reject lost: %+v", o)
	}
	if _, ok := o.Field("from"); ok {
		t.Fatalf("ambiguous metadata was asserted: %+v", o)
	}
}

func TestAllLetterHexQueueIDAndSubmissionService(t *testing.T) {
	line := "Oct  3 12:34:56 mx postfix/submission/smtpd[1]: ABCDEF: client=sender.example.org[192.0.2.2]"
	o := Parse([]byte(line), Options{})
	if o.QueueID != "ABCDEF" || o.Service != "smtpd" || o.Program != "postfix/submission/smtpd" {
		t.Fatalf("valid queue or service not recognised: %+v", o)
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte("Oct  3 12:34:56 mx postfix/smtp[1]: ABC123: to=<a@example.org>, status=sent (250 OK, x=y)"))
	f.Add([]byte("Oct  3 12:34:56 mx postfix/smtpd[1]: NOQUEUE: reject: RCPT from x; from=<a@example.org>"))
	f.Add([]byte{0, 255, '\n'})
	f.Fuzz(func(t *testing.T, input []byte) {
		o := Parse(input, Options{})
		if len(o.Raw) > model.MaxLineBytes || len(o.Fields) > 32 {
			t.Fatalf("unbounded parse result: raw=%d fields=%d", len(o.Raw), len(o.Fields))
		}
	})
}
