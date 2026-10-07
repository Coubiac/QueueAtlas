package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

var ErrTimelineRequest = errors.New("invalid timeline request")

type timelineQuery struct {
	Limit int
	Raw   bool
	Start int
}

// A cursor is a public position, not an authorization token. Its fixed digest
// binds the canonical candidate ID (including revision) and raw mode, not limit.
func timelineDigest(id string, raw bool) [32]byte {
	mode := byte(0)
	if raw {
		mode = 1
	}
	return sha256.Sum256(append([]byte("queueatlas.timeline.v1\x00"+id+"\x00"), mode))
}

func encodeTimelineCursor(id string, raw bool, start int) (string, error) {
	if start < 1 || start >= correlation.MaxPartitionFacts {
		return "", ErrTimelineRequest
	}
	var wire [35]byte
	wire[0] = 1
	digest := timelineDigest(id, raw)
	copy(wire[1:33], digest[:])
	binary.BigEndian.PutUint16(wire[33:], uint16(start))
	return base64.RawURLEncoding.EncodeToString(wire[:]), nil
}

func parseTimelineRequest(rawQuery, id string) (timelineQuery, error) {
	if len(rawQuery) > MaxSearchQueryBytes || !utf8.ValidString(rawQuery) || strings.HasPrefix(rawQuery, "&") || strings.HasSuffix(rawQuery, "&") || strings.Contains(rawQuery, "&&") {
		return timelineQuery{}, ErrTimelineRequest
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return timelineQuery{}, ErrTimelineRequest
	}
	for name, entries := range values {
		if (name != "limit" && name != "raw" && name != "cursor") || len(entries) != 1 {
			return timelineQuery{}, ErrTimelineRequest
		}
	}
	q := timelineQuery{Limit: DefaultSearchLimit}
	if values.Has("limit") {
		q.Limit, err = strconv.Atoi(values.Get("limit"))
		if err != nil || strconv.Itoa(q.Limit) != values.Get("limit") || q.Limit < 1 || q.Limit > sqlite.MaxSearchResults {
			return timelineQuery{}, ErrTimelineRequest
		}
	}
	if values.Has("raw") {
		switch values.Get("raw") {
		case "0":
		case "1":
			q.Raw = true
		default:
			return timelineQuery{}, ErrTimelineRequest
		}
	}
	if values.Has("cursor") {
		cursor := values.Get("cursor")
		if len(cursor) != 47 {
			return timelineQuery{}, ErrTimelineRequest
		}
		wire, err := base64.RawURLEncoding.Strict().DecodeString(cursor)
		if err != nil || len(wire) != 35 || wire[0] != 1 || base64.RawURLEncoding.EncodeToString(wire) != cursor {
			return timelineQuery{}, ErrTimelineRequest
		}
		digest := timelineDigest(id, q.Raw)
		if string(wire[1:33]) != string(digest[:]) {
			return timelineQuery{}, ErrTimelineRequest
		}
		q.Start = int(binary.BigEndian.Uint16(wire[33:]))
		if q.Start < 1 || q.Start >= correlation.MaxPartitionFacts {
			return timelineQuery{}, ErrTimelineRequest
		}
	}
	return q, nil
}
