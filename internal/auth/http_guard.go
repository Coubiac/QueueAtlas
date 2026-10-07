package auth

import (
	"context"
	"net/http"
)

type authenticatedSessionKey struct{}

// SessionFromContext returns independent metadata installed by Protect, never a
// bearer. It is request-local, not an authorization role or transferable proof.
// HTTP inputs cannot select the private context key; trusted Go code still can
// manufacture contexts. Protect always resolves the cookie anew before replacing
// any previous metadata. Do not retain a context as proof for another request.
func SessionFromContext(ctx context.Context) (Session, bool) {
	if ctx == nil {
		return Session{}, false
	}
	session, ok := ctx.Value(authenticatedSessionKey{}).(Session)
	return session, ok
}

// Protect wraps every request to next; there are no public-path exemptions. Mount
// login/logout separately. GET/HEAD in next must be read-only; POST/PUT/PATCH/DELETE
// require the configured Origin. next must enforce its own route, body/query and
// application permissions and preserve response cache/CORS policy. No listener
// or application route is created. A nil/uninitialized setup fails at construction.
func (h *HTTPHandler) Protect(next http.Handler) (http.Handler, error) {
	if h == nil || next == nil {
		return nil, ErrInvalidHTTPAuth
	}
	if _, err := NewHTTPHandler(h.login, h.origin); err != nil {
		return nil, ErrInvalidHTTPAuth
	}
	return &protectedHTTPHandler{auth: h, next: next}, nil
}

type protectedHTTPHandler struct {
	auth *HTTPHandler
	next http.Handler
}

func (p *protectedHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authResponseHeaders(w)
	if p == nil || p.auth == nil || p.next == nil {
		authHTTPError(w, http.StatusServiceUnavailable)
		return
	}
	mutation := false
	switch r.Method {
	case http.MethodGet, http.MethodHead:
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		mutation = true
	default:
		w.Header().Set("Allow", "GET, HEAD, POST, PUT, PATCH, DELETE")
		authHTTPError(w, http.StatusMethodNotAllowed)
		return
	}
	if r.URL == nil || r.URL.RawPath != "" {
		authHTTPError(w, http.StatusBadRequest)
		return
	}
	if !p.auth.requestAllowed(r, mutation) {
		authHTTPError(w, http.StatusForbidden)
		return
	}
	if len(r.Header.Values("Content-Encoding")) != 0 {
		authHTTPError(w, http.StatusUnsupportedMediaType)
		return
	}
	token, status := authCookieToken(r)
	if status != 0 {
		authHTTPError(w, status)
		return
	}
	if r.Context().Err() != nil {
		authHTTPError(w, http.StatusServiceUnavailable)
		return
	}
	session, err := p.auth.login.sessions.resolve(token, &p.auth.login.account.Identity)
	if err != nil {
		if err == ErrInvalidSession {
			authHTTPError(w, http.StatusUnauthorized)
		} else {
			authHTTPError(w, http.StatusServiceUnavailable)
		}
		return
	}
	if r.Context().Err() != nil {
		authHTTPError(w, http.StatusServiceUnavailable)
		return
	}
	ctx := context.WithValue(r.Context(), authenticatedSessionKey{}, session)
	p.next.ServeHTTP(w, r.WithContext(ctx))
}

func authResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Add("Vary", "Cookie, Origin, Sec-Fetch-Site")
}

// All checks precede cookie resolution/body reading. Fetch metadata supplements,
// never replaces, Origin on mutations. Missing metadata is supported; missing
// Origin only on read-only requests. No proxy or Referer fallback is trusted.
func (h *HTTPHandler) requestAllowed(r *http.Request, mutation bool) bool {
	if r.TLS == nil || r.Host != h.host {
		return false
	}
	origins := r.Header.Values("Origin")
	if len(origins) != 0 || mutation {
		if len(origins) != 1 || origins[0] != h.origin {
			return false
		}
	}
	sites := r.Header.Values("Sec-Fetch-Site")
	if len(sites) != 0 {
		if len(sites) != 1 || sites[0] != "same-origin" && (mutation || sites[0] != "none") {
			return false
		}
	}
	return true
}

func authCookieToken(r *http.Request) (string, int) {
	// Bound parsing independently of the listener's eventual header limit.
	cookieBytes := 0
	for _, field := range r.Header.Values("Cookie") {
		cookieBytes += len(field)
		if cookieBytes > MaxAuthCookieBytes {
			return "", http.StatusRequestHeaderFieldsTooLarge
		}
	}
	cookies := r.CookiesNamed(SessionCookieName)
	if len(cookies) > 1 {
		return "", http.StatusBadRequest
	}
	if len(cookies) == 1 {
		return cookies[0].Value, 0
	}
	return "", 0
}
