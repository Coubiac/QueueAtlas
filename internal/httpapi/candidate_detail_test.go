package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

// Independent Python struct/base64 vector, not an encoder-generated expectation.
const goldenCandidateID = "AeSZ9dLH4MjDkfyKk-vCYXgwOkTQv9UypDMxYQK6JJ_PACoAEQZzeW50aGV0aWMtcG9zdGZpeEFCQzEyMw"

func goldenCandidateKey() correlation.QueueInstanceKey {
	return correlation.QueueInstanceKey{Revision: goldenSearchRevision, Instance: "synthetic-postfix", QueueID: "ABC123", Generation: 42}
}

func TestCandidateIDCanonicalVectorBoundsAndPrivateFailures(t *testing.T) {
	key := goldenCandidateKey()
	encoded, err := EncodeCandidateID(key)
	if err != nil || encoded != goldenCandidateID {
		t.Fatal("identifier differs from independent vector", err)
	}
	if decoded, err := decodeCandidateID(goldenCandidateID); err != nil || decoded != key {
		t.Fatal("independent identifier decoded incorrectly", err)
	}
	for _, ordinal := range []int{0, correlation.MaxPartitionFacts - 1} {
		max := key
		max.Instance = strings.Repeat("é", 512)
		max.QueueID = strings.Repeat("Q", 32)
		max.Generation = ordinal
		id, err := EncodeCandidateID(max)
		if err != nil || len(id) != MaxCandidateIDChars {
			t.Fatal("maximum wire bounds refused", err)
		}
		if got, err := decodeCandidateID(id); err != nil || got != max {
			t.Fatal("UTF-8 byte lengths or ordinal changed", err)
		}
	}
	for _, change := range []func(*correlation.QueueInstanceKey){
		func(k *correlation.QueueInstanceKey) { k.Revision = "" }, func(k *correlation.QueueInstanceKey) { k.Revision = strings.ToUpper(k.Revision) }, func(k *correlation.QueueInstanceKey) { k.Revision = strings.Repeat("z", 64) },
		func(k *correlation.QueueInstanceKey) { k.Instance = "" }, func(k *correlation.QueueInstanceKey) { k.Instance = strings.Repeat("i", 1025) }, func(k *correlation.QueueInstanceKey) { k.Instance = "\xff" }, func(k *correlation.QueueInstanceKey) { k.Instance = "synthetic\nprivate" },
		func(k *correlation.QueueInstanceKey) { k.QueueID = "" }, func(k *correlation.QueueInstanceKey) { k.QueueID = strings.Repeat("Q", 33) }, func(k *correlation.QueueInstanceKey) { k.QueueID = "Q\x00" }, func(k *correlation.QueueInstanceKey) { k.Generation = -1 }, func(k *correlation.QueueInstanceKey) { k.Generation = 4096 },
	} {
		bad := key
		change(&bad)
		if id, err := EncodeCandidateID(bad); id != "" || err != ErrCandidateID {
			t.Fatal("invalid key returned identifier or non-private error", err)
		}
	}
	invalid := []string{"", goldenCandidateID + "==", goldenCandidateID[:len(goldenCandidateID)-1] + "x", goldenCandidateID + "\n", goldenCandidateID[1:], "!" + goldenCandidateID[1:], strings.Repeat("A", MaxCandidateIDChars+1)}
	for _, change := range []func([]byte){
		func(b []byte) { b[0] = 2 }, func(b []byte) { binary.BigEndian.PutUint16(b[33:35], 4096) }, func(b []byte) { binary.BigEndian.PutUint16(b[35:37], 0) },
		func(b []byte) { binary.BigEndian.PutUint16(b[35:37], 1025) }, func(b []byte) { b[37] = 0 }, func(b []byte) { b[37] = 33 }, func(b []byte) { b[38] = 0xff }, func(b []byte) { b[38] = 0 },
	} {
		wire, _ := base64.RawURLEncoding.DecodeString(goldenCandidateID)
		change(wire)
		invalid = append(invalid, base64.RawURLEncoding.EncodeToString(wire))
	}
	for _, id := range invalid {
		if got, err := decodeCandidateID(id); got != (correlation.QueueInstanceKey{}) || err != ErrCandidateID {
			t.Fatal("bad identifier returned key or wrong fixed error", err)
		}
	}
}

