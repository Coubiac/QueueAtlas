package source

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

var ErrImportIdentity = errors.New("import source ID and canonical content SHA-256 are required")

// ImportOriginID identifies whole decompressed content within one exact source
// ID. contentSHA256 must be 64 lowercase hexadecimal characters from successful
// full-content validation. This function does not validate a file or prove EOF.
// Paths, encoding and physical file IDs play no part in the identity. Identical
// lines at different offsets remain distinct records, and this ID supplies no
// proof of overlap with a live file source. Source IDs are opaque, not trimmed
// or case-folded. The versioned framing is a durable persistence contract.
func ImportOriginID(sourceID, contentSHA256 string) (string, error) {
	if sourceID == "" || len(contentSHA256) != sha256.Size*2 {
		return "", ErrImportIdentity
	}
	digest, err := hex.DecodeString(contentSHA256)
	if err != nil || hex.EncodeToString(digest) != contentSHA256 {
		return "", ErrImportIdentity
	}
	h := sha256.New()
	_, _ = h.Write([]byte("queueatlas/import-origin/v1\x00"))
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(sourceID)))
	_, _ = h.Write(length[:])
	_, _ = h.Write([]byte(sourceID))
	_, _ = h.Write(digest)
	return "import-v1:" + hex.EncodeToString(h.Sum(nil)), nil
}
