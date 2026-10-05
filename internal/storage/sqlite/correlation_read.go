package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Coubiac/mailtrace/internal/correlation"
)

const MaxCorrelationScopeParts = 64

var (
	ErrCorrelationScope        = errors.New("invalid correlation read scope")
	ErrCorrelationStoredFields = errors.New("invalid stored correlation fields")
	ErrCorrelationStoredFact   = errors.New("invalid stored correlation fact")
)

// CorrelationScope is explicit selection, never a certificate of log coverage.
// Queues selects every stored origin for each exact configured instance/queue ID.
// UnqueuedInstances includes queue-less/NOQUEUE facts for the exact instances.
type CorrelationScope struct {
	Queues            []correlation.QueueKey
	UnqueuedInstances []string
}

// CorrelationFacts reads immutable persisted values in one SQL statement. It
// never reparses raw logs or adds a new date context. An extra row makes the
// entire read fail: no truncated snapshot can be mistaken for absence of facts.
// Missing queues mean absence in the selected database snapshot, not the logs.
func (s *Store) CorrelationFacts(ctx context.Context, scope CorrelationScope, limit int) ([]correlation.Fact, error) {
	if limit < 1 || limit > correlation.MaxPartitionFacts {
		return nil, correlation.ErrPartitionLimit
	}
	where, args, err := correlationSelection(scope)
	if err != nil {
		return nil, err
	}
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, `SELECT r.source_id, r.generation_id,
		r.start_offset, r.end_offset, r.raw, e.instance, e.time_utc_ns,
		e.time_raw, e.time_quality, e.time_year, e.time_zone, e.host, e.program,
		e.service, e.pid, e.kind, e.queue_id, e.no_queue, e.message, e.fields_json, e.parse_error
		FROM events e JOIN raw_records r ON r.id = e.raw_record_id
		WHERE `+where+` ORDER BY r.source_id, r.generation_id, r.start_offset LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []correlation.Fact
	for rows.Next() {
		if len(facts) == limit {
			return nil, correlation.ErrPartitionLimit
		}
		var f correlation.Fact
		var raw []byte
		var date sql.NullInt64
		var fieldsJSON string
		o := &f.Observation
		if err := rows.Scan(&f.Ref.SourceID, &f.Ref.OriginID, &f.Ref.Start, &f.Ref.End,
			&raw, &f.Instance, &date, &o.Timestamp.Raw, &o.Timestamp.Quality,
			&o.Timestamp.Year, &o.Timestamp.Zone, &o.Host, &o.Program, &o.Service,
			&o.PID, &o.Kind, &o.QueueID, &o.NoQueue, &o.Message, &fieldsJSON, &o.ParseError); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, ErrCorrelationStoredFact
		}
		o.SourceID, o.Raw = f.Ref.SourceID, string(raw)
		if date.Valid {
			value := time.Unix(0, date.Int64).UTC()
			o.Timestamp.Value = &value
		}
		fields, present, err := decodeCorrelationFields(fieldsJSON)
		if err != nil {
			return nil, err
		}
		o.Fields, o.Present = fields, present
		facts = append(facts, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if _, err := correlation.PartitionFacts(facts, limit); err != nil {
		return nil, err
	}
	return facts, nil
}

// Require the writer's exact object shape. Go's struct/map decoding silently
// accepts duplicate/case-folded members and converts null entries to zero values,
// which could fabricate an explicitly empty sender. Whole null maps are valid.
func decodeCorrelationFields(raw string) (map[string]string, map[string]bool, error) {
	if !utf8.ValidString(raw) {
		return nil, nil, ErrCorrelationStoredFields
	}
	d := json.NewDecoder(strings.NewReader(raw))
	if token, err := d.Token(); err != nil || token != json.Delim('{') {
		return nil, nil, ErrCorrelationStoredFields
	}
	var fields map[string]string
	var present map[string]bool
	seen := make(map[string]bool)
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return nil, nil, ErrCorrelationStoredFields
		}
		seen[key] = true
		switch key {
		case "fields":
			fields, err = decodeCorrelationMap[string](d)
		case "present":
			present, err = decodeCorrelationMap[bool](d)
		default:
			return nil, nil, ErrCorrelationStoredFields
		}
		if err != nil {
			return nil, nil, ErrCorrelationStoredFields
		}
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') || len(seen) != 2 {
		return nil, nil, ErrCorrelationStoredFields
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, nil, ErrCorrelationStoredFields
	}
	return fields, present, nil
}

func decodeCorrelationMap[T string | bool](d *json.Decoder) (map[string]T, error) {
	token, err := d.Token()
	if err != nil {
		return nil, ErrCorrelationStoredFields
	}
	if token == nil {
		return nil, nil
	}
	if token != json.Delim('{') {
		return nil, ErrCorrelationStoredFields
	}
	out := make(map[string]T)
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok {
			return nil, ErrCorrelationStoredFields
		}
		if _, duplicate := out[key]; duplicate {
			return nil, ErrCorrelationStoredFields
		}
		token, err = d.Token()
		value, ok := token.(T)
		if err != nil || !ok {
			return nil, ErrCorrelationStoredFields
		}
		out[key] = value
	}
	if token, err := d.Token(); err != nil || token != json.Delim('}') {
		return nil, ErrCorrelationStoredFields
	}
	return out, nil
}

func correlationSelection(scope CorrelationScope) (string, []any, error) {
	parts := len(scope.Queues) + len(scope.UnqueuedInstances)
	if parts < 1 || parts > MaxCorrelationScopeParts {
		return "", nil, ErrCorrelationScope
	}
	valid := func(v string, max int) bool {
		return v != "" && len(v) <= max && !strings.ContainsAny(v, "\x00\r\n\t")
	}
	var conditions []string
	var args []any
	queues := make(map[correlation.QueueKey]bool)
	for _, key := range scope.Queues {
		if !valid(key.Instance, 1024) || !valid(key.QueueID, 32) || queues[key] {
			return "", nil, ErrCorrelationScope
		}
		queues[key] = true
		conditions = append(conditions, "(e.instance = ? AND e.queue_id = ?)")
		args = append(args, key.Instance, key.QueueID)
	}
	instances := make(map[string]bool)
	for _, instance := range scope.UnqueuedInstances {
		if !valid(instance, 1024) || instances[instance] {
			return "", nil, ErrCorrelationScope
		}
		instances[instance] = true
		conditions = append(conditions, "(e.instance = ? AND (e.queue_id = '' OR e.no_queue = 1))")
		args = append(args, instance)
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args, nil
}
