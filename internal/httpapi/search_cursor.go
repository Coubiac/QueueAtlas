package httpapi

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"

	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

const (
	searchCursorBytes = 49 // version1 + SHA-256 query revision32 + time8 + row8
	SearchCursorChars = 66 // canonical unpadded base64url of 49 bytes
)

// EncodeSearchCursor encodes a store-provided position. It is neither a signature,
// bearer, message ID nor authorization proof. Decode validates shape; request
// validation separately checks query revision and time window before any search.
func EncodeSearchCursor(cursor sqlite.SearchCursor) (string, error) {
	if len(cursor.QueryRevision) != 64 || cursor.RowID < 1 {
		return "", ErrSearchCursor
	}
	revision, err := hex.DecodeString(cursor.QueryRevision)
	if err != nil || hex.EncodeToString(revision) != cursor.QueryRevision {
		return "", ErrSearchCursor
	}
	var wire [searchCursorBytes]byte
	wire[0] = 1
	copy(wire[1:33], revision)
	binary.BigEndian.PutUint64(wire[33:41], uint64(cursor.TimeNS))
	binary.BigEndian.PutUint64(wire[41:49], uint64(cursor.RowID))
	return base64.RawURLEncoding.EncodeToString(wire[:]), nil
}

func decodeSearchCursor(value string) (sqlite.SearchCursor, error) {
	if len(value) != SearchCursorChars {
		return sqlite.SearchCursor{}, ErrSearchCursor
	}
	wire, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil || len(wire) != searchCursorBytes || base64.RawURLEncoding.EncodeToString(wire) != value || wire[0] != 1 {
		return sqlite.SearchCursor{}, ErrSearchCursor
	}
	cursor := sqlite.SearchCursor{QueryRevision: hex.EncodeToString(wire[1:33]),
		TimeNS: int64(binary.BigEndian.Uint64(wire[33:41])), RowID: int64(binary.BigEndian.Uint64(wire[41:49]))}
	if cursor.RowID < 1 {
		return sqlite.SearchCursor{}, ErrSearchCursor
	}
	return cursor, nil
}
