package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Coubiac/mailtrace/internal/correlation"
	"github.com/Coubiac/mailtrace/internal/model"
)

const (
	MaxSearchResults = 200
	MaxSearchWindow  = 31 * 24 * time.Hour
)

type SearchField string

const (
	SearchSender          SearchField = "sender"
	SearchRecipient       SearchField = "recipient"
	SearchQueueID         SearchField = "queue_id"
	SearchMessageID       SearchField = "message_id"
	SearchSenderDomain    SearchField = "sender_domain"
	SearchRecipientDomain SearchField = "recipient_domain"
)

var (
	ErrSearchQuery     = errors.New("invalid event search query")
	ErrSearchCursor    = errors.New("invalid event search cursor")
	ErrSearchStoredHit = errors.New("invalid stored event search result")
)

// SearchQuery selects exact persisted field values, not inferred identities.
// From is inclusive, Until exclusive; both must fit stored UTC nanoseconds.
// Native address values may be explicitly empty; identifiers cannot. Domain
// criteria use a normalized ASCII DNS subset. SQL NULL never matches.
type SearchQuery struct {
	Instance    string
	Field       SearchField
	Value       string
	From, Until time.Time
	Limit       int
	After       *SearchCursor
}

// SearchCursor is pagination position, not fact identity or authorization.
// QueryRevision binds it to instance/field/value/window (not page size).
// Subsequent pages use new snapshots; earlier late imports require a new search.
type SearchCursor struct {
	QueryRevision string
	TimeNS, RowID int64
}

// SearchHit identifies an observed event by physical provenance. It is not a
// complete correlation input, a message identity, or a delivery status summary.
type SearchHit struct {
	Ref         correlation.FactRef
	Instance    string
	QueueID     string
	NoQueue     bool
	At          time.Time
	TimeQuality model.TimeQuality
	Kind        model.Kind
}

type SearchPage struct {
	Hits []SearchHit
	Next *SearchCursor
}

// SearchEvents uses field/time indexes. No raw log is
// reparsed. Values are literal parameters and columns come from a closed enum.
// Only events with a stored UTC instant in the window are included; missing
// dates do not mean absence in the logs. Caller provides its context/deadline.
func (s *Store) SearchEvents(ctx context.Context, query SearchQuery) (SearchPage, error) {
	where, args, revision, err := eventSearchSelection(query)
	if err != nil {
		return SearchPage{}, err
	}
	args = append(args, query.Limit+1)
	rows, err := s.db.QueryContext(ctx, `SELECT e.id, e.time_utc_ns,
		r.source_id, r.generation_id, r.start_offset, r.end_offset,
		e.instance, e.queue_id, e.no_queue, e.time_quality, e.kind
		FROM `+eventSearchFrom(query.Field)+` WHERE `+where+
		` ORDER BY `+eventSearchOrder(query.Field)+` LIMIT ?`, args...)
	if err != nil {
		return SearchPage{}, err
	}
	defer rows.Close()
	var page SearchPage
	var last SearchCursor
	for rows.Next() {
		if len(page.Hits) == query.Limit {
			cursor := last
			page.Next = &cursor
			break
		}
		var hit SearchHit
		var rowID, ns int64
		if err := rows.Scan(&rowID, &ns, &hit.Ref.SourceID, &hit.Ref.OriginID,
			&hit.Ref.Start, &hit.Ref.End, &hit.Instance, &hit.QueueID, &hit.NoQueue,
			&hit.TimeQuality, &hit.Kind); err != nil {
			if ctx.Err() != nil {
				return SearchPage{}, ctx.Err()
			}
			return SearchPage{}, ErrSearchStoredHit
		}
		if hit.Ref.SourceID == "" || hit.Ref.OriginID == "" || hit.Ref.Start < 0 || hit.Ref.End <= hit.Ref.Start || hit.Instance != query.Instance {
			return SearchPage{}, ErrSearchStoredHit
		}
		hit.At = time.Unix(0, ns).UTC()
		page.Hits = append(page.Hits, hit)
		last = SearchCursor{QueryRevision: revision, TimeNS: ns, RowID: rowID}
	}
	if err := rows.Err(); err != nil {
		return SearchPage{}, err
	}
	return page, nil
}

