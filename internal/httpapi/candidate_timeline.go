package httpapi

import (
	"context"
	"encoding/base64"
	"time"
	"unicode/utf8"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

// NativeField distinguishes absence from a logged empty value. Arbitrary maps
// and Message are never serialized; only these explicitly selected fields are.
type NativeField struct {
	Present bool         `json:"present"`
	Value   *NativeValue `json:"value"`
}

type TimelineDelivery struct {
	Recipient         NativeValue                `json:"recipient"`
	OriginalRecipient NativeField                `json:"original_recipient"`
	NativeStatus      NativeValue                `json:"native_status"`
	ObservedStatus    correlation.DeliveryStatus `json:"observed_status"`
	Scope             correlation.DeliveryScope  `json:"scope"`
	Relay             NativeField                `json:"relay"`
	DSN               NativeField                `json:"dsn"`
	Reply             NativeField                `json:"reply"`
}

type TimelineEvent struct {
	Ref          SearchRef         `json:"ref"`
	At           time.Time         `json:"at"`
	TimeQuality  model.TimeQuality `json:"time_quality"`
	TimestampRaw NativeValue       `json:"timestamp_raw"`
	Host         NativeValue       `json:"host"`
	Service      NativeValue       `json:"service"`
	PID          NativeValue       `json:"pid"`
	Kind         model.Kind        `json:"kind"`
	ParseFailed  bool              `json:"parse_failed"`
	Sender       NativeField       `json:"sender"`
	MessageID    NativeField       `json:"message_id"`
	Delivery     *TimelineDelivery `json:"delivery"`
	Raw          *NativeValue      `json:"raw,omitempty"`
}

type TimelineResponse struct {
	CandidateID          string          `json:"candidate_id"`
	CoverageUnproven     bool            `json:"coverage_unproven"`
	HasNonExplicitTime   bool            `json:"has_non_explicit_time"`
	CrossStreamUncertain bool            `json:"cross_stream_uncertain"`
	Ordering             string          `json:"ordering"`
	Limit                int             `json:"limit"`
	RawIncluded          bool            `json:"raw_included"`
	Events               []TimelineEvent `json:"events"`
	NextCursor           string          `json:"next_cursor,omitempty"`
}

func wireNative(value string, max int) (NativeValue, error) {
	if len(value) > max {
		return NativeValue{}, sqlite.ErrSearchStoredHit
	}
	if utf8.ValidString(value) {
		return NativeValue{Encoding: "utf8", Value: value}, nil
	}
	return NativeValue{Encoding: "base64", Value: base64.StdEncoding.EncodeToString([]byte(value))}, nil
}

func wireNativeField(value string, present bool, max int) (NativeField, error) {
	if !present {
		return NativeField{}, nil
	}
	native, err := wireNative(value, max)
	if err != nil {
		return NativeField{}, err
	}
	return NativeField{Present: true, Value: &native}, nil
}

func (h *searchHandler) timeline(ctx context.Context, key correlation.QueueInstanceKey, id string, q timelineQuery) (TimelineResponse, error) {
	queue, facts, err := h.candidateSnapshot(ctx, key)
	if err != nil {
		return TimelineResponse{}, err
	}
	generation := queue.Summary.Queue.Generation
	refs := generation.Facts
	// Check the snapshot revision first, including when a supplied position has
	// disappeared. Never apply an old position to a changed generation.
	if q.Start >= len(refs) {
		return TimelineResponse{}, ErrTimelineRequest
	}
	lookup := make(map[correlation.FactRef]correlation.Fact, len(facts))
	for _, fact := range facts {
		lookup[fact.Ref] = fact
	}
	end := min(q.Start+q.Limit, len(refs))
	out := TimelineResponse{CandidateID: id, CoverageUnproven: true, HasNonExplicitTime: generation.HasNonExplicitTime, CrossStreamUncertain: generation.CrossStreamUncertain,
		Ordering: "timestamp_then_provenance", Limit: q.Limit, RawIncluded: q.Raw, Events: []TimelineEvent{}}
	for _, ref := range refs[q.Start:end] {
		if err := ctx.Err(); err != nil {
			return TimelineResponse{}, err
		}
		fact, ok := lookup[ref]
		if !ok {
			return TimelineResponse{}, sqlite.ErrSearchStoredHit
		}
		event, err := timelineEvent(fact, q.Raw)
		if err != nil {
			return TimelineResponse{}, err
		}
		out.Events = append(out.Events, event)
	}
	if end < len(refs) {
		out.NextCursor, err = encodeTimelineCursor(id, q.Raw, end)
		if err != nil {
			return TimelineResponse{}, err
		}
	}
	return out, nil
}

func timelineEvent(f correlation.Fact, raw bool) (TimelineEvent, error) {
	o := f.Observation
	if o.Timestamp.Value == nil || !searchInstantFits(*o.Timestamp.Value) {
		return TimelineEvent{}, sqlite.ErrSearchStoredHit
	}
	ref, err := wireFactRef(f.Ref)
	if err != nil {
		return TimelineEvent{}, err
	}
	out := TimelineEvent{Ref: ref, At: o.Timestamp.Value.UTC(), TimeQuality: o.Timestamp.Quality, Kind: o.Kind, ParseFailed: o.ParseError != ""}
	for _, field := range []struct {
		value string
		max   int
		dest  *NativeValue
	}{{o.Timestamp.Raw, model.MaxLineBytes, &out.TimestampRaw}, {o.Host, 1024, &out.Host}, {o.Service, 1024, &out.Service}, {o.PID, 1024, &out.PID}} {
		*field.dest, err = wireNative(field.value, field.max)
		if err != nil {
			return TimelineEvent{}, err
		}
	}
	for _, field := range []struct {
		name string
		dest *NativeField
	}{{"from", &out.Sender}, {"message-id", &out.MessageID}} {
		value, present := o.Field(field.name)
		*field.dest, err = wireNativeField(value, present, 1024)
		if err != nil {
			return TimelineEvent{}, err
		}
	}
	if d, ok := correlation.DeliveryFrom(o); ok {
		delivery := TimelineDelivery{ObservedStatus: d.Status, Scope: d.Scope}
		delivery.Recipient, err = wireNative(d.Recipient, 1024)
		if err != nil {
			return TimelineEvent{}, err
		}
		delivery.NativeStatus, err = wireNative(d.NativeStatus, 1024)
		if err != nil {
			return TimelineEvent{}, err
		}
		for _, field := range []struct {
			value correlation.Field
			max   int
			dest  *NativeField
		}{{d.OriginalRecipient, 1024, &delivery.OriginalRecipient}, {d.Relay, model.MaxLineBytes, &delivery.Relay}, {d.DSN, 1024, &delivery.DSN}, {d.Reply, model.MaxLineBytes, &delivery.Reply}} {
			*field.dest, err = wireNativeField(field.value.Value, field.value.Present, field.max)
			if err != nil {
				return TimelineEvent{}, err
			}
		}
		out.Delivery = &delivery
	}
	if raw {
		value, err := wireNative(o.Raw, model.MaxLineBytes)
		if err != nil {
			return TimelineEvent{}, err
		}
		out.Raw = &value
	}
	return out, nil
}