func readDetailResponse(t *testing.T, h http.Handler, id, token string) DetailResponse {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+id, token))
	assertSearchHTTP(t, w, 200)
	var response DetailResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private raw", "private reply", "private.log", "PasswordHash", "Fields", "synthetic@example.test"} {
		if strings.Contains(w.Body.String(), private) {
			t.Fatal("detail exposed non-DTO observation or query")
		}
	}
	return response
}

func TestCandidateDetailSQLiteCompleteScopeCyclesAndStaleRefusal(t *testing.T) {
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
	search := decodeSearchResponse(t, w)
	candidate := search.Matches[0].Candidate
	if candidate == nil || candidate.ID == "" {
		t.Fatal("search did not expose detail identifier")
	}
	detail := readDetailResponse(t, h, candidate.ID, token)
	if detail.Candidate.ID != candidate.ID || detail.Candidate.Revision != candidate.Revision || detail.Candidate.Counts.Sent != 1 || detail.Candidate.Counts.Delivered != 0 ||
		!detail.CoverageUnproven || !detail.ReceiptObserved || detail.Removed == nil || !detail.CrossStreamUncertain || len(detail.Recipients) != 1 || detail.Recipients[0].AttemptCount != 1 || detail.Recipients[0].ObservedStatus != correlation.DeliverySent {
		t.Fatal("full scope detail lost later removal/delivery or merged origins")
	}
	if detail.First.SourceID != "synthetic-first" || detail.Recipients[0].Address != (NativeValue{Encoding: "utf8", Value: "recipient@example.test"}) {
		t.Fatal("wrong candidate or native recipient")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("HEAD", SearchPath+"/"+candidate.ID, token))
	assertSearchHTTP(t, w, 200)
	if w.Body.Len() != 0 {
		t.Fatal("successful detail HEAD returned data")
	}
	// Change only the ordinal under the same valid full-queue revision: select
	// the other origin's candidate, never merge it with the first one.
	key, err := decodeCandidateID(candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	key.Generation = 1
	secondID, err := EncodeCandidateID(key)
	if err != nil {
		t.Fatal(err)
	}
	second := readDetailResponse(t, h, secondID, token)
	if second.First.SourceID != "synthetic-second" || second.Candidate.Counts.Sent != 1 {
		t.Fatal("ordinal incorrectly merged independent cycles")
	}
	key.Generation = 2
	missingID, _ := EncodeCandidateID(key)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+missingID, token))
	assertSearchHTTP(t, w, 404)
	options := DefaultSearchOptions()
	options.FactLimit = 2
	narrow, err := NewSearchHandler(guard, s, options)
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	narrow.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+candidate.ID, token))
	assertSearchHTTP(t, w, 422)
	if w.Body.String() != "{\"error\":\"detail_too_broad\"}\n" {
		t.Fatal("partial detail or storage error exposed")
	}
	storeSearchCycle(t, s, "synthetic-late", "synthetic-postfix", "ABC123", true)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+candidate.ID, token))
	assertSearchHTTP(t, w, 409)
	if w.Body.String() != "{\"error\":\"stale_candidate\"}\n" {
		t.Fatal("old revision silently reused or replacement disclosed")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	fresh := decodeSearchResponse(t, w)
	if fresh.Matches[0].Candidate.ID == candidate.ID {
		t.Fatal("late import did not revise identifier")
	}
}

func TestCandidateDetailProtocolBeforeReadPrivateErrorsAndHead(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	var calls atomic.Int32
	reader := &stubSearchReader{facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		calls.Add(1)
		return nil, nil
	}, search: func(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error) {
		t.Fatal("detail performed a search")
		return sqlite.SearchPage{}, nil
	}}
	h, err := newSearchHandler(guard, reader, DefaultSearchOptions(), func() time.Time { panic("detail should not consult search clock") })
	if err != nil {
		t.Fatal(err)
	}
	path := SearchPath + "/" + goldenCandidateID
	for _, tc := range []struct {
		change func(*http.Request)
		status int
	}{
		{func(r *http.Request) { r.Header.Del("Cookie") }, 401}, {func(r *http.Request) { r.Header.Set("Origin", "https://foreign.example.test") }, 403},
		{func(r *http.Request) { r.URL.Path = SearchPath + "/invalid" }, 400}, {func(r *http.Request) { r.URL.RawQuery = "password=synthetic-private" }, 400},
		{func(r *http.Request) { r.URL.ForceQuery = true }, 400}, {func(r *http.Request) { r.URL.Path += "/events" }, 404},
		{func(r *http.Request) { r.Method = "POST"; r.Header.Set("Origin", searchTestOrigin) }, 405},
		{func(r *http.Request) { r.ContentLength = 1; r.Body = panicSearchBody{} }, 400},
	} {
		w := httptest.NewRecorder()
		r := searchReadRequest("GET", path, token)
		tc.change(r)
		h.ServeHTTP(w, r)
		assertSearchHTTP(t, w, tc.status)
		if calls.Load() != 0 {
			t.Fatal("bad detail reached facts")
		}
	}
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest(method, path, token))
		assertSearchHTTP(t, w, 404)
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD error body")
		}
	}
	reader.facts = func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) {
		return nil, errors.New("synthetic-private SQL file account")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", path, token))
	assertSearchHTTP(t, w, 503)
	if w.Body.String() != "{\"error\":\"unavailable\"}\n" {
		t.Fatal("detail error exposed stored data")
	}
}

