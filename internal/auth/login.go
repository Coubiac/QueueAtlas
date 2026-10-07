package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"sync"
	"time"
)

const (
	MaxLoginAttempts   = 100
	MaxLoginConcurrent = 4
	MinLoginWindow     = time.Second
	MaxLoginWindow     = time.Hour
)

var (
	ErrInvalidLoginOptions = errors.New("invalid login options")
	ErrInvalidLoginSetup   = errors.New("invalid local login configuration")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrLoginLimited        = errors.New("login admission limited")
	ErrLoginUnavailable    = errors.New("local login unavailable")
)

type LoginOptions struct {
	AttemptLimit  int
	Window        time.Duration
	MaxConcurrent int
}

func DefaultLoginOptions() LoginOptions {
	return LoginOptions{AttemptLimit: 5, Window: time.Minute, MaxConcurrent: 1}
}

func (o LoginOptions) Validate() error {
	if o.AttemptLimit < 1 || o.AttemptLimit > MaxLoginAttempts ||
		o.Window < MinLoginWindow || o.Window > MaxLoginWindow ||
		o.MaxConcurrent < 1 || o.MaxConcurrent > MaxLoginConcurrent {
		return ErrInvalidLoginOptions
	}
	return nil
}

// LocalLogin owns one immutable account snapshot and one process-local admission
// budget shared across all supplied names. Construct once and share the pointer;
// never copy after use or create an instance per request. It performs no file IO,
// HTTP, authorization or account reload. The session store must also be shared.
type LocalLogin struct {
	mu          sync.Mutex
	account     LocalAccount
	sessions    *SessionStore
	options     LoginOptions
	now         func() time.Time
	verify      func([]byte, string) (bool, error)
	lastTime    time.Time
	windowStart time.Time
	attempts    int
	inFlight    int
}

func NewLocalLogin(account LocalAccount, sessions *SessionStore, options LoginOptions) (*LocalLogin, error) {
	return newLocalLogin(account, sessions, options, time.Now, VerifyPassword)
}

// Seams stay private: public construction always uses real time and Argon2id.
func newLocalLogin(account LocalAccount, sessions *SessionStore, options LoginOptions,
	now func() time.Time, verify func([]byte, string) (bool, error)) (*LocalLogin, error) {
	if account.Validate() != nil || options.Validate() != nil || sessions == nil || now == nil || verify == nil {
		return nil, ErrInvalidLoginSetup
	}
	sessions.mu.Lock()
	ready := sessions.ready()
	sessions.mu.Unlock()
	if !ready {
		return nil, ErrInvalidLoginSetup
	}
	return &LocalLogin{account: account, sessions: sessions, options: options, now: now, verify: verify}, nil
}

// Login issues a fresh session only after both exact identity and password match.
// Unknown well-formed names use the same stored hash and costs as known names.
// Every admitted attempt counts, even malformed input or success; success never
// resets the budget. Saturation returns immediately, without queuing or hashing.
// The caller owns password and must not mutate it concurrently or log secrets.
// Cancellation is checked before admission and after hashing; Argon2id cannot be
// interrupted. A slot stays occupied until hashing/session issuance completes.
// Every failure returns an empty token and zero metadata.
func (l *LocalLogin) Login(ctx context.Context, username string, password []byte) (string, Session, error) {
	if l == nil || ctx == nil {
		return "", Session{}, ErrLoginUnavailable
	}
	if err := ctx.Err(); err != nil {
		return "", Session{}, err
	}
	if err := l.admit(); err != nil {
		return "", Session{}, err
	}
	defer l.release()
	if (LocalIdentity{Username: username}).Validate() != nil || ValidatePassword(password) != nil {
		return "", Session{}, ErrInvalidCredentials
	}
	matched, err := l.verify(password, l.account.PasswordHash)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", Session{}, ctxErr
	}
	if err != nil || !l.clockValid() {
		return "", Session{}, ErrLoginUnavailable
	}
	if subtle.ConstantTimeCompare([]byte(username), []byte(l.account.Identity.Username)) != 1 || !matched {
		return "", Session{}, ErrInvalidCredentials
	}
	// Cancellation may race this check. No cancellation guarantee is made once
	// Issue starts (the store's entropy read is synchronous and not cancelable).
	if err := ctx.Err(); err != nil {
		return "", Session{}, err
	}
	token, session, err := l.sessions.Issue(l.account.Identity)
	if err != nil {
		return "", Session{}, ErrLoginUnavailable
	}
	return token, session, nil
}

func (l *LocalLogin) admit() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.now == nil || l.verify == nil || l.sessions == nil {
		return ErrLoginUnavailable
	}
	now := l.now()
	if now.IsZero() || now.Before(l.lastTime) {
		return ErrLoginUnavailable
	}
	l.lastTime = now
	if l.windowStart.IsZero() || !now.Before(l.windowStart.Add(l.options.Window)) {
		l.windowStart, l.attempts = now, 0
	}
	if l.inFlight >= l.options.MaxConcurrent || l.attempts >= l.options.AttemptLimit {
		return ErrLoginLimited
	}
	l.attempts++
	l.inFlight++
	return nil
}

func (l *LocalLogin) release() {
	l.mu.Lock()
	l.inFlight--
	l.mu.Unlock()
}

func (l *LocalLogin) clockValid() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.IsZero() || now.Before(l.lastTime) {
		return false
	}
	l.lastTime = now
	return true
}
