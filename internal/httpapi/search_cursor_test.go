package httpapi

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

// Independent Python hashlib/struct/base64 vector: length-prefixed UTF-8 scope
// event-search-v1, synthetic-postfix, sender, synthetic@example.test, From, Until.
const goldenSearchRevision = "e499f5d2c7e0c8c391fc8a93ebc26178303a44d0bfd532a433316102ba249fcf"
const goldenSearchWire = "AeSZ9dLH4MjDkfyKk-vCYXgwOkTQv9UypDMxYQK6JJ_PGNwWPIatygAAAAAAAAAAKg"
const goldenSearchFromNS int64 = 1791331200000000000

func goldenSearchCursor() sqlite.SearchCursor {
	return sqlite.SearchCursor{QueryRevision: goldenSearchRevision, TimeNS: goldenSearchFromNS + int64(time.Second), RowID: 42}
}

func TestSearchCursorCanonicalCodecAndPrivateFailures(t *testing.T) {
	cursor := goldenSearchCursor()
	wire, err := EncodeSearchCursor(cursor)
	if err != nil || wire != goldenSearchWire || len(wire) != SearchCursorChars {
		t.Fatal("wire differs from independent vector", err)
	}
	decoded, err := decodeSearchCursor(goldenSearchWire)
	if err != nil || decoded != cursor {
		t.Fatal("independent vector decoded incorrectly", err)
	}
	for _, instant := range []int64{-1 << 63, -1, 0, 1<<63 - 1} {
		position := cursor
		position.TimeNS, position.RowID = instant, 1<<63-1
		wire, err := EncodeSearchCursor(position)
		if err != nil {
			t.Fatal("signed instant or positive row refused", err)
		}
		if decoded, err := decodeSearchCursor(wire); err != nil || decoded != position {
			t.Fatal("signed wire boundary lost", err)
		}
	}
	for _, change := range []func(*sqlite.SearchCursor){
		func(c *sqlite.SearchCursor) { c.QueryRevision = "" },
		func(c *sqlite.SearchCursor) { c.QueryRevision = strings.Repeat("f", 63) },
		func(c *sqlite.SearchCursor) { c.QueryRevision = strings.Repeat("z", 64) },
		func(c *sqlite.SearchCursor) { c.QueryRevision = strings.ToUpper(c.QueryRevision) },
		func(c *sqlite.SearchCursor) { c.RowID = 0 },
		func(c *sqlite.SearchCursor) { c.RowID = -1 },
	} {
		bad := cursor
		change(&bad)
		if wire, err := EncodeSearchCursor(bad); wire != "" || err != ErrSearchCursor {
			t.Fatal("invalid position produced wire or non-private error", err)
		}
	}
	invalid := []string{"", goldenSearchWire[:65], goldenSearchWire + "g", goldenSearchWire + "==",
		" " + goldenSearchWire[1:], "+" + goldenSearchWire[1:], "!" + goldenSearchWire[1:],
		goldenSearchWire[:65] + "h", goldenSearchWire[:32] + "\r\n" + goldenSearchWire[32:]}
	for _, change := range []func([]byte){
		func(b []byte) { b[0] = 0 }, func(b []byte) { b[0] = 2 },
		func(b []byte) { binary.BigEndian.PutUint64(b[41:49], 0) },
		func(b []byte) { binary.BigEndian.PutUint64(b[41:49], 1<<63) },
	} {
		bytes, _ := base64.RawURLEncoding.DecodeString(goldenSearchWire)
		change(bytes)
		invalid = append(invalid, base64.RawURLEncoding.EncodeToString(bytes))
	}
	for _, bad := range invalid {
		if decoded, err := decodeSearchCursor(bad); decoded != (sqlite.SearchCursor{}) || err != ErrSearchCursor {
			t.Fatal("invalid wire returned state or wrong fixed error", err)
		}
	}
}

func TestSearchRequestCursorBindsSelectorAndFrozenWindow(t *testing.T) {
	params, now := searchFixture()
	params.Set("from", "2026-10-07T00:00:00Z")
	params.Set("until", "2026-10-08T00:00:00Z")
	params.Set("cursor", goldenSearchWire)
	for _, clock := range []time.Time{time.Time{}, now.Add(24 * time.Hour)} {
		query, err := ParseSearchRequest(params.Encode(), clock)
		if err != nil || query.After == nil || *query.After != goldenSearchCursor() {
			t.Fatal("cursor rejected or moving clock changed page", err)
		}
		query.After.RowID = 99 // A caller cannot mutate the next parsing result.
	}
	for _, limit := range []string{"1", "200"} {
		v := cloneSearchValues(params)
		v.Set("limit", limit)
		if _, err := ParseSearchRequest(v.Encode(), now); err != nil {
			t.Fatal("page size incorrectly bound to cursor", err)
		}
	}
	for _, change := range []struct{ key, value string }{
		{"instance", "another-synthetic-postfix"}, {"field", "recipient"}, {"value", "another@example.test"},
		{"from", "2026-10-06T23:59:59Z"}, {"until", "2026-10-08T00:00:01Z"}, {"cursor", ""},
	} {
		v := cloneSearchValues(params)
		v.Set(change.key, change.value)
		assertSearchRequestFailure(t, v.Encode(), now, ErrSearchCursor)
	}
	v := cloneSearchValues(params)
	v.Del("from")
	v.Del("until")
	assertSearchRequestFailure(t, v.Encode(), now, ErrSearchRequest)
	untilNS := now.UnixNano()
	for _, instant := range []int64{goldenSearchFromNS - 1, untilNS, goldenSearchFromNS, untilNS - 1} {
		cursor := goldenSearchCursor()
		cursor.TimeNS = instant
		wire, err := EncodeSearchCursor(cursor)
		if err != nil {
			t.Fatal("valid codec position refused", err)
		}
		params.Set("cursor", wire)
		if instant < goldenSearchFromNS || instant >= untilNS {
			assertSearchRequestFailure(t, params.Encode(), now, ErrSearchCursor)
		} else if query, err := ParseSearchRequest(params.Encode(), now); err != nil || *query.After != cursor {
			t.Fatal("inclusive/exclusive cursor window changed", err)
		}
	}
	// Independently calculated scope uses normalized domain bytes, while the
	// request result preserves the user's literal value for the caller.
	domain := goldenSearchCursor()
	domain.QueryRevision = "0e4ede0afcf53423fc69b41056d30562b013e859b726a59597571942391e4d09"
	wire, err := EncodeSearchCursor(domain)
	if err != nil {
		t.Fatal("domain cursor encoding failed", err)
	}
	params.Set("cursor", wire)
	params.Set("field", "sender_domain")
	for _, value := range []string{"synthetic.example.test", "SYNTHETIC.Example.test"} {
		params.Set("value", value)
		if query, err := ParseSearchRequest(params.Encode(), now); err != nil || query.Value != value || *query.After != domain {
			t.Fatal("canonical domain cursor scope disagrees with native validator", err)
		}
	}
}
