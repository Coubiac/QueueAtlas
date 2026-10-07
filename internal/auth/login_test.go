package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func loginAccount() LocalAccount {
	return LocalAccount{Identity: LocalIdentity{Username: "Operator"}, PasswordHash: syntheticPasswordHash}
}

func loginFixture(t *testing.T, options LoginOptions, verify func([]byte, string) (bool, error)) (*LocalLogin, *SessionStore, *time.Time) {
	t.Helper()
	sessions, clock := sessionFixture(t, DefaultSessionOptions())
	login, err := newLocalLogin(loginAccount(), sessions, options, func() time.Time { return *clock }, verify)
	if err != nil {
		t.Fatal(err)
	}
	return login, sessions, clock
}

func assertLoginFailure(t *testing.T, login *LocalLogin, ctx context.Context, username string, password []byte, want error) {
	t.Helper()
	token, session, err := login.Login(ctx, username, password)
	if token != "" || session != (Session{}) || err != want {
		t.Fatalf("login failure: token empty=%v metadata zero=%v error=%v; want %v", token == "", session == (Session{}), err, want)
	}
}

func TestLoginOptionsAndConstructionFailSafely(t *testing.T) {
	defaults := DefaultLoginOptions()
	if defaults != (LoginOptions{AttemptLimit: 5, Window: time.Minute, MaxConcurrent: 1}) || defaults.Validate() != nil {
		t.Fatal("unexpected defaults")
	}
	for _, o := range []LoginOptions{{1, MinLoginWindow, 1}, {MaxLoginAttempts, MaxLoginWindow, MaxLoginConcurrent}} {
		if o.Validate() != nil {
			t.Fatal("inclusive bounds rejected")
		}
	}
	for _, o := range []LoginOptions{{}, {0, time.Minute, 1}, {101, time.Minute, 1},
		{1, MinLoginWindow - 1, 1}, {1, MaxLoginWindow + 1, 1}, {1, time.Minute, 0}, {1, time.Minute, 5}} {
		if o.Validate() != ErrInvalidLoginOptions {
			t.Fatal("invalid options accepted")
		}
	}
	sessions, _ := sessionFixture(t, DefaultSessionOptions())
	badHash := loginAccount()
	badHash.PasswordHash = "private malformed record"
	badName := loginAccount()
	badName.Identity.Username = "private invalid name"
	for _, a := range []LocalAccount{{}, badHash, badName} {
		if l, err := NewLocalLogin(a, sessions, defaults); l != nil || err != ErrInvalidLoginSetup {
			t.Fatal("invalid account configuration accepted")
		}
	}
	for _, store := range []*SessionStore{nil, {}} {
		if l, err := NewLocalLogin(loginAccount(), store, defaults); l != nil || err != ErrInvalidLoginSetup {
			t.Fatal("uninitialized store accepted")
		}
	}
	if l, err := NewLocalLogin(loginAccount(), sessions, LoginOptions{}); l != nil || err != ErrInvalidLoginSetup {
		t.Fatal("invalid login configuration accepted")
	}
	for _, nilClock := range []bool{true, false} {
		now, verify := time.Now, VerifyPassword
		if nilClock {
			now = nil
		} else {
			verify = nil
		}
		if l, err := newLocalLogin(loginAccount(), sessions, defaults, now, verify); l != nil || err != ErrInvalidLoginSetup {
			t.Fatal("nil dependency accepted")
		}
	}
	for _, login := range []*LocalLogin{nil, {}} {
		assertLoginFailure(t, login, context.Background(), "Operator", []byte("synthetic secret phrase"), ErrLoginUnavailable)
	}
	// Construction validates/copies without invoking the clock, verifier or entropy.
	account := loginAccount()
	l, err := newLocalLogin(account, sessions, defaults,
		func() time.Time { t.Fatal("constructor read clock"); return time.Time{} },
		func([]byte, string) (bool, error) { t.Fatal("constructor hashed"); return false, nil })
	if err != nil {
		t.Fatal(err)
	}
	account.Identity.Username = "Changed"
	account.PasswordHash = "changed"
	defaults.AttemptLimit = 99
	if l.account != loginAccount() || l.options != DefaultLoginOptions() {
		t.Fatal("caller mutation changed snapshot")
	}
}

