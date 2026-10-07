// Package auth implements local credentials, bounded login and memory sessions.
// It provides no HTTP transport or application authorization.
package auth

import "errors"

const MaxLocalUsernameBytes = 64

// ErrInvalidLocalIdentity never includes the supplied identifier.
var ErrInvalidLocalIdentity = errors.New("invalid local identity")

// LocalIdentity identifies an operator-chosen local account. Usernames are ASCII
// and case-sensitive, without trimming or case folding. This value is not proof
// of authentication or authorization, and contains no password or credential.
// External providers must use a separate provider/subject namespace in future.
type LocalIdentity struct {
	Username string
}

// Validate checks syntax without normalization, mutation, IO or account lookup.
func (i LocalIdentity) Validate() error {
	if len(i.Username) == 0 || len(i.Username) > MaxLocalUsernameBytes {
		return ErrInvalidLocalIdentity
	}
	for index := 0; index < len(i.Username); index++ {
		b := i.Username[index]
		alphanumeric := b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
		if !alphanumeric && (index == 0 || b != '.' && b != '_' && b != '-') {
			return ErrInvalidLocalIdentity
		}
	}
	return nil
}

// NewLocalIdentity returns a validated value, or the zero value on any error.
// There is no default account name and no account creation or uniqueness check.
func NewLocalIdentity(username string) (LocalIdentity, error) {
	i := LocalIdentity{Username: username}
	if err := i.Validate(); err != nil {
		return LocalIdentity{}, err
	}
	return i, nil
}
