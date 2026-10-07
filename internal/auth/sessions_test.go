package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

func sessionFixture(t *testing.T, options SessionOptions) (*SessionStore, *time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	// Distinct deterministic blocks, only for tests. Public construction uses rand.
	var entropy []byte
	for i := 0; i < 16; i++ {
		entropy = append(entropy, bytes.Repeat([]byte{byte(i)}, SessionTokenBytes)...)
	}
	store, err := newSessionStore(options, func() time.Time { return now }, bytes.NewReader(entropy))
	if err != nil {
		t.Fatal(err)
	}
	return store, &now
}

func issueSyntheticSession(t *testing.T, store *SessionStore, username string) (string, Session) {
	t.Helper()
	token, session, err := store.Issue(LocalIdentity{Username: username})
	if err != nil {
		t.Fatal(err)
	}
	return token, session
}

func TestSessionOptionsAndUninitializedStoresFailSafely(t *testing.T) {
	defaults := DefaultSessionOptions()
	if defaults.Lifetime != 8*time.Hour || defaults.IdleTimeout != 30*time.Minute || defaults.Capacity != 64 || defaults.Validate() != nil {
		t.Fatal("unexpected defaults")
	}
	defaults.Capacity = 1
	if DefaultSessionOptions().Capacity != 64 {
		t.Fatal("shared defaults")
	}
	for _, options := range []SessionOptions{
		{}, {time.Minute - 1, time.Minute, 1}, {MaxSessionLifetime + 1, time.Minute, 1},
		{time.Hour, 0, 1}, {time.Hour, time.Minute - 1, 1}, {time.Hour, time.Hour + 1, 1},
		{time.Hour, time.Minute, 0}, {time.Hour, time.Minute, -1}, {time.Hour, time.Minute, MaxSessions + 1},
	} {
		if store, err := NewSessionStore(options); store != nil || err != ErrInvalidSessionOptions {
			t.Fatal("invalid options constructed store", err)
		}
	}
	for _, options := range []SessionOptions{{time.Minute, time.Minute, 1}, {MaxSessionLifetime, MaxSessionLifetime, MaxSessions}} {
		if options.Validate() != nil {
			t.Fatal("inclusive bounds refused")
		}
	}
	if store, err := newSessionStore(DefaultSessionOptions(), nil, strings.NewReader("")); store != nil || err != ErrInvalidSessionOptions {
		t.Fatal("nil clock accepted")
	}
	if store, err := newSessionStore(DefaultSessionOptions(), time.Now, nil); store != nil || err != ErrInvalidSessionOptions {
		t.Fatal("nil entropy accepted")
	}
	for _, store := range []*SessionStore{nil, {}} {
		if token, session, err := store.Issue(LocalIdentity{Username: "synthetic"}); token != "" || session != (Session{}) || err != ErrSessionUnavailable {
			t.Fatal("uninitialized store issued session", err)
		}
		if session, err := store.Resolve(strings.Repeat("A", SessionTokenChars)); session != (Session{}) || err != ErrSessionUnavailable {
			t.Fatal("uninitialized store resolved session", err)
		}
		if store.Revoke("synthetic-private") != ErrSessionUnavailable || store.RevokeAll() != ErrSessionUnavailable {
			t.Fatal("uninitialized store claimed revocation")
		}
	}
}

func TestSessionPublicTokensOwnershipRevocationAndRestart(t *testing.T) {
	store, err := NewSessionStore(DefaultSessionOptions())
	if err != nil {
		t.Fatal(err)
	}
	first, original := issueSyntheticSession(t, store, "synthetic-first")
	second, _ := issueSyntheticSession(t, store, "synthetic-second")
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(first)
	if err != nil || len(decoded) != SessionTokenBytes || len(first) != SessionTokenChars || first == second {
		t.Fatal("public token not random/canonical/bounded", err)
	}
	key := sha256.Sum256(decoded)
	if stored, ok := store.sessions[key]; !ok || stored != original {
		t.Fatal("store not keyed by one-way verifier")
	}
	original.Identity.Username = "changed"
	original.ExpiresAt = time.Time{}
	resolved, err := store.Resolve(first)
	if err != nil || resolved.Identity.Username != "synthetic-first" || resolved.ExpiresAt.IsZero() {
		t.Fatal("caller mutated stored metadata", err)
	}
	resolved.Identity.Username = "also-changed"
	if store.Revoke(first) != nil || store.Revoke(first) != nil || store.Revoke("malformed") != nil {
		t.Fatal("revocation not idempotent")
	}
	if session, err := store.Resolve(first); session != (Session{}) || err != ErrInvalidSession {
		t.Fatal("revoked session survived", err)
	}
	if _, err := store.Resolve(second); err != nil {
		t.Fatal("revocation removed unrelated session", err)
	}
	if err := store.RevokeAll(); err != nil || len(store.sessions) != 0 {
		t.Fatal("global revocation failed", err)
	}
	if session, err := store.Resolve(second); session != (Session{}) || err != ErrInvalidSession {
		t.Fatal("global revoke left session valid", err)
	}
	restarted, err := NewSessionStore(DefaultSessionOptions())
	if err != nil {
		t.Fatal(err)
	}
	if session, err := restarted.Resolve(first); session != (Session{}) || err != ErrInvalidSession {
		t.Fatal("new store accepted token from prior instance", err)
	}
}