func TestCandidateDetailLiteralRecipientsConflictAndDeadline(t *testing.T) {
	guard, token, _ := searchGuardFixture(t)
	at := time.Unix(0, goldenSearchFromNS).UTC()
	var facts []correlation.Fact
	observations := []model.Observation{{Kind: model.KindMessage, Service: "cleanup", QueueID: "ABC123", Fields: map[string]string{"message-id": "synthetic@example.test"}, Present: map[string]bool{"message-id": true}}}
	for _, address := range []string{"\xffsynthetic", "", "Case@example.test", "case@example.test"} {
		for _, status := range []string{"sent", "bounced"} {
			observations = append(observations, model.Observation{Kind: model.KindDelivery, Service: "smtp", QueueID: "ABC123", Message: "to=<synthetic>, status=" + status + " (private reply)", Fields: map[string]string{"to": address, "status": status}, Present: map[string]bool{"to": true, "status": true}})
		}
	}
	for i, o := range observations {
		o.SourceID = "synthetic"
		o.Timestamp = model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset}
		facts = append(facts, correlation.Fact{Ref: correlation.FactRef{SourceID: "synthetic", OriginID: "origin", Start: int64(i), End: int64(i + 1)}, Instance: "synthetic-postfix", Observation: o})
	}
	projection, err := buildQueueProjection(context.Background(), facts, 1024)
	if err != nil || len(projection.Queues) != 1 {
		t.Fatal("synthetic recipient projection", err)
	}
	id, err := EncodeCandidateID(projection.Queues[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	reader := &stubSearchReader{facts: func(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error) { return facts, nil }}
	h, err := newSearchHandler(guard, reader, DefaultSearchOptions(), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	detail := readDetailResponse(t, h, id, token)
	if len(detail.Recipients) != 4 || detail.Candidate.Counts.Unknown != 4 || detail.Candidate.Counts.Sent != 0 || detail.Candidate.Counts.Bounced != 0 {
		t.Fatal("conflict or exact recipient bytes merged/promoted")
	}
	var empty, binaryAddress bool
	for _, recipient := range detail.Recipients {
		if recipient.ObservedStatus != correlation.DeliveryUnknown || !recipient.OrderUncertain || recipient.AttemptCount != 2 || len(recipient.Latest) != 2 {
			t.Fatal("latest conflict lost evidence")
		}
		if recipient.Address.Encoding == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(recipient.Address.Value)
			if err != nil || string(decoded) != "\xffsynthetic" {
				t.Fatal("invalid UTF-8 address replaced")
			}
			binaryAddress = true
		}
		if recipient.Address == (NativeValue{Encoding: "utf8", Value: ""}) {
			empty = recipient.AddressUnspecified
		}
	}
	if !empty || !binaryAddress {
		t.Fatal("literal empty/binary address not preserved")
	}
	reader.facts = func(ctx context.Context, _ sqlite.CorrelationScope, _ int) ([]correlation.Fact, error) {
		<-ctx.Done()
		return facts, nil
	}
	options := DefaultSearchOptions()
	options.Timeout = time.Second
	limited, err := newSearchHandler(guard, reader, options, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	limited.ServeHTTP(w, searchReadRequest("GET", SearchPath+"/"+id, token))
	assertSearchHTTP(t, w, 503)
	if strings.Contains(w.Body.String(), "recipients") {
		t.Fatal("detail returned facts after deadline")
	}
}
