package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func timelinePageURL(id string) string { return CandidatePagePrefix + id + "/events" }

type timelinePageData struct {
	Response   *TimelineResponse
	RawAllowed bool
	NextURL    string
	Error      string
}

func (h *searchHandler) serveTimelinePage(w http.ResponseWriter, r *http.Request, id string) {
	key, err := decodeCandidateID(id)
	if err != nil {
		timelinePageError(w, r, 400)
		return
	}
	query, err := parseTimelineRequest(r.URL.RawQuery, id)
	if err != nil {
		timelinePageError(w, r, 400)
		return
	}
	if query.Raw && !h.options.AllowRawLogs {
		timelinePageError(w, r, 403)
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		timelinePageError(w, r, 429)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.options.Timeout)
	defer cancel()
	response, err := h.timeline(ctx, key, id, query)
	if err != nil || ctx.Err() != nil {
		status := http.StatusServiceUnavailable
		if ctx.Err() == nil {
			switch {
			case errors.Is(err, ErrCandidateStale):
				status = http.StatusConflict
			case errors.Is(err, ErrCandidateNotFound):
				status = http.StatusNotFound
			case errors.Is(err, ErrTimelineRequest):
				status = http.StatusBadRequest
			case errors.Is(err, correlation.ErrPartitionLimit), errors.Is(err, sqlite.ErrCorrelationScope):
				status = http.StatusUnprocessableEntity
			}
		}
		timelinePageError(w, r, status)
		return
	}
	data := timelinePageData{Response: &response, RawAllowed: h.options.AllowRawLogs}
	if response.NextCursor != "" {
		mode := "0"
		if query.Raw {
			mode = "1"
		}
		values := url.Values{"limit": {strconv.Itoa(query.Limit)}, "raw": {mode}, "cursor": {response.NextCursor}}
		data.NextURL = timelinePageURL(id) + "?" + values.Encode()
	}
	if writeHTMLPage(w, r, ctx, 200, "timeline_page.html", data) != nil {
		timelinePageError(w, r, 503)
	}
}

func timelinePageError(w http.ResponseWriter, r *http.Request, status int) {
	message := candidatePageErrorMessage(status)
	if status == 400 {
		message = "Référence ou paramètres de timeline invalides. Relancez la recherche."
	} else if status == 403 {
		message = "La consultation des lignes brutes n’est pas autorisée pour ce serveur."
	} else if status == 503 {
		message = "Timeline indisponible. Réessayez plus tard ou réduisez le nombre de faits par page."
	}
	_ = writeHTMLPage(w, r, context.Background(), status, "timeline_page.html", timelinePageData{Error: message})
}

// Visible notation prevents controls and direction overrides from disguising
// native text in HTML. The DTO/stored bytes are unchanged; html/template still
// escapes the returned ordinary string. Binary values remain labeled base64.
func visibleNativeText(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '\r':
			out.WriteString(`\r`)
		case '\n':
			out.WriteString(`\n`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if unicode.IsControl(r) || r == '\u061c' || r == '\u200e' || r == '\u200f' || r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069' {
				if r < 128 {
					fmt.Fprintf(&out, `\x%02x`, r)
				} else {
					fmt.Fprintf(&out, `\u%04x`, r)
				}
			} else {
				out.WriteRune(r)
			}
		}
	}
	return out.String()
}
