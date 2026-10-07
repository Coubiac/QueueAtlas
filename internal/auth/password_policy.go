package auth

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

var ErrBlockedPassword = errors.New("password is common or account-related")

// Sorted, lowercase hex digests, one 64-byte digest plus LF per entry.
// Source, license and reproducible import: docs/password-blocklist.md.
//
//go:embed password_blocklist.sha256
var passwordBlocklist string

func corpusBlocksPassword(candidate string) bool {
	digest := sha256.Sum256([]byte(candidate))
	var encoded [64]byte
	hex.Encode(encoded[:], digest[:])
	key := string(encoded[:])
	const stride = 65
	count := len(passwordBlocklist) / stride
	i := sort.Search(count, func(i int) bool {
		return passwordBlocklist[i*stride:i*stride+64] >= key
	})
	return i < count && passwordBlocklist[i*stride:i*stride+64] == key
}

// ValidateNewPassword is enrollment policy, never verification policy. It checks
// complete values, ignoring surrounding whitespace and case for comparison.
// Accepted bytes remain literal for hashing. The pinned public corpus and local
// examples are finite; acceptance is not an assurance of password strength.
func ValidateNewPassword(password []byte, identity LocalIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	candidate := strings.ToLower(strings.TrimSpace(string(password)))
	if candidate == "" || corpusBlocksPassword(candidate) {
		return ErrBlockedPassword
	}
	for _, blocked := range []string{
		"123456789012345", "1234567890123456", "12345678901234567890",
		"123456789123456789", "0123456789012345", "111111111111111",
		"000000000000000", "aaaaaaaaaaaaaaa", "passwordpassword", "passwordpassword1!",
		"password123456789", "password12345678", "qwertyuiopasdfgh",
		"qwertyuiop123456", "qwertyqwertyqwerty", "asdfghjklqwertyuiop",
		"changemechangeme", "changemepassword", "letmeinletmeinletmein",
		"adminadminadmin", "administrator123", "administrator123!",
		"welcome123456789", "iloveyouiloveyou", "motdepassemotdepasse",
		"motdepasse123456", "correct horse battery staple",
	} {
		if candidate == blocked {
			return ErrBlockedPassword
		}
	}
	for _, root := range []string{strings.ToLower(identity.Username), "queueatlas"} {
		for _, suffix := range []string{"", "!", "1", "123", "123!", "1234", "12345", "123456", "12345678", "123456789", "1234567890", "2026", "2026!", "password", "password!"} {
			if candidate == root+suffix {
				return ErrBlockedPassword
			}
		}
		for _, separator := range []string{"", "-", " ", "_"} {
			if candidate == root+separator+root || candidate == root+separator+root+separator+root {
				return ErrBlockedPassword
			}
		}
	}
	return nil
}
