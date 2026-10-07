package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestLocalIdentityLiteralSyntaxAndOwnership(t *testing.T) {
	for _, username := range []string{"a", "0", "Synthetic.Admin-1_2", strings.Repeat("s", 64)} {
		i, err := NewLocalIdentity(username)
		if err != nil || i.Username != username {
			t.Fatal("valid literal identifier refused or changed", err)
		}
		before := i
		if err := i.Validate(); err != nil || i != before {
			t.Fatal("validation changed caller identity", err)
		}
		other, err := NewLocalIdentity(username)
		i.Username = "changed"
		if err != nil || other != before {
			t.Fatal("independent identity changed", err)
		}
	}
	upper, _ := NewLocalIdentity("Synthetic")
	lower, _ := NewLocalIdentity("synthetic")
	if upper == lower {
		t.Fatal("identifiers were case folded")
	}
}

func TestLocalIdentityRejectsSafelyAndReturnsZero(t *testing.T) {
	for _, username := range []string{
		"", strings.Repeat("s", 65), " synthetic-private", "synthetic-private ",
		".synthetic-private", "_synthetic-private", "-synthetic-private",
		"synthetic-private/user", "synthetic-private@example.invalid",
		"synthetic-private\n", "synthetic-private\x00", "synthetic-private\x1b",
		"synthetic-private\u202e", "synthétique-private", string([]byte{0xff}),
	} {
		i := LocalIdentity{Username: username}
		before := i
		err := i.Validate()
		if !errors.Is(err, ErrInvalidLocalIdentity) || i != before || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatal("invalid identifier accepted, changed or disclosed", err)
		}
		got, err := NewLocalIdentity(username)
		if !errors.Is(err, ErrInvalidLocalIdentity) || got != (LocalIdentity{}) || err.Error() != ErrInvalidLocalIdentity.Error() {
			t.Fatal("construction returned partial identity or unsafe diagnostic", err)
		}
	}
}
