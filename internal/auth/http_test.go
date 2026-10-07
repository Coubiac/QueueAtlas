package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testAuthOrigin = "https://queueatlas.example"

func httpAuthFixture(t *testing.T, options LoginOptions, verify func([]byte, string) (bool, error)) (*HTTPHandler, *SessionStore) {
	t.Helper()
	login, sessions, _ := loginFixture(t, options, verify)
	handler, err := NewHTTPHandler(login, testAuthOrigin)
	if err != nil {
		t.Fatal(err)
	}
	return handler, sessions
}

func authRequest(path, body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, testAuthOrigin+path, strings.NewReader(body))
	r.Header.Set("Origin", testAuthOrigin)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func authForm(name, password string) string {
	return url.Values{"username": {name}, "password": {password}}.Encode()
}

func assertAuthResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int) {
	t.Helper()
	if recorder.Code != status || recorder.Header().Get("Cache-Control") != "no-store" ||
		recorder.Header().Get("X-Content-Type-Options") != "nosniff" ||
		recorder.Header().Get("Access-Control-Allow-Origin") != "" || recorder.Header().Get("Location") != "" {
		t.Fatalf("HTTP response: code=%d want=%d or unsafe headers", recorder.Code, status)
	}
	if status >= 400 && len(recorder.Result().Cookies()) != 0 {
		t.Fatal("failure changed a cookie")
	}
}

func assertSessionCookie(t *testing.T, cookie *http.Cookie, remove bool) {
	t.Helper()
	if cookie.Name != SessionCookieName || cookie.Path != "/" || cookie.Domain != "" ||
		!cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || !cookie.Expires.IsZero() {
		t.Fatal("unsafe cookie attributes")
	}
	if remove && (cookie.Value != "" || cookie.MaxAge != -1) || !remove && (len(cookie.Value) != SessionTokenChars || cookie.MaxAge != 0) {
		t.Fatal("unexpected cookie lifetime/value")
	}
}

func TestHTTPAuthConstructorOriginAndZeroValues(t *testing.T) {
	h, _ := httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { return false, nil })
	for _, origin := range []string{testAuthOrigin, "https://localhost", "https://127.0.0.1:8443", "https://[::1]:8443", "https://xn--example-9za.example"} {
		if handler, err := NewHTTPHandler(h.login, origin); err != nil || handler == nil {
			t.Fatal("valid canonical origin rejected")
		}
	}
	for _, origin := range []string{"", "http://localhost", "https://LOCALHOST", "https://localhost/", "https://localhost?", "https://localhost#",
		"https://user:private@localhost", "https://localhost/path", "https://localhost:443", "https://localhost:0", "https://localhost:08443",
		"https://localhost:65536", "https://localhost:", "https://[127.0.0.1]", "https://bad_name", "https://-bad.example", "https://bad..example", "https://é.example", "https://localhost.", "https://123", "https://127.000.0.1"} {
		if handler, err := NewHTTPHandler(h.login, origin); handler != nil || err != ErrInvalidHTTPAuth {
			t.Fatalf("invalid origin accepted: %q", origin)
		}
	}
	for _, login := range []*LocalLogin{nil, {}} {
		if handler, err := NewHTTPHandler(login, testAuthOrigin); handler != nil || err != ErrInvalidHTTPAuth {
			t.Fatal("uninitialized login accepted")
		}
	}
	for _, handler := range []*HTTPHandler{nil, {}} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, authRequest(LoginPath, ""))
		assertAuthResponse(t, w, http.StatusServiceUnavailable)
	}
}

