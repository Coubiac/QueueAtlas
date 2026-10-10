package httpapi

import (
	"context"
	"encoding/base64"
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
	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func TestTimelinePageSQLiteLinksPaginationFullScopeAndStale(t *testing.T) {
	s := syntheticSearchStore(t)
	storeSearchCycle(t, s, "synthetic-first", "synthetic-postfix", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-second", "synthetic-postfix", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-undated", "synthetic-postfix", "ABC123", false)
	guard, token, _ := searchGuardFixture(t)
	options := DefaultSearchOptions()
	options.AllowRawLogs = true
	h, _ := NewConsultationHandler(guard, s, options)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	id := decodeSearchResponse(t, w).Matches[0].Candidate.ID
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", CandidatePagePrefix+id, token))
	assertSearchPage(t, w, 200)
	link := regexp.MustCompile(`<a class="timeline-link" href="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(link) != 2 || html.UnescapeString(link[1]) != timelinePageURL(id) {
		t.Fatal("detail lost canonical local timeline link")
	}
	path := timelinePageURL(id) + "?limit=1"
	for i, kind := range []string{"Message observé", "Tentative observée", "Retrait de file observé"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest("GET", path, token))
		assertSearchPage(t, w, 200)
		for _, required := range []string{kind, "synthetic-first-origin", "Continuité entre origines incertaine", "ordre causal", "Afficher depuis le début"} {
			if !strings.Contains(w.Body.String(), required) {
				t.Fatal("timeline page lost generation/reserve", required)
			}
		}
		for _, forbidden := range []string{"synthetic-second", "synthetic-undated", "synthetic private raw", "private.log", `name="cursor"`} {
			if strings.Contains(w.Body.String(), forbidden) {
				t.Fatal("timeline crossed origin/exposed raw/carried cursor into fresh submit")
			}
		}
		if i == 1 && (!strings.Contains(w.Body.String(), "sent (transport)") || !strings.Contains(w.Body.String(), "Prochain saut SMTP") || !strings.Contains(w.Body.String(), "Non observé")) {
			t.Fatal("delivery metadata hidden by raw policy or transport changed")
		}
		next := regexp.MustCompile(`<a class="next" href="([^"]+)"`).FindStringSubmatch(w.Body.String())
		if i == 2 {
			if len(next) != 0 {
				t.Fatal("final timeline page continued")
			}
			break
		}
		if len(next) != 2 {
			t.Fatal("timeline pagination link missing")
		}
		path = html.UnescapeString(next[1])
		parsed, err := url.Parse(path)
		if err != nil || parsed.Path != timelinePageURL(id) {
			t.Fatal("pagination escaped selected candidate")
		}
		q, err := parseTimelineRequest(parsed.RawQuery, id)
		if err != nil || q.Raw || q.Limit != 1 || q.Start != i+1 {
			t.Fatal("pagination lost candidate/mode/limit/index", err)
		}
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id)+"?limit=1&raw=1", token))
	assertSearchPage(t, w, 200)
	if !strings.Contains(w.Body.String(), `name="raw" value="1" checked`) || !strings.Contains(w.Body.String(), "synthetic private raw 0\\n") {
		t.Fatal("explicit raw request did not show labeled record/control notation")
	}
	next := regexp.MustCompile(`<a class="next" href="([^"]+)"`).FindStringSubmatch(w.Body.String())
	parsed, err := url.Parse(html.UnescapeString(next[1]))
	if err != nil {
		t.Fatal(err)
	}
	q, err := parseTimelineRequest(parsed.RawQuery, id)
	if err != nil || !q.Raw || q.Start != 1 {
		t.Fatal("raw pagination lost binding")
	}
	options.FactLimit = 2
	small, _ := NewConsultationHandler(guard, s, options)
	w = httptest.NewRecorder()
	small.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id)+"?limit=1", token))
	assertSearchPage(t, w, 422)
	outside, _ := encodeTimelineCursor(id, false, 4095)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id)+"?cursor="+outside, token))
	assertSearchPage(t, w, 400)
	storeSearchCycle(t, s, "synthetic-late", "synthetic-postfix", "ABC123", true)
	for _, method := range []string{"GET", "HEAD"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest(method, timelinePageURL(id)+"?cursor="+outside, token))
		assertSearchPage(t, w, 409)
		if method == "HEAD" && w.Body.Len() != 0 || strings.Contains(w.Body.String(), "synthetic-first") {
			t.Fatal("stale timeline emitted data or applied old position")
		}
	}
}

