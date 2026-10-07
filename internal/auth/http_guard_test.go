package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func mustProtect(t *testing.T, h *HTTPHandler, next http.Handler) http.Handler {
	t.Helper()
	guard, err := h.Protect(next)
	if err != nil {
		t.Fatal(err)
	}
	return guard
}

func TestHTTPGuardRealTLSLoginDataMutationAndLogout(t *testing.T) {
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
	h, err := NewHTTPHandler(login, origin)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	var calls atomic.Int32
	guard := mustProtect(t, h, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := SessionFromContext(r.Context())
		if !ok || session.Identity != loginAccount().Identity {
			t.Error("missing/wrong request identity")
		}
		calls.Add(1)
		_, _ = io.WriteString(w, "synthetic protected data\n")
	}))
	mux := http.NewServeMux()
	mux.Handle(LoginPath, h)
	mux.Handle(LogoutPath, h)
	mux.Handle("/api/data", guard)
	server.Config.Handler = mux
	server.StartTLS()
	defer server.Close()
	client := server.Client()
	client.Jar, err = cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, attempt := range []struct {
		method, path, body string
		origin             bool
		status             int
	}{
		{"GET", "/api/data?q=synthetic", "", false, 401},
		{"POST", LoginPath, authForm("Operator", "synthetic secret phrase"), true, 200},
		{"GET", "/api/data?q=synthetic", "", false, 200},
		{"POST", "/api/data", "synthetic mutation", false, 403},
		{"POST", "/api/data", "synthetic mutation", true, 200},
		{"POST", LogoutPath, "", true, 204},
		{"GET", "/api/data", "", false, 401},
	} {
		r, err := http.NewRequest(attempt.method, origin+attempt.path, strings.NewReader(attempt.body))
		if err != nil {
			t.Fatal(err)
		}
		if attempt.origin {
			r.Header.Set("Origin", origin)
		}
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil || response.StatusCode != attempt.status || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatal("TLS integration response failed")
		}
		if attempt.path == "/api/data" && attempt.status >= 400 && strings.Contains(string(body), "synthetic") {
			t.Fatal("denied data leaked")
		}
	}
	if calls.Load() != 2 {
		t.Fatal("denied request reached data handler")
	}
}

func TestHTTPGuardProtocolDenialPrecedesResolutionAndBody(t *testing.T) {
	h, sessions := httpAuthFixture(t, DefaultLoginOptions(), VerifyPassword)
	token, _ := issueSyntheticSession(t, sessions, "Operator")
	sessions.now = func() time.Time { t.Fatal("denied request resolved session"); return time.Time{} }
	guard := mustProtect(t, h, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("denied request reached next") }))
	for _, tc := range []struct {
		status int
		change func(*http.Request)
	}{
		{403, func(r *http.Request) { r.TLS = nil; r.Header.Set("X-Forwarded-Proto", "https") }},
		{403, func(r *http.Request) {
			r.Host = "foreign.example"
			r.Header.Set("X-Forwarded-Host", "queueatlas.example")
		}},
		{403, func(r *http.Request) { r.Header.Del("Origin"); r.Header.Set("Sec-Fetch-Site", "same-origin") }},
		{403, func(r *http.Request) { r.Header.Set("Origin", "null") }},
		{403, func(r *http.Request) { r.Header.Set("Origin", testAuthOrigin+".attacker.example") }},
		{403, func(r *http.Request) { r.Header.Add("Origin", testAuthOrigin) }},
		{403, func(r *http.Request) { r.Method = "GET"; r.Header.Set("Origin", "https://foreign.example") }},
		{403, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-site") }},
		{403, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }},
		{403, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "none") }},
		{403, func(r *http.Request) { r.Method = "GET"; r.Header.Set("Sec-Fetch-Site", "cross-site") }},
		{403, func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "unknown") }},
		{403, func(r *http.Request) {
			r.Header.Add("Sec-Fetch-Site", "same-origin")
			r.Header.Add("Sec-Fetch-Site", "same-origin")
		}},
		{405, func(r *http.Request) { r.Method = "OPTIONS" }}, {405, func(r *http.Request) { r.Method = "TRACE" }},
		{405, func(r *http.Request) { r.Method = "CONNECT" }}, {405, func(r *http.Request) { r.Method = "CUSTOM" }},
		{400, func(r *http.Request) { r.URL.RawPath = "/api/%64ata" }},
		{400, func(r *http.Request) { r.Header.Add("Cookie", SessionCookieName+"="+token) }},
		{431, func(r *http.Request) { r.Header.Add("Cookie", strings.Repeat("x", MaxAuthCookieBytes)) }},
		{415, func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }},
	} {
		r := authRequest("/api/data", "")
		r.AddCookie(authCookie(token, false))
		body := &authUnreadBody{}
		r.Body = body
		tc.change(r)
		w := httptest.NewRecorder()
		guard.ServeHTTP(w, r)
		assertAuthResponse(t, w, tc.status)
		if body.reads != 0 || h.login.attempts != 0 {
			t.Fatal("guard read body or attempted login")
		}
		if tc.status == 405 && w.Header().Get("Allow") != "GET, HEAD, POST, PUT, PATCH, DELETE" {
			t.Fatal("missing allowed methods")
		}
	}
}

