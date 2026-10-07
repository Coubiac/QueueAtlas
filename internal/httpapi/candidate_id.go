package httpapi

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
)

const (
	candidateHeaderBytes = 38 // version + revision32 + ordinal2 + instance length2 + queue length1
	MaxCandidateIDChars  = ((candidateHeaderBytes+1024+32)*8 + 5) / 6
)

var ErrCandidateID = errors.New("invalid candidate identifier")

// EncodeCandidateID identifies a revisable candidate in one exact full queue
// scope. It is not signed, secret, an authorization proof or a global message ID.
func EncodeCandidateID(key correlation.QueueInstanceKey) (string, error) {
	if !searchText(key.Instance, 1024) || !searchText(key.QueueID, 32) || key.Generation < 0 || key.Generation >= correlation.MaxPartitionFacts || len(key.Revision) != 64 {
		return "", ErrCandidateID
	}
	revision, err := hex.DecodeString(key.Revision)
	if err != nil || hex.EncodeToString(revision) != key.Revision {
		return "", ErrCandidateID
	}
	wire := make([]byte, candidateHeaderBytes+len(key.Instance)+len(key.QueueID))
	wire[0] = 1
	copy(wire[1:33], revision)
	binary.BigEndian.PutUint16(wire[33:35], uint16(key.Generation))
	binary.BigEndian.PutUint16(wire[35:37], uint16(len(key.Instance)))
	wire[37] = byte(len(key.QueueID))
	copy(wire[38:], key.Instance)
	copy(wire[38+len(key.Instance):], key.QueueID)
	return base64.RawURLEncoding.EncodeToString(wire), nil
}

func decodeCandidateID(value string) (correlation.QueueInstanceKey, error) {
	if len(value) < 54 || len(value) > MaxCandidateIDChars {
		return correlation.QueueInstanceKey{}, ErrCandidateID
	}
	wire, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(wire) < candidateHeaderBytes || wire[0] != 1 || base64.RawURLEncoding.EncodeToString(wire) != value {
		return correlation.QueueInstanceKey{}, ErrCandidateID
	}
	instanceBytes, queueBytes := int(binary.BigEndian.Uint16(wire[35:37])), int(wire[37])
	if instanceBytes < 1 || instanceBytes > 1024 || queueBytes < 1 || queueBytes > 32 || len(wire) != candidateHeaderBytes+instanceBytes+queueBytes {
		return correlation.QueueInstanceKey{}, ErrCandidateID
	}
	key := correlation.QueueInstanceKey{Revision: hex.EncodeToString(wire[1:33]), Generation: int(binary.BigEndian.Uint16(wire[33:35])),
		Instance: string(wire[38 : 38+instanceBytes]), QueueID: string(wire[38+instanceBytes:])}
	if !searchText(key.Instance, 1024) || !searchText(key.QueueID, 32) || key.Generation >= correlation.MaxPartitionFacts {
		return correlation.QueueInstanceKey{}, ErrCandidateID
	}
	return key, nil
}
