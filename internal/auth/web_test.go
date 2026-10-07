package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testWebRenderer(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write([]byte("synthetic public form"))
	}
}

func TestBrowserLoginConstructionPublicFormAndGuards(t *testing.T) {
	var hashes atomic.Int32
	h, sessions := httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { hashes.Add(1); return false, nil })
	if handler, err := NewBrowserLoginHandler(nil, testWebRenderer); handler != nil || err != ErrInvalidHTTPAuth {
		t.Fatal("nil guard accepted")
	}
	if handler, err := NewBrowserLoginHandler(&HTTPHandler{}, testWebRenderer); handler != nil || err != ErrInvalidHTTPAuth {
		t.Fatal("uninitialized guard accepted")
	}
	if handler, err := NewBrowserLoginHandler(h, nil); handler != nil || err != ErrInvalidHTTPAuth {
		t.Fatal("nil renderer accepted")
	}
	web, err := NewBrowserLoginHandler(h, testWebRenderer)
	if err != nil {
		t.Fatal(err)
	}
	oldToken, _, _ := sessions.Issue(loginAccount().Identity)
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		web.ServeHTTP(w, httptest.NewRequest(method, testAuthOrigin+WebLoginPath, nil))
		assertAuthResponse(t, w, 200)
		if method == "HEAD" && w.Body.Len() != 0 || hashes.Load() != 0 || len(sessions.sessions) != 1 {
			t.Fatal("public form consumed login or session budget")
		}
	}
	for _, tc := range []struct {
		change func(*http.Request)
		status int
	}{
		{func(r *http.Request) { r.TLS = nil; r.Header.Set("X-Forwarded-Proto", "https") }, 403},
		{func(r *http.Request) {
			r.Host = "foreign.example"
			r.Header.Set("X-Forwarded-Host", "queueatlas.example")
		}, 403},
		{func(r *http.Request) { r.Header.Del("Origin") }, 403},
		{func(r *http.Request) { r.Header.Set("Origin", "null") }, 403},
		{func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }, 403},
		{func(r *http.Request) { r.URL.RawQuery = "next=https://foreign.example/private" }, 400},
		{func(r *http.Request) { r.URL.ForceQuery = true }, 400},
		{func(r *http.Request) { r.URL.RawPath = "/%6cogin" }, 400},
		{func(r *http.Request) { r.URL.Fragment = "private" }, 400},
		{func(r *http.Request) { r.URL.Path += "/" }, 404},
		{func(r *http.Request) { r.Method = "PUT" }, 405},
		{func(r *http.Request) { r.Method = "GET"; r.Header.Del("Origin"); r.ContentLength = 1 }, 400},
		{func(r *http.Request) { r.Header.Set("Content-Encoding", "gzip") }, 415},
		{func(r *http.Request) {
			r.Header.Set("Cookie", SessionCookieName+"="+oldToken+"; "+SessionCookieName+"="+oldToken)
		}, 400},
		{func(r *http.Request) { r.Header.Set("Cookie", strings.Repeat("c", MaxAuthCookieBytes+1)) }, 431},
	} {
		r := authRequest(WebLoginPath, "")
		body := &authUnreadBody{}
		r.Body = body
		r.Header.Set("Cookie", SessionCookieName+"="+oldToken)
		tc.change(r)
		w := httptest.NewRecorder()
		web.ServeHTTP(w, r)
		assertAuthResponse(t, w, tc.status)
		if body.reads != 0 || hashes.Load() != 0 {
			t.Fatal("browser guard read/hash before rejection")
		}
		if _, err := sessions.Resolve(oldToken); err != nil {
			t.Fatal("invalid browser request revoked old session")
		}
	}
}

func TestBrowserLoginSharesAPIBudgetAndFixedRedirect(t *testing.T) {
	options := DefaultLoginOptions()
	options.AttemptLimit = 2
	var hashes atomic.Int32
	h, sessions := httpAuthFixture(t, options, func([]byte, string) (bool, error) { hashes.Add(1); return true, nil })
	web, _ := NewBrowserLoginHandler(h, testWebRenderer)
	oldToken, _, _ := sessions.Issue(loginAccount().Identity)
	r := authRequest(WebLoginPath, authForm("Operator", "synthetic secret phrase"))
	r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: oldToken})
	w := httptest.NewRecorder()
	web.ServeHTTP(w, r)
	if w.Code != 303 || w.Header().Get("Location") != WebLoginSuccessPath || w.Header().Get("Cache-Control") != "no-store" || len(w.Result().Cookies()) != 1 {
		t.Fatal("browser did not use fixed redirect/session policy")
	}
	assertSessionCookie(t, w.Result().Cookies()[0], false)
	if _, err := sessions.Resolve(oldToken); err != ErrInvalidSession {
		t.Fatal("browser login did not rotate old session")
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, authRequest(LoginPath, authForm("Operator", "synthetic secret phrase")))
	assertAuthResponse(t, w, 200)
	if w.Body.String() != "authenticated\n" {
		t.Fatal("API login response changed")
	}
	w = httptest.NewRecorder()
	web.ServeHTTP(w, authRequest(WebLoginPath, authForm("Operator", "synthetic secret phrase")))
	assertAuthResponse(t, w, 429)
	if hashes.Load() != 2 {
		t.Fatal("browser and API obtained separate admission budgets")
	}
}

func TestBrowserLoginFailedWritesAndCancellationRevokeIssuedSession(t *testing.T) {
	h, sessions := httpAuthFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { return true, nil })
	web, _ := NewBrowserLoginHandler(h, testWebRenderer)
	for _, mode := range []string{"error", "short", "panic"} {
		panicked := false
		func() {
			defer func() {
				if recover() != nil {
					panicked = true
				}
			}()
			w := &authFailedWriter{header: make(http.Header), mode: mode}
			web.ServeHTTP(w, authRequest(WebLoginPath, authForm("Operator", "synthetic secret phrase")))
		}()
		if panicked != (mode == "panic") || len(sessions.sessions) != 0 {
			t.Fatal("failed redirect retained new session")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sessions.random = authCancelReader{sessions.random, cancel}
	w := httptest.NewRecorder()
	web.ServeHTTP(w, authRequest(WebLoginPath, authForm("Operator", "synthetic secret phrase")).WithContext(ctx))
	assertAuthResponse(t, w, 503)
	if len(sessions.sessions) != 0 {
		t.Fatal("cancelled browser entropy retained session")
	}
}
