package auth

import "net/http"

const (
	WebLoginPath  = "/login"
	WebLogoutPath = "/logout"
	// Fixed local destination, never selected from a request or log field.
	WebLoginSuccessPath = "/messages"
)

// WebLoginRenderer is trusted application rendering for the public login form
// (200) or a fixed login error. Do not reflect credentials/query/error details,
// log request bodies, change cookies or weaken cache/CORS/security headers.
type WebLoginRenderer func(http.ResponseWriter, *http.Request, int)

type browserLoginHandler struct {
	auth   *HTTPHandler
	render WebLoginRenderer
}

// NewBrowserLoginHandler reuses the exact same local login, admission, sessions
// and origin as the API. Successful writes issue a cookie and redirect to the
// fixed Web destination; failed/short writes revoke the newly issued session.
// The public form is /login; /logout accepts only an origin-checked POST.
// Mount protected consultation and API auth separately.
func NewBrowserLoginHandler(h *HTTPHandler, render WebLoginRenderer) (http.Handler, error) {
	if h == nil || render == nil {
		return nil, ErrInvalidHTTPAuth
	}
	if _, err := NewHTTPHandler(h.login, h.origin); err != nil {
		return nil, ErrInvalidHTTPAuth
	}
	return &browserLoginHandler{auth: h, render: render}, nil
}

func (b *browserLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL != nil && (r.URL.Path == WebLoginPath || r.URL.Path == WebLogoutPath) && r.URL.RawPath == "" && r.URL.Fragment == "" && r.Method == http.MethodPost {
		// Clone request metadata. The shared API validator/parser owns the original
		// bounded body; no intermediate response buffer or second password copy.
		clone := r.Clone(r.Context())
		clone.URL.Path = LoginPath
		if r.URL.Path == WebLogoutPath {
			clone.URL.Path = LogoutPath
		}
		b.auth.serveAuth(w, clone, b.render)
		return
	}
	authResponseHeaders(w)
	status := http.StatusOK
	switch {
	case r.URL == nil || r.URL.RawPath != "" || r.URL.Fragment != "":
		status = http.StatusBadRequest
	case r.URL.Path == WebLogoutPath:
		w.Header().Set("Allow", http.MethodPost)
		status = http.StatusMethodNotAllowed
	case r.URL.Path != WebLoginPath:
		status = http.StatusNotFound
	case r.Method != http.MethodGet && r.Method != http.MethodHead:
		w.Header().Set("Allow", "GET, HEAD, POST")
		status = http.StatusMethodNotAllowed
	case !b.auth.requestAllowed(r, false):
		status = http.StatusForbidden
	case r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength != 0 || len(r.TransferEncoding) != 0:
		status = http.StatusBadRequest
	case len(r.Header.Values("Content-Encoding")) != 0:
		status = http.StatusUnsupportedMediaType
	default:
		if _, cookieStatus := authCookieToken(r); cookieStatus != 0 {
			status = cookieStatus
		} else if r.Context().Err() != nil {
			status = http.StatusServiceUnavailable
		}
	}
	b.render(w, r, status) // no session issuance/resolution on a public form GET
}

func authLoginError(w http.ResponseWriter, r *http.Request, status int, render WebLoginRenderer) {
	if render == nil {
		authHTTPError(w, status)
		return
	}
	render(w, r, status)
}
