package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
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

const searchTestOrigin = "https://queueatlas.example.test"

func searchGuardFixture(t *testing.T) (*auth.HTTPHandler, string, *auth.SessionStore) {
	t.Helper()
	return searchGuardAtOrigin(t, searchTestOrigin)
}

func searchGuardAtOrigin(t *testing.T, origin string) (*auth.HTTPHandler, string, *auth.SessionStore) {
	t.Helper()
	account := auth.LocalAccount{Identity: auth.LocalIdentity{Username: "SyntheticOperator"},
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$U7bF6tMN+ytOCGfrrY1N5M+a10UUftgXbiHnZVmZ29U"}
	sessions, err := auth.NewSessionStore(auth.DefaultSessionOptions())
	if err != nil {
		t.Fatal(err)
	}
	login, err := auth.NewLocalLogin(account, sessions, auth.DefaultLoginOptions())
	if err != nil {
		t.Fatal(err)
	}
	guard, err := auth.NewHTTPHandler(login, origin)
	if err != nil {
		t.Fatal(err)
	}
	// Trusted test setup issues a session. Credential verification is already
	// tested by auth; these integration tests exercise Protect on every request.
	token, _, err := sessions.Issue(account.Identity)
	if err != nil {
		t.Fatal(err)
	}
	return guard, token, sessions
}

func searchReadRequest(method, path, token string) *http.Request {
	r := httptest.NewRequest(method, searchTestOrigin+path, nil)
	if token != "" {
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	}
	return r
}

func searchTestURL() string {
	params, _ := searchFixture()
	params.Set("from", "2026-10-07T00:00:00Z")
	params.Set("until", "2026-10-08T00:00:00Z")
	params.Set("limit", "1")
	return SearchPath + "?" + params.Encode()
}

func assertSearchHTTP(t *testing.T, w *httptest.ResponseRecorder, status int) {
	t.Helper()
	if w.Code != status || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" ||
		w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("Location") != "" || len(w.Result().Cookies()) != 0 {
		t.Fatal("wrong status or response policy", w.Code, status)
	}
}

type stubSearchReader struct {
	search func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error)
	facts  func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error)
}

func (s *stubSearchReader) SearchEvents(ctx context.Context, q sqlite.SearchQuery) (sqlite.SearchPage, error) {
	return s.search(ctx, q)
}
func (s *stubSearchReader) CorrelationFacts(ctx context.Context, scope sqlite.CorrelationScope, limit int) ([]correlation.Fact, error) {
	return s.facts(ctx, scope, limit)
}

func TestSearchHandlerConstructionAndProtocolBeforeStorage(t *testing.T) {
	guard, token, sessions := searchGuardFixture(t)
	var calls atomic.Int32
	reader := &stubSearchReader{search: func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) {
		calls.Add(1)
		return sqlite.SearchPage{}, nil
	}, facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		t.Fatal("empty page requested facts")
		return nil, nil
	}}
	options := DefaultSearchOptions()
	if options != (SearchOptions{Timeout: 5 * time.Second, FactLimit: 1024, MaxConcurrent: 2}) {
		t.Fatal("defaults changed")
	}
	for _, bad := range []SearchOptions{{}, {time.Second - 1, 1, 1, false}, {30*time.Second + 1, 1, 1, false}, {time.Second, 0, 1, false}, {time.Second, 4097, 1, false}, {time.Second, 1, 0, false}, {time.Second, 1, 9, false}} {
		if h, err := newSearchHandler(guard, reader, bad, time.Now); h != nil || err != ErrSearchSetup {
			t.Fatal("bad options accepted")
		}
	}
	if h, err := NewSearchHandler(guard, nil, options); h != nil || err != ErrSearchSetup {
		t.Fatal("nil store accepted")
	}
	if h, err := newSearchHandler(&auth.HTTPHandler{}, reader, options, time.Now); h != nil || err != ErrSearchSetup {
		t.Fatal("uninitialized guard accepted")
	}
	h, err := newSearchHandler(guard, reader, options, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*http.Request)
		status int
	}{
		{"missing cookie", func(r *http.Request) { r.Header.Del("Cookie") }, 401},
		{"foreign origin", func(r *http.Request) { r.Header.Set("Origin", "https://foreign.example.test") }, 403},
		{"plain HTTP", func(r *http.Request) { r.TLS = nil }, 403},
		{"wrong path", func(r *http.Request) { r.URL.Path = SearchPath + "/" }, 404},
		{"write method", func(r *http.Request) { r.Method = "POST"; r.Header.Set("Origin", searchTestOrigin) }, 405},
		{"escaped path", func(r *http.Request) { r.URL.RawPath = "/api/v1/%6dessages" }, 400},
		{"query duplication", func(r *http.Request) { r.URL.RawQuery += "&value=synthetic-private" }, 400},
		{"encoded body", func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }, 415},
		{"nonempty body", func(r *http.Request) { r.ContentLength = 1; r.Body = panicSearchBody{} }, 400},
		{"unknown body length", func(r *http.Request) { r.ContentLength = -1; r.Body = panicSearchBody{} }, 400},
		{"transfer encoding", func(r *http.Request) { r.TransferEncoding = []string{"chunked"}; r.Body = panicSearchBody{} }, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := searchReadRequest("GET", searchTestURL(), token)
			tc.change(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			assertSearchHTTP(t, w, tc.status)
			if calls.Load() != 0 {
				t.Fatal("invalid request reached storage")
			}
		})
	}
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest(method, searchTestURL(), token))
		assertSearchHTTP(t, w, 200)
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD returned a body")
		}
		if method == "GET" && !strings.Contains(w.Body.String(), `"matches":[]`) {
			t.Fatal("empty results must be array")
		}
	}
	if calls.Load() != 2 {
		t.Fatal("valid reads not executed")
	}
	if err := sessions.Revoke(token); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	assertSearchHTTP(t, w, 401)
	if calls.Load() != 2 {
		t.Fatal("revoked session reached reader")
	}
}

