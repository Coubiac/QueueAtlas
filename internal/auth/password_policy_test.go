package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestNewPasswordPolicyCompleteValuesAndLiteralBytes(t *testing.T) {
	identity := LocalIdentity{Username: "Synthetic-Operator"}
	for _, value := range []string{strings.Repeat(" ", 15), strings.Repeat(" ", 256), strings.Repeat("\u00a0", 15), strings.Repeat("\u2003", 15)} {
		password := []byte(value)
		before := bytes.Clone(password)
		if err := ValidateNewPassword(password, identity); err != ErrBlockedPassword || !bytes.Equal(password, before) {
			t.Fatal("whitespace-only enrollment accepted or input changed", err)
		}
		// Verification bounds remain literal; enrollment policy must not lock out
		// a previously provisioned credential when the starter list changes.
		if err := ValidatePassword(password); err != nil {
			t.Fatal("enrollment policy leaked into verification bounds", err)
		}
	}
	for _, value := range []string{"PASSWORDPASSWORD", " passwordpassword ", "123456789012345", "correct horse battery staple", "SYNTHETIC-OPERATOR", "Synthetic-Operator2026!", "Synthetic-Operator Synthetic-Operator", "QueueAtlas123456", "QueueAtlas-QueueAtlas"} {
		if err := ValidateNewPassword([]byte(value), identity); err != ErrBlockedPassword {
			t.Fatal("known complete value accepted", err)
		}
	}
	for _, value := range []string{"unique synthetic phrase 🔐", "  unique synthetic phrase  ", "long phrase with QueueAtlas in it", "synthetic-operator unrelated phrase"} {
		password := []byte(value)
		before := bytes.Clone(password)
		if err := ValidateNewPassword(password, identity); err != nil || !bytes.Equal(before, password) {
			t.Fatal("unlisted complete value rejected or input mutated", err)
		}
	}
	if err := ValidateNewPassword([]byte("unique synthetic phrase"), LocalIdentity{}); err != ErrInvalidLocalIdentity {
		t.Fatal("invalid identity used for enrollment", err)
	}
	if err := ValidateNewPassword([]byte("short"), identity); err != ErrInvalidPassword {
		t.Fatal("base password bounds ignored", err)
	}
}

func TestPasswordCorpusIntegrityAndEnrollment(t *testing.T) {
	const count = 10898
	const expectedSHA256 = "a5b8b74b66c0ed54d90098285cb0dbe02217ae41d7c91f0c57cf4a761e14521a"
	digest := sha256.Sum256([]byte(passwordBlocklist))
	if len(passwordBlocklist) != count*65 || hex.EncodeToString(digest[:]) != expectedSHA256 {
		t.Fatal("embedded corpus differs from reviewed import")
	}
	previous := ""
	for i := 0; i < count; i++ {
		entry := passwordBlocklist[i*65 : i*65+64]
		decoded, err := hex.DecodeString(entry)
		if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != entry || passwordBlocklist[i*65+64] != '\n' || entry <= previous {
			t.Fatal("corpus must be canonical, sorted and unique")
		}
		previous = entry
	}
	identity := LocalIdentity{Username: "Synthetic-Operator"}
	// Synthetic repeated digit / keyboard patterns, not credentials of test users.
	for _, value := range []string{"11111111111111111111", "qwertyuiopasdfghjkl", "  QWERTYUIOPASDFGHJKL  "} {
		password := []byte(value)
		before := bytes.Clone(password)
		if !corpusBlocksPassword(strings.ToLower(strings.TrimSpace(value))) || ValidateNewPassword(password, identity) != ErrBlockedPassword || !bytes.Equal(password, before) {
			t.Fatal("corpus enrollment refusal missing or input changed")
		}
	}
	for _, value := range []string{"a synthetic phrase with qwertyuiopasdfghjkl inside", "  unique synthetic corpus phrase 🔐  "} {
		password := []byte(value)
		before := bytes.Clone(password)
		if err := ValidateNewPassword(password, identity); err != nil || !bytes.Equal(password, before) {
			t.Fatal("substring blocked or accepted literal input changed", err)
		}
	}
}

func TestCorpusDoesNotLockOutExistingLiteralCredential(t *testing.T) {
	password := []byte("  QWERTYUIOPASDFGHJKL  ")
	identity := LocalIdentity{Username: "Synthetic-Operator"}
	if ValidateNewPassword(password, identity) != ErrBlockedPassword {
		t.Fatal("fixture no longer exercises blocked enrollment")
	}
	// Provision through the preexisting hashing primitive, as an old account would
	// have been. Login and VerifyPassword must not reapply enrollment policy.
	hash, err := HashPassword(password, DefaultParameters())
	if err != nil {
		t.Fatal(err)
	}
	if match, err := VerifyPassword(password, hash); !match || err != nil {
		t.Fatal("existing credential rejected", err)
	}
	sessions, _ := sessionFixture(t, DefaultSessionOptions())
	login, err := NewLocalLogin(LocalAccount{Identity: identity, PasswordHash: hash}, sessions, DefaultLoginOptions())
	if err != nil {
		t.Fatal(err)
	}
	token, session, err := login.Login(context.Background(), identity.Username, password)
	if err != nil || token == "" || session.Identity != identity {
		t.Fatal("existing literal account locked out", err)
	}
	assertLoginFailure(t, login, context.Background(), identity.Username, []byte("qwertyuiopasdfghjkl"), ErrInvalidCredentials)
}
