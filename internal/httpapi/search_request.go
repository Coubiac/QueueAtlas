// Package httpapi defines bounded HTTP inputs for the future authenticated API.
// It creates no listener, handlers, database connections or application routes.
package httpapi

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

const (
	MaxSearchQueryBytes = 8192
	DefaultSearchLimit  = 50
	DefaultSearchWindow = 24 * time.Hour
)

var (
	ErrSearchRequest = errors.New("invalid search request")
	ErrSearchCursor  = errors.New("invalid search cursor")
	ErrSearchClock   = errors.New("search clock unavailable")
	searchUTCDate    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?Z$`)
)

// ParseSearchRequest validates URL.RawQuery, not a URL or Request.Form. Supply a
// single trusted clock snapshot for defaults. Pagination requires explicit dates
// from the first page, never moving clock defaults. The result is an event search
// selector, not a message identity or an authenticated/authorized request.
// Values stay literal (domain matching is normalized by SQLite's validator).
// All failures return a zero query and fixed errors, never supplied input.
func ParseSearchRequest(rawQuery string, now time.Time) (sqlite.SearchQuery, error) {
	if len(rawQuery) > MaxSearchQueryBytes || !utf8.ValidString(rawQuery) ||
		strings.HasPrefix(rawQuery, "&") || strings.HasSuffix(rawQuery, "&") || strings.Contains(rawQuery, "&&") {
		return sqlite.SearchQuery{}, ErrSearchRequest
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return sqlite.SearchQuery{}, ErrSearchRequest
	}
	for key, entries := range values {
		switch key {
		case "instance", "field", "value", "from", "until", "limit", "cursor":
		default:
			return sqlite.SearchQuery{}, ErrSearchRequest
		}
		if len(entries) != 1 {
			return sqlite.SearchQuery{}, ErrSearchRequest
		}
	}
	for _, key := range []string{"instance", "field", "value"} {
		if !values.Has(key) || !utf8.ValidString(values.Get(key)) {
			return sqlite.SearchQuery{}, ErrSearchRequest
		}
	}
	query := sqlite.SearchQuery{Instance: values.Get("instance"), Field: sqlite.SearchField(values.Get("field")),
		Value: values.Get("value"), Limit: DefaultSearchLimit}
	if values.Has("limit") {
		query.Limit, err = strconv.Atoi(values.Get("limit"))
		if err != nil || strconv.Itoa(query.Limit) != values.Get("limit") {
			return sqlite.SearchQuery{}, ErrSearchRequest
		}
	}
	if values.Has("from") != values.Has("until") || values.Has("cursor") && !values.Has("from") {
		return sqlite.SearchQuery{}, ErrSearchRequest
	}
	if values.Has("from") {
		var ok bool
		query.From, ok = parseSearchDate(values.Get("from"))
		if !ok {
			return sqlite.SearchQuery{}, ErrSearchRequest
		}
		query.Until, ok = parseSearchDate(values.Get("until"))
		if !ok {
			return sqlite.SearchQuery{}, ErrSearchRequest
		}
	} else {
		query.Until = now.UTC()
		query.From = query.Until.Add(-DefaultSearchWindow)
		if !searchInstantFits(query.From) || !searchInstantFits(query.Until) {
			return sqlite.SearchQuery{}, ErrSearchClock
		}
	}
	if values.Has("cursor") {
		cursor, err := decodeSearchCursor(values.Get("cursor"))
		if err != nil {
			return sqlite.SearchQuery{}, ErrSearchCursor
		}
		query.After = &cursor
	}
	if err := query.Validate(); err != nil {
		if err == sqlite.ErrSearchCursor {
			return sqlite.SearchQuery{}, ErrSearchCursor
		}
		return sqlite.SearchQuery{}, ErrSearchRequest
	}
	return query, nil
}

func parseSearchDate(value string) (time.Time, bool) {
	if len(value) < 20 || len(value) > 30 || !searchUTCDate.MatchString(value) {
		return time.Time{}, false
	}
	instant, err := time.Parse(time.RFC3339Nano, value)
	return instant, err == nil && searchInstantFits(instant)
}

func searchInstantFits(value time.Time) bool {
	return !value.IsZero() && time.Unix(0, value.UnixNano()).Equal(value)
}
