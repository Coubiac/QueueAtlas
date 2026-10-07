package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/auth"
	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func candidatePageFacts(instance, queue, sourceID, origin string, addresses []string, attempts int) []correlation.Fact {
	at := time.Unix(0, goldenSearchFromNS).UTC()
	observations := []model.Observation{{Kind: model.KindMessage, Service: "cleanup", Fields: map[string]string{"message-id": "synthetic@example.test"}, Present: map[string]bool{"message-id": true}}}
	for _, address := range addresses {
		for i := 0; i < attempts; i++ {
			status := "sent"
			if i%2 != 0 {
				status = "bounced"
			}
			observations = append(observations, model.Observation{Kind: model.KindDelivery, Service: "smtp", Message: "to=<synthetic>, status=" + status + " (synthetic private reply)", Fields: map[string]string{"to": address, "status": status}, Present: map[string]bool{"to": true, "status": true}})
		}
	}
	facts := make([]correlation.Fact, 0, len(observations))
	for i, o := range observations {
		o.SourceID, o.QueueID = sourceID, queue
		o.Timestamp = model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}
		start := int64(1<<53+1) + int64(i*2)
		facts = append(facts, correlation.Fact{Ref: correlation.FactRef{SourceID: sourceID, OriginID: origin, Start: start, End: start + 1}, Instance: instance, Observation: o})
	}
	return facts
}

