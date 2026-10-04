package model

import "time"

// MaxLineBytes is the maximum number of input bytes a parser will retain or inspect.
// Sources may choose a lower limit, but must consume an oversized record separately.
const MaxLineBytes = 64 * 1024

type TimeQuality string

const (
	TimeUnknown               TimeQuality = "unknown"
	TimeWallOnly              TimeQuality = "wall_only"
	TimeYearWithoutZone       TimeQuality = "year_without_zone"
	TimeConfiguredYearAndZone TimeQuality = "configured_year_and_zone"
	TimeInferredYearAndZone   TimeQuality = "inferred_year_and_zone"
	TimeExplicitOffset        TimeQuality = "explicit_offset"
)

// Timestamp preserves the original date even when it cannot be placed on a
// timeline. Value is set only when both year and timezone are known.
type Timestamp struct {
	Raw     string
	Value   *time.Time
	Quality TimeQuality
	Year    int    // zero means unknown
	Zone    string // empty means unknown
}

type Kind string

const (
	KindUnknown    Kind = "unknown"
	KindConnect    Kind = "connect"
	KindDisconnect Kind = "disconnect"
	KindReject     Kind = "reject"
	KindMessage    Kind = "message"
	KindDelivery   Kind = "delivery"
	KindRemoved    Kind = "removed"
	KindBounce     Kind = "bounce"
)

// Observation is a single parsed log record, not a delivery verdict. Empty
// fields are absent unless their names appear in Present.
type Observation struct {
	Raw        string
	SourceID   string
	Timestamp  Timestamp
	Host       string
	Program    string
	Service    string
	PID        string
	QueueID    string
	NoQueue    bool
	Kind       Kind
	Message    string // Postfix text after the optional queue ID
	Fields     map[string]string
	Present    map[string]bool
	ParseError string
}

// Field reports a field's value and whether it was observed. It distinguishes
// an absent field from one explicitly logged as empty.
func (o Observation) Field(name string) (string, bool) {
	return o.Fields[name], o.Present[name]
}
