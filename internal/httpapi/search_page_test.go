package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func assertSearchPage(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	assertSearchHTTP(t, w, status)
	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" || w.Header().Get("X-Frame-Options") != "DENY" || w.Header().Get("Referrer-Policy") != "strict-origin" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "form-action 'self'") {
		t.Fatal("HTML response security policy changed")
	}
}

func TestConsultationSearchPageDefaultsProtocolAndAPICompatibility(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	var reads atomic.Int32
	reader := &stubSearchReader{search: func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) {
		reads.Add(1)
		return sqlite.SearchPage{}, nil
	}}
	now := func() time.Time { return time.Unix(0, goldenSearchFromNS).UTC() }
	h, err := newReadHandler(guard, reader, DefaultSearchOptions(), now, true)
	if err != nil {
		t.Fatal(err)
	}
	if h, err := NewConsultationHandler(guard, nil, DefaultSearchOptions()); h != nil || err != ErrSearchSetup {
		t.Fatal("nil store accepted")
	}
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest(method, SearchPagePath, token))
		assertSearchPage(t, w, 200)
		if reads.Load() != 0 || method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("empty form read storage or HEAD returned data")
		}
		if method == "GET" {
			body := w.Body.String()
			for _, required := range []string{`<html lang="fr">`, `method="get" action="/messages"`, `for="instance"`, `for="field"`, `for="value"`, `2026-10-06T00:00:00Z`, `2026-10-07T00:00:00Z`, `value="50"`, `href="#main"`, `<main id="main" tabindex="-1">`} {
				if !strings.Contains(body, required) {
					t.Fatal("form default/label missing", required)
				}
			}
			style := regexp.MustCompile(`(?s)<style>(.*?)</style>`).FindStringSubmatch(body)
			if len(style) != 2 {
				t.Fatal("embedded stylesheet missing")
			}
			// Hash the style text as an HTML parser sees it, including Windows CRLF.
			parsedStyle := strings.ReplaceAll(strings.ReplaceAll(style[1], "\r\n", "\n"), "\r", "\n")
			digest := sha256.Sum256([]byte(parsedStyle))
			if !strings.Contains(w.Header().Get("Content-Security-Policy"), "'sha256-"+base64.StdEncoding.EncodeToString(digest[:])+"'") || strings.Contains(body, "<script") {
				t.Fatal("style CSP does not authorize actual stylesheet or page added scripts")
			}
		}
	}
	for _, tc := range []struct {
		change func(*http.Request)
		status int
	}{
		{func(r *http.Request) { r.Header.Del("Cookie") }, 401}, {func(r *http.Request) { r.Header.Set("Origin", "https://foreign.example.test") }, 403},
		{func(r *http.Request) { r.TLS = nil }, 403}, {func(r *http.Request) { r.URL.RawQuery = "password=synthetic-secret" }, 400},
		{func(r *http.Request) { r.URL.Path += "/" }, 404}, {func(r *http.Request) { r.URL.RawPath = "/%6dessages" }, 400},
		{func(r *http.Request) { r.ContentLength = 1; r.Body = panicSearchBody{} }, 400}, {func(r *http.Request) { r.Method = "POST"; r.Header.Set("Origin", searchTestOrigin) }, 405},
	} {
		w := httptest.NewRecorder()
		r := searchReadRequest("GET", SearchPagePath, token)
		tc.change(r)
		h.ServeHTTP(w, r)
		assertSearchHTTP(t, w, tc.status)
		if reads.Load() != 0 || strings.Contains(w.Body.String(), "synthetic-secret") {
			t.Fatal("bad page request reached storage or reflected private input")
		}
	}
	apiOnly, _ := newSearchHandler(guard, reader, DefaultSearchOptions(), now)
	w := httptest.NewRecorder()
	apiOnly.ServeHTTP(w, searchReadRequest("GET", SearchPagePath, token))
	assertSearchHTTP(t, w, 404)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	decodeSearchResponse(t, w)
	if reads.Load() != 1 {
		t.Fatal("combined router changed API behavior")
	}
}

