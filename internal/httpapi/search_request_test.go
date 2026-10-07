package httpapi

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func searchFixture() (url.Values, time.Time) {
	return url.Values{"instance": {"synthetic-postfix"}, "field": {"sender"}, "value": {"synthetic@example.test"}},
		time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
}

func cloneSearchValues(params url.Values) url.Values {
	copy := make(url.Values, len(params))
	for key, values := range params {
		copy[key] = append([]string(nil), values...)
	}
	return copy
}

func assertSearchRequestFailure(t *testing.T, raw string, now time.Time, want error) {
	t.Helper()
	query, err := ParseSearchRequest(raw, now)
	if !reflect.DeepEqual(query, sqlite.SearchQuery{}) || err != want {
		t.Fatal("failed request returned state or wrong fixed error", err)
	}
}

func TestSearchRequestDefaultsExactValuesAndSupportedFields(t *testing.T) {
	params, now := searchFixture()
	query, err := ParseSearchRequest(params.Encode(), now.In(time.FixedZone("synthetic", 2*3600)))
	if err != nil || query.Instance != "synthetic-postfix" || query.Field != sqlite.SearchSender || query.Value != "synthetic@example.test" ||
		query.Limit != 50 || query.After != nil || !query.Until.Equal(now) || query.Until.Location() != time.UTC || !query.From.Equal(now.Add(-24*time.Hour)) {
		t.Fatal("incorrect default selector", err)
	}
	for _, field := range []string{"sender", "recipient", "queue_id", "message_id", "sender_domain", "recipient_domain"} {
		params.Set("field", field)
		value := " synthetic + % _ ' literal "
		if strings.HasSuffix(field, "domain") {
			value = "SYNTHETIC.Example.test"
		}
		params.Set("value", value)
		params.Set("limit", "200")
		query, err := ParseSearchRequest(params.Encode(), now)
		if err != nil || string(query.Field) != field || query.Value != value || query.Limit != sqlite.MaxSearchResults {
			t.Fatal("criterion changed or unsupported", err)
		}
	}
	for _, field := range []string{"sender", "recipient"} {
		params.Set("field", field)
		params.Set("value", "")
		if query, err := ParseSearchRequest(params.Encode(), now); err != nil || query.Value != "" {
			t.Fatal("explicit empty native address refused", err)
		}
	}
	params.Set("instance", strings.Repeat("%", 1024))
	params.Set("value", strings.Repeat("%", 1024))
	if len(params.Encode()) > MaxSearchQueryBytes {
		t.Fatal("query byte limit cannot represent maximum encoded native inputs")
	}
	if query, err := ParseSearchRequest(params.Encode(), now); err != nil || len(query.Instance) != 1024 || len(query.Value) != 1024 {
		t.Fatal("maximum native byte bounds refused", err)
	}
}

func TestSearchRequestRejectsAmbiguousHostileAndOversizedInputs(t *testing.T) {
	params, now := searchFixture()
	base := params.Encode()
	for _, raw := range []string{"", "?" + base, "&" + base, base + "&", base + "&&limit=50", base + "&instance=other", base + "&%69nstance=other",
		base + "&sort=raw_column", base + "&offset=1", base + "&status=sent", base + "&password=synthetic-private",
		base + "&extra=%ZZ", base + ";limit=50", base + "&LIMIT=50", base + "&limit=50&limit=50",
		strings.Repeat("x", MaxSearchQueryBytes), strings.Repeat("x", MaxSearchQueryBytes+1), base + "\xff"} {
		assertSearchRequestFailure(t, raw, now, ErrSearchRequest)
	}
	for _, change := range []func(url.Values){
		func(v url.Values) { v.Del("instance") }, func(v url.Values) { v.Del("field") }, func(v url.Values) { v.Del("value") },
		func(v url.Values) { v.Set("instance", "") }, func(v url.Values) { v.Set("instance", strings.Repeat("i", 1025)) },
		func(v url.Values) { v.Set("instance", "synthetic\nprivate") }, func(v url.Values) { v.Set("field", "sender OR 1=1") },
		func(v url.Values) { v.Set("field", "Sender") }, func(v url.Values) { v.Set("value", "\xffsynthetic") },
		func(v url.Values) { v.Set("instance", "\xffsynthetic") }, func(v url.Values) { v.Set("value", "synthetic\x00private") },
		func(v url.Values) { v.Set("value", "synthetic\r\nprivate") }, func(v url.Values) { v.Set("value", "synthetic\tprivate") },
		func(v url.Values) { v.Set("value", strings.Repeat("v", 1025)) },
		func(v url.Values) { v.Set("field", "queue_id"); v.Set("value", "") },
		func(v url.Values) { v.Set("field", "queue_id"); v.Set("value", strings.Repeat("q", 33)) },
		func(v url.Values) { v.Set("field", "message_id"); v.Set("value", "") },
		func(v url.Values) { v.Set("field", "sender_domain"); v.Set("value", "synthetic.test.") },
		func(v url.Values) { v.Set("field", "recipient_domain"); v.Set("value", "éxample.test") },
	} {
		v := cloneSearchValues(params)
		change(v)
		assertSearchRequestFailure(t, v.Encode(), now, ErrSearchRequest)
	}
	for _, limit := range []string{"", "0", "201", "-1", "+1", "01", "1.0", "1e2", " 50", strings.Repeat("9", 100)} {
		v := cloneSearchValues(params)
		v.Set("limit", limit)
		assertSearchRequestFailure(t, v.Encode(), now, ErrSearchRequest)
	}
}

func TestSearchRequestTimeWindowPrecisionAndClock(t *testing.T) {
	params, now := searchFixture()
	for _, date := range []string{"", "2026-10-07", "2026-10-07 00:00:00Z", "2026-10-07T0:00:00Z", "2026-10-07T00:00:00z",
		"2026-10-07T00:00:00+00:00", "2026-10-07T00:00:00.1234567890Z", "2026-10-07T00:00:00,1Z", "2026-02-30T00:00:00Z",
		"2026-10-07T00:00:60Z", "2500-10-07T00:00:00Z", "0001-01-01T00:00:00Z"} {
		v := cloneSearchValues(params)
		v.Set("from", date)
		v.Set("until", "2026-10-08T00:00:00Z")
		assertSearchRequestFailure(t, v.Encode(), now, ErrSearchRequest)
	}
	for _, key := range []string{"from", "until"} {
		v := cloneSearchValues(params)
		v.Set(key, "2026-10-07T00:00:00Z")
		assertSearchRequestFailure(t, v.Encode(), now, ErrSearchRequest)
	}
	params.Set("from", "2026-10-07T00:00:00.123456789Z")
	params.Set("until", "2026-11-07T00:00:00.123456789Z")
	query, err := ParseSearchRequest(params.Encode(), time.Time{})
	if err != nil || query.From.Nanosecond() != 123456789 || query.Until.Sub(query.From) != sqlite.MaxSearchWindow {
		t.Fatal("explicit nanosecond/date boundary depends on clock or truncated", err)
	}
	for _, until := range []string{"2026-10-07T00:00:00.123456789Z", "2026-10-06T00:00:00Z", "2026-11-07T00:00:00.123456790Z"} {
		params.Set("until", until)
		assertSearchRequestFailure(t, params.Encode(), now, ErrSearchRequest)
	}
	params.Del("from")
	params.Del("until")
	for _, clock := range []time.Time{time.Time{}, time.Date(2500, 1, 1, 0, 0, 0, 0, time.UTC), time.Unix(0, -1<<63)} {
		assertSearchRequestFailure(t, params.Encode(), clock, ErrSearchClock)
	}
}