func TestLocalLoginPublicArgon2AndSessionOwnership(t *testing.T) {
	sessions, err := NewSessionStore(DefaultSessionOptions())
	if err != nil {
		t.Fatal(err)
	}
	login, err := NewLocalLogin(loginAccount(), sessions, DefaultLoginOptions())
	if err != nil {
		t.Fatal(err)
	}
	password := []byte("synthetic secret phrase")
	for _, attempt := range []struct{ name, password string }{
		{"Operator", "synthetic wrong phrase"}, {"Unknown", string(password)}, {"operator", string(password)},
	} {
		assertLoginFailure(t, login, context.Background(), attempt.name, []byte(attempt.password), ErrInvalidCredentials)
	}
	if len(sessions.sessions) != 0 {
		t.Fatal("rejected credentials issued a session")
	}
	var previous string
	for range 2 {
		token, session, err := login.Login(context.Background(), "Operator", password)
		if err != nil || len(token) != SessionTokenChars || token == previous || session.Identity != loginAccount().Identity {
			t.Fatal("valid login did not issue a fresh owned session")
		}
		previous = token
		session.Identity.Username = "caller mutation"
		resolved, err := sessions.Resolve(token)
		if err != nil || resolved.Identity != loginAccount().Identity {
			t.Fatal("login/session metadata ownership failed")
		}
	}
	if string(password) != "synthetic secret phrase" {
		t.Fatal("password mutated")
	}
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrLoginLimited)
}

func TestLoginSharedBudgetBoundaryAndSuccessDoesNotReset(t *testing.T) {
	calls := 0
	login, sessions, clock := loginFixture(t, LoginOptions{3, time.Minute, 1}, func(_ []byte, hash string) (bool, error) {
		calls++
		if hash != syntheticPasswordHash {
			t.Fatal("unknown name did not use the configured hash")
		}
		return true, nil
	})
	password := []byte("synthetic secret phrase")
	assertLoginFailure(t, login, context.Background(), "Unknown", password, ErrInvalidCredentials)
	token, _, err := login.Login(context.Background(), "Operator", password)
	if err != nil || token == "" {
		t.Fatal("successful credentials rejected")
	}
	assertLoginFailure(t, login, context.Background(), "Another", password, ErrInvalidCredentials)
	start := *clock
	*clock = start.Add(time.Minute - time.Nanosecond)
	for _, name := range []string{"Unknown", "Operator", "YetAnother"} {
		assertLoginFailure(t, login, context.Background(), name, password, ErrLoginLimited)
	}
	if calls != 3 || len(sessions.sessions) != 1 {
		t.Fatal("success reset budget or refusal hashed/issued a session")
	}
	*clock = start.Add(time.Minute)
	assertLoginFailure(t, login, context.Background(), "Unknown", password, ErrInvalidCredentials)
	if calls != 4 || login.attempts != 1 {
		t.Fatal("window did not reopen at exact boundary")
	}
	// A second engine/restart has independent counters: never build per request.
	other, err := NewLocalLogin(loginAccount(), sessions, DefaultLoginOptions())
	if err != nil || other.attempts != 0 {
		t.Fatal("new instance budget not empty")
	}
}

func TestLoginMalformedInputsCountWithoutHashing(t *testing.T) {
	login, sessions, _ := loginFixture(t, LoginOptions{6, time.Minute, 1}, func([]byte, string) (bool, error) {
		t.Fatal("malformed/limited input hashed")
		return false, nil
	})
	for _, attempt := range []struct {
		name     string
		password []byte
	}{
		{"", []byte("synthetic secret phrase")}, {strings.Repeat("x", 65), []byte("synthetic secret phrase")},
		{"bad name", []byte("synthetic secret phrase")}, {"Operator", []byte("short")},
		{"Operator", []byte{0xff}}, {"Operator", []byte(strings.Repeat("x", 1025))},
	} {
		assertLoginFailure(t, login, context.Background(), attempt.name, attempt.password, ErrInvalidCredentials)
	}
	assertLoginFailure(t, login, context.Background(), "Operator", []byte("synthetic secret phrase"), ErrLoginLimited)
	if len(sessions.sessions) != 0 || login.inFlight != 0 || login.attempts != 6 {
		t.Fatal("malformed input leaked slot/session or bypassed budget")
	}
}

func TestLoginConcurrentAdmissionHasNoQueueOrBudgetBypass(t *testing.T) {
	entered := make(chan struct{}, 2)
	gate := make(chan struct{})
	var calls, active, peak atomic.Int32
	sessions, err := NewSessionStore(DefaultSessionOptions())
	if err != nil {
		t.Fatal(err)
	}
	login, err := newLocalLogin(loginAccount(), sessions, LoginOptions{3, time.Minute, 2}, time.Now,
		func([]byte, string) (bool, error) {
			calls.Add(1)
			n := active.Add(1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			entered <- struct{}{}
			<-gate
			active.Add(-1)
			return false, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	password := []byte("synthetic secret phrase")
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			assertLoginFailure(t, login, context.Background(), "Operator", password, ErrInvalidCredentials)
		})
	}
	<-entered
	<-entered
	var refused sync.WaitGroup
	for i := range 32 {
		refused.Go(func() {
			assertLoginFailure(t, login, context.Background(), fmt.Sprintf("Unknown%d", i), password, ErrLoginLimited)
		})
	}
	refused.Wait() // must return while both admitted verifiers are still blocked
	close(gate)
	workers.Wait()
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrInvalidCredentials)
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrLoginLimited)
	if calls.Load() != 3 || peak.Load() != 2 || login.inFlight != 0 || len(sessions.sessions) != 0 {
		t.Fatal("concurrency limit, immediate refusal or slot release failed")
	}
}