func TestConsultationSearchPageSQLitePaginationAndReserves(t *testing.T) {
	s := syntheticSearchStore(t)
	storeSearchCycle(t, s, "synthetic-first", "synthetic-postfix", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-second", "synthetic-postfix", "DEF456", true)
	storeSearchCycle(t, s, "synthetic-undated", "synthetic-postfix", "ABC123", false)
	guard, token, _ := searchGuardFixture(t)
	h, err := NewConsultationHandler(guard, s, DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	path := strings.Replace(searchTestURL(), SearchPath, SearchPagePath, 1)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchPage(t, w, 200)
	for _, required := range []string{"ABC123", "sent (transport) : 1", "delivered (remise reconnue) : 0", "Couverture non prouvée", "Continuité entre origines incertaine", "Page suivante", "synthetic-first"} {
		if !strings.Contains(w.Body.String(), required) {
			t.Fatal("full queue summary/reserve missing", required)
		}
	}
	for _, private := range []string{"private raw", "private reply", "private.log", "PasswordHash"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("search exposed unselected log/account content")
		}
	}
	link := regexp.MustCompile(`<a class="next" href="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(link) != 2 {
		t.Fatal("missing pagination link")
	}
	next := html.UnescapeString(link[1])
	parsed, err := url.Parse(next)
	if err != nil || parsed.Path != SearchPagePath {
		t.Fatal("pagination escaped its fixed local path")
	}
	query, err := ParseSearchRequest(parsed.RawQuery, time.Time{})
	if err != nil || query.After == nil || query.Limit != 1 || query.From.UnixNano() != goldenSearchFromNS {
		t.Fatal("pagination lost canonical scope/frozen dates", err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", next, token))
	assertSearchPage(t, w, 200)
	if !strings.Contains(w.Body.String(), "DEF456") || strings.Contains(w.Body.String(), "ABC123") || strings.Contains(w.Body.String(), `name="cursor"`) || strings.Contains(w.Body.String(), `class="next"`) {
		t.Fatal("next page duplicates candidate or form retains continuation")
	}
	options := DefaultSearchOptions()
	options.FactLimit = 1
	small, _ := NewConsultationHandler(guard, s, options)
	w = httptest.NewRecorder()
	small.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchPage(t, w, 422)
	if strings.Contains(w.Body.String(), "ABC123") || !strings.Contains(w.Body.String(), "Trop de faits") {
		t.Fatal("truncated page exposed a summary")
	}
	params, _ := searchFixture()
	// This test's stored facts have fixed dates; do not use today's default window.
	params.Set("from", "2026-10-07T00:00:00Z")
	params.Set("until", "2026-10-08T00:00:00Z")
	params.Set("value", "absent@example.test")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPagePath+"?"+params.Encode(), token))
	assertSearchPage(t, w, 200)
	if !strings.Contains(w.Body.String(), "Aucun événement correspondant") || !strings.Contains(w.Body.String(), "Cela ne prouve pas") {
		t.Fatal("empty page implied complete log absence")
	}
	at := time.Unix(0, goldenSearchFromNS).UTC()
	storeSearchObservations(t, s, "synthetic-prequeue-ui", "synthetic-postfix", []model.Observation{{
		Kind: model.KindReject, Service: "smtpd", NoQueue: true,
		Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset},
		Message:   "reject_warning: synthetic private diagnostic",
		Fields:    map[string]string{"from": "warning@example.test"}, Present: map[string]bool{"from": true},
	}})
	params.Set("value", "warning@example.test")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPagePath+"?"+params.Encode(), token))
	assertSearchPage(t, w, 200)
	for _, required := range []string{"NOQUEUE", "Avertissement, pas un rejet", "Candidat non attribué"} {
		if !strings.Contains(w.Body.String(), required) {
			t.Fatal("prequeue warning acquired a queue or rejection", required)
		}
	}
	if strings.Contains(w.Body.String(), "synthetic private diagnostic") {
		t.Fatal("prequeue display exposed raw diagnostic")
	}
}

func TestConsultationSearchPageRealHTTPSHostileTextAndRevocation(t *testing.T) {
	server := httptest.NewUnstartedServer(nil)
	defer server.Close()
	origin := "https://" + server.Listener.Addr().String()
	guard, token, sessions := searchGuardAtOrigin(t, origin)
	payload := `"><script>alert(1)</script>`
	queue := `Q"><img onerror=x>`
	at := time.Unix(0, goldenSearchFromNS).UTC()
	ref := correlation.FactRef{SourceID: payload, OriginID: payload, Start: 1<<53 + 1, End: 1<<53 + 2}
	o := model.Observation{SourceID: payload, QueueID: queue, Kind: model.KindMessage, Service: "cleanup", Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}, Fields: map[string]string{"message-id": "synthetic@example.test"}, Present: map[string]bool{"message-id": true}}
	var calls atomic.Int32
	reader := &stubSearchReader{search: func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) {
		calls.Add(1)
		return sqlite.SearchPage{Hits: []sqlite.SearchHit{{Ref: ref, Instance: payload, QueueID: queue, At: at, Kind: o.Kind, TimeQuality: o.Timestamp.Quality}}}, nil
	}, facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		return []correlation.Fact{{Ref: ref, Instance: payload, Observation: o}}, nil
	}}
	h, err := newReadHandler(guard, reader, DefaultSearchOptions(), func() time.Time { return at.Add(time.Hour) }, true)
	if err != nil {
		t.Fatal(err)
	}
	server.Config.Handler = h
	server.StartTLS()
	client := server.Client()
	params, _ := searchFixture()
	params.Set("instance", payload)
	params.Set("value", payload)
	for _, tc := range []struct {
		method string
		cookie bool
		status int
	}{{"GET", false, 401}, {"GET", true, 200}, {"HEAD", true, 200}} {
		r, _ := http.NewRequest(tc.method, origin+SearchPagePath+"?"+params.Encode(), nil)
		if tc.cookie {
			r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != tc.status || response.Header.Get("Cache-Control") != "no-store" || len(response.Cookies()) != 0 {
			t.Fatal("real HTTPS page policy failed")
		}
		if tc.method == "HEAD" && len(body) != 0 {
			t.Fatal("HEAD sent HTML")
		}
		if tc.method == "GET" && tc.status == 200 {
			if strings.Contains(string(body), "<script>") || strings.Contains(string(body), "<img") || !strings.Contains(string(body), html.EscapeString(payload)) || !strings.Contains(string(body), html.EscapeString(queue)) || !strings.Contains(string(body), "9007199254740993") {
				t.Fatal("hostile input/ref/candidate became markup or offsets rounded")
			}
		}
	}
	if calls.Load() != 2 {
		t.Fatal("unauthenticated page read storage")
	}
	if err := sessions.Revoke(token); err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", origin+SearchPagePath+"?"+params.Encode(), nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 || calls.Load() != 2 {
		t.Fatal("revoked page session read data")
	}
}

