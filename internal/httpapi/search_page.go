package httpapi

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/auth"
	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

const SearchPagePath = "/messages"

//go:embed search_page.html search_page.css candidate_page.html timeline_page.html
var searchPageAssets embed.FS

var (
	searchPageCSS, searchPageCSP = embeddedSearchStyle()
	searchPageTemplate           = template.Must(template.New("search_page.html").Funcs(template.FuncMap{
		// The only trusted CSS is our embedded stylesheet, never request/log text.
		"styles": func() template.CSS { return template.CSS(searchPageCSS) },
		"label":  searchPageLabel,
		"date":   func(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) },
		// Candidate IDs come from the canonical encoder, not native log URLs.
		"candidateURL": func(id string) string { return CandidatePagePrefix + id },
		"timelineURL":  timelinePageURL,
		"visible":      visibleNativeText,
	}).ParseFS(searchPageAssets, "search_page.html", "candidate_page.html", "timeline_page.html"))
)

func embeddedSearchStyle() (string, string) {
	css, err := searchPageAssets.ReadFile("search_page.css")
	if err != nil {
		panic("missing embedded search stylesheet")
	}
	digest := sha256.Sum256(css)
	return string(css), "default-src 'none'; style-src 'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'"
}

// NewConsultationHandler returns one protected router for the API and HTML
// search/detail/timeline pages, sharing its store, admission and deadlines. Mount auth separately.
// No listener or login page is created. NewSearchHandler remains API-only.
func NewConsultationHandler(guard *auth.HTTPHandler, store *sqlite.Store, options SearchOptions) (http.Handler, error) {
	if store == nil {
		return nil, ErrSearchSetup
	}
	return newReadHandler(guard, store, options, time.Now, true)
}

type searchPageData struct {
	Form     bool
	Searched bool
	Error    string
	Query    sqlite.SearchQuery
	Response SearchResponse
	NextURL  string
	Choices  []searchPageChoice
}

type searchPageChoice struct{ Value, Label string }

var searchPageChoices = []searchPageChoice{
	{"sender", "Expéditeur exact"}, {"recipient", "Destinataire exact"}, {"queue_id", "Queue ID"},
	{"message_id", "Message-ID"}, {"sender_domain", "Domaine expéditeur"}, {"recipient_domain", "Domaine destinataire"},
}

func searchPageHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", searchPageCSP)
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (h *searchHandler) serveSearchPage(w http.ResponseWriter, r *http.Request) {
	data := searchPageData{Form: true, Choices: searchPageChoices}
	if r.URL.RawQuery == "" {
		until := h.now().UTC()
		from := until.Add(-DefaultSearchWindow)
		if !searchInstantFits(from) || !searchInstantFits(until) {
			searchPageError(w, r, 503)
			return
		}
		data.Query = sqlite.SearchQuery{Field: sqlite.SearchSender, From: from, Until: until, Limit: DefaultSearchLimit}
	} else {
		query, err := ParseSearchRequest(r.URL.RawQuery, h.now())
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, ErrSearchClock) {
				status = http.StatusServiceUnavailable
			}
			searchPageError(w, r, status)
			return
		}
		select {
		case h.slots <- struct{}{}:
			defer func() { <-h.slots }()
		default:
			searchPageError(w, r, 429)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), h.options.Timeout)
		defer cancel()
		page, err := h.search(ctx, query)
		if err != nil || ctx.Err() != nil {
			status := http.StatusServiceUnavailable
			if ctx.Err() == nil && (errors.Is(err, correlation.ErrPartitionLimit) || errors.Is(err, sqlite.ErrCorrelationScope)) {
				status = http.StatusUnprocessableEntity
			}
			searchPageError(w, r, status)
			return
		}
		data.Query, data.Response, data.Searched = query, page, true
		// A new submit begins a new search; pagination freezes the effective dates
		// in its link, including when the first request used clock defaults.
		data.Query.After = nil
		if page.NextCursor != "" {
			values := url.Values{"instance": {query.Instance}, "field": {string(query.Field)}, "value": {query.Value},
				"from": {page.From.UTC().Format(time.RFC3339Nano)}, "until": {page.Until.UTC().Format(time.RFC3339Nano)},
				"limit": {strconv.Itoa(query.Limit)}, "cursor": {page.NextCursor}}
			data.NextURL = SearchPagePath + "?" + values.Encode()
		}
		if writeSearchPage(w, r, ctx, 200, data) != nil {
			searchPageError(w, r, 503)
		}
		return
	}
	if writeSearchPage(w, r, r.Context(), 200, data) != nil {
		searchPageError(w, r, 503)
	}
}

