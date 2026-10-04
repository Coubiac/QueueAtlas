package file

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// CheckpointAnchor hashes the last min(Offset, MaxFingerprintBytes) bytes
// before a checkpoint. It is bounded evidence, not a digest of the whole file.
type CheckpointAnchor struct {
	Offset int64
	Length int
	Digest [sha256.Size]byte
}

func (a CheckpointAnchor) String() string {
	return fmt.Sprintf("sha256:%d:%d:%x", a.Offset, a.Length, a.Digest)
}

// CaptureAnchor reads a complete window ending at offset without moving the
// file's read position. An offset past EOF or truncation during the read is an
// error; a partial window must not be persisted as a valid anchor.
func CaptureAnchor(f *os.File, offset int64) (CheckpointAnchor, error) {
	if offset < 0 {
		return CheckpointAnchor{}, errors.New("checkpoint offset is negative")
	}
	id, err := Inspect(f)
	if err != nil {
		return CheckpointAnchor{}, err
	}
	if offset > id.Size {
		return CheckpointAnchor{}, io.ErrUnexpectedEOF
	}
	a := CheckpointAnchor{Offset: offset, Length: int(min(offset, int64(MaxFingerprintBytes)))}
	var window [MaxFingerprintBytes]byte
	if a.Length > 0 {
		n, err := f.ReadAt(window[:a.Length], offset-int64(a.Length))
		if err != nil && !errors.Is(err, io.EOF) {
			return CheckpointAnchor{}, err
		}
		if n != a.Length {
			return CheckpointAnchor{}, io.ErrUnexpectedEOF
		}
	}
	a.Digest = sha256.Sum256(window[:a.Length])
	return a, nil
}

// Matches checks the saved window. Offset zero is a valid checkpoint but has
// no anchor bytes, so it never claims a match. A truncated file does not match.
func (a CheckpointAnchor) Matches(f *os.File) (bool, error) {
	if err := a.validate(); err != nil {
		return false, err
	}
	id, err := Inspect(f)
	if err != nil {
		return false, err
	}
	if a.Length == 0 || a.Offset > id.Size {
		return false, nil
	}
	var window [MaxFingerprintBytes]byte
	n, err := f.ReadAt(window[:a.Length], a.Offset-int64(a.Length))
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return n == a.Length && sha256.Sum256(window[:n]) == a.Digest, nil
}

func (a CheckpointAnchor) validate() error {
	if a.Offset < 0 || a.Length < 0 || a.Length != int(min(a.Offset, int64(MaxFingerprintBytes))) {
		return errors.New("invalid checkpoint anchor window")
	}
	if a.Length == 0 && a.Digest != sha256.Sum256(nil) {
		return errors.New("invalid empty anchor digest")
	}
	return nil
}

// ParsePrefixFingerprint accepts the canonical form emitted by String. The
// length and digest are validated before any persisted window can be read.
func ParsePrefixFingerprint(value string) (PrefixFingerprint, error) {
	parts, err := fingerprintParts(value, 3)
	if err != nil {
		return PrefixFingerprint{}, err
	}
	length, err := strconv.Atoi(parts[1])
	if err != nil || length < 0 || length > MaxFingerprintBytes {
		return PrefixFingerprint{}, errors.New("invalid prefix window length")
	}
	digest, err := parseDigest(parts[2])
	if err != nil {
		return PrefixFingerprint{}, err
	}
	p := PrefixFingerprint{Length: length, Digest: digest}
	if p.String() != value || length == 0 && digest != sha256.Sum256(nil) {
		return PrefixFingerprint{}, errors.New("noncanonical prefix fingerprint")
	}
	return p, nil
}

func ParseCheckpointAnchor(value string) (CheckpointAnchor, error) {
	parts, err := fingerprintParts(value, 4)
	if err != nil {
		return CheckpointAnchor{}, err
	}
	offset, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return CheckpointAnchor{}, errors.New("invalid anchor offset")
	}
	length, err := strconv.Atoi(parts[2])
	if err != nil {
		return CheckpointAnchor{}, errors.New("invalid anchor window length")
	}
	digest, err := parseDigest(parts[3])
	if err != nil {
		return CheckpointAnchor{}, err
	}
	a := CheckpointAnchor{Offset: offset, Length: length, Digest: digest}
	if err := a.validate(); err != nil {
		return CheckpointAnchor{}, err
	}
	if a.String() != value {
		return CheckpointAnchor{}, errors.New("noncanonical checkpoint anchor")
	}
	return a, nil
}

func fingerprintParts(value string, count int) ([]string, error) {
	// Both formats are shorter than 128 bytes even at the maximum int64 offset.
	if len(value) > 128 {
		return nil, errors.New("encoded fingerprint exceeds size limit")
	}
	parts := strings.Split(value, ":")
	if len(parts) != count || parts[0] != "sha256" {
		return nil, errors.New("invalid fingerprint format")
	}
	return parts, nil
}

func parseDigest(value string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	if len(value) != hex.EncodedLen(len(digest)) {
		return digest, errors.New("invalid SHA-256 digest length")
	}
	if _, err := hex.Decode(digest[:], []byte(value)); err != nil {
		return digest, errors.New("invalid SHA-256 digest encoding")
	}
	return digest, nil
}