func TestSessionExpiryBoundariesActivityAndCapacity(t *testing.T) {
	options := SessionOptions{Lifetime: 3 * time.Minute, IdleTimeout: time.Minute, Capacity: 2}
	store, clock := sessionFixture(t, options)
	base := *clock
	active, issued := issueSyntheticSession(t, store, "synthetic-active")
	idle, _ := issueSyntheticSession(t, store, "synthetic-idle")
	if token, session, err := store.Issue(LocalIdentity{Username: "synthetic-full"}); token != "" || session != (Session{}) || err != ErrSessionsFull {
		t.Fatal("capacity evicted or overfilled", err)
	}
	*clock = base.Add(59 * time.Second)
	if session, err := store.Resolve(active); err != nil || session.ExpiresAt != issued.ExpiresAt || session.LastSeenAt != *clock {
		t.Fatal("activity extended absolute deadline or failed", err)
	}
	*clock = base.Add(time.Minute)
	if session, err := store.Resolve(idle); session != (Session{}) || err != ErrInvalidSession {
		t.Fatal("idle exact boundary accepted", err)
	}
	replacement, _ := issueSyntheticSession(t, store, "synthetic-replacement")
	if replacement == idle || len(store.sessions) != 2 {
		t.Fatal("expired capacity not reclaimed")
	}
	for _, seconds := range []int{118, 177, 179} {
		*clock = base.Add(time.Duration(seconds) * time.Second)
		if session, err := store.Resolve(active); err != nil || session.ExpiresAt != issued.ExpiresAt {
			t.Fatal("active session expired early or absolute deadline moved", err)
		}
	}
	*clock = base.Add(3 * time.Minute)
	if session, err := store.Resolve(active); session != (Session{}) || err != ErrInvalidSession || len(store.sessions) != 0 {
		t.Fatal("absolute exact boundary accepted or expired entries retained", err)
	}
	// Defaults/options are copied into the store, not borrowed from the caller.
	options.Capacity = 0
	if _, _, err := store.Issue(LocalIdentity{Username: "synthetic-after-expiry"}); err != nil {
		t.Fatal("caller changed store options", err)
	}
}

func TestSessionHostileTokensHaveOnePrivateRejection(t *testing.T) {
	store, clock := sessionFixture(t, DefaultSessionOptions())
	token, _ := issueSyntheticSession(t, store, "synthetic-private-identity")
	secret, _ := base64.RawURLEncoding.DecodeString(token)
	verifier := sha256.Sum256(secret)
	for _, invalid := range []string{
		"", "synthetic-private-token", strings.Repeat("A", 10000), token + "=", token + "\n",
		token[:SessionTokenChars-1] + "B", " " + token[1:], strings.Repeat("/", SessionTokenChars),
		base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{99}, SessionTokenBytes)),
		base64.RawURLEncoding.EncodeToString(verifier[:]), // verifier is not a bearer
	} {
		if session, err := store.Resolve(invalid); session != (Session{}) || err != ErrInvalidSession || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatal("token rejected with data/state or accepted", err)
		}
	}
	*clock = clock.Add(DefaultSessionOptions().IdleTimeout)
	if session, err := store.Resolve(token); session != (Session{}) || err != ErrInvalidSession {
		t.Fatal("expiry diagnostic differed", err)
	}
}

type sessionEntropy struct {
	reader io.Reader
	bytes  int
}

func (r *sessionEntropy) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.bytes += n
	return n, err
}

