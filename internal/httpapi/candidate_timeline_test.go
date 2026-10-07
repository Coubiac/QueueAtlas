package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

// Independent Python hashlib/struct/base64 vector, candidate golden, raw=false,
// next index 42. No encoder call generates this expected value.
const goldenTimelineCursor = "AcIvGJguIptFgzHrNM-9Ihx8_uBRITeAx4QUl0K7yMSVACo"

func TestTimelineQueryCanonicalCursorBindingsAndPrivateFailures(t *testing.T) {
	encoded, err := encodeTimelineCursor(goldenCandidateID, false, 42)
	if err != nil || encoded != goldenTimelineCursor {
		t.Fatal("cursor differs from independent vector", err)
	}
	q, err := parseTimelineRequest("limit=200&raw=0&cursor="+goldenTimelineCursor, goldenCandidateID)
	if err != nil || q != (timelineQuery{Limit: 200, Start: 42}) {
		t.Fatal("independent vector or limit change refused", err)
	}
	for _, query := range []string{"", "raw=0", "raw=1&limit=1"} {
		q, err := parseTimelineRequest(query, goldenCandidateID)
		if err != nil || q.Start != 0 || q.Raw != strings.Contains(query, "raw=1") {
			t.Fatal("defaults or raw mode changed", err)
		}
	}
	invalid := []string{"limit=", "limit=0", "limit=201", "limit=-1", "limit=01", "limit=%2B1", "limit=1&limit=2", "raw=", "raw=true", "raw=01", "raw=0&raw=1", "from=x", "cursor=", "cursor=%", "raw=0;limit=1", "&raw=0", "raw=0&", "raw=0&&limit=1", strings.Repeat("x", MaxSearchQueryBytes+1), "\xff=x", "cursor=" + goldenTimelineCursor + "=", "cursor=" + goldenTimelineCursor + "%0A", "cursor=" + goldenTimelineCursor[:46] + "p", "raw=1&cursor=" + goldenTimelineCursor}
	for _, change := range []func([]byte){
		func(b []byte) { b[0] = 2 }, func(b []byte) { b[1] ^= 1 }, func(b []byte) { binary.BigEndian.PutUint16(b[33:], 0) }, func(b []byte) { binary.BigEndian.PutUint16(b[33:], 4096) },
	} {
		wire, _ := base64.RawURLEncoding.DecodeString(goldenTimelineCursor)
		change(wire)
		invalid = append(invalid, "cursor="+base64.RawURLEncoding.EncodeToString(wire))
	}
	for _, query := range invalid {
		if q, err := parseTimelineRequest(query, goldenCandidateID); q != (timelineQuery{}) || err != ErrTimelineRequest {
			t.Fatal("bad query returned data or non-private error", err)
		}
	}
	for _, change := range []func(*correlation.QueueInstanceKey){
		func(k *correlation.QueueInstanceKey) { k.Generation++ }, func(k *correlation.QueueInstanceKey) { k.Instance = "other" }, func(k *correlation.QueueInstanceKey) { k.QueueID = "OTHER" }, func(k *correlation.QueueInstanceKey) { k.Revision = strings.Repeat("0", 64) },
	} {
		key := goldenCandidateKey()
		change(&key)
		id, _ := EncodeCandidateID(key)
		if _, err := parseTimelineRequest("cursor="+goldenTimelineCursor, id); err != ErrTimelineRequest {
			t.Fatal("cursor crossed candidate scope or revision")
		}
	}
	for _, position := range []int{0, 4096, -1} {
		if cursor, err := encodeTimelineCursor(goldenCandidateID, false, position); cursor != "" || err != ErrTimelineRequest {
			t.Fatal("invalid cursor position accepted")
		}
	}
	for _, position := range []int{1, 4095} {
		cursor, err := encodeTimelineCursor(goldenCandidateID, true, position)
		if err != nil {
			t.Fatal(err)
		}
		q, err := parseTimelineRequest("raw=1&cursor="+cursor, goldenCandidateID)
		if err != nil || q.Start != position || !q.Raw {
			t.Fatal("valid cursor bounds/raw mode refused", err)
		}
	}
}

func readTimelineResponse(t *testing.T, h http.Handler, id, query, token string) TimelineResponse {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+id+"/events?"+query, token))
	assertSearchHTTP(t, w, 200)
	var out TimelineResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.CandidateID != id || !out.CoverageUnproven || out.Ordering != "timestamp_then_provenance" || out.Events == nil {
		t.Fatal("timeline identity, reserves or arrays changed")
	}
	for _, private := range []string{"private.log", "PasswordHash", "Fields", "synthetic-arbitrary-field"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("timeline serialized an unselected observation field")
		}
	}
	if !out.RawIncluded && strings.Contains(w.Body.String(), `"raw":`) {
		t.Fatal("raw record included without explicit request")
	}
	return out
}