func TestHTTPAuthRealTLSCookieLoginReplacementAndLogout(t *testing.T) {
	sessions, err := NewSessionStore(DefaultSessionOptions())
	if err != nil {
		t.Fatal(err)
	}
	login, err := NewLocalLogin(loginAccount(), sessions, DefaultLoginOptions())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	origin := "https://" + server.Listener.Addr().String()
	handler, err := NewHTTPHandler(login, origin)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	server.Config.Handler = handler
	server.StartTLS()
	defer server.Close()
	client := server.Client()
	client.Jar, err = cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	var oldToken string
	for range 2 {
		r, err := http.NewRequest(http.MethodPost, origin+LoginPath, strings.NewReader(authForm("Operator", "synthetic secret phrase")))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		cookies := response.Cookies()
		if readErr != nil || response.StatusCode != 200 || string(body) != "authenticated\n" || len(cookies) != 1 || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("HTTPS login failed")
		}
		assertSessionCookie(t, cookies[0], false)
		token := cookies[0].Value
		if session, err := sessions.Resolve(token); err != nil || session.Identity != loginAccount().Identity || token == oldToken {
			t.Fatal("HTTPS login did not issue a fresh owned session")
		}
		if oldToken != "" {
			if _, err := sessions.Resolve(oldToken); err != ErrInvalidSession {
				t.Fatal("replacement did not revoke old cookie")
			}
		}
		oldToken = token
	}
	r, err := http.NewRequest(http.MethodPost, origin+LogoutPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Origin", origin)
	response, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 || len(response.Cookies()) != 1 {
		t.Fatal("HTTPS logout failed")
	}
	assertSessionCookie(t, response.Cookies()[0], true)
	if _, err := sessions.Resolve(oldToken); err != ErrInvalidSession {
		t.Fatal("logout left token valid")
	}
	u, _ := url.Parse(origin)
	if len(client.Jar.Cookies(u)) != 0 {
		t.Fatal("logout did not delete client cookie")
	}
}

type authUnreadBody struct{ reads int }

func (b *authUnreadBody) Read([]byte) (int, error) {
	b.reads++
	return 0, errors.New("private body failure")
}
func (*authUnreadBody) Close() error { return nil }

func TestHTTPAuthGuardsBeforeBodyHashOrRevocation(t *testing.T) {
	h, sessions := httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { t.Fatal("guard hashed"); return false, nil })
	token, _, err := sessions.Issue(loginAccount().Identity)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		status int
		change func(*http.Request)
	}{
		{"cleartext", 403, func(r *http.Request) { r.TLS = nil; r.Header.Set("X-Forwarded-Proto", "https") }},
		{"host", 403, func(r *http.Request) {
			r.Host = "attacker.example"
			r.Header.Set("X-Forwarded-Host", "queueatlas.example")
		}},
		{"no origin", 403, func(r *http.Request) { r.Header.Del("Origin") }},
		{"null origin", 403, func(r *http.Request) { r.Header.Set("Origin", "null") }},
		{"foreign origin", 403, func(r *http.Request) { r.Header.Set("Origin", "https://queueatlas.example.attacker.example") }},
		{"duplicate origin", 403, func(r *http.Request) { r.Header.Add("Origin", testAuthOrigin) }},
		{"referer only", 403, func(r *http.Request) { r.Header.Del("Origin"); r.Header.Set("Referer", testAuthOrigin+"/") }},
		{"query", 400, func(r *http.Request) { r.URL.RawQuery = "password=private" }},
		{"empty query", 400, func(r *http.Request) { r.URL.ForceQuery = true }},
		{"encoding", 415, func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }},
		{"GET", 405, func(r *http.Request) { r.Method = "GET" }},
		{"OPTIONS", 405, func(r *http.Request) { r.Method = "OPTIONS" }},
		{"encoded alias", 404, func(r *http.Request) { r.URL.RawPath = "/api/auth/%6cogout" }},
		{"unknown route", 404, func(r *http.Request) { r.URL.Path += "/" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := authRequest(LogoutPath, "")
			r.AddCookie(authCookie(token, false))
			body := &authUnreadBody{}
			r.Body = body
			tc.change(r)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			assertAuthResponse(t, w, tc.status)
			if body.reads != 0 || h.login.attempts != 0 || strings.Contains(w.Body.String(), "private") {
				t.Fatal("guard read body, consumed budget or leaked input")
			}
			if tc.status == 405 && w.Header().Get("Allow") != "POST" {
				t.Fatal("missing Allow")
			}
			if _, err := sessions.Resolve(token); err != nil {
				t.Fatal("denied logout revoked session")
			}
		})
	}
}