func eventSearchSelection(query SearchQuery) (string, []any, string, error) {
	column := ""
	maxValue := 1024
	condition := ""
	instanceColumn, timeColumn, idColumn := "e.instance", "e.time_utc_ns", "e.id"
	switch query.Field {
	case SearchSender:
		column = "e.sender"
	case SearchRecipient:
		column = "e.recipient"
	case SearchQueueID:
		column = "e.queue_id"
		maxValue = 32
		// Match the existing partial index predicate explicitly. Queue-less
		// events are not searched as if they had a queue identity.
		condition = ` AND e.queue_id <> ''`
	case SearchMessageID:
		column = "e.message_id"
	case SearchSenderDomain, SearchRecipientDomain:
		var ok bool
		query.Value, ok = normalizeSearchDomain(query.Value)
		if !ok {
			return "", nil, "", ErrSearchQuery
		}
		column = "d.sender_domain"
		if query.Field == SearchRecipientDomain {
			column = "d.recipient_domain"
		}
		maxValue = 253
		instanceColumn, timeColumn, idColumn = "d.instance", "d.time_utc_ns", "d.event_id"
	default:
		return "", nil, "", ErrSearchQuery
	}
	from, fromOK := searchTimeNS(query.From)
	until, untilOK := searchTimeNS(query.Until)
	if query.Instance == "" || len(query.Instance) > 1024 || len(query.Value) > maxValue ||
		((query.Field == SearchQueueID || query.Field == SearchMessageID) && query.Value == "") ||
		strings.ContainsAny(query.Instance+query.Value, "\x00\r\n\t") || query.Limit < 1 || query.Limit > MaxSearchResults ||
		!fromOK || !untilOK || !query.Until.After(query.From) || query.Until.Sub(query.From) > MaxSearchWindow {
		return "", nil, "", ErrSearchQuery
	}
	h := sha256.New()
	for _, value := range []string{"event-search-v1", query.Instance, string(query.Field), query.Value, strconv.FormatInt(from, 10), strconv.FormatInt(until, 10)} {
		projectionScopeFrame(h, value)
	}
	revision := hex.EncodeToString(h.Sum(nil))
	where := column + ` = ? AND ` + instanceColumn + ` = ? AND ` + timeColumn + ` >= ? AND ` + timeColumn + ` < ?`
	where += condition
	args := []any{query.Value, query.Instance, from, until}
	if query.After != nil {
		cursor := *query.After
		if cursor.QueryRevision != revision || cursor.TimeNS < from || cursor.TimeNS >= until {
			return "", nil, "", ErrSearchCursor
		}
		// Narrow the scalar index range as well as the tuple predicate. Keeping
		// the original From here can make SQLite scan earlier matching times
		// before applying the seek condition. Equal-time IDs still use the tuple.
		args[2] = cursor.TimeNS
		where += ` AND (` + timeColumn + `, ` + idColumn + `) > (?, ?)`
		args = append(args, cursor.TimeNS, cursor.RowID)
	}
	return where, args, revision, nil
}

func isDomainSearch(field SearchField) bool {
	return field == SearchSenderDomain || field == SearchRecipientDomain
}

func eventSearchFrom(field SearchField) string {
	if isDomainSearch(field) {
		return `event_search_domains d JOIN events e ON e.id=d.event_id JOIN raw_records r ON r.id=e.raw_record_id`
	}
	return `events e JOIN raw_records r ON r.id=e.raw_record_id`
}

func eventSearchOrder(field SearchField) string {
	if isDomainSearch(field) {
		return `d.time_utc_ns, d.event_id`
	}
	return `e.time_utc_ns, e.id`
}

func searchTimeNS(value time.Time) (int64, bool) {
	if value.IsZero() {
		return 0, false
	}
	ns := value.UTC().UnixNano()
	return ns, time.Unix(0, ns).Equal(value)
}