func TestTimelineSQLitePaginationCompleteScopeAndStaleRefusal(t *testing.T) {
	s := syntheticSearchStore(t)
	storeSearchCycle(t, s, "synthetic-first", "synthetic-postfix", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-second", "synthetic-postfix", "ABC123", true)
	storeSearchCycle(t, s, "synthetic-undated", "synthetic-postfix", "ABC123", false)
	storeSearchCycle(t, s, "synthetic-foreign", "other-instance", "ABC123", true)
	guard, token, _ := searchGuardFixture(t)
	h, err := NewSearchHandler(guard, s, DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	id := decodeSearchResponse(t, w).Matches[0].Candidate.ID
	first := readTimelineResponse(t, h, id, "limit=1", token)
	if len(first.Events) != 1 || first.Events[0].Kind != model.KindMessage || first.NextCursor == "" || first.RawIncluded || !first.CrossStreamUncertain || first.Events[0].Ref.SourceID != "synthetic-first" {
		t.Fatal("timeline first page includes another cycle or origin")
	}
	rest := readTimelineResponse(t, h, id, "limit=200&cursor="+url.QueryEscape(first.NextCursor), token)
	if len(rest.Events) != 2 || rest.NextCursor != "" || rest.Events[0].Delivery == nil || rest.Events[0].Delivery.ObservedStatus != correlation.DeliverySent || rest.Events[0].Delivery.Scope != correlation.ScopeSMTPPeer || rest.Events[1].Kind != model.KindRemoved || !rest.Events[1].At.After(time.Unix(0, goldenSearchFromNS)) {
		t.Fatal("timeline page omitted full-scope facts outside search window")
	}
	for _, e := range rest.Events {
		if e.Ref.SourceID != "synthetic-first" || e.Ref.Start < first.Events[0].Ref.End {
			t.Fatal("pagination duplicated or crossed selected generation")
		}
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("HEAD", SearchPath+"/"+id+"/events?limit=1", token))
	assertSearchHTTP(t, w, 200)
	if w.Body.Len() != 0 {
		t.Fatal("timeline HEAD body")
	}
	options := DefaultSearchOptions()
	options.FactLimit = 2
	small, _ := NewSearchHandler(guard, s, options)
	w = httptest.NewRecorder()
	small.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+id+"/events?limit=1", token))
	assertSearchHTTP(t, w, 422)
	if w.Body.String() != "{\"error\":\"timeline_too_broad\"}\n" {
		t.Fatal("a small page bypassed full-queue budget")
	}
	outside, _ := encodeTimelineCursor(id, false, 4095)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+id+"/events?cursor="+outside, token))
	assertSearchHTTP(t, w, 400)
	storeSearchCycle(t, s, "synthetic-late", "synthetic-postfix", "ABC123", true)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+id+"/events?cursor="+first.NextCursor, token))
	assertSearchHTTP(t, w, 409)
	for _, method := range []string{"GET", "HEAD"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest(method, SearchPath+"/"+id+"/events?cursor="+outside, token))
		assertSearchHTTP(t, w, 409)
		if method == "HEAD" && w.Body.Len() != 0 || method == "GET" && w.Body.String() != "{\"error\":\"stale_candidate\"}\n" {
			t.Fatal("stale revision applied position or exposed replacement")
		}
	}
}