type panicSearchBody struct{}

func (panicSearchBody) Read([]byte) (int, error) { panic("search must not read body") }
func (panicSearchBody) Close() error             { return nil }

func syntheticSearchStore(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "synthetic.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func storeSearchCycle(t *testing.T, s *sqlite.Store, sourceID, instance, queue string, dated bool) {
	t.Helper()
	at := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	observations := []model.Observation{
		{Kind: model.KindMessage, Service: "cleanup", QueueID: queue, Fields: map[string]string{"message-id": "synthetic@example.test", "from": "synthetic@example.test"}, Present: map[string]bool{"message-id": true, "from": true}},
		{Kind: model.KindDelivery, Service: "smtp", QueueID: queue, Message: "to=<recipient@example.test>, status=sent (synthetic private reply)", Fields: map[string]string{"to": "recipient@example.test", "status": "sent"}, Present: map[string]bool{"to": true, "status": true}},
		{Kind: model.KindRemoved, Service: "qmgr", QueueID: queue},
	}
	for i, o := range observations {
		o.SourceID = sourceID
		if dated {
			instant := at.Add(time.Duration(i) * 24 * time.Hour)
			o.Timestamp = model.Timestamp{Value: &instant, Quality: model.TimeExplicitOffset}
		}
		observations[i] = o
	}
	storeSearchObservations(t, s, sourceID, instance, observations)
}

func storeSearchObservations(t *testing.T, s *sqlite.Store, sourceID, instance string, observations []model.Observation) {
	t.Helper()
	at := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	originID := sourceID + "-origin"
	batch := source.Batch{Source: source.Identity{ID: sourceID, Kind: "file", Name: "synthetic", TrustedHost: instance}, Origins: []source.Origin{{ID: originID, Path: "/synthetic/private.log", Fingerprint: sourceID, FirstSeen: at}}}
	var offset int64
	for i, o := range observations {
		o.SourceID = sourceID
		raw := []byte("synthetic private raw " + strconv.Itoa(i) + "\n")
		end := offset + int64(len(raw))
		batch.Records = append(batch.Records, source.Record{OriginID: originID, Start: offset, End: end, Raw: raw, ReadAt: at, Observation: o})
		offset = end
	}
	if err := s.Commit(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
}

func decodeSearchResponse(t *testing.T, w *httptest.ResponseRecorder) SearchResponse {
	t.Helper()
	assertSearchHTTP(t, w, 200)
	var response SearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"synthetic private raw", "private reply", "private.log", "PasswordHash", "Raw", "Fields", "synthetic@example.test"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("response exposed observations or query")
		}
	}
	return response
}

