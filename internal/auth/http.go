package auth

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

const (
	LoginPath          = "/api/auth/login"
	LogoutPath         = "/api/auth/logout"
	SessionCookieName  = "__Host-queueatlas_session"
	MaxLoginBodyBytes  = 4096
	MaxAuthCookieBytes = 4096
)

var ErrInvalidHTTPAuth = errors.New("invalid HTTP authentication configuration")

// HTTPHandler serves login/logout and can protect data handlers with Protect.
// It creates no listener or application routes. Construct once with a shared
// login and a trusted canonical HTTPS origin. TLS must terminate here: forwarded
// headers never establish TLS, identity or origin.
type HTTPHandler struct {
	login  *LocalLogin
	origin string
	host   string
}

func NewHTTPHandler(login *LocalLogin, origin string) (*HTTPHandler, error) {
	host, ok := authOriginHost(origin)
	if !ok || login == nil {
		return nil, ErrInvalidHTTPAuth
	}
	login.mu.Lock()
	ready := login.now != nil && login.verify != nil && login.sessions != nil &&
		login.account.Validate() == nil && login.options.Validate() == nil
	login.mu.Unlock()
	if !ready {
		return nil, ErrInvalidHTTPAuth
	}
	return &HTTPHandler{login: login, origin: origin, host: host}, nil
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.serveAuth(w, r, nil)
}

func (h *HTTPHandler) serveAuth(w http.ResponseWriter, r *http.Request, render WebLoginRenderer) {
	authResponseHeaders(w)
	if h == nil || h.login == nil || h.origin == "" {
		authLoginError(w, r, http.StatusServiceUnavailable, render)
		return
	}
	if r.URL == nil || r.URL.RawPath != "" || (r.URL.Path != LoginPath && r.URL.Path != LogoutPath) {
		authLoginError(w, r, http.StatusNotFound, render)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		authLoginError(w, r, http.StatusMethodNotAllowed, render)
		return
	}
	if !h.requestAllowed(r, true) {
		authLoginError(w, r, http.StatusForbidden, render)
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery {
		authLoginError(w, r, http.StatusBadRequest, render)
		return
	}
	if len(r.Header.Values("Content-Encoding")) != 0 {
		authLoginError(w, r, http.StatusUnsupportedMediaType, render)
		return
	}
	oldToken, status := authCookieToken(r)
	if status != 0 {
		authLoginError(w, r, status, render)
		return
	}
	if r.URL.Path == LogoutPath {
		h.logout(w, r, oldToken, render)
		return
	}
	h.connect(w, r, oldToken, render)
}

func (h *HTTPHandler) connect(w http.ResponseWriter, r *http.Request, oldToken string, render WebLoginRenderer) {
	contentTypes := r.Header.Values("Content-Type")
	if len(contentTypes) != 1 {
		authLoginError(w, r, http.StatusUnsupportedMediaType, render)
		return
	}
	mediaType, params, err := mime.ParseMediaType(contentTypes[0])
	if err != nil || mediaType != "application/x-www-form-urlencoded" || len(params) > 1 ||
		(len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) {
		authLoginError(w, r, http.StatusUnsupportedMediaType, render)
		return
	}
	body := http.MaxBytesReader(w, r.Body, MaxLoginBodyBytes)
	defer body.Close()
	raw, err := io.ReadAll(body)
	defer clear(raw)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			authLoginError(w, r, http.StatusRequestEntityTooLarge, render)
		} else {
			authLoginError(w, r, http.StatusBadRequest, render)
		}
		return
	}
	form, err := url.ParseQuery(string(raw))
	if err != nil || len(form) != 2 || len(form["username"]) != 1 || len(form["password"]) != 1 {
		authLoginError(w, r, http.StatusBadRequest, render)
		return
	}
	password := []byte(form["password"][0])
	defer clear(password)
	token, _, err := h.login.Login(r.Context(), form["username"][0], password)
	if err != nil {
		switch err {
		case ErrInvalidCredentials:
			authLoginError(w, r, http.StatusUnauthorized, render)
		case ErrLoginLimited:
			authLoginError(w, r, http.StatusTooManyRequests, render)
		default:
			authLoginError(w, r, http.StatusServiceUnavailable, render)
		}
		return
	}
	completed := false
	defer func() {
		if !completed {
			_ = h.login.sessions.Revoke(token)
		}
	}()
	if r.Context().Err() != nil || h.login.sessions.Revoke(oldToken) != nil {
		authLoginError(w, r, http.StatusServiceUnavailable, render)
		return
	}
	http.SetCookie(w, authCookie(token, false))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if render == nil {
		w.WriteHeader(http.StatusOK)
	} else {
		w.Header().Set("Location", WebLoginSuccessPath)
		w.WriteHeader(http.StatusSeeOther)
	}
	const message = "authenticated\n"
	n, err := io.WriteString(w, message)
	// Headers may already be sent: invalidate on failure/short write/panic or
	// detected cancellation, never retry output. Successful Write is not receipt.
	completed = err == nil && n == len(message) && r.Context().Err() == nil
}