func TestHTTPGuardInvalidSessionsShareOnePrivateRejection(t *testing.T) {
	for _, mode := range []string{"missing", "malformed", "unknown", "revoked", "expired", "foreign", "headers only"} {
		t.Run(mode, func(t *testing.T) {
			sessions, clock := sessionFixture(t, DefaultSessionOptions())
			login, err := newLocalLogin(loginAccount(), sessions, DefaultLoginOptions(), func() time.Time { return *clock }, VerifyPassword)
			if err != nil {
				t.Fatal(err)
			}
			h, err := NewHTTPHandler(login, testAuthOrigin)
			if err != nil {
				t.Fatal(err)
			}
			token, _ := issueSyntheticSession(t, sessions, "Operator")
			presented := token
			switch mode {
			case "missing", "headers only":
				presented = ""
			case "malformed":
				presented = "private malformed token"
			case "unknown":
				presented = strings.Repeat("A", SessionTokenChars-1) + "E"
			case "revoked":
				if err := sessions.Revoke(token); err != nil {
					t.Fatal(err)
				}
			case "expired":
				*clock = clock.Add(DefaultSessionOptions().Lifetime)
			case "foreign":
				presented, _ = issueSyntheticSession(t, sessions, "OtherOperator")
				*clock = clock.Add(time.Second)
			}
			guard := mustProtect(t, h, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("rejected session reached next") }))
			r := authRequest("/api/data?token="+token, "")
			r.Method = "GET"
			if presented != "" {
				r.AddCookie(authCookie(presented, false))
			}
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-Forwarded-User", "Operator")
			r = r.WithContext(context.WithValue(r.Context(), authenticatedSessionKey{}, Session{Identity: loginAccount().Identity}))
			body := &authUnreadBody{}
			r.Body = body
			w := httptest.NewRecorder()
			guard.ServeHTTP(w, r)
			assertAuthResponse(t, w, 401)
			if w.Body.String() != "invalid credentials\n" || body.reads != 0 {
				t.Fatal("session state leaked or body read")
			}
			if mode == "foreign" {
				key, _ := sessionKey(presented)
				if sessions.sessions[key].LastSeenAt != clock.Add(-time.Second) {
					t.Fatal("foreign session activity refreshed on denial")
				}
			}
		})
	}
}