func TestLoginCancellationAndFailuresReleaseSlotsWithoutSessions(t *testing.T) {
	password := []byte("synthetic secret phrase")
	ctx, cancel := context.WithCancel(context.Background())
	login, sessions, _ := loginFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) {
		cancel()
		return true, nil
	})
	assertLoginFailure(t, login, nil, "Operator", password, ErrLoginUnavailable)
	preCanceled, stop := context.WithCancel(context.Background())
	stop()
	assertLoginFailure(t, login, preCanceled, "Operator", password, context.Canceled)
	deadline, stopDeadline := context.WithDeadline(context.Background(), time.Unix(1, 0))
	defer stopDeadline()
	assertLoginFailure(t, login, deadline, "Operator", password, context.DeadlineExceeded)
	if login.attempts != 0 {
		t.Fatal("pre-cancellation consumed budget")
	}
	assertLoginFailure(t, login, ctx, "Operator", password, context.Canceled)
	if login.attempts != 1 || login.inFlight != 0 || len(sessions.sessions) != 0 {
		t.Fatal("cancellation during hash issued session or retained slot")
	}
	login.verify = func([]byte, string) (bool, error) { return false, errors.New("private verifier failure") }
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrLoginUnavailable)
	login.verify = func([]byte, string) (bool, error) { return true, nil }
	sessions.random = failedRandom{}
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrLoginUnavailable)
	if login.inFlight != 0 || len(sessions.sessions) != 0 {
		t.Fatal("internal failure leaked slot or partial session")
	}
}

func TestLoginCanceledHashStillOccupiesSlot(t *testing.T) {
	entered, gate := make(chan struct{}), make(chan struct{})
	sessions, err := NewSessionStore(DefaultSessionOptions())
	if err != nil {
		t.Fatal(err)
	}
	login, err := newLocalLogin(loginAccount(), sessions, DefaultLoginOptions(), time.Now,
		func([]byte, string) (bool, error) { close(entered); <-gate; return true, nil })
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	password := []byte("synthetic secret phrase")
	var worker sync.WaitGroup
	worker.Go(func() { assertLoginFailure(t, login, ctx, "Operator", password, context.Canceled) })
	<-entered
	cancel()
	assertLoginFailure(t, login, context.Background(), "Unknown", password, ErrLoginLimited)
	close(gate)
	worker.Wait()
	if login.inFlight != 0 || login.attempts != 1 || len(sessions.sessions) != 0 {
		t.Fatal("canceled hash released early or issued a session")
	}
}

func TestLoginClockAndSessionCapacityFailClosed(t *testing.T) {
	password := []byte("synthetic secret phrase")
	calls := 0
	login, sessions, clock := loginFixture(t, DefaultLoginOptions(), func([]byte, string) (bool, error) { calls++; return true, nil })
	start := *clock
	*clock = time.Time{}
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrLoginUnavailable)
	if calls != 0 || login.attempts != 0 {
		t.Fatal("invalid clock admitted a hash")
	}
	*clock = start
	login.verify = func([]byte, string) (bool, error) { calls++; *clock = start.Add(-time.Second); return true, nil }
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrLoginUnavailable)
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrLoginUnavailable)
	if calls != 1 || login.inFlight != 0 || len(sessions.sessions) != 0 {
		t.Fatal("backward clock hashed again or issued a session")
	}
	*clock = start
	login.verify = func([]byte, string) (bool, error) { return true, nil }
	sessions.options.Capacity = 1
	token, _, err := login.Login(context.Background(), "Operator", password)
	if err != nil {
		t.Fatal(err)
	}
	assertLoginFailure(t, login, context.Background(), "Operator", password, ErrLoginUnavailable)
	if _, err := sessions.Resolve(token); err != nil || len(sessions.sessions) != 1 || login.inFlight != 0 {
		t.Fatal("full store evicted active session or retained login slot")
	}
	if err := sessions.Revoke(token); err != nil {
		t.Fatal(err)
	}
	if token, _, err := login.Login(context.Background(), "Operator", password); err != nil || token == "" {
		t.Fatal("login did not recover after capacity reclaimed")
	}
}
