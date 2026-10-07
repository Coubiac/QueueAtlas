package httpapi

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/auth"
)

func loginPageRequest(method, body string) *http.Request {
	r := httptest.NewRequest(method, searchTestOrigin+auth.WebLoginPath, strings.NewReader(body))
	if method == "POST" {
		r.Header.Set("Origin", searchTestOrigin)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return r
}

func TestWebLoginFormHeadersPrivateErrorsAndClosedInputs(t *testing.T) {
	guard, _, _ := searchGuardFixture(t)
	h, err := NewWebLoginHandler(guard)
	if err != nil {
		t.Fatal(err)
	}
	if handler, err := NewWebLoginHandler(nil); handler != nil || err != auth.ErrInvalidHTTPAuth {
		t.Fatal("nil web login guard accepted")
	}
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, loginPageRequest(method, ""))
		assertSearchPage(t, w, 200)
		if method == "HEAD" {
			if w.Body.Len() != 0 {
				t.Fatal("login HEAD body")
			}
			continue
		}
		for _, required := range []string{`method="post" action="/login"`, `for="username"`, `for="password"`, `type="password"`, `autocomplete="username"`, `autocomplete="current-password"`, `href="#main"`} {
			if !strings.Contains(w.Body.String(), required) {
				t.Fatal("login form missing label/safe destination", required)
			}
		}
		if strings.Contains(w.Body.String(), "<script") || strings.Contains(w.Body.String(), `name="next"`) {
			t.Fatal("login added active code or request-selected redirect")
		}
	}
	secret := `synthetic private"><script>alert(1)</script>`
	for _, body := range []string{url.Values{"username": {secret}, "password": {secret}}.Encode(), url.Values{"username": {"SyntheticOperator"}, "password": {secret}}.Encode()} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, loginPageRequest("POST", body))
		assertSearchPage(t, w, 401)
		if !strings.Contains(w.Body.String(), "Identifiants invalides") || strings.Contains(w.Body.String(), "synthetic private") || strings.Contains(w.Body.String(), "SyntheticOperator") || strings.Contains(w.Body.String(), "<script") {
			t.Fatal("login enumerated identity or reflected credential")
		}
	}
	for _, tc := range []struct {
		body   string
		status int
	}{{"username=private&password=private&next=https://foreign.example", 400}, {"username=private&username=private&password=private", 400}, {strings.Repeat("x", auth.MaxLoginBodyBytes+1), 413}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, loginPageRequest("POST", tc.body))
		assertSearchPage(t, w, tc.status)
		if strings.Contains(w.Body.String(), "private") {
			t.Fatal("invalid form echoed body")
		}
	}
	r := loginPageRequest("POST", "")
	r.Body = panicSearchBody{}
	r.Header.Del("Origin")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	assertSearchPage(t, w, 403)
}

func TestWebLoginRealHTTPSRedirectRotationAndProtectedConsultation(t *testing.T) {
	server := httptest.NewUnstartedServer(nil)
	defer server.Close()
	origin := "https://" + server.Listener.Addr().String()
	guard, oldToken, sessions := searchGuardAtOrigin(t, origin)
	login, err := NewWebLoginHandler(guard)
	if err != nil {
		t.Fatal(err)
	}
	consultation, err := NewConsultationHandler(guard, syntheticSearchStore(t), DefaultSearchOptions())
	if err != nil {
		t.Fatal(err)
	}
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case auth.WebLoginPath:
			login.ServeHTTP(w, r)
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
	client.Jar.SetCookies(u, []*http.Cookie{{Name: auth.SessionCookieName, Value: oldToken, Path: "/", Secure: true}})
	for range 2 {
		body := url.Values{"username": {"SyntheticOperator"}, "password": {"synthetic secret phrase"}}.Encode()
		r, _ := http.NewRequest("POST", origin+auth.WebLoginPath, strings.NewReader(body))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		data, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		cookies := response.Cookies()
		if readErr != nil || response.StatusCode != 303 || response.Header.Get("Location") != "/messages" || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Content-Security-Policy") == "" || len(cookies) != 1 || strings.Contains(string(data), "synthetic secret") {
			t.Fatal("HTTPS web login redirect/headers")
		}
		cookie := cookies[0]
		if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.Path != "/" || cookie.Value == oldToken {
			t.Fatal("browser login cookie policy/rotation")
		}
		if _, err := sessions.Resolve(oldToken); err != auth.ErrInvalidSession {
			t.Fatal("old browser session remained valid")
		}
		oldToken = cookie.Value
		response, err = client.Get(origin + "/messages")
		if err != nil {
			t.Fatal(err)
		}
		data, readErr = io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil || response.StatusCode != 200 || !strings.Contains(string(data), "Rechercher un événement") {
			t.Fatal("issued cookie could not read protected Web page")
		}
	}
	r, _ := http.NewRequest("POST", origin+auth.LogoutPath, nil)
	r.Header.Set("Origin", origin)
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 || len(response.Cookies()) != 1 {
		t.Fatal("existing API logout compatibility")
	}
	response, err = client.Get(origin + "/messages")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 || len(client.Jar.Cookies(u)) != 0 {
		t.Fatal("logout did not remove access/cookie")
	}
}