func TestHTTPGuardActivityExpiryAndDeniedMutation(t *testing.T) {
	for _, idleOnly := range []bool{true, false} {
		sessions, clock := sessionFixture(t, SessionOptions{Lifetime: 3 * time.Minute, IdleTimeout: time.Minute, Capacity: 2})
		login, err := newLocalLogin(loginAccount(), sessions, DefaultLoginOptions(), func() time.Time { return *clock }, VerifyPassword)
		if err != nil {
			t.Fatal(err)
		}
		h, err := NewHTTPHandler(login, testAuthOrigin)
		if err != nil {
			t.Fatal(err)
		}
		token, issued := issueSyntheticSession(t, sessions, "Operator")
		start := *clock
		calls := 0
		guard := mustProtect(t, h, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			s, ok := SessionFromContext(r.Context())
			if !ok || s.LastSeenAt != *clock || s.ExpiresAt != issued.ExpiresAt {
				t.Fatal("wrong activity/deadline in context")
			}
			w.WriteHeader(204)
		}))
		request := func(method string, status int) {
			r := authRequest("/api/data", "")
			r.Method = method
			r.Header.Del("Origin")
			r.AddCookie(authCookie(token, false))
			w := httptest.NewRecorder()
			guard.ServeHTTP(w, r)
			assertAuthResponse(t, w, status)
		}
		*clock = start.Add(59 * time.Second)
		request("POST", 403)
		key, _ := sessionKey(token)
		if sessions.sessions[key].LastSeenAt != issued.LastSeenAt {
			t.Fatal("denied mutation refreshed idle timeout")
		}
		if idleOnly {
			*clock = start.Add(time.Minute)
			request("GET", 401)
			if calls != 0 {
				t.Fatal("idle-expired session reached next")
			}
		} else {
			for _, seconds := range []int{59, 118, 177} {
				*clock = start.Add(time.Duration(seconds) * time.Second)
				request("GET", 204)
			}
			*clock = start.Add(3 * time.Minute)
			request("GET", 401)
			if calls != 3 {
				t.Fatal("absolute deadline extended")
			}
		}
	}
}

func TestHTTPGuardSetupMetadataOwnershipAndAllowedMethods(t *testing.T) {
	h, sessions := httpAuthFixture(t, DefaultLoginOptions(), VerifyPassword)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok || s.Identity != loginAccount().Identity {
			t.Fatal("wrong metadata")
		}
		s.Identity.Username = "mutated copy"
		again, _ := SessionFromContext(r.Context())
		if again.Identity != loginAccount().Identity {
			t.Fatal("metadata alias")
		}
		w.WriteHeader(204)
	})
	for _, handler := range []*HTTPHandler{nil, {}} {
		if guard, err := handler.Protect(next); guard != nil || err != ErrInvalidHTTPAuth {
			t.Fatal("invalid guard setup accepted")
		}
	}
	if guard, err := h.Protect(nil); guard != nil || err != ErrInvalidHTTPAuth {
		t.Fatal("nil next accepted")
	}
	guard := mustProtect(t, h, next)
	for _, path := range []string{LoginPath, LogoutPath, "/health", "/static/index.html"} {
		r := authRequest(path, "")
		r.Method = "GET"
		w := httptest.NewRecorder()
		guard.ServeHTTP(w, r)
		assertAuthResponse(t, w, 401) // no built-in public-path exemption
	}
	for _, ctx := range []context.Context{nil, context.Background(), context.WithValue(context.Background(), "authenticatedSessionKey", Session{Identity: loginAccount().Identity})} {
		if s, ok := SessionFromContext(ctx); ok || s != (Session{}) {
			t.Fatal("untrusted metadata key accepted")
		}
	}
	token, _ := issueSyntheticSession(t, sessions, "Operator")
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"} {
		r := authRequest("/api/data?q=synthetic", "")
		r.Method = method
		if method == "GET" || method == "HEAD" {
			r.Header.Del("Origin")
			r.Header.Set("Sec-Fetch-Site", "none")
		} else {
			r.Header.Set("Sec-Fetch-Site", "same-origin")
		}
		r.Header.Set("X-Forwarded-User", "OtherOperator")
		r.AddCookie(authCookie(token, false))
		original := Session{Identity: LocalIdentity{Username: "Previous"}}
		r = r.WithContext(context.WithValue(r.Context(), authenticatedSessionKey{}, original))
		body := &authUnreadBody{}
		r.Body = body
		w := httptest.NewRecorder()
		guard.ServeHTTP(w, r)
		assertAuthResponse(t, w, 204)
		if previous, _ := SessionFromContext(r.Context()); previous != original || body.reads != 0 {
			t.Fatal("guard changed caller context or read body")
		}
		if !strings.Contains(w.Header().Get("Vary"), "Sec-Fetch-Site") {
			t.Fatal("missing vary policy")
		}
	}
}