func TestTimelinePageProtocolAndRawPermissionBeforeStorage(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	var calls atomic.Int32
	reader := &stubSearchReader{facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		calls.Add(1)
		return nil, nil
	}}
	h, _ := newReadHandler(guard, reader, DefaultSearchOptions(), func() time.Time { panic("timeline must not read search clock") }, true)
	path := timelinePageURL(goldenCandidateID)
	for _, tc := range []struct {
		change func(*http.Request)
		status int
	}{
		{func(r *http.Request) { r.Header.Del("Cookie") }, 401}, {func(r *http.Request) { r.TLS = nil }, 403},
		{func(r *http.Request) { r.Header.Set("Origin", "https://foreign.example.test") }, 403},
		{func(r *http.Request) { r.URL.RawQuery = "raw=1" }, 403}, {func(r *http.Request) { r.URL.RawQuery = "limit=0" }, 400},
		{func(r *http.Request) { r.URL.RawQuery = "cursor=" + goldenTimelineCursor + "&raw=1" }, 400},
		{func(r *http.Request) { r.URL.RawQuery = "password=synthetic-secret" }, 400}, {func(r *http.Request) { r.URL.RawQuery = "raw=0&raw=1" }, 400},
		{func(r *http.Request) { r.URL.Path = timelinePageURL("invalid") }, 400}, {func(r *http.Request) { r.URL.Path += "/" }, 404},
		{func(r *http.Request) { r.URL.RawPath = "/%6dessages/" + goldenCandidateID + "/events" }, 400},
		{func(r *http.Request) { r.Method = "POST"; r.Header.Set("Origin", searchTestOrigin) }, 405},
		{func(r *http.Request) { r.ContentLength = 1; r.Body = panicSearchBody{} }, 400},
		{func(r *http.Request) { r.ContentLength = -1; r.TransferEncoding = []string{"chunked"} }, 400},
	} {
		w := httptest.NewRecorder()
		r := searchReadRequest("GET", path, token)
		tc.change(r)
		h.ServeHTTP(w, r)
		assertSearchHTTP(t, w, tc.status)
		if calls.Load() != 0 || strings.Contains(w.Body.String(), "synthetic-secret") {
			t.Fatal("bad timeline read facts or reflected private query")
		}
	}
	api, _ := newSearchHandler(guard, reader, DefaultSearchOptions(), time.Now)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchHTTP(t, w, 404)
	if calls.Load() != 0 {
		t.Fatal("API-only handler acquired HTML route")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("HEAD", path, token))
	assertSearchPage(t, w, 404)
	if w.Body.Len() != 0 || calls.Load() != 1 {
		t.Fatal("HEAD absence body/read count")
	}
	facts, id := timelineSyntheticFacts(t)
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return facts, nil }
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id), token))
	assertSearchPage(t, w, 200)
	if !strings.Contains(w.Body.String(), "Lignes brutes non autorisées") || strings.Contains(w.Body.String(), `name="raw"`) || !strings.Contains(w.Body.String(), html.EscapeString("<script>synthetic reply</script>")) {
		t.Fatal("raw prohibition hid allowed metadata or offered unauthorized control")
	}
}

