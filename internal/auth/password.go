package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	MinPasswordRunes    = 15
	MaxPasswordRunes    = 256
	MaxPasswordBytes    = 1024
	MaxEncodedHashBytes = 128
)

var (
	ErrInvalidPassword = errors.New("invalid password length or encoding")
	ErrInvalidHash     = errors.New("invalid password hash")
	ErrRandom          = errors.New("cannot generate password salt")
)

// ValidatePassword enforces the local password input bounds, not strength or a
// compromised-password blocklist. UTF-8 code points count individually. Bytes
// are used literally: no trimming, case folding, normalization or truncation.
func ValidatePassword(password []byte) error {
	if len(password) > MaxPasswordBytes || !utf8.Valid(password) {
		return ErrInvalidPassword
	}
	count := utf8.RuneCount(password)
	if count < MinPasswordRunes || count > MaxPasswordRunes {
		return ErrInvalidPassword
	}
	return nil
}

// HashPassword validates costs and input before generating a fresh random salt.
// It returns a canonical Argon2id record, or an empty string on error. No account
// is created. The caller owns the unchanged password and must not mutate it
// concurrently. Costs bound one call, not concurrency, elapsed time or admission.
func HashPassword(password []byte, p Parameters) (string, error) {
	return hashPassword(password, p, rand.Reader)
}

func hashPassword(password []byte, p Parameters, random io.Reader) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	var salt [SaltBytes]byte
	if random == nil {
		return "", ErrRandom
	}
	if _, err := io.ReadFull(random, salt[:]); err != nil {
		return "", ErrRandom
	}
	key := argon2.IDKey(password, salt[:], p.Iterations, p.MemoryKiB, p.Parallelism, KeyBytes)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", Argon2Version,
		p.MemoryKiB, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt[:]), base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword rejects invalid input/records before invoking Argon2id. Only
// equal fixed-size derived hashes are compared in constant time; validation and
// the entire function are not constant time. A well-formed mismatch is false,nil.
// Callers must enforce admission/rate limits and avoid revealing account state.
func VerifyPassword(password []byte, encoded string) (bool, error) {
	if err := ValidatePassword(password); err != nil {
		return false, err
	}
	r, err := parsePasswordHash(encoded)
	if err != nil {
		return false, err
	}
	key := argon2.IDKey(password, r.salt[:], r.params.Iterations, r.params.MemoryKiB, r.params.Parallelism, KeyBytes)
	return subtle.ConstantTimeCompare(key, r.key[:]) == 1, nil
}

// ValidatePasswordHash validates syntax, sizes, version and costs without hashing
// or IO. This does not attest the hash's origin or the strength of its password.
func ValidatePasswordHash(encoded string) error {
	_, err := parsePasswordHash(encoded)
	return err
}

type passwordRecord struct {
	params Parameters
	salt   [SaltBytes]byte
	key    [KeyBytes]byte
}

func parsePasswordHash(encoded string) (passwordRecord, error) {
	invalid := passwordRecord{}
	if len(encoded) == 0 || len(encoded) > MaxEncodedHashBytes {
		return invalid, ErrInvalidHash
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return invalid, ErrInvalidHash
	}
	costs := strings.Split(parts[3], ",")
	if len(costs) != 3 {
		return invalid, ErrInvalidHash
	}
	memory, okM := decimalCost(costs[0], "m=", 32)
	iterations, okT := decimalCost(costs[1], "t=", 32)
	parallelism, okP := decimalCost(costs[2], "p=", 8)
	if !okM || !okT || !okP {
		return invalid, ErrInvalidHash
	}
	p := Parameters{MemoryKiB: uint32(memory), Iterations: uint32(iterations), Parallelism: uint8(parallelism)}
	if p.Validate() != nil {
		return invalid, ErrInvalidHash
	}
	var r passwordRecord
	if !decodeCanonicalBase64(parts[4], r.salt[:]) || !decodeCanonicalBase64(parts[5], r.key[:]) {
		return invalid, ErrInvalidHash
	}
	r.params = p
	return r, nil
}

func decimalCost(field, prefix string, bits int) (uint64, bool) {
	if !strings.HasPrefix(field, prefix) {
		return 0, false
	}
	value := strings.TrimPrefix(field, prefix)
	if value == "" || len(value) > 1 && value[0] == '0' {
		return 0, false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(value, 10, bits)
	return n, err == nil
}

func decodeCanonicalBase64(encoded string, out []byte) bool {
	if len(encoded) != base64.RawStdEncoding.EncodedLen(len(out)) {
		return false
	}
	decoded, err := base64.RawStdEncoding.Strict().DecodeString(encoded)
	// Encoding comparison also refuses CR/LF, which the base64 decoder ignores.
	if err != nil || len(decoded) != len(out) || base64.RawStdEncoding.EncodeToString(decoded) != encoded {
		return false
	}
	copy(out, decoded)
	return true
}
