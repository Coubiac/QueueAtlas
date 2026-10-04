package postfix

import (
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
)

func TestQuotedSenderCannotCreateObservedFields(t *testing.T) {
	for _, sender := range []string{
		`"x> to=<victim@example.org>"@example.org`,
		`"x\" > to=<victim@example.org>; status=sent; uid=9"@example.org`,
	} {
		line := `Oct  3 12:34:56 mx postfix/pickup[1]: ABC123: uid=1001 from=<` + sender + `>`
		o := Parse([]byte(line), Options{})
		if o.Raw != line || o.Kind != model.KindMessage || o.QueueID != "ABC123" || o.Fields["uid"] != "1001" || o.Fields["from"] != sender || len(o.Fields) != 2 {
			t.Fatalf("quoted sender split or lost: %+v", o)
		}
		for _, key := range []string{"to", "status", "reply"} {
			if _, present := o.Field(key); present {
				t.Fatalf("sender forged %s: %+v", key, o)
			}
		}
	}
}

func TestSpaceFieldsKeepEmptySenderAndSubmissionMetadata(t *testing.T) {
	for _, tc := range []struct {
		service, message string
		fields           map[string]string
	}{
		{"pickup", "uid=1001 from=<>", map[string]string{"uid": "1001", "from": ""}},
		{"smtpd", `client=sender.example.org[192.0.2.2], sasl_method=PLAIN, sasl_username="x; to=<victim@example.org>"`, map[string]string{"client": "sender.example.org[192.0.2.2]", "sasl_method": "PLAIN", "sasl_username": `"x; to=<victim@example.org>"`}},
	} {
		line := "Oct  3 12:34:56 mx postfix/" + tc.service + "[1]: ABC123: " + tc.message
		o := Parse([]byte(line), Options{})
		if len(o.Fields) != len(tc.fields) {
			t.Fatalf("unexpected fields: %+v", o)
		}
		for key, want := range tc.fields {
			if got, present := o.Field(key); !present || got != want {
				t.Fatalf("%s=%q present=%v, want %q", key, got, present, want)
			}
		}
	}
}

func TestUnterminatedSenderDoesNotExposeInnerFields(t *testing.T) {
	for _, value := range []string{`<"x> to=<victim@example.org>`, `<x to=victim@example.org`} {
		line := "Oct  3 12:34:56 mx postfix/pickup[1]: ABC123: uid=1001 from=" + value
		o := Parse([]byte(line), Options{})
		if o.Raw != line || len(o.Fields) != 1 || o.Fields["uid"] != "1001" {
			t.Fatalf("incomplete sender asserted: %+v", o)
		}
	}
}
