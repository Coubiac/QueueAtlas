package file

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
)

// Identity is a snapshot of an open regular file. Device and Inode are decimal
// Linux identifiers; other platforms leave them empty and use SameFile only.
// Neither physical identity nor a prefix hash proves a file's generation:
// truncation and inode reuse still require checkpoint anchors and diagnostics.
type Identity struct {
	Device string
	Inode  string
	Size   int64
	info   os.FileInfo
}

// Inspect uses the opened descriptor, so a concurrent path replacement cannot
// switch the file being inspected. It does not change the read position.
func Inspect(f *os.File) (Identity, error) {
	if f == nil {
		return Identity{}, errors.New("file is nil")
	}
	info, err := f.Stat()
	if err != nil {
		return Identity{}, err
	}
	if !info.Mode().IsRegular() {
		return Identity{}, errors.New("log input is not a regular file")
	}
	device, inode := physicalIDs(info)
	return Identity{Device: device, Inode: inode, Size: info.Size(), info: info}, nil
}

// SameFile compares descriptor snapshots. A rename keeps this identity; a new
// file at the old path has a different identity while the old file still exists.
func (id Identity) SameFile(other Identity) bool {
	return id.info != nil && other.info != nil && os.SameFile(id.info, other.info)
}

const MaxFingerprintBytes = 4096

// PrefixFingerprint records both a digest and the number of bytes hashed.
// Comparing the same window avoids changing identity just because a short file
// grows. A zero-length window provides no evidence and never matches.
type PrefixFingerprint struct {
	Length int
	Digest [sha256.Size]byte
}

// String preserves the window length with its SHA-256 digest for storage.
func (p PrefixFingerprint) String() string {
	return fmt.Sprintf("sha256:%d:%x", p.Length, p.Digest)
}

// CapturePrefix hashes up to MaxFingerprintBytes at byte zero, without moving
// the caller's read position. Inspect and ReadAt are separate observations;
// callers must handle concurrent truncation when choosing a file generation.
func CapturePrefix(f *os.File) (PrefixFingerprint, error) {
	if _, err := Inspect(f); err != nil {
		return PrefixFingerprint{}, err
	}
	var window [MaxFingerprintBytes]byte
	n, err := f.ReadAt(window[:], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return PrefixFingerprint{}, err
	}
	return PrefixFingerprint{Length: n, Digest: sha256.Sum256(window[:n])}, nil
}

// Matches compares exactly the saved window. A shorter file or changed prefix
// does not match. Bytes beyond the window are deliberately not checked.
func (p PrefixFingerprint) Matches(f *os.File) (bool, error) {
	if p.Length < 0 || p.Length > MaxFingerprintBytes {
		return false, errors.New("fingerprint window length is outside bounds")
	}
	if _, err := Inspect(f); err != nil {
		return false, err
	}
	if p.Length == 0 {
		return false, nil
	}
	var window [MaxFingerprintBytes]byte
	n, err := f.ReadAt(window[:p.Length], 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	return n == p.Length && sha256.Sum256(window[:n]) == p.Digest, nil
}