func writeSearchPage(w http.ResponseWriter, r *http.Request, ctx context.Context, status int, data searchPageData) error {
	return writeHTMLPage(w, r, ctx, status, "search_page.html", data)
}

func writeHTMLPage(w http.ResponseWriter, r *http.Request, ctx context.Context, status int, name string, data any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var buffer responseBuffer
	if err := searchPageTemplate.ExecuteTemplate(&buffer, name, data); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(buffer.Bytes())
	}
	return nil
}

func searchPageError(w http.ResponseWriter, r *http.Request, status int) {
	message := "Recherche indisponible. Réessayez plus tard ou réduisez le nombre de résultats."
	switch status {
	case 400:
		message = "Paramètres de recherche invalides. Recommencez depuis le formulaire."
	case 422:
		message = "Trop de faits pour reconstruire cette recherche. Réduisez le nombre de résultats ou précisez le critère."
	case 429:
		message = "Recherche occupée. Réessayez dans quelques instants."
	}
	// Error output contains only fixed text. A canceled read must still yield a
	// complete error page, not the successfully rendered data from that read.
	_ = writeSearchPage(w, r, context.Background(), status, searchPageData{Error: message})
}

func searchPageLabel(value any) string {
	labels := map[string]string{
		"message": "Message observé", "delivery": "Tentative observée", "removed": "Retrait de file observé", "bounce": "Notification d'échec", "reject": "Rapport pré-file", "unknown": "Inconnu",
		"explicit_offset": "Fuseau explicite", "configured_year_and_zone": "Année et fuseau configurés", "inferred_year_and_zone": "Année et fuseau inférés",
		"coverage_unproven": "Couverture non prouvée", "cross_stream_uncertain": "Continuité entre origines incertaine", "non_explicit_time": "Date sous hypothèse",
		"receipt_not_observed": "Réception non observée", "removal_not_observed": "Retrait non observé", "no_recipients_observed": "Aucun destinataire observé",
		"address_unspecified": "Adresse non spécifiée", "latest_order_uncertain": "Dernières tentatives simultanées", "unknown_result": "Résultat inconnu", "unprojected_deliveries": "Tentatives non interprétées",
		"undated": "Origine sans date exploitable", "boundary_unproven": "Limite de génération non prouvée", "conflicting_message_ids": "Message-ID contradictoires",
		"warning": "Avertissement, pas un rejet", "rejected": "Rejet observé",
		"sent": "sent (transport)", "delivered": "delivered (remise reconnue)", "deferred": "Différé", "bounced": "Échec rapporté",
		"smtp_peer": "Prochain saut SMTP", "lmtp_peer": "Transport LMTP", "pipe_command": "Commande pipe", "local_agent": "Agent local", "virtual_agent": "Agent virtuel", "local_mailbox": "Boîte locale", "virtual_mailbox": "Boîte virtuelle",
		"connect": "Connexion observée", "disconnect": "Déconnexion observée", "wall_only": "Heure sans année ni fuseau", "year_without_zone": "Année sans fuseau",
	}
	var key string
	switch v := value.(type) {
	case correlation.SummaryReserve:
		key = string(v)
	case correlation.GenerationReason:
		key = string(v)
	case correlation.PrequeueDisposition:
		key = string(v)
	case model.Kind:
		key = string(v)
	case model.TimeQuality:
		key = string(v)
	case correlation.DeliveryStatus:
		key = string(v)
	case correlation.DeliveryScope:
		key = string(v)
	default:
		return "Inconnu"
	}
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}
