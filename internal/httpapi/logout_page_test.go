package httpapi

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/auth"
)

func TestWebLogoutHTTPSFromEveryViewClearsCookieAndRevokesOnlyCurrentSession(t *testing.T) {
	server := httptest.NewUnstartedServer(nil)
	defer server.Close()
	origin := "https://" + server.Listener.Addr().String()
	guard, token, sessions := searchGuardAtOrigin(t, origin)
	other, _, err := sessions.Issue(auth.LocalIdentity{Username: "SyntheticOperator"})
	if err != nil {
		t.Fatal(err)
	}
	authWeb, err := NewWebLoginHandler(guard)
	if err != nil {
		t.Fatal(err)
	}
	store := syntheticSearchStore(t)
	storeSearchCycle(t, store, "synthetic-first", "synthetic-postfix", "ABC123", true)
	consultation, err := NewConsultationHandler(guard, store, DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case auth.WebLoginPath, auth.WebLogoutPath:
			authWeb.ServeHTTP(w, r)
		case auth.LoginPath, auth.LogoutPath:
			guard.ServeHTTP(w, r)
		default:
			consultation.ServeHTTP(w, r)
		}
	})
	server.StartTLS()
	client := server.Client()
	client.Jar, _ = cookiejar.New(nil)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	u, _ := url.Parse(origin)
	client.Jar.SetCookies(u, []*http.Cookie{{Name: auth.SessionCookieName, Value: token, Path: "/", Secure: true}})
	// Obtain the real candidate ID through the API, then exercise all three HTML views.
	w := httptest.NewRecorder()
	apiRequest := httptest.NewRequest("GET", origin+searchTestURL(), nil)
	apiRequest.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	consultation.ServeHTTP(w, apiRequest)
	id := decodeSearchResponse(t, w).Matches[0].Candidate.ID
	action := ""
	for _, path := range []string{SearchPagePath, CandidatePagePrefix + id, timelinePageURL(id)} {
		response, err := client.Get(origin + path)
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		form := regexp.MustCompile(`(?s)<form method="post" action="([^"]+)">\s*<button type="submit">Se déconnecter</button>\s*</form>`).FindStringSubmatch(string(data))
		if readErr != nil || response.StatusCode != 200 || len(form) != 2 || form[1] != auth.WebLogoutPath || strings.Contains(string(data), token) || !strings.Contains(response.Header.Get("Content-Security-Policy"), "form-action 'self'") {
			t.Fatal("view lost logout form or exposed session")
		}
		action = form[1]
	}
	// Rejected cross-origin POST must retain the cookie and its authority.
	r, _ := http.NewRequest("POST", origin+action, nil)
	r.Header.Set("Origin", "https://foreign.example")
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil || response.StatusCode != 403 || len(response.Cookies()) != 0 || !strings.Contains(string(data), "Déconnexion interrompue") || strings.Contains(string(data), "foreign.example") || strings.Contains(string(data), token) {
		t.Fatal("logout refusal leaked or modified cookie")
	}
	if _, err := sessions.Resolve(token); err != nil {
		t.Fatal("cross-origin logout revoked session")
	}
	// Submit the button's actual action as an empty form, with the browser's origin.
	for range 2 {
		r, _ := http.NewRequest("POST", origin+action, strings.NewReader(""))
		r.Header.Set("Origin", origin)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		cookies := response.Cookies()
		if readErr != nil || response.StatusCode != 303 || response.Header.Get("Location") != auth.WebLoginPath || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("X-Frame-Options") != "DENY" || response.Header.Get("Referrer-Policy") != "no-referrer" || response.Header.Get("Content-Security-Policy") == "" || len(cookies) != 1 || string(data) != "logged out\n" {
			t.Fatal("logout redirect/security changed or differs on retry")
		}
		cookie := cookies[0]
		if cookie.Name != auth.SessionCookieName || cookie.Value != "" || cookie.MaxAge != -1 || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" || len(client.Jar.Cookies(u)) != 0 {
			t.Fatal("logout failed secure cookie deletion")
		}
	}
	if _, err := sessions.Resolve(token); err != auth.ErrInvalidSession {
		t.Fatal("logout cookie deleted without revocation")
	}
	if _, err := sessions.Resolve(other); err != nil {
		t.Fatal("logout revoked another session")
	}
	response, err = client.Get(origin + auth.WebLoginPath)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr = io.ReadAll(response.Body)
	response.Body.Close()
	if readErr != nil || response.StatusCode != 200 || !strings.Contains(string(data), `action="/login"`) || strings.Contains(string(data), `action="/logout"`) {
		t.Fatal("logout did not return to public login form")
	}
	for _, path := range []string{SearchPagePath, CandidatePagePrefix + id, timelinePageURL(id), searchTestURL()} {
		r, _ := http.NewRequest("GET", origin+path, nil)
		r.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token}) // replay a revoked token
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 401 {
			t.Fatal("data route remained accessible after logout")
		}
	}
}
