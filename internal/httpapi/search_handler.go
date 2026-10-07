package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/auth"
	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

const (
	SearchPath             = "/api/v1/messages"
	MaxSearchResponseBytes = 1024 * 1024
)

var ErrSearchSetup = errors.New("invalid search handler configuration")

type SearchOptions struct {
	Timeout       time.Duration
	FactLimit     int
	MaxConcurrent int
}

func DefaultSearchOptions() SearchOptions {
	return SearchOptions{Timeout: 5 * time.Second, FactLimit: 1024, MaxConcurrent: 2}
}

func (o SearchOptions) Validate() error {
	if o.Timeout < time.Second || o.Timeout > 30*time.Second || o.FactLimit < 1 || o.FactLimit > correlation.MaxPartitionFacts || o.MaxConcurrent < 1 || o.MaxConcurrent > 8 {
		return ErrSearchSetup
	}
	return nil
}

type searchReader interface {
	SearchEvents(context.Context, sqlite.SearchQuery) (sqlite.SearchPage, error)
	CorrelationFacts(context.Context, sqlite.CorrelationScope, int) ([]correlation.Fact, error)
}

// NewSearchHandler returns only the protected handler, never a public unguarded
// reader. Share one instance and an already opened store. Login/logout are mounted
// separately. No listener, SQL writes, projection installation or roles are added.
func NewSearchHandler(guard *auth.HTTPHandler, store *sqlite.Store, options SearchOptions) (http.Handler, error) {
	if store == nil {
		return nil, ErrSearchSetup
	}
	return newSearchHandler(guard, store, options, time.Now)
}

func newSearchHandler(guard *auth.HTTPHandler, reader searchReader, options SearchOptions, now func() time.Time) (http.Handler, error) {
	if guard == nil || reader == nil || now == nil || options.Validate() != nil {
		return nil, ErrSearchSetup
	}
	h := &searchHandler{reader: reader, options: options, now: now, slots: make(chan struct{}, options.MaxConcurrent)}
	protected, err := guard.Protect(h)
	if err != nil {
		return nil, ErrSearchSetup
	}
	return protected, nil
}

type searchHandler struct {
	reader  searchReader
	options SearchOptions
	now     func() time.Time
	slots   chan struct{}
}

func (h *searchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.SessionFromContext(r.Context()); !ok {
		searchHTTPError(w, r, http.StatusUnauthorized, "unauthorized")
		return
	}
	if r.URL == nil || r.URL.RawPath != "" || r.URL.Fragment != "" {
		searchHTTPError(w, r, http.StatusBadRequest, "invalid_request")
		return
	}
	if r.URL.Path != SearchPath {
		searchHTTPError(w, r, http.StatusNotFound, "not_found")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		searchHTTPError(w, r, http.StatusMethodNotAllowed, "method_not_allowed")
		return
	}
	// Do not read or merge a body into query parameters. Unknown/chunked bodies
	// are refused even when they would happen to be empty.
	if r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
		searchHTTPError(w, r, http.StatusBadRequest, "invalid_request")
		return
	}
	query, err := ParseSearchRequest(r.URL.RawQuery, h.now())
	if err != nil {
		if err == ErrSearchClock {
			searchHTTPError(w, r, http.StatusServiceUnavailable, "unavailable")
		} else {
			searchHTTPError(w, r, http.StatusBadRequest, "invalid_request")
		}
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		searchHTTPError(w, r, http.StatusTooManyRequests, "busy")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.options.Timeout)
	defer cancel()
	response, err := h.search(ctx, query)
	if err != nil || ctx.Err() != nil {
		if ctx.Err() == nil && (errors.Is(err, correlation.ErrPartitionLimit) || errors.Is(err, sqlite.ErrCorrelationScope)) {
			searchHTTPError(w, r, http.StatusUnprocessableEntity, "search_too_broad")
		} else {
			searchHTTPError(w, r, http.StatusServiceUnavailable, "unavailable")
		}
		return
	}
	var buffer responseBuffer
	if err := json.NewEncoder(&buffer).Encode(response); err != nil || ctx.Err() != nil {
		searchHTTPError(w, r, http.StatusServiceUnavailable, "unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(buffer.Bytes())
	}
}

func (h *searchHandler) search(ctx context.Context, query sqlite.SearchQuery) (SearchResponse, error) {
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	page, err := h.reader.SearchEvents(ctx, query)
	if err != nil {
		return SearchResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	if len(page.Hits) > query.Limit {
		return SearchResponse{}, sqlite.ErrSearchStoredHit
	}
	out := SearchResponse{From: query.From, Until: query.Until, Limit: query.Limit, CoverageUnproven: true, Matches: []SearchMatch{}}
	if page.Next != nil {
		if len(page.Hits) != query.Limit {
			return SearchResponse{}, sqlite.ErrSearchStoredHit
		}
		check := query
		check.After = page.Next
		if check.Validate() != nil {
			return SearchResponse{}, sqlite.ErrSearchStoredHit
		}
		out.NextCursor, err = EncodeSearchCursor(*page.Next)
		if err != nil {
			return SearchResponse{}, err
		}
	}
	if len(page.Hits) == 0 {
		return out, nil
	}
	scope, err := searchScope(page.Hits, query)
	if err != nil {
		return SearchResponse{}, err
	}
	facts, err := h.reader.CorrelationFacts(ctx, scope, h.options.FactLimit)
	if err != nil {
		return SearchResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	out.Matches, err = searchMatches(ctx, page.Hits, facts, h.options.FactLimit)
	if err != nil {
		return SearchResponse{}, err
	}
	return out, nil
}

type responseBuffer struct{ bytes.Buffer }

func (b *responseBuffer) Write(value []byte) (int, error) {
	if len(value) > MaxSearchResponseBytes-b.Len() {
		return 0, errors.New("search response limit")
	}
	return b.Buffer.Write(value)
}

func searchHTTPError(w http.ResponseWriter, r *http.Request, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte(`{"error":"` + code + `"}` + "\n"))
	}
}