func TestHTTPAuthRejectsHostileFormsBeforeHashing(t *testing.T) {
	h, sessions := httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { t.Fatal("bad form hashed"); return false, nil })
	for _, tc := range []struct {
		body, contentType string
		status            int
	}{
		{"", "application/x-www-form-urlencoded", 400},
		{"username=Operator", "application/x-www-form-urlencoded", 400},
		{"Username=Operator&password=secret", "application/x-www-form-urlencoded", 400},
		{"username=Operator&password=secret&password=other", "application/x-www-form-urlencoded", 400},
		{"username=Operator&%75sername=Other&password=secret", "application/x-www-form-urlencoded", 400},
		{"username=Operator&password=secret&extra=private", "application/x-www-form-urlencoded", 400},
		{"username=Operator&password=%ZZ", "application/x-www-form-urlencoded", 400},
		{"username=Operator;password=private", "application/x-www-form-urlencoded", 400},
		{strings.Repeat("x", MaxLoginBodyBytes+1), "application/x-www-form-urlencoded", 413},
		{"private", "", 415}, {"private", "application/json", 415}, {"private", "text/plain", 415},
		{"private", "application/x-www-form-urlencoded; charset=iso-8859-1", 415},
		{"private", "application/x-www-form-urlencoded; arbitrary=utf-8", 415},
	} {
		r := authRequest(LoginPath, tc.body)
		if len(tc.body) > MaxLoginBodyBytes {
			r.ContentLength = -1 // unknown length must still be bounded
		}
		r.Header.Set("Content-Type", tc.contentType)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assertAuthResponse(t, w, tc.status)
		if strings.Contains(w.Body.String(), "private") || h.login.attempts != 0 || len(sessions.sessions) != 0 {
			t.Fatal("bad form leaked, counted or issued session")
		}
	}
	for _, broken := range []bool{true, false} {
		r := authRequest(LoginPath, authForm("Operator", "synthetic secret phrase"))
		if broken {
			r.Body = &authUnreadBody{}
		} else {
			r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		status := 415
		if broken {
			status = 400
		}
		assertAuthResponse(t, w, status)
	}
}

func TestHTTPAuthLiteralFormPasswordsAndPrivateFailures(t *testing.T) {
	for _, password := range []string{"literal + & = % secret", strings.Repeat("🙂", 256)} {
		h, _ := httpAuthFixture(t, DefaultLoginOptions(), func(received []byte, hash string) (bool, error) {
			if string(received) != password || hash != syntheticPasswordHash {
				t.Fatal("form changed password/hash")
			}
			return true, nil
		})
		r := authRequest(LoginPath, authForm("Operator", password))
		if len(password) == MaxPasswordBytes {
			form := authForm("Operator", password)
			r = authRequest(LoginPath, form+strings.Repeat("&", MaxLoginBodyBytes-len(form)))
		}
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assertAuthResponse(t, w, 200)
	}
	h, sessions := httpAuthFixture(t, LoginOptions{AttemptLimit: 2, Window: MinLoginWindow, MaxConcurrent: 1}, func([]byte, string) (bool, error) { return false, nil })
	oldToken, _, err := sessions.Issue(loginAccount().Identity)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"Operator", "Unknown", "Operator"} {
		w := httptest.NewRecorder()
		r := authRequest(LoginPath, authForm(name, "synthetic secret phrase"))
		r.AddCookie(authCookie(oldToken, false))
		h.ServeHTTP(w, r)
		status := 401
		if i == 2 {
			status = 429
		}
		assertAuthResponse(t, w, status)
		if status == 401 && w.Body.String() != "invalid credentials\n" {
			t.Fatal("credentials leaked account state")
		}
		if _, err := sessions.Resolve(oldToken); err != nil {
			t.Fatal("failed login revoked existing session")
		}
	}
	if len(sessions.sessions) != 1 {
		t.Fatal("failure issued a session")
	}
	h, _ = httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { t.Fatal("invalid password hashed"); return false, nil })
	for _, password := range []string{"short", "\xffsynthetic secret phrase"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, authRequest(LoginPath, authForm("Operator", password)))
		assertAuthResponse(t, w, 401)
	}
	if h.login.attempts != 2 {
		t.Fatal("decoded invalid credentials bypassed login budget")
	}
	h, _ = httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { return false, errors.New("private verifier error") })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, authRequest(LoginPath, authForm("Operator", "synthetic secret phrase")))
	assertAuthResponse(t, w, 503)
	if strings.Contains(w.Body.String(), "private") {
		t.Fatal("private error leaked")
	}
}

