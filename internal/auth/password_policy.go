package auth

import (
	"errors"
	"strings"
)

var ErrBlockedPassword = errors.New("password is common or account-related")

// ValidateNewPassword is enrollment policy, never verification policy. It checks
// complete values, ignoring surrounding whitespace and case for comparison.
// Accepted bytes remain literal for hashing. The finite starter list is not a complete
// breach corpus or an assurance of password strength; review before login release.
func ValidateNewPassword(password []byte, identity LocalIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	candidate := strings.ToLower(strings.TrimSpace(string(password)))
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
