package syslog

import (
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/model"
)

func TestParseEnvelopeAndTimeQuality(t *testing.T) {
	line := []byte("Oct  3 12:34:56 mx1 postfix/smtp[1234]: ABC123: to=<a@example.org>\n")
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		context TimeContext
		quality model.TimeQuality
		year    int
		value   bool
	}{
		{"no context", TimeContext{}, model.TimeWallOnly, 0, false},
		{"year without zone", TimeContext{Year: 2026}, model.TimeYearWithoutZone, 2026, false},
		{"year and zone", TimeContext{Year: 2026, Location: paris}, model.TimeConfiguredYearAndZone, 2026, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Parse(line, tt.context)
			if e.Error != "" || e.Raw != string(line) || e.Host != "mx1" || e.Program != "postfix/smtp" || e.Service != "smtp" || e.PID != "1234" || e.Message != "ABC123: to=<a@example.org>" {
				t.Fatalf("unexpected envelope: %+v", e)
			}
			if e.Timestamp.Raw != "Oct  3 12:34:56" || e.Timestamp.Quality != tt.quality || e.Timestamp.Year != tt.year || (e.Timestamp.Value != nil) != tt.value {
				t.Fatalf("unexpected timestamp: %+v", e.Timestamp)
			}
			if tt.value && (e.Timestamp.Value.Location() != paris || e.Timestamp.Value.Hour() != 12) {
				t.Fatalf("incorrect location/value: %+v", e.Timestamp.Value)
			}
		})
	}
}

func TestInferYearAcrossNewYear(t *testing.T) {
	loc := time.FixedZone("UTC+1", 3600)
	reference := time.Date(2026, 1, 1, 0, 1, 0, 0, loc)
	ctx := TimeContext{Reference: &reference, Location: loc}
	for _, tt := range []struct {
		line string
		year int
	}{
		{"Dec 31 23:59:59 mx postfix/qmgr[1]: ABC123: removed", 2025},
		{"Jan  1 00:02:00 mx postfix/qmgr[1]: ABC123: removed", 2026},
	} {
		e := Parse([]byte(tt.line), ctx)
		if e.Error != "" || e.Timestamp.Year != tt.year || e.Timestamp.Quality != model.TimeInferredYearAndZone {
			t.Fatalf("%q: %+v", tt.line, e)
		}
	}
	withoutZone := Parse([]byte("Dec 31 23:59:59 mx postfix/qmgr[1]: ABC123: removed"), TimeContext{Reference: &reference})
	if withoutZone.Timestamp.Year != 0 || withoutZone.Timestamp.Value != nil || withoutZone.Timestamp.Quality != model.TimeWallOnly {
		t.Fatalf("inferred a date without log timezone: %+v", withoutZone.Timestamp)
	}
}

func TestMalformedAndOversized(t *testing.T) {
	for _, line := range []string{
		"", "not syslog", "Foo  3 12:34:56 mx postfix/smtp[1]: text",
		"Feb 30 12:34:56 mx postfix/smtp[1]: text",
		"Oct  3 12:34:56 mx postfix/smtp[x]: text",
		"Oct  3 12:34:56 mx postfix/smtp[1] text",
	} {
		if e := Parse([]byte(line), TimeContext{}); e.Error == "" {
			t.Fatalf("accepted malformed line %q", line)
		}
	}
	line := []byte(strings.Repeat("x", model.MaxLineBytes+100))
	e := Parse(line, TimeContext{})
	if e.Error != "line_too_long" || len(e.Raw) != model.MaxLineBytes {
		t.Fatalf("oversized line not bounded: error=%q len=%d", e.Error, len(e.Raw))
	}
}

func TestRFC5424Envelope(t *testing.T) {
	line := "<134>1 2026-10-03T12:34:56+02:00 mx.example.org postfix/smtp 42 - [meta key=\"a] b\"] ABC123: to=<bob@example.org>, status=sent (250 OK)"
	e := Parse([]byte(line), TimeContext{})
	if e.Error != "" || e.Service != "smtp" || e.PID != "42" || e.Host != "mx.example.org" || e.Timestamp.Quality != model.TimeExplicitOffset || e.Timestamp.Value == nil || e.Message != "ABC123: to=<bob@example.org>, status=sent (250 OK)" {
		t.Fatalf("RFC5424 envelope: %+v", e)
	}
}

func FuzzParse(f *testing.F) {
	f.Add([]byte("Oct  3 12:34:56 mx postfix/smtp[123]: ABC123: status=sent (250 OK)"))
	f.Add([]byte{0, 255, '\n'})
	f.Fuzz(func(t *testing.T, input []byte) {
		e := Parse(input, TimeContext{})
		if len(e.Raw) > model.MaxLineBytes {
			t.Fatalf("unbounded raw record: %d", len(e.Raw))
		}
	})
}