func (h *HTTPHandler) logout(w http.ResponseWriter, r *http.Request, token string, render WebLoginRenderer) {
	body := http.MaxBytesReader(w, r.Body, 0)
	defer body.Close()
	if raw, err := io.ReadAll(body); err != nil || len(raw) != 0 {
		clear(raw)
		authLoginError(w, r, http.StatusBadRequest, render)
		return
	}
	if render != nil && r.Context().Err() != nil {
		authLoginError(w, r, http.StatusServiceUnavailable, render)
		return
	}
	if err := h.login.sessions.Revoke(token); err != nil {
		authLoginError(w, r, http.StatusServiceUnavailable, render)
		return
	}
	http.SetCookie(w, authCookie("", true))
	if render == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Location", WebLoginPath)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusSeeOther)
	// Revocation is already committed, even if redirect delivery fails.
	// Never restore a session after logout; a retry remains idempotent.
	_, _ = io.WriteString(w, "logged out\n")
}

func authCookie(token string, remove bool) *http.Cookie {
	cookie := &http.Cookie{Name: SessionCookieName, Value: token, Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}
	if remove {
		cookie.MaxAge = -1
	}
	return cookie // browser session cookie; server deadlines remain authoritative
}

func authHTTPError(w http.ResponseWriter, status int) {
	// Only fixed public strings; never http.Error with a parser/context/IO error.
	messages := map[int]string{
		400: "invalid request", 401: "invalid credentials", 403: "request denied",
		404: "not found", 405: "method not allowed", 413: "request too large",
		415: "unsupported request encoding", 429: "login admission limited",
		431: "request headers too large", 503: "authentication unavailable",
	}
	http.Error(w, messages[status], status)
}

func authOriginHost(origin string) (string, bool) {
	if len(origin) == 0 || len(origin) > 300 || origin != strings.ToLower(origin) {
		return "", false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" ||
		u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || origin != "https://"+u.Host {
		return "", false
	}
	host := u.Hostname()
	authority := host
	if addr, err := netip.ParseAddr(host); err == nil {
		if addr.Zone() != "" || addr.String() != host {
			return "", false
		}
		if addr.Is6() {
			authority = "[" + host + "]"
		}
	} else {
		if len(host) == 0 || len(host) > 253 || strings.Trim(host, "0123456789.") == "" {
			return "", false
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return "", false
			}
			for _, b := range []byte(label) {
				if !(b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '-') {
					return "", false
				}
			}
		}
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || n == 443 || strconv.Itoa(n) != port {
			return "", false
		}
		authority += ":" + port
	} else if strings.HasSuffix(u.Host, ":") {
		return "", false
	}
	if u.Host != authority {
		return "", false
	}
	return u.Host, true
}
