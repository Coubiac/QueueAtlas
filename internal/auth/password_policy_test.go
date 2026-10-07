package auth

import (
	"bytes"
	"testing"
)

func TestNewPasswordPolicyCompleteValuesAndLiteralBytes(t *testing.T) {
	identity := LocalIdentity{Username: "Synthetic-Operator"}
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