func TestSessionEntropyCollisionsAndClockFailClosed(t *testing.T) {
	for _, random := range []io.Reader{failedRandom{}, strings.NewReader("partial")} {
		store, err := newSessionStore(DefaultSessionOptions(), time.Now, random)
		if err != nil {
			t.Fatal(err)
		}
		if token, session, err := store.Issue(LocalIdentity{Username: "synthetic"}); token != "" || session != (Session{}) || err != ErrSessionUnavailable || len(store.sessions) != 0 {
			t.Fatal("failed entropy returned token/state or raw error", err)
		}
	}
	store, clock := sessionFixture(t, DefaultSessionOptions())
	random := &sessionEntropy{reader: bytes.NewReader(make([]byte, 4*SessionTokenBytes))}
	store.random = random
	token, original := issueSyntheticSession(t, store, "synthetic-original")
	if retry, session, err := store.Issue(LocalIdentity{Username: "synthetic-collision"}); retry != "" || session != (Session{}) || err != ErrSessionUnavailable || random.bytes != 4*SessionTokenBytes || len(store.sessions) != 1 {
		t.Fatal("collision retries unbounded or live verifier overwritten", err)
	}
	if got, err := store.Resolve(token); err != nil || got != original {
		t.Fatal("collision changed live identity", err)
	}
	*clock = clock.Add(-time.Second)
	if session, err := store.Resolve(token); session != (Session{}) || err != ErrSessionUnavailable {
		t.Fatal("backward clock served session", err)
	}
	*clock = time.Time{}
	if retry, session, err := store.Issue(LocalIdentity{Username: "synthetic-zero-clock"}); retry != "" || session != (Session{}) || err != ErrSessionUnavailable {
		t.Fatal("zero clock issued session", err)
	}
	if store.Revoke(token) != nil || store.RevokeAll() != nil || len(store.sessions) != 0 {
		t.Fatal("clock anomaly blocked revocation")
	}
	for _, delta := range []time.Duration{2 * time.Hour, -time.Second} {
		base := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
		calls := 0
		clock := func() time.Time {
			calls++
			if calls == 1 {
				return base
			}
			return base.Add(delta)
		}
		postEntropy, err := newSessionStore(DefaultSessionOptions(), clock, bytes.NewReader(make([]byte, SessionTokenBytes)))
		if err != nil {
			t.Fatal(err)
		}
		token, session, err := postEntropy.Issue(LocalIdentity{Username: "synthetic-delayed"})
		if delta < 0 {
			if token != "" || session != (Session{}) || err != ErrSessionUnavailable || len(postEntropy.sessions) != 0 {
				t.Fatal("post-entropy backward clock published session", err)
			}
		} else if err != nil || session.CreatedAt != base.Add(delta) || session.ExpiresAt != base.Add(delta+DefaultSessionOptions().Lifetime) {
			t.Fatal("session timestamp predates completed entropy", err)
		}
	}
	// Invalid identities/capacity never consume entropy.
	bounded, _ := sessionFixture(t, SessionOptions{Lifetime: time.Minute, IdleTimeout: time.Minute, Capacity: 1})
	bounded.random = forbiddenRandom{t}
	if token, session, err := bounded.Issue(LocalIdentity{}); token != "" || session != (Session{}) || err != ErrInvalidLocalIdentity {
		t.Fatal("invalid identity used", err)
	}
	bounded.random = bytes.NewReader(make([]byte, SessionTokenBytes))
	issueSyntheticSession(t, bounded, "synthetic-full")
	bounded.random = forbiddenRandom{t}
	if token, session, err := bounded.Issue(LocalIdentity{Username: "synthetic-loser"}); token != "" || session != (Session{}) || err != ErrSessionsFull {
		t.Fatal("full store consumed entropy", err)
	}
}

func TestSessionConcurrentCapacityResolveAndRevoke(t *testing.T) {
	options := DefaultSessionOptions()
	options.Capacity = 8
	store, err := NewSessionStore(options)
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		token   string
		session Session
		err     error
	}
	results := make(chan result, 32)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			token, session, err := store.Issue(LocalIdentity{Username: fmt.Sprintf("synthetic-%d", index)})
			results <- result{token, session, err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	var winners []result
	for r := range results {
		if r.err == nil {
			winners = append(winners, r)
		} else if r.err != ErrSessionsFull || r.token != "" || r.session != (Session{}) {
			t.Fatal("concurrent issue returned partial state", r.err)
		}
	}
	if len(winners) != options.Capacity || len(store.sessions) != options.Capacity {
		t.Fatal("concurrent capacity exceeded or active entries evicted")
	}
	for _, winner := range winners {
		wg.Add(1)
		go func(r result) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				if session, err := store.Resolve(r.token); err != nil || session.Identity != r.session.Identity || session.ExpiresAt != r.session.ExpiresAt {
					t.Error("concurrent resolve changed identity/deadline", err)
				}
			}
			if err := store.Revoke(r.token); err != nil {
				t.Error(err)
			}
		}(winner)
	}
	wg.Wait()
	if len(store.sessions) != 0 {
		t.Fatal("concurrent revocation left sessions")
	}
}