func TestSearchHandlerSQLiteFullScopePaginationAndStableCandidate(t *testing.T) {
	s := syntheticSearchStore(t)
	storeSearchCycle(t, s, "synthetic-dated", "synthetic-postfix", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-undated", "synthetic-postfix", "ABC123", false)
	storeSearchCycle(t, s, "synthetic-foreign", "other-instance", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-second", "synthetic-postfix", "DEF456", true)
	guard, token, _ := searchGuardFixture(t)
	h, err := NewSearchHandler(guard, s, DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	first := decodeSearchResponse(t, w)
	if len(first.Matches) != 1 || first.NextCursor == "" || !first.CoverageUnproven {
		t.Fatal("page/cursor/coverage metadata lost")
	}
	candidate := first.Matches[0].Candidate
	if candidate == nil || candidate.QueueID != "ABC123" || candidate.Counts.Sent != 1 || candidate.Counts.Delivered != 0 {
		t.Fatal("full queue lost delivery outside search window or promoted SMTP")
	}
	if !containsSearchReserve(candidate.Reserves, correlation.ReserveCoverageUnproven) || !containsSearchReserve(candidate.Reserves, correlation.ReserveCrossStreamUncertain) {
		t.Fatal("undated origin/coverage reserve lost")
	}
	params, _ := url.ParseQuery(strings.SplitN(searchTestURL(), "?", 2)[1])
	params.Set("limit", "200")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"?"+params.Encode(), token))
	combined := decodeSearchResponse(t, w)
	if len(combined.Matches) != 2 || combined.Matches[0].Candidate.Revision != candidate.Revision {
		t.Fatal("unrelated queue/page size changed candidate revision")
	}
	params.Set("cursor", first.NextCursor)
	params.Set("limit", "1")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"?"+params.Encode(), token))
	second := decodeSearchResponse(t, w)
	if len(second.Matches) != 1 || second.Matches[0].Candidate.QueueID != "DEF456" || second.NextCursor != "" {
		t.Fatal("cursor paging lost second candidate")
	}
	options := DefaultSearchOptions()
	options.FactLimit = 2
	narrow, err := NewSearchHandler(guard, s, options)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	narrow.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	assertSearchHTTP(t, w, 422)
	if w.Body.String() != "{\"error\":\"search_too_broad\"}\n" {
		t.Fatal("truncated reconstruction returned data")
	}
}

func containsSearchReserve(values []correlation.SummaryReserve, value correlation.SummaryReserve) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func TestSearchHandlerDeadlineCancellationAdmissionAndPrivateErrors(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	entered := make(chan context.Context, 1)
	release := make(chan struct{})
	reader := &stubSearchReader{search: func(ctx context.Context, _ sqlite.SearchQuery) (sqlite.SearchPage, error) {
		entered <- ctx
		<-release
		return sqlite.SearchPage{}, errors.New("synthetic-private SQL file account")
	}}
	options := DefaultSearchOptions()
	options.MaxConcurrent = 1
	h, err := newSearchHandler(guard, reader, options, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	request := searchReadRequest("GET", searchTestURL(), token)
	ctx, cancel := context.WithCancel(request.Context())
	request = request.WithContext(ctx)
	go func() { w := httptest.NewRecorder(); h.ServeHTTP(w, request); done <- w }()
	storeCtx := <-entered
	deadline, ok := storeCtx.Deadline()
	if !ok || time.Until(deadline) > options.Timeout {
		t.Fatal("reader has no bounded deadline")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	assertSearchHTTP(t, w, 429)
	// Search, detail and timeline share this same admission budget.
	detailID, err := EncodeCandidateID(goldenCandidateKey())
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+detailID, token))
	assertSearchHTTP(t, w, 429)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+detailID+"/events", token))
	assertSearchHTTP(t, w, 429)
	cancel()
	if storeCtx.Err() != context.Canceled {
		t.Fatal("reader did not inherit request cancellation")
	}
	close(release)
	w = <-done
	assertSearchHTTP(t, w, 503)
	if w.Body.String() != "{\"error\":\"unavailable\"}\n" {
		t.Fatal("error leaked storage detail")
	}
	// Released slot can be used again after the canceled reader returns.
	reader.search = func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) { return sqlite.SearchPage{}, nil }
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("HEAD", searchTestURL(), token))
	assertSearchHTTP(t, w, 200)
	if w.Body.Len() != 0 {
		t.Fatal("HEAD returned body")
	}
	// Pre-expired parent context reaches neither query nor response data.
	expired, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelExpired()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token).WithContext(expired))
	assertSearchHTTP(t, w, 503)
}

