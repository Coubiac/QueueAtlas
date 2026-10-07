package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"sync"
	"time"
)

const (
	SessionTokenBytes  = 32
	SessionTokenChars  = 43
	MaxSessions        = 1024
	MinSessionTimeout  = time.Minute
	MaxSessionLifetime = 24 * time.Hour
)

var (
	ErrInvalidSessionOptions = errors.New("invalid session options")
	ErrInvalidSession        = errors.New("invalid or expired session")
	ErrSessionsFull          = errors.New("session capacity reached")
	ErrSessionUnavailable    = errors.New("session store unavailable")
)

type SessionOptions struct {
	Lifetime    time.Duration
	IdleTimeout time.Duration
	Capacity    int
}

func DefaultSessionOptions() SessionOptions {
	return SessionOptions{Lifetime: 8 * time.Hour, IdleTimeout: 30 * time.Minute, Capacity: 64}
}

func (o SessionOptions) Validate() error {
	if o.Lifetime < MinSessionTimeout || o.Lifetime > MaxSessionLifetime ||
		o.IdleTimeout < MinSessionTimeout || o.IdleTimeout > o.Lifetime ||
		o.Capacity < 1 || o.Capacity > MaxSessions {
		return ErrInvalidSessionOptions
	}
	return nil
}

// Session contains metadata, never the bearer token or password. Returned values
// are independent copies; an identity here is not a role or proof of login.
type Session struct {
	Identity   LocalIdentity
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time // absolute deadline, never extended by Resolve
}

// SessionStore is process-local and bounded; restart loses every session. Use the
// constructor and share its pointer, never copy after first use. The caller must
// authenticate before Issue and must not log returned tokens. No HTTP/cookie,
// credential verification, persistence or authorization is provided here.
type SessionStore struct {
	mu       sync.Mutex
	options  SessionOptions
	sessions map[[sha256.Size]byte]Session
	now      func() time.Time
	random   io.Reader
	lastTime time.Time
}

func NewSessionStore(options SessionOptions) (*SessionStore, error) {
	return newSessionStore(options, time.Now, rand.Reader)
}

// Dependency seams stay private; production always uses the system clock/CSPRNG.
func newSessionStore(options SessionOptions, now func() time.Time, random io.Reader) (*SessionStore, error) {
	if options.Validate() != nil || now == nil || random == nil {
		return nil, ErrInvalidSessionOptions
	}
	return &SessionStore{options: options, sessions: make(map[[sha256.Size]byte]Session), now: now, random: random}, nil
}

// Issue never accepts a client token, evicts an active session or reuses a live
// verifier. It returns an empty token and zero metadata on every failure.
func (s *SessionStore) Issue(identity LocalIdentity) (string, Session, error) {
	if identity.Validate() != nil {
		return "", Session{}, ErrInvalidLocalIdentity
	}
	if s == nil {
		return "", Session{}, ErrSessionUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		return "", Session{}, ErrSessionUnavailable
	}
	now, err := s.currentTime()
	if err != nil {
		return "", Session{}, err
	}
	s.prune(now)
	if len(s.sessions) >= s.options.Capacity {
		return "", Session{}, ErrSessionsFull
	}
	for attempt := 0; attempt < 3; attempt++ {
		var secret [SessionTokenBytes]byte
		if _, err := io.ReadFull(s.random, secret[:]); err != nil {
			clear(secret[:])
			return "", Session{}, ErrSessionUnavailable
		}
		key := sha256.Sum256(secret[:])
		if _, exists := s.sessions[key]; exists {
			clear(secret[:])
			continue
		}
		// Timestamp after entropy: a slow source must not return an already-aged
		// session. Clock anomalies fail closed; no session is installed.
		now, err = s.currentTime()
		if err != nil {
			clear(secret[:])
			return "", Session{}, err
		}
		s.prune(now)
		session := Session{Identity: identity, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(s.options.Lifetime)}
		token := base64.RawURLEncoding.EncodeToString(secret[:])
		clear(secret[:])
		s.sessions[key] = session
		return token, session, nil
	}
	return "", Session{}, ErrSessionUnavailable
}

// Resolve accepts only an issued canonical token. Unknown, malformed, expired or
// revoked tokens share one fixed error and return zero metadata. Activity extends
// only the idle deadline; absolute expiry wins even at the exact boundary.
func (s *SessionStore) Resolve(token string) (Session, error) {
	if s == nil {
		return Session{}, ErrSessionUnavailable
	}
	key, ok := sessionKey(token)
	if !ok {
		return Session{}, ErrInvalidSession
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		return Session{}, ErrSessionUnavailable
	}
	now, err := s.currentTime()
	if err != nil {
		return Session{}, err
	}
	s.prune(now)
	session, exists := s.sessions[key]
	if !exists {
		return Session{}, ErrInvalidSession
	}
	session.LastSeenAt = now
	s.sessions[key] = session
	return session, nil
}

// Revoke is idempotent, including unknown/malformed tokens, and does not need
// entropy or a working clock. RevokeAll invalidates all sessions in this store.
func (s *SessionStore) Revoke(token string) error {
	if s == nil {
		return ErrSessionUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		return ErrSessionUnavailable
	}
	if key, ok := sessionKey(token); ok {
		delete(s.sessions, key)
	}
	return nil
}

func (s *SessionStore) RevokeAll() error {
	if s == nil {
		return ErrSessionUnavailable
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.ready() {
		return ErrSessionUnavailable
	}
	clear(s.sessions)
	return nil
}

// All helpers touching store state run under mu. There is no background goroutine;
// expired entries are reclaimed on valid Issue/Resolve, with at most Capacity work.
func (s *SessionStore) ready() bool { return s.sessions != nil && s.now != nil && s.random != nil }

func (s *SessionStore) currentTime() (time.Time, error) {
	now := s.now()
	if now.IsZero() || now.Before(s.lastTime) {
		return time.Time{}, ErrSessionUnavailable
	}
	s.lastTime = now
	return now, nil
}

func (s *SessionStore) prune(now time.Time) {
	for key, session := range s.sessions {
		if !now.Before(session.ExpiresAt) || !now.Before(session.LastSeenAt.Add(s.options.IdleTimeout)) {
			delete(s.sessions, key)
		}
	}
}

func sessionKey(token string) ([sha256.Size]byte, bool) {
	if len(token) != SessionTokenChars {
		return [sha256.Size]byte{}, false
	}
	secret, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(secret) != SessionTokenBytes || base64.RawURLEncoding.EncodeToString(secret) != token {
		clear(secret)
		return [sha256.Size]byte{}, false
	}
	key := sha256.Sum256(secret)
	clear(secret)
	return key, true
}