func TestHTTPAuthLogoutCookieAmbiguityAndIdempotence(t *testing.T) {
	h, sessions := httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { return false, nil })
	token, _, err := sessions.Issue(loginAccount().Identity)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cookie, body string
		status       int
	}{
		{SessionCookieName + "=" + token + "; " + SessionCookieName + "=" + token, "", 400},
		{SessionCookieName + "=" + token, "private", 400}, {strings.Repeat("x", MaxAuthCookieBytes+1), "", 431},
	} {
		r := authRequest(LogoutPath, tc.body)
		r.Header.Set("Cookie", tc.cookie)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assertAuthResponse(t, w, tc.status)
		if _, err := sessions.Resolve(token); err != nil {
			t.Fatal("ambiguous/invalid logout revoked token")
		}
	}
	for _, cookie := range []string{"", SessionCookieName + "=malformed", SessionCookieName + "=" + token, SessionCookieName + "=" + token} {
		r := authRequest(LogoutPath, "")
		r.Header.Set("Cookie", cookie)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		assertAuthResponse(t, w, 204)
		if w.Body.Len() != 0 || len(w.Result().Cookies()) != 1 {
			t.Fatal("logout response differs by cookie state")
		}
		assertSessionCookie(t, w.Result().Cookies()[0], true)
	}
	if _, err := sessions.Resolve(token); err != ErrInvalidSession {
		t.Fatal("logout failed revocation")
	}
}

type authFailedWriter struct {
	header http.Header
	mode   string
}

func (w *authFailedWriter) Header() http.Header { return w.header }
func (*authFailedWriter) WriteHeader(int)       {}
func (w *authFailedWriter) Write([]byte) (int, error) {
	if w.mode == "panic" {
		panic("synthetic writer panic")
	}
	if w.mode == "short" {
		return 1, nil
	}
	return 0, errors.New("private write failure")
}

type authCancelReader struct {
	reader io.Reader
	cancel context.CancelFunc
}

func (r authCancelReader) Read(p []byte) (int, error) { r.cancel(); return r.reader.Read(p) }

func TestHTTPAuthFailedWriteAndCancellationRollBackSession(t *testing.T) {
	h, sessions := httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { return true, nil })
	for _, mode := range []string{"error", "short", "panic"} {
		panicked := false
		func() {
			defer func() {
				if recover() != nil {
					panicked = true
				}
			}()
			w := &authFailedWriter{header: make(http.Header), mode: mode}
			h.ServeHTTP(w, authRequest(LoginPath, authForm("Operator", "synthetic secret phrase")))
		}()
		if panicked != (mode == "panic") || len(sessions.sessions) != 0 {
			t.Fatal("failed response retained new session or changed panic behavior")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sessions.random = authCancelReader{sessions.random, cancel}
	r := authRequest(LoginPath, authForm("Operator", "synthetic secret phrase")).WithContext(ctx)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, r)
	assertAuthResponse(t, recorder, 503)
	if len(sessions.sessions) != 0 {
		t.Fatal("cancellation during entropy retained session")
	}
}