func TestTimelinePageSQLiteRawBinaryAndHTTPSHostileNativeValues(t *testing.T) {
	s := syntheticSearchStore(t)
	at := time.Unix(0, goldenSearchFromNS).UTC()
	raw := []byte("synthetic\xff\x00</pre><script>\r\n")
	o := model.Observation{SourceID: "synthetic-binary", QueueID: "ABC123", Kind: model.KindMessage, Service: "cleanup", Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}, Fields: map[string]string{"message-id": "synthetic@example.test"}, Present: map[string]bool{"message-id": true}}
	batch := source.Batch{Source: source.Identity{ID: o.SourceID, Kind: "file", Name: "synthetic", TrustedHost: "synthetic-postfix"}, Origins: []source.Origin{{ID: "binary-origin", Path: "/synthetic/private.log", Fingerprint: "synthetic", FirstSeen: at}}, Records: []source.Record{{OriginID: "binary-origin", Start: 0, End: int64(len(raw)), Raw: raw, ReadAt: at, Observation: o}}}
	if err := s.Commit(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	facts, err := s.CorrelationFacts(context.Background(), sqlite.CorrelationScope{Queues: []correlation.QueueKey{{Instance: "synthetic-postfix", QueueID: "ABC123"}}}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	id := candidatePageID(t, facts)
	guard, token, _ := searchGuardFixture(t)
	options := DefaultSearchOptions()
	options.AllowRawLogs = true
	h, _ := NewConsultationHandler(guard, s, options)
	encoded := base64.StdEncoding.EncodeToString(raw)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id), token))
	assertSearchPage(t, w, 200)
	if strings.Contains(w.Body.String(), encoded) || strings.Contains(w.Body.String(), `<h4>Ligne brute</h4>`) {
		t.Fatal("permission alone exposed raw")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id)+"?raw=1", token))
	assertSearchPage(t, w, 200)
	if !strings.Contains(w.Body.String(), "Octets non UTF-8 — base64") || !strings.Contains(w.Body.String(), "<pre>"+encoded+"</pre>") || strings.Contains(w.Body.String(), "<script>") {
		t.Fatal("SQLite raw bytes changed or became markup")
	}
	server := httptest.NewUnstartedServer(nil)
	defer server.Close()
	origin := "https://" + server.Listener.Addr().String()
	guard, token, sessions := searchGuardAtOrigin(t, origin)
	facts, _ = timelineSyntheticFacts(t)
	payload := `</pre><script>alert(1)</script><a href="javascript:x">`
	controls := "\x00\r\n\t\x1b\u202e\\n"
	for i := range facts {
		facts[i].Observation.Host = payload + controls
		facts[i].Observation.Raw = "synthetic visible raw " + payload + controls
	}
	facts[3].Observation.Fields["from"] = "\xffsynthetic"
	facts[3].Observation.Present["from"] = true
	id = candidatePageID(t, facts)
	var calls atomic.Int32
	reader := &stubSearchReader{facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		calls.Add(1)
		return facts, nil
	}}
	h, _ = newReadHandler(guard, reader, options, time.Now, true)
	server.Config.Handler = h
	server.StartTLS()
	client := server.Client()
	for _, tc := range []struct{ method, query string }{{"GET", ""}, {"GET", "raw=1"}, {"HEAD", "raw=1"}} {
		r, _ := http.NewRequest(tc.method, origin+timelinePageURL(id)+"?"+tc.query, nil)
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-store" || len(response.Cookies()) != 0 {
			t.Fatal("real HTTPS timeline policy")
		}
		if tc.method == "HEAD" {
			if len(body) != 0 {
				t.Fatal("HTTPS timeline HEAD body")
			}
			continue
		}
		text := string(body)
		for _, required := range []string{html.EscapeString(payload), `\x00\r\n\t\x1b\u202e\\n`, html.EscapeString("<script>synthetic reply</script>"), "Valeur vide observée", "Non observé", "sent (transport)", "Échec rapporté", "Dates sous hypothèse", "Octets non UTF-8 — base64", base64.StdEncoding.EncodeToString([]byte("\xffsynthetic"))} {
			if !strings.Contains(text, required) {
				t.Fatal("timeline native/tie/presence presentation lost", required)
			}
		}
		if tc.query == "" && strings.Contains(text, "synthetic visible raw") {
			t.Fatal("metadata-only HTTPS response exposed raw")
		}
		for _, forbidden := range []string{"<script", `<a href="javascript:`, "\x00", "\x1b", "\u202e", "synthetic-arbitrary-field", "private.log"} {
			if strings.Contains(text, forbidden) {
				t.Fatal("timeline activated/reflected unselected data", forbidden)
			}
		}
	}
	if err := sessions.Revoke(token); err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", origin+timelinePageURL(id)+"?raw=1", nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 || calls.Load() != 3 {
		t.Fatal("revoked timeline read data")
	}
}

func TestTimelinePageSharedAdmissionCancellationAndExpandedRawCap(t *testing.T) {
	facts, id := timelineSyntheticFacts(t)
	guard, token, _ := searchGuardFixture(t)
	options := DefaultSearchOptions()
	options.MaxConcurrent, options.AllowRawLogs = 1, true
	entered := make(chan struct{}, 1)
	reader := &stubSearchReader{facts: func(ctx context.Context, _ sqlite.CorrelationScope, _ int) ([]correlation.Fact, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return facts, nil
	}}
	h, _ := newReadHandler(guard, reader, options, time.Now, true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id), token).WithContext(ctx))
		done <- w
	}()
	<-entered
	for _, path := range []string{timelinePageURL(id), CandidatePagePrefix + id, SearchPath + "/" + id + "/events", strings.Replace(searchTestURL(), SearchPath, SearchPagePath, 1)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest("GET", path, token))
		assertSearchHTTP(t, w, 429)
	}
	cancel()
	w := <-done
	assertSearchPage(t, w, 503)
	if strings.Contains(w.Body.String(), "synthetic-origin") {
		t.Fatal("late timeline exposed facts")
	}
	for i := range facts {
		facts[i].Observation.Raw = strings.Repeat("\x00", model.MaxLineBytes)
	}
	id = candidatePageID(t, facts)
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return facts, nil }
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id)+"?raw=1", token))
	assertSearchPage(t, w, 503)
	if strings.Contains(w.Body.String(), "synthetic-origin") || strings.Contains(w.Body.String(), `<h4>Ligne brute</h4>`) {
		t.Fatal("expanded raw emitted partial timeline")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", timelinePageURL(id)+"?raw=1&limit=1", token))
	assertSearchPage(t, w, 200)
	if strings.Count(w.Body.String(), `\x00`) != model.MaxLineBytes {
		t.Fatal("small raw page truncated native record")
	}
}