func TestSearchHandlerMissingFactsAndResponseLimitFailWhole(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	hit := sqlite.SearchHit{Instance: "synthetic-postfix", QueueID: "ABC123", Ref: correlation.FactRef{SourceID: "synthetic", OriginID: "origin", Start: 0, End: 1}, At: time.Unix(0, goldenSearchFromNS).UTC(), Kind: model.KindMessage, TimeQuality: model.TimeExplicitOffset}
	reader := &stubSearchReader{search: func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) {
		return sqlite.SearchPage{Hits: []sqlite.SearchHit{hit}}, nil
	}, facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return nil, nil }}
	h, err := newSearchHandler(guard, reader, DefaultSearchOptions(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	assertSearchHTTP(t, w, 503)
	if strings.Contains(w.Body.String(), "matches") {
		t.Fatal("missing searched fact fabricated partial response")
	}
	// Offsets above JavaScript's exact integer range remain decimal strings.
	hit.Ref.Start, hit.Ref.End = 1<<53+1, 1<<53+2
	at := hit.At
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		return []correlation.Fact{{Ref: hit.Ref, Instance: hit.Instance, Observation: model.Observation{SourceID: hit.Ref.SourceID, QueueID: hit.QueueID, Kind: hit.Kind, Timestamp: model.Timestamp{Value: &at, Quality: hit.TimeQuality}}}}, nil
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	assertSearchHTTP(t, w, 200)
	if !strings.Contains(w.Body.String(), `"start":"9007199254740993"`) || !strings.Contains(w.Body.String(), `"end":"9007199254740994"`) {
		t.Fatal("provenance offset lost JSON integer precision")
	}
	var buffer responseBuffer
	if _, err := buffer.Write(make([]byte, MaxSearchResponseBytes)); err != nil {
		t.Fatal(err)
	}
	if _, err := buffer.Write([]byte{1}); err == nil || buffer.Len() != MaxSearchResponseBytes {
		t.Fatal("response buffer cap failed")
	}
	// Valid stored identity bytes can expand sixfold when JSON-escaped. Test the
	// actual HTTP path, not just a byte count: no successful prefix is written.
	var hits []sqlite.SearchHit
	var facts []correlation.Fact
	for i := 0; i < sqlite.MaxSearchResults; i++ {
		item := hit
		item.Ref = correlation.FactRef{SourceID: strings.Repeat("\x01", 1024), OriginID: strings.Repeat("\x02", 1024), Start: int64(i), End: int64(i + 1)}
		hits = append(hits, item)
		at := item.At
		facts = append(facts, correlation.Fact{Ref: item.Ref, Instance: item.Instance, Observation: model.Observation{SourceID: item.Ref.SourceID, QueueID: item.QueueID, Kind: item.Kind, Timestamp: model.Timestamp{Value: &at, Quality: item.TimeQuality}}})
	}
	reader.search = func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) {
		return sqlite.SearchPage{Hits: hits}, nil
	}
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return facts, nil }
	params, _ := url.ParseQuery(strings.SplitN(searchTestURL(), "?", 2)[1])
	params.Set("limit", "200")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"?"+params.Encode(), token))
	assertSearchHTTP(t, w, 503)
	if w.Body.String() != "{\"error\":\"unavailable\"}\n" {
		t.Fatal("oversized encoded response leaked partial data")
	}
}

