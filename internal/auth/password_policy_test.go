package auth

import (
	"bytes"
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