func TestTimelineProtocolAndRawPolicyBeforeStorage(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	var calls atomic.Int32
	reader := &stubSearchReader{facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		calls.Add(1)
		return nil, nil
	}}
	h, _ := newSearchHandler(guard, reader, DefaultSearchOptions(), time.Now)
	path := SearchPath + "/" + goldenCandidateID + "/events"
	for _, tc := range []struct {
		change func(*http.Request)
		status int
	}{
		{func(r *http.Request) { r.Header.Del("Cookie") }, 401}, {func(r *http.Request) { r.Header.Set("Origin", "https://foreign.example.test") }, 403},
		{func(r *http.Request) { r.URL.RawQuery = "raw=1" }, 403}, {func(r *http.Request) { r.URL.RawQuery = "limit=0" }, 400}, {func(r *http.Request) { r.URL.RawQuery = "cursor=" + goldenTimelineCursor + "&raw=1" }, 400},
		{func(r *http.Request) { r.ContentLength = 1; r.Body = panicSearchBody{} }, 400}, {func(r *http.Request) { r.ContentLength = -1; r.TransferEncoding = []string{"chunked"} }, 400},
		{func(r *http.Request) { r.URL.Path += "/" }, 404}, {func(r *http.Request) { r.Method = "POST"; r.Header.Set("Origin", searchTestOrigin) }, 405},
	} {
		w := httptest.NewRecorder()
		r := searchReadRequest("GET", path, token)
		tc.change(r)
		h.ServeHTTP(w, r)
		assertSearchHTTP(t, w, tc.status)
		if calls.Load() != 0 {
			t.Fatal("unauthorized/malformed/raw forbidden request reached storage")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchHTTP(t, w, 404)
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		return nil, errors.New("synthetic private path/password")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchHTTP(t, w, 503)
	if w.Body.String() != "{\"error\":\"unavailable\"}\n" {
		t.Fatal("storage error exposed data")
	}
}

func TestTimelineRawSQLiteBytesAndExplicitOptIn(t *testing.T) {
	s := syntheticSearchStore(t)
	at := time.Unix(0, goldenSearchFromNS).UTC()
	raw := []byte("synthetic\xff\x00<script>\r\n")
	o := model.Observation{SourceID: "synthetic-binary", QueueID: "ABC123", Kind: model.KindMessage, Service: "cleanup", Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}, Fields: map[string]string{"message-id": "synthetic@example.test"}, Present: map[string]bool{"message-id": true}}
	batch := source.Batch{Source: source.Identity{ID: o.SourceID, Kind: "file", Name: "synthetic", TrustedHost: "synthetic-postfix"}, Origins: []source.Origin{{ID: "synthetic-binary-origin", Path: "/synthetic/private.log", Fingerprint: "synthetic", FirstSeen: at}}, Records: []source.Record{{OriginID: "synthetic-binary-origin", Start: 0, End: int64(len(raw)), Raw: raw, ReadAt: at, Observation: o}}}
	if err := s.Commit(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	facts, err := s.CorrelationFacts(context.Background(), sqlite.CorrelationScope{Queues: []correlation.QueueKey{{Instance: "synthetic-postfix", QueueID: "ABC123"}}}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := buildQueueProjection(context.Background(), facts, 1024)
	if err != nil || len(projection.Queues) != 1 {
		t.Fatal("binary raw projection", err)
	}
	id, _ := EncodeCandidateID(projection.Queues[0].Key)
	guard, token, _ := searchGuardFixture(t)
	options := DefaultSearchOptions()
	options.AllowRawLogs = true
	h, _ := NewSearchHandler(guard, s, options)
	metadata := readTimelineResponse(t, h, id, "", token)
	if metadata.Events[0].Raw != nil {
		t.Fatal("server permission alone exposed raw")
	}
	page := readTimelineResponse(t, h, id, "raw=1", token)
	if !page.RawIncluded || page.Events[0].Raw == nil || page.Events[0].Raw.Encoding != "base64" {
		t.Fatal("raw binary record missing")
	}
	decoded, err := base64.StdEncoding.Strict().DecodeString(page.Events[0].Raw.Value)
	if err != nil || string(decoded) != string(raw) {
		t.Fatal("raw bytes changed through SQLite/JSON", err)
	}
}

func timelineSyntheticFacts(t *testing.T) ([]correlation.Fact, string) {
	t.Helper()
	var facts []correlation.Fact
	for i := 0; i < 4; i++ {
		at := time.Unix(0, goldenSearchFromNS).UTC().Add(time.Duration(i) * time.Second)
		if i == 2 {
			at = at.Add(-time.Second)
		}
		o := model.Observation{SourceID: "synthetic", QueueID: "ABC123", Kind: model.KindMessage, Service: "cleanup", Timestamp: model.Timestamp{Value: &at, Quality: model.TimeConfiguredYearAndZone, Raw: "Oct  7 00:00:00"}, Fields: map[string]string{"message-id": "synthetic@example.test", "arbitrary": "synthetic-arbitrary-field"}, Present: map[string]bool{"message-id": true, "arbitrary": true}}
		if i == 1 || i == 2 {
			status := "sent"
			if i == 2 {
				status = "bounced"
			}
			o.Kind, o.Service = model.KindDelivery, "smtp"
			o.Message = "to=<Case@example.test>, status=" + status + " (<script>synthetic reply</script>)"
			o.Fields = map[string]string{"to": "Case@example.test", "status": status, "orig_to": "", "dsn": "2.0.0", "reply": "<script>synthetic reply</script>"}
			o.Present = map[string]bool{"to": true, "status": true, "orig_to": true, "dsn": true, "reply": true}
		}
		facts = append(facts, correlation.Fact{Ref: correlation.FactRef{SourceID: "synthetic", OriginID: "synthetic-origin", Start: int64(i * model.MaxLineBytes), End: int64((i + 1) * model.MaxLineBytes)}, Instance: "synthetic-postfix", Observation: o})
	}
	p, err := buildQueueProjection(context.Background(), facts, 1024)
	if err != nil || len(p.Queues) != 1 {
		t.Fatal("timeline fixture", err)
	}
	id, _ := EncodeCandidateID(p.Queues[0].Key)
	return facts, id
}

func TestTimelineNativeDeliveryTiesRawLimitAndCancellation(t *testing.T) {
	facts, id := timelineSyntheticFacts(t)
	guard, token, _ := searchGuardFixture(t)
	reader := &stubSearchReader{facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return facts, nil }}
	options := DefaultSearchOptions()
	options.AllowRawLogs = true
	h, _ := newSearchHandler(guard, reader, options, time.Now)
	page := readTimelineResponse(t, h, id, "", token)
	a, b := page.Events[1], page.Events[2]
	if !a.At.Equal(b.At) || a.Ref.Start >= b.Ref.Start || a.TimeQuality != model.TimeConfiguredYearAndZone || a.TimestampRaw.Value != "Oct  7 00:00:00" || a.Delivery == nil || b.Delivery == nil || a.Delivery.ObservedStatus != correlation.DeliverySent || b.Delivery.ObservedStatus != correlation.DeliveryBounced || a.Delivery.Scope != correlation.ScopeSMTPPeer || a.Delivery.Recipient.Value != "Case@example.test" || !a.Delivery.OriginalRecipient.Present || a.Delivery.OriginalRecipient.Value.Value != "" || a.Delivery.Relay.Present || a.Delivery.Relay.Value != nil || a.Delivery.DSN.Value.Value != "2.0.0" || a.Delivery.Reply.Value.Value != "<script>synthetic reply</script>" {
		t.Fatal("native fields/presence/ties/status scope changed")
	}
	detail := readDetailResponse(t, h, id, token)
	if detail.Candidate.Counts.Unknown != 1 || !detail.Recipients[0].OrderUncertain || !page.HasNonExplicitTime {
		t.Fatal("display order changed simultaneous conflict")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+id+"/events", token))
	if strings.Contains(w.Body.String(), "<script>") || !strings.Contains(w.Body.String(), `\u003cscript\u003e`) {
		t.Fatal("native log text was not JSON-escaped")
	}
	// Parsed field binary preservation is a DTO seam test, distinct from the
	// real SQLite raw-record byte test above (stored fields already use JSON).
	f, err := timelineEvent(correlation.Fact{Ref: facts[1].Ref, Instance: facts[1].Instance, Observation: func() model.Observation {
		o := facts[1].Observation
		o.Fields = map[string]string{"to": "\xffsynthetic", "status": "sent"}
		o.Present = map[string]bool{"to": true, "status": true}
		return o
	}()}, false)
	if err != nil || f.Delivery == nil || f.Delivery.Recipient != (NativeValue{Encoding: "base64", Value: base64.StdEncoding.EncodeToString([]byte("\xffsynthetic"))}) {
		t.Fatal("native binary DTO replaced bytes", err)
	}
	// Enforce the actual JSON byte cap, including escaping expansion, atomically.
	for i := range facts {
		facts[i].Observation.Raw = strings.Repeat("\x00", model.MaxLineBytes)
	}
	p, err := buildQueueProjection(context.Background(), facts, 1024)
	if err != nil {
		t.Fatal(err)
	}
	id, _ = EncodeCandidateID(p.Queues[0].Key)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+id+"/events?raw=1", token))
	assertSearchHTTP(t, w, 503)
	if w.Body.String() != "{\"error\":\"unavailable\"}\n" {
		t.Fatal("oversized raw response emitted partial events")
	}
	one := readTimelineResponse(t, h, id, "raw=1&limit=1", token)
	if len(one.Events) != 1 || one.Events[0].Raw.Value != facts[0].Observation.Raw || one.NextCursor == "" {
		t.Fatal("bounded single raw page refused")
	}
	reader.facts = func(ctx context.Context, _ sqlite.CorrelationScope, _ int) ([]correlation.Fact, error) {
		<-ctx.Done()
		return facts, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := searchReadRequest("GET", SearchPath+"/"+id+"/events", token).WithContext(ctx)
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assertSearchHTTP(t, w, 503)
	if w.Body.String() != "{\"error\":\"unavailable\"}\n" {
		t.Fatal("canceled read returned late timeline")
	}
}