func TestSearchHandlerNoQueueWarningAndUnresolvedStayUnassigned(t *testing.T) {
	s := syntheticSearchStore(t)
	at := time.Unix(0, goldenSearchFromNS).UTC()
	stamp := model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}
	storeSearchObservations(t, s, "synthetic-prequeue", "synthetic-postfix", []model.Observation{{Kind: model.KindReject, Service: "smtpd", NoQueue: true, Timestamp: stamp,
		Message: "reject_warning: synthetic private diagnostic", Fields: map[string]string{"from": "synthetic@example.test"}, Present: map[string]bool{"from": true}}})
	var conflicts []model.Observation
	for i, id := range []string{"one@example.test", "two@example.test"} {
		instant := at.Add(time.Duration(i+1) * time.Minute)
		conflicts = append(conflicts, model.Observation{Kind: model.KindMessage, Service: "cleanup", QueueID: "ABC123", Timestamp: model.Timestamp{Value: &instant, Quality: model.TimeExplicitOffset},
			Fields: map[string]string{"from": "synthetic@example.test", "message-id": id}, Present: map[string]bool{"from": true, "message-id": true}})
	}
	storeSearchObservations(t, s, "synthetic-conflict", "synthetic-postfix", conflicts)
	guard, token, _ := searchGuardFixture(t)
	h, err := NewSearchHandler(guard, s, DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	params, _ := url.ParseQuery(strings.SplitN(searchTestURL(), "?", 2)[1])
	params.Set("limit", "200")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"?"+params.Encode(), token))
	response := decodeSearchResponse(t, w)
	if len(response.Matches) != 3 || !response.Matches[0].NoQueue || response.Matches[0].Candidate != nil || response.Matches[0].PrequeueDisposition != correlation.PrequeueWarning {
		t.Fatal("warning promoted to rejected/accepted queue")
	}
	for _, match := range response.Matches[1:] {
		if match.Candidate != nil || match.UnresolvedReason != correlation.GenerationConflictingIDs {
			t.Fatal("unresolved stream assigned a candidate")
		}
	}
}

func TestSearchHandlerTimeoutDiscardsLateSuccessAndScopeBound(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	reader := &stubSearchReader{search: func(ctx context.Context, _ sqlite.SearchQuery) (sqlite.SearchPage, error) {
		<-ctx.Done()
		return sqlite.SearchPage{}, nil
	}}
	options := DefaultSearchOptions()
	options.Timeout = time.Second
	h, err := newSearchHandler(guard, reader, options, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	assertSearchHTTP(t, w, 503)
	if strings.Contains(w.Body.String(), "matches") {
		t.Fatal("deadline returned late success")
	}
	params, now := searchFixture()
	query, err := ParseSearchRequest(params.Encode(), now)
	if err != nil {
		t.Fatal(err)
	}
	var hits []sqlite.SearchHit
	for i := 0; i < sqlite.MaxCorrelationScopeParts+1; i++ {
		hits = append(hits, sqlite.SearchHit{Instance: query.Instance, QueueID: "Q" + strconv.Itoa(i), At: query.From, Ref: correlation.FactRef{SourceID: "synthetic", OriginID: "origin", Start: int64(i), End: int64(i + 1)}})
	}
	if _, err := searchScope(hits, query); err != sqlite.ErrCorrelationScope {
		t.Fatal("unbounded scope accepted")
	}
}

func TestSearchHandlerRealHTTPSProtectsMountedRoute(t *testing.T) {
	server := httptest.NewUnstartedServer(nil)
	defer server.Close()
	origin := "https://" + server.Listener.Addr().String()
	guard, token, sessions := searchGuardAtOrigin(t, origin)
	var calls atomic.Int32
	reader := &stubSearchReader{search: func(ctx context.Context, _ sqlite.SearchQuery) (sqlite.SearchPage, error) {
		if session, ok := auth.SessionFromContext(ctx); !ok || session.Identity.Username != "SyntheticOperator" {
			t.Error("reader lacks resolved session")
		}
		calls.Add(1)
		return sqlite.SearchPage{}, nil
	}}
	h, err := newSearchHandler(guard, reader, DefaultSearchOptions(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	server.Config.Handler = h
	server.StartTLS()
	client := server.Client()
	for _, tc := range []struct {
		method string
		cookie bool
		status int
	}{{"GET", false, 401}, {"GET", true, 200}, {"HEAD", true, 200}} {
		r, err := http.NewRequest(tc.method, origin+searchTestURL(), nil)
		if err != nil {
			t.Fatal(err)
		}
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
			t.Fatal("HTTPS response policy failed")
		}
		if tc.method == "HEAD" && len(body) != 0 {
			t.Fatal("real HEAD returned data")
		}
	}
	if calls.Load() != 2 {
		t.Fatal("unauthenticated HTTPS request reached storage")
	}
	if err := sessions.Revoke(token); err != nil {
		t.Fatal(err)
	}
	r, _ := http.NewRequest("GET", origin+searchTestURL(), nil)
	r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 || calls.Load() != 2 {
		t.Fatal("revoked HTTPS session reached storage")
	}
}
