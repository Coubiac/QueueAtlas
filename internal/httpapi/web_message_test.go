package httpapi

import (
	"html"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
)

func TestWebMessageImportedOperationalFieldsAndGenerationIsolation(t *testing.T) {
	s := importedOperationalReviewStore(t)
	// Same instance/Queue ID in another origin: no sender, Message-ID or SMTP
	// client from this generation may enter the selected operational view.
	at := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	storeSearchObservations(t, s, "zz-other-origin", "synthetic-postfix", []model.Observation{
		{QueueID: "ABC123", Kind: model.KindMessage, Service: "smtpd", Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}, Fields: map[string]string{"client": "foreign.example.test[192.0.2.99]"}, Present: map[string]bool{"client": true}},
		{QueueID: "ABC123", Kind: model.KindMessage, Service: "cleanup", Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}, Fields: map[string]string{"message-id": "foreign-id@example.test"}, Present: map[string]bool{"message-id": true}},
		{QueueID: "ABC123", Kind: model.KindMessage, Service: "qmgr", Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}, Fields: map[string]string{"from": "foreign-sender@example.test"}, Present: map[string]bool{"from": true}},
	})
	guard, token, _ := searchGuardFixture(t)
	h, _ := NewConsultationHandler(guard, s, DefaultSearchOptions())
	params := url.Values{"instance": {"synthetic-postfix"}, "field": {"sender"}, "value": {"synthetic@example.test"}, "from": {"2026-10-07T00:00:00Z"}, "until": {"2026-10-08T00:00:00Z"}}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPagePath+"?"+params.Encode(), token))
	assertSearchPage(t, w, 200)
	for _, required := range []string{"synthetic@example.test", "alice@example.test", "bob@example.test", "demo-164@example.test", "Transmis", "Différé"} {
		if !strings.Contains(w.Body.String(), required) {
			t.Fatal("operational search field missing", required)
		}
	}
	link := regexp.MustCompile(`<a class="candidate" href="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(link) != 2 {
		t.Fatal("operational candidate link missing")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", html.UnescapeString(link[1]), token))
	assertSearchPage(t, w, 200)
	for _, required := range []string{"Expéditeur SMTP (enveloppe)", "Expéditeur visible — From:", "Sujet — Subject:", "Non observé — en-tête non collecté", "synthetic@example.test", "demo-164@example.test", "sender.example.test[192.0.2.10]", "remote.example.test[192.0.2.20]:25", "second.example.test[192.0.2.30]:25", "alice@example.test", "bob@example.test", "Transmis", "Différé", "250", "450", "2.0.0", "4.2.0", "Mailbox temporarily unavailable", "Durée de traitement rapportée (secondes)", "2.0", "3.0", "07/10/2026 12:00:00", "07/10/2026 12:00:03"} {
		if !strings.Contains(w.Body.String(), required) {
			t.Fatal("operational detail field missing", required)
		}
	}
	for _, forbidden := range []string{"foreign-sender@example.test", "foreign-id@example.test", "foreign.example.test[192.0.2.99]", "<script", "<img", "Ligne brute"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatal("operational detail merged generations or exposed raw", forbidden)
		}
	}
	// The shared API keeps its original selection/JSON contract, even on the
	// consultation router. Metadata requested by the Web is not serialized there.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"?"+params.Encode(), token))
	response := decodeSearchResponse(t, w)
	if len(response.Matches) != 1 {
		t.Fatal("operational view changed API matches")
	}
	for _, forbidden := range []string{"alice@example.test", "demo-164@example.test", "Mailbox temporarily unavailable", `"Web"`, `"web"`} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatal("Web selection leaked into search JSON", forbidden)
		}
	}
}

func TestWebSMTPCodeIsOnlyLiteralReplyPrefix(t *testing.T) {
	for _, tc := range []struct{ reply, code string }{
		{"250 2.0.0 Message accepted", "250"}, {"450 4.2.0 Try again", "450"}, {"550", "550"}, {"250-more lines", "250"},
		{"2.0.0", ""}, {"quoted 250 success", ""}, {"2500 not a code", ""}, {"<script>250</script>", ""}, {"999 arbitrary", ""},
	} {
		reply, _ := wireNativeField(tc.reply, true, 1024)
		d := TimelineDelivery{Scope: correlation.ScopeSMTPPeer, DSN: reply, Reply: reply}
		if code := observedSMTPCode(d); code != tc.code {
			t.Fatal("reply prefix confused with verdict/DSN", tc.reply, code)
		}
		d.Scope = correlation.ScopeLocalMailbox
		if observedSMTPCode(d) != "" {
			t.Fatal("local report manufactured SMTP code")
		}
		d.Scope, d.Reply.Present = correlation.ScopeSMTPPeer, false
		if observedSMTPCode(d) != "" {
			t.Fatal("absent reply manufactured SMTP code")
		}
	}
}
