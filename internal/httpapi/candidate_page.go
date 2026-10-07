package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

const CandidatePagePrefix = SearchPagePath + "/"

type candidatePageData struct {
	Detail *DetailResponse
	Error  string
}

func (h *searchHandler) serveCandidatePage(w http.ResponseWriter, r *http.Request, id string) {
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		candidatePageError(w, r, 400)
		return
	}
	key, err := decodeCandidateID(id)
	if err != nil {
		candidatePageError(w, r, 400)
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		candidatePageError(w, r, 429)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.options.Timeout)
	defer cancel()
	detail, err := h.detail(ctx, key)
	if err != nil || ctx.Err() != nil {
		status := http.StatusServiceUnavailable
		if ctx.Err() == nil {
			switch {
			case errors.Is(err, ErrCandidateStale):
				status = http.StatusConflict
			case errors.Is(err, ErrCandidateNotFound):
				status = http.StatusNotFound
			case errors.Is(err, correlation.ErrPartitionLimit), errors.Is(err, sqlite.ErrCorrelationScope):
				status = http.StatusUnprocessableEntity
			}
		}
		candidatePageError(w, r, status)
		return
	}
	if writeHTMLPage(w, r, ctx, 200, "candidate_page.html", candidatePageData{Detail: &detail}) != nil {
		candidatePageError(w, r, 503)
	}
}

func candidatePageError(w http.ResponseWriter, r *http.Request, status int) {
	_ = writeHTMLPage(w, r, context.Background(), status, "candidate_page.html", candidatePageData{Error: candidatePageErrorMessage(status)})
}

func candidatePageErrorMessage(status int) string {
	message := "Détail indisponible. Réessayez plus tard."
	switch status {
	case 400:
		message = "Référence de candidat invalide. Relancez la recherche."
	case 404:
		message = "Candidat introuvable dans les faits actuels. Cela ne prouve pas l’absence dans les journaux."
	case 409:
		message = "Les faits ont changé depuis cette recherche. Relancez la recherche pour consulter un candidat à jour."
	case 422:
		message = "Cette file dépasse le budget de reconstruction. Son détail ne peut pas être présenté partiellement."
	case 429:
		message = "Consultation occupée. Réessayez dans quelques instants."
	}
	return message
}