func TestConsultationSearchPageSharedAdmissionCancellationAndByteCap(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	entered := make(chan struct{}, 1)
	reader := &stubSearchReader{search: func(ctx context.Context, _ sqlite.SearchQuery) (sqlite.SearchPage, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return sqlite.SearchPage{}, nil
	}}
	options := DefaultSearchOptions()
	options.MaxConcurrent = 1
	h, err := newReadHandler(guard, reader, options, time.Now, true)
	if err != nil {
		t.Fatal(err)
	}
	path := strings.Replace(searchTestURL(), SearchPath, SearchPagePath, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest("GET", path, token).WithContext(ctx))
		done <- w
	}()
	<-entered
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	assertSearchHTTP(t, w, 429)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchPage(t, w, 429)
	cancel()
	w = <-done
	assertSearchPage(t, w, 503)
	if strings.Contains(w.Body.String(), "Événements trouvés") {
		t.Fatal("late successful read exposed results")
	}
	// Distinct refs of one queue fit the native count budget. HTML expansion of
	// quoted provenance exceeds the shared one MiB response cap.
	var facts []correlation.Fact
	var hits []sqlite.SearchHit
	quoted := strings.Repeat(`"`, 1024)
	at := time.Unix(0, goldenSearchFromNS).UTC()
	for i := 0; i < 200; i++ {
		ref := correlation.FactRef{SourceID: quoted, OriginID: quoted, Start: int64(i * 2), End: int64(i*2 + 1)}
		o := model.Observation{SourceID: quoted, QueueID: "ABC123", Kind: model.KindMessage, Service: "cleanup", Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}, Fields: map[string]string{"message-id": "synthetic@example.test"}, Present: map[string]bool{"message-id": true}}
		facts = append(facts, correlation.Fact{Ref: ref, Instance: "synthetic-postfix", Observation: o})
		hits = append(hits, sqlite.SearchHit{Ref: ref, Instance: "synthetic-postfix", QueueID: "ABC123", At: at, Kind: o.Kind, TimeQuality: o.Timestamp.Quality})
	}
	reader.search = func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) {
		return sqlite.SearchPage{Hits: hits}, nil
	}
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return facts, nil }
	params, _ := url.ParseQuery(strings.SplitN(path, "?", 2)[1])
	params.Set("limit", "200")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPagePath+"?"+params.Encode(), token))
	assertSearchPage(t, w, 503)
	if strings.Contains(w.Body.String(), "ABC123") || strings.Contains(w.Body.String(), "Événements trouvés") {
		t.Fatal("HTML byte cap emitted partial results")
	}
	reader.search = func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) {
		return sqlite.SearchPage{}, errors.New("synthetic private SQL/path/password")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchPage(t, w, 503)
	if strings.Contains(w.Body.String(), "SQL/path/password") {
		t.Fatal("page exposed internal error")
	}
	reader.search = func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) { return sqlite.SearchPage{}, nil }
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("HEAD", path, token))
	assertSearchPage(t, w, 200)
	if w.Body.Len() != 0 {
		t.Fatal("released slot HEAD body")
	}
}
