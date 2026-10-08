package httpapi

import (
	"context"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/auth"
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/parser/postfix"
	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/source/importfile"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

const reviewRecipient = `"synthetic</code><img/src=x/onerror=alert(164)>` + "\u202e" + `"@example.test`
const reviewReply = `</pre><script>alert(164)</script><a href="javascript:alert(164)">synthetic</a>`

func importedWebReviewStore(t *testing.T) *sqlite.Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "synthetic-postfix.log")
	lines := "<22>1 2026-10-07T12:00:00Z mx-synthetic postfix/qmgr 1 - - ABC123: from=<synthetic@example.test>, size=42, nrcpt=1 (queue active)\n" +
		"<22>1 2026-10-07T12:00:01Z mx-synthetic postfix/smtp 2 - - ABC123: to=<" + reviewRecipient + ">, relay=remote.example.test[192.0.2.1]:25, dsn=2.0.0, status=sent (" + reviewReply + ")\n" +
		"<22>1 2026-10-07T12:00:02Z mx-synthetic postfix/qmgr 3 - - ABC123: removed\n"
	if err := os.WriteFile(path, []byte(lines), 0600); err != nil {
		t.Fatal(err)
	}
	s := syntheticSearchStore(t)
	importer, err := importfile.New(importfile.Config{
		Identity: source.Identity{ID: "synthetic-web-review", Kind: "import", Name: "synthetic Web review", TrustedHost: "synthetic-postfix"},
		Inputs:   []importfile.Input{{RunID: 164, Path: path}}, MaxFiles: 1, MaxDuration: time.Minute,
		Limits: importfile.GzipLimits{ContentBytes: 16384}, TempDir: dir,
	}, s, func(raw []byte) model.Observation { return postfix.Parse(raw, postfix.Options{}) })
	if err != nil {
		t.Fatal(err)
	}
	if err := importer.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWebReviewImportedHostileValuesAcrossViews(t *testing.T) {
	s := importedWebReviewStore(t)
	guard, token, _ := searchGuardFixture(t)
	options := DefaultSearchOptions()
	options.AllowRawLogs = true
	h, _ := NewConsultationHandler(guard, s, options)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	response := decodeSearchResponse(t, w)
	if len(response.Matches) != 1 || response.Matches[0].Candidate == nil {
		t.Fatal("imported candidate missing")
	}
	id := response.Matches[0].Candidate.ID
	params := url.Values{"instance": {"synthetic-postfix"}, "field": {"recipient"}, "value": {reviewRecipient}, "from": {"2026-10-07T00:00:00Z"}, "until": {"2026-10-08T00:00:00Z"}}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", SearchPagePath+"?"+params.Encode(), token))
	assertSearchPage(t, w, 200)
	if !strings.Contains(w.Body.String(), "ABC123") || !strings.Contains(w.Body.String(), `value="`+html.EscapeString(reviewRecipient)+`"`) || strings.Contains(w.Body.String(), "<img") {
		t.Fatal("search escaped value changed literal matching or became markup")
	}
	for _, path := range []string{CandidatePagePrefix + id, timelinePageURL(id) + "?raw=1"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest("GET", path, token))
		assertSearchPage(t, w, 200)
		if !strings.Contains(w.Body.String(), html.EscapeString(visibleNativeText(reviewRecipient))) || strings.Contains(w.Body.String(), "\u202e") || strings.Contains(w.Body.String(), "<script") || strings.Contains(w.Body.String(), "<img") {
			t.Fatal("imported hostile address hidden or activated")
		}
		if strings.Contains(path, "/events") && !strings.Contains(w.Body.String(), html.EscapeString(reviewReply)) {
			t.Fatal("imported native reply lost")
		}
	}
}

