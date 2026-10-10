package httpapi

import (
	"context"
	"net/http"

	"github.com/Coubiac/QueueAtlas/internal/auth"
)

// NewWebLoginHandler serves the public local login form/submission and POST logout.
// Share guard with consultation/API auth so rate/session budgets are shared.
// No listener, automatic public data-route exception or external provider.
func NewWebLoginHandler(guard *auth.HTTPHandler) (http.Handler, error) {
	handler, err := auth.NewBrowserLoginHandler(guard, renderLoginPage)
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		searchPageHeaders(w)
		handler.ServeHTTP(w, r)
	}), nil
}

type loginPageData struct{ Error, ErrorTitle string }

func renderLoginPage(w http.ResponseWriter, r *http.Request, status int) {
	searchPageHeaders(w)
	message := ""
	title := "Connexion interrompue"
	if status != http.StatusOK {
		message = "Connexion indisponible. Réessayez plus tard."
		switch status {
		case 401:
			message = "Identifiants invalides."
		case 429:
			message = "Trop de tentatives. Réessayez plus tard."
		case 400, 413, 415, 431:
			message = "Requête de connexion invalide. Recommencez depuis le formulaire."
		case 403:
			message = "Requête de connexion refusée."
		case 404, 405:
			message = "Cette route ou méthode n’est pas disponible."
		}
		if r.URL != nil && (r.URL.Path == auth.LogoutPath || r.URL.Path == auth.WebLogoutPath) {
			title = "Déconnexion interrompue"
			message = "Déconnexion non confirmée. Réessayez depuis une vue de consultation."
		}
	}
	// Only fixed public text, even on a canceled request. Credentials never
	// enter template data; fields stay empty on every failure.
	_ = writeHTMLPage(w, r, context.Background(), status, "login_page.html", loginPageData{Error: message, ErrorTitle: title})
}