func TestHTTPGuardClockAndCancellationFailClosed(t *testing.T) {
	for _, mode := range []string{"zero", "backward", "canceled before", "canceled during"} {
		h, sessions := httpAuthFixture(t, DefaultLoginOptions(), VerifyPassword)
		token, issued := issueSyntheticSession(t, sessions, "Operator")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		switch mode {
		case "zero":
			sessions.now = func() time.Time { return time.Time{} }
		case "backward":
			sessions.now = func() time.Time { return issued.CreatedAt.Add(-time.Second) }
		case "canceled before":
			cancel()
			sessions.now = func() time.Time { t.Fatal("pre-canceled guard resolved token"); return time.Time{} }
		case "canceled during":
			sessions.now = func() time.Time { cancel(); return issued.CreatedAt }
		}
		guard := mustProtect(t, h, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unavailable request reached next") }))
		r := authRequest("/api/data", "").WithContext(ctx)
		r.AddCookie(authCookie(token, false))
		w := httptest.NewRecorder()
		guard.ServeHTTP(w, r)
		assertAuthResponse(t, w, 503)
	}
}

func TestHTTPGuardConcurrentRequestsAndSubsequentRevocation(t *testing.T) {
	sessions, err := NewSessionStore(DefaultSessionOptions())
	if err != nil {
		t.Fatal(err)
	}
	login, err := NewLocalLogin(loginAccount(), sessions, DefaultLoginOptions())
	if err != nil {
		t.Fatal(err)
	}
	h, err := NewHTTPHandler(login, testAuthOrigin)
	if err != nil {
		t.Fatal(err)
	}
	token, _ := issueSyntheticSession(t, sessions, "Operator")
	var calls atomic.Int32
	guard := mustProtect(t, h, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s, ok := SessionFromContext(r.Context())
		if !ok || s.Identity != loginAccount().Identity {
			t.Error("concurrent metadata lost")
		}
		calls.Add(1)
		w.WriteHeader(204)
	}))
	var workers sync.WaitGroup
	for range 32 {
		workers.Go(func() {
			r := authRequest("/api/data", "")
			r.Method = "GET"
			r.AddCookie(authCookie(token, false))
			w := httptest.NewRecorder()
			guard.ServeHTTP(w, r)
			assertAuthResponse(t, w, 204)
		})
	}
	workers.Wait()
	if err := sessions.Revoke(token); err != nil {
		t.Fatal(err)
	}
	r := authRequest("/api/data", "")
	r.AddCookie(authCookie(token, false))
	w := httptest.NewRecorder()
	guard.ServeHTTP(w, r)
	assertAuthResponse(t, w, 401)
	if calls.Load() != 32 {
		t.Fatal("revoked request reached next or valid request lost")
	}
	// Revocation protects subsequent admission; it does not interrupt a handler
	// that already received an authenticated request snapshot.
	secondToken, _ := issueSyntheticSession(t, sessions, "Operator")
	entered, finish, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	inFlight := mustProtect(t, h, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-finish
		w.WriteHeader(204)
	}))
	r = authRequest("/api/data", "")
	r.AddCookie(authCookie(secondToken, false))
	w = httptest.NewRecorder()
	go func() { defer close(done); inFlight.ServeHTTP(w, r) }()
	<-entered
	if err := sessions.Revoke(secondToken); err != nil {
		t.Fatal(err)
	}
	close(finish)
	<-done
	assertAuthResponse(t, w, 204)
	w = httptest.NewRecorder()
	inFlight.ServeHTTP(w, r)
	assertAuthResponse(t, w, 401)
}