func candidatePageID(t *testing.T, facts []correlation.Fact) string {
	t.Helper()
	projection, err := buildQueueProjection(context.Background(), facts, 1024)
	if err != nil || len(projection.Queues) != 1 {
		t.Fatal("synthetic page candidate", err)
	}
	id, err := EncodeCandidateID(projection.Queues[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCandidatePageSQLiteSearchLinkFullGenerationAndStale(t *testing.T) {
	s := syntheticSearchStore(t)
	storeSearchCycle(t, s, "synthetic-first", "synthetic-postfix", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-second", "synthetic-postfix", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-undated", "synthetic-postfix", "ABC123", false)
	storeSearchCycle(t, s, "synthetic-foreign", "other-instance", "ABC123", true)
	guard, token, _ := searchGuardFixture(t)
	h, err := NewConsultationHandler(guard, s, DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", strings.Replace(searchTestURL(), SearchPath, SearchPagePath, 1), token))
	assertSearchPage(t, w, 200)
	link := regexp.MustCompile(`<a class="candidate" href="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(link) != 2 {
		t.Fatal("search did not link its assigned candidate")
	}
	path := html.UnescapeString(link[1])
	if !strings.HasPrefix(path, CandidatePagePrefix) {
		t.Fatal("detail link escaped its local path")
	}
	key, err := decodeCandidateID(strings.TrimPrefix(path, CandidatePagePrefix))
	if err != nil || key.Instance != "synthetic-postfix" || key.QueueID != "ABC123" {
		t.Fatal("detail link changed candidate scope", err)
	}
	for _, method := range []string{"GET", "HEAD"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest(method, path, token))
		assertSearchPage(t, w, 200)
		if method == "HEAD" {
			if w.Body.Len() != 0 {
				t.Fatal("detail HEAD exposed data")
			}
			continue
		}
		for _, required := range []string{"Détail du candidat", "ABC123", "recipient@example.test", "sent (transport) : 1", "delivered (remise reconnue) : 0", "Continuité entre origines incertaine", "Couverture non prouvée", "synthetic-first", "synthetic-first-origin", "Retrait de file"} {
			if !strings.Contains(w.Body.String(), required) {
				t.Fatal("detail lost complete-generation evidence", required)
			}
		}
		for _, private := range []string{"synthetic-second", "synthetic-foreign", "synthetic-undated", "private reply", "private raw", "private.log", "synthetic@example.test"} {
			if strings.Contains(w.Body.String(), private) {
				t.Fatal("detail merged origins or exposed unselected data", private)
			}
		}
	}
	key.Generation = 1
	second, _ := EncodeCandidateID(key)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", CandidatePagePrefix+second, token))
	assertSearchPage(t, w, 200)
	if !strings.Contains(w.Body.String(), "synthetic-second-origin") || strings.Contains(w.Body.String(), "synthetic-first") {
		t.Fatal("detail selected or merged the wrong generation")
	}
	key.Generation = 2
	missing, _ := EncodeCandidateID(key)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", CandidatePagePrefix+missing, token))
	assertSearchPage(t, w, 404)
	if strings.Contains(w.Body.String(), "ABC123") || !strings.Contains(w.Body.String(), "ne prouve pas") {
		t.Fatal("missing candidate disclosed data or claimed log absence")
	}
	options := DefaultSearchOptions()
	options.FactLimit = 2
	narrow, _ := NewConsultationHandler(guard, s, options)
	w = httptest.NewRecorder()
	narrow.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchPage(t, w, 422)
	if strings.Contains(w.Body.String(), "ABC123") || !strings.Contains(w.Body.String(), "partiellement") {
		t.Fatal("partial candidate page")
	}
	storeSearchCycle(t, s, "synthetic-late", "synthetic-postfix", "ABC123", true)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchPage(t, w, 409)
	if strings.Contains(w.Body.String(), "ABC123") || !strings.Contains(w.Body.String(), "faits ont changé") {
		t.Fatal("stale page silently replaced or disclosed candidate")
	}
}

func TestCandidatePageProtocolBeforeReadAndHTTPSNativeText(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	var calls atomic.Int32
	reader := &stubSearchReader{facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		calls.Add(1)
		return nil, nil
	}}
	h, err := newReadHandler(guard, reader, DefaultSearchOptions(), func() time.Time { panic("detail must not read search clock") }, true)
	if err != nil {
		t.Fatal(err)
	}
	path := CandidatePagePrefix + goldenCandidateID
	for _, tc := range []struct {
		change func(*http.Request)
		status int
	}{
		{func(r *http.Request) { r.Header.Del("Cookie") }, 401}, {func(r *http.Request) { r.TLS = nil }, 403},
		{func(r *http.Request) { r.Header.Set("Origin", "https://foreign.example.test") }, 403},
		{func(r *http.Request) { r.URL.Path = CandidatePagePrefix + "invalid" }, 400},
		{func(r *http.Request) { r.URL.RawQuery = "raw=1&password=synthetic-secret" }, 400}, {func(r *http.Request) { r.URL.ForceQuery = true }, 400},
		{func(r *http.Request) { r.URL.Path += "/events" }, 404}, {func(r *http.Request) { r.URL.Path = CandidatePagePrefix }, 404},
		{func(r *http.Request) { r.URL.RawPath = "/%6dessages/" + goldenCandidateID }, 400},
		{func(r *http.Request) { r.Method = "POST"; r.Header.Set("Origin", searchTestOrigin) }, 405},
		{func(r *http.Request) { r.ContentLength = 1; r.Body = panicSearchBody{} }, 400},
	} {
		w := httptest.NewRecorder()
		r := searchReadRequest("GET", path, token)
		tc.change(r)
		h.ServeHTTP(w, r)
		assertSearchHTTP(t, w, tc.status)
		if calls.Load() != 0 || strings.Contains(w.Body.String(), "synthetic-secret") {
			t.Fatal("invalid detail read facts or reflected query")
		}
	}
	apiOnly, _ := newSearchHandler(guard, reader, DefaultSearchOptions(), time.Now)
	w := httptest.NewRecorder()
	apiOnly.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchHTTP(t, w, 404)
	if calls.Load() != 0 {
		t.Fatal("API-only router acquired Web route")
	}
	server := httptest.NewUnstartedServer(nil)
	defer server.Close()
	origin := "https://" + server.Listener.Addr().String()
	guard, token, sessions := searchGuardAtOrigin(t, origin)
	payload := `javascript:"><script>alert(1)</script>`
	queue := `Q"><img onerror=x>`
	facts := candidatePageFacts(payload, queue, payload, payload, []string{payload, "", "\xffsynthetic", "Case@example.test", "case@example.test"}, 2)
	id := candidatePageID(t, facts)
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		calls.Add(1)
		return facts, nil
	}
	h, err = newReadHandler(guard, reader, DefaultSearchOptions(), time.Now, true)
	if err != nil {
		t.Fatal(err)
	}
	server.Config.Handler = h
	server.StartTLS()
	client := server.Client()
	for _, method := range []string{"GET", "HEAD"} {
		r, _ := http.NewRequest(method, origin+CandidatePagePrefix+id, nil)
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("X-Frame-Options") != "DENY" || len(response.Cookies()) != 0 {
			t.Fatal("HTTPS detail policy")
		}
		if method == "HEAD" {
			if len(body) != 0 {
				t.Fatal("HTTPS HEAD body")
			}
			continue
		}
		text := string(body)
		for _, required := range []string{html.EscapeString(payload), html.EscapeString(queue), "9007199254740993", "Adresse vide observée", "Adresse non spécifiée", "Octets non UTF-8 — base64", base64.StdEncoding.EncodeToString([]byte("\xffsynthetic")), "Case@example.test", "case@example.test", "Dernières tentatives simultanées", "Inconnus : 5"} {
			if !strings.Contains(text, required) {
				t.Fatal("literal recipient/conflict/provenance lost", required)
			}
		}
		for _, forbidden := range []string{"<script", "<img", `href="javascript:`, "private reply", "synthetic@example.test"} {
			if strings.Contains(text, forbidden) {
				t.Fatal("hostile/native data became active or escaped allowlist", forbidden)
			}
		}
	}
	if err := sessions.Revoke(token); err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", origin+CandidatePagePrefix+id, nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 || calls.Load() != 2 {
		t.Fatal("revoked detail session read facts")
	}
}

func TestCandidatePageSharedBudgetCancellationAndHTMLExpansion(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	facts := candidatePageFacts("synthetic-postfix", "ABC123", "synthetic", "origin", []string{"recipient@example.test"}, 1)
	id := candidatePageID(t, facts)
	entered := make(chan struct{}, 1)
	reader := &stubSearchReader{facts: func(ctx context.Context, _ sqlite.CorrelationScope, _ int) ([]correlation.Fact, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return facts, nil
	}}
	options := DefaultSearchOptions()
	options.MaxConcurrent = 1
	h, err := newReadHandler(guard, reader, options, time.Now, true)
	if err != nil {
		t.Fatal(err)
	}
	path := CandidatePagePrefix + id
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest("GET", path, token).WithContext(ctx))
		done <- w
	}()
	<-entered
	for _, target := range []string{path, SearchPath + "/" + id, strings.Replace(searchTestURL(), SearchPath, SearchPagePath, 1)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest("GET", target, token))
		assertSearchHTTP(t, w, 429)
	}
	cancel()
	w := <-done
	assertSearchPage(t, w, 503)
	if strings.Contains(w.Body.String(), "ABC123") {
		t.Fatal("late detail returned data")
	}
	// All equal-time latest references must survive. Their escaped provenance
	// fits the fact budget but expands past the encoded HTML cap.
	quoted := strings.Repeat(`"`, 1024)
	facts = candidatePageFacts("synthetic-postfix", "ABC123", quoted, quoted, []string{"recipient@example.test"}, 200)
	id = candidatePageID(t, facts)
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return facts, nil }
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", CandidatePagePrefix+id, token))
	assertSearchPage(t, w, 503)
	if strings.Contains(w.Body.String(), "ABC123") || strings.Contains(w.Body.String(), "Destinataires observés") {
		t.Fatal("expanded page emitted partial candidate data")
	}
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		return nil, errors.New("synthetic private SQL/path/password")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchPage(t, w, 503)
	if strings.Contains(w.Body.String(), "SQL/path/password") {
		t.Fatal("detail leaked internal error")
	}
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return nil, nil }
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("HEAD", path, token))
	assertSearchPage(t, w, 404)
	if w.Body.Len() != 0 {
		t.Fatal("error HEAD body or slot not released")
	}
}
