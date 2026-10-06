package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Derived search columns live separately so migration never updates immutable
// events referenced by a projection. Native addresses remain byte-for-byte.
const schemaV6 = `
CREATE TABLE event_search_domains (
    event_id INTEGER PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    instance TEXT NOT NULL,
    time_utc_ns INTEGER,
    sender_domain TEXT,
    recipient_domain TEXT
);
CREATE INDEX search_sender_domain_time ON event_search_domains(instance,sender_domain,time_utc_ns,event_id) WHERE sender_domain IS NOT NULL;
CREATE INDEX search_recipient_domain_time ON event_search_domains(instance,recipient_domain,time_utc_ns,event_id) WHERE recipient_domain IS NOT NULL;
`

// This conservative ASCII DNS subset performs no IDNA, trailing-dot or domain
// literal conversion. A-labels are accepted as literal DNS labels, not verified.
func normalizeSearchDomain(value string) (string, bool) {
	if len(value) == 0 || len(value) > 253 {
		return "", false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", false
		}
		for i := 0; i < len(label); i++ {
			b := label[i]
			if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-') {
				return "", false
			}
		}
	}
	return strings.ToLower(value), true
}

func addressSearchDomain(value any) any {
	address, ok := value.(string)
	if !ok || !utf8.ValidString(address) || strings.Count(address, "@") != 1 {
		return nil
	}
	for _, r := range address {
		if unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune("<>\"", r) {
			return nil
		}
	}
	at := strings.IndexByte(address, '@')
	if at == 0 {
		return nil
	}
	domain, ok := normalizeSearchDomain(address[at+1:])
	if !ok {
		return nil
	}
	return domain
}

func insertSearchDomains(ctx context.Context, tx *sql.Tx, eventID int64, instance string, ns, sender, recipient any) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO event_search_domains(event_id,instance,time_utc_ns,sender_domain,recipient_domain) VALUES(?,?,?,?,?)`,
		eventID, instance, ns, addressSearchDomain(sender), addressSearchDomain(recipient))
	return err
}

func backfillSearchDomains(ctx context.Context, tx *sql.Tx) error {
	type row struct {
		id                int64
		instance          string
		ns                sql.NullInt64
		sender, recipient sql.NullString
	}
	// One migration transaction, bounded working memory. Existing rows may have
	// no date/address; NULL stays absent and no log or JSON is reparsed.
	var last int64
	first := true
	for {
		where := ""
		var args []any
		if !first {
			where = ` WHERE id > ?`
			args = append(args, last)
		}
		rows, err := tx.QueryContext(ctx, `SELECT id,instance,time_utc_ns,sender,recipient FROM events`+where+` ORDER BY id LIMIT 256`, args...)
		if err != nil {
			return err
		}
		batch := make([]row, 0, 256)
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.id, &r.instance, &r.ns, &r.sender, &r.recipient); err != nil {
				rows.Close()
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return errors.New("invalid stored search domain input")
			}
			batch = append(batch, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, r := range batch {
			var ns, sender, recipient any
			if r.ns.Valid {
				ns = r.ns.Int64
			}
			if r.sender.Valid {
				sender = r.sender.String
			}
			if r.recipient.Valid {
				recipient = r.recipient.String
			}
			if err := insertSearchDomains(ctx, tx, r.id, r.instance, ns, sender, recipient); err != nil {
				return err
			}
			last = r.id
		}
		if len(batch) < 256 {
			return nil
		}
		first = false
	}
}