// Explicit opt-in loopback fixture for a human/browser review, never production.
// No trust-store modifications, TLS bypass, real credentials or external logs.
func TestWebBrowserReview(t *testing.T) {
	if os.Getenv("QUEUEATLAS_BROWSER_REVIEW") != "1" {
		t.Skip("manual browser review only")
	}
	s := importedWebReviewStore(t)
	server := httptest.NewUnstartedServer(nil)
	defer server.Close()
	// Reuse the operator's URL when restarting after a correction. Always loopback.
	if rawPort := os.Getenv("QUEUEATLAS_BROWSER_REVIEW_PORT"); rawPort != "" {
		port, err := strconv.ParseUint(rawPort, 10, 16)
		if err != nil || port == 0 {
			t.Fatal("manual browser review port must be 1..65535")
		}
		listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.FormatUint(port, 10))
		if err != nil {
			t.Fatal(err)
		}
		_ = server.Listener.Close()
		server.Listener = listener
	}
	origin := "https://" + server.Listener.Addr().String()
	guard, _, _ := searchGuardAtOrigin(t, origin)
	webAuth, _ := NewWebLoginHandler(guard)
	options := DefaultSearchOptions()
	options.AllowRawLogs = true
	consultation, _ := NewConsultationHandler(guard, s, options)
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case auth.WebLoginPath, auth.WebLogoutPath:
			webAuth.ServeHTTP(w, r)
		case auth.LoginPath, auth.LogoutPath:
			guard.ServeHTTP(w, r)
		default:
			consultation.ServeHTTP(w, r)
		}
	})
	server.Config.ReadHeaderTimeout = 5 * time.Second
	server.Config.ReadTimeout = 10 * time.Second
	server.Config.WriteTimeout = 10 * time.Second
	server.Config.MaxHeaderBytes = 8192
	server.StartTLS()
	waitForWebReview(t, origin+"/login")
}

// Static synthetic HTML only: no login, sessions, proxy or HTTP auth bypass.
// Useful for renderer/keyboard/viewport review when HTTPS trust is unavailable.
func TestWebRenderedBrowserReview(t *testing.T) {
	if os.Getenv("QUEUEATLAS_BROWSER_REVIEW") != "render" {
		t.Skip("manual rendering review only")
	}
	s := importedWebReviewStore(t)
	guard, token, _ := searchGuardFixture(t)
	options := DefaultSearchOptions()
	options.AllowRawLogs = true
	h, _ := NewConsultationHandler(guard, s, options)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, searchReadRequest("GET", searchTestURL(), token))
	id := decodeSearchResponse(t, w).Matches[0].Candidate.ID
	snapshots := make(map[string]*httptest.ResponseRecorder)
	for _, path := range []string{SearchPagePath, CandidatePagePrefix + id, timelinePageURL(id), timelinePageURL(id) + "?raw=1", timelinePageURL(id) + "?limit=1", timelinePageURL(id) + "?limit=1&raw=1"} {
		target := path
		if path == SearchPagePath {
			target = strings.Replace(searchTestURL(), SearchPath, SearchPagePath, 1)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, searchReadRequest("GET", target, token))
		assertSearchPage(t, w, 200)
		snapshots[path] = w
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "static synthetic preview only", 405)
			return
		}
		path := r.URL.Path
		if strings.HasSuffix(path, "/events") {
			if r.URL.Query().Get("limit") == "1" {
				path += "?limit=1"
			}
			if r.URL.Query().Get("raw") == "1" {
				if strings.Contains(path, "?") {
					path += "&raw=1"
				} else {
					path += "?raw=1"
				}
			}
		}
		snapshot := snapshots[path]
		if snapshot == nil {
			http.NotFound(w, r)
			return
		}
		for name, values := range snapshot.Header() {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.WriteHeader(snapshot.Code)
		if r.Method != "HEAD" {
			_, _ = w.Write(snapshot.Body.Bytes())
		}
	}))
	defer server.Close()
	waitForWebReview(t, server.URL+SearchPagePath)
}

func waitForWebReview(t *testing.T, startURL string) {
	t.Helper()
	stop := filepath.Join(t.TempDir(), "stop-review")
	fmt.Printf("BROWSER_REVIEW_URL=%s\nBROWSER_REVIEW_STOP=%s\n", startURL, stop)
	timer := time.NewTimer(30 * time.Minute)
	defer timer.Stop()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timer.C:
			t.Fatal("manual browser review timed out")
		case <-ticker.C:
			if _, err := os.Stat(stop); err == nil {
				return
			}
		}
	}
}
