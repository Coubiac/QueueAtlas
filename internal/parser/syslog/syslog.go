// Package syslog parses the envelope around a Postfix log message.
package syslog

import (
	"errors"
	"strings"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
)

// TimeContext contains only caller-provided facts. Reference enables nearest-
// year inference around New Year, but only with Location, which must be the
// log host's timezone. Year takes precedence over Reference.
type TimeContext struct {
	Year      int
	Reference *time.Time
	Location  *time.Location
}

type Envelope struct {
	Raw       string
	Timestamp model.Timestamp
	Host      string
	Program   string
	Service   string
	PID       string
	Message   string
	Error     string
}

// Parse never panics and never inspects more than MaxLineBytes. Invalid lines
// retain their bounded raw content with a diagnostic in Error.
func Parse(line []byte, ctx TimeContext) Envelope {
	if len(line) > model.MaxLineBytes {
		return Envelope{Raw: string(line[:model.MaxLineBytes]), Error: "line_too_long"}
	}
	original := string(line)
	raw := strings.TrimSuffix(original, "\n")
	raw = strings.TrimSuffix(raw, "\r")
	e := Envelope{Raw: original}
	if strings.HasPrefix(raw, "<") {
		parseRFC5424(&e, raw)
		return e
	}
	if len(raw) < 17 || raw[15] != ' ' {
		e.Error = "invalid_syslog_envelope"
		return e
	}
	timestamp := raw[:15]
	parsed, err := time.Parse("Jan _2 15:04:05", timestamp)
	if err != nil || parsed.Format("Jan _2 15:04:05") != timestamp {
		e.Error = "invalid_syslog_timestamp"
		return e
	}
	e.Timestamp = resolveTime(timestamp, parsed, ctx)
	rest := raw[16:]
	hostEnd := strings.IndexByte(rest, ' ')
	if hostEnd <= 0 || hostEnd == len(rest)-1 {
		e.Error = "invalid_syslog_host"
		return e
	}
	e.Host = rest[:hostEnd]
	rest = rest[hostEnd+1:]
	colon := strings.Index(rest, ": ")
	if colon <= 0 {
		e.Error = "invalid_syslog_program"
		return e
	}
	program, pid, err := parseProgram(rest[:colon])
	if err != nil {
		e.Error = "invalid_syslog_program"
		return e
	}
	e.Program, e.PID = program, pid
	e.Service = postfixService(program)
	e.Message = rest[colon+2:]
	return e
}

func postfixService(program string) string {
	if !strings.HasPrefix(program, "postfix/") {
		return ""
	}
	service := program[strings.LastIndexByte(program, '/')+1:]
	if service == "" {
		return ""
	}
	return service
}

// parseRFC5424 handles the RFC 5424 header and leaves structured data opaque.
// Postfix's message begins after the structured data delimiter.
func parseRFC5424(e *Envelope, raw string) {
	close := strings.IndexByte(raw, '>')
	if close < 2 || close > 4 || close+1 >= len(raw) || raw[close+1] != '1' {
		e.Error = "invalid_syslog_envelope"
		return
	}
	priority := 0
	for _, c := range raw[1:close] {
		if c < '0' || c > '9' {
			e.Error = "invalid_syslog_envelope"
			return
		}
		priority = priority*10 + int(c-'0')
	}
	if priority > 191 || close+2 >= len(raw) || raw[close+2] != ' ' {
		e.Error = "invalid_syslog_envelope"
		return
	}
	rest := raw[close+3:]
	var tokens [5]string
	for i := range tokens {
		end := strings.IndexByte(rest, ' ')
		if end <= 0 {
			e.Error = "invalid_syslog_envelope"
			return
		}
		tokens[i] = rest[:end]
		rest = rest[end+1:]
	}
	stamp, host, program, pid := tokens[0], tokens[1], tokens[2], tokens[3]
	if stamp != "-" {
		v, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			e.Error = "invalid_syslog_timestamp"
			return
		}
		e.Timestamp = model.Timestamp{Raw: stamp, Value: &v, Quality: model.TimeExplicitOffset, Year: v.Year(), Zone: v.Format("-07:00")}
	} else {
		e.Timestamp = model.Timestamp{Raw: stamp, Quality: model.TimeUnknown}
	}
	if host != "-" {
		e.Host = host
	}
	if program != "-" {
		e.Program = program
		e.Service = postfixService(program)
	}
	if pid != "-" {
		e.PID = pid
	}
	if rest == "-" {
		return // NILVALUE structured data, no message
	}
	if strings.HasPrefix(rest, "- ") {
		e.Message = rest[2:]
		return
	}
	if !strings.HasPrefix(rest, "[") {
		e.Error = "invalid_syslog_structured_data"
		return
	}
	// A series of SD-ELEMENTs can contain spaces, escaped brackets and quotes.
	i := 0
	for i < len(rest) && rest[i] == '[' {
		quoted, escaped := false, false
		closed := false
		for i++; i < len(rest); i++ {
			c := rest[i]
			switch {
			case escaped:
				escaped = false
			case c == '\\' && quoted:
				escaped = true
			case c == '"':
				quoted = !quoted
			case c == ']' && !quoted:
				i++
				closed = true
			}
			if closed {
				break
			}
		}
		if !closed {
			e.Error = "invalid_syslog_structured_data"
			return
		}
	}
	if i == len(rest) {
		return
	}
	if rest[i] != ' ' {
		e.Error = "invalid_syslog_structured_data"
		return
	}
	e.Message = rest[i+1:]
}

func parseProgram(tag string) (string, string, error) {
	if tag == "" || strings.ContainsAny(tag, " \t\r\n") {
		return "", "", errors.New("invalid program")
	}
	if !strings.HasSuffix(tag, "]") {
		if strings.ContainsAny(tag, "[]") {
			return "", "", errors.New("invalid pid")
		}
		return tag, "", nil
	}
	open := strings.LastIndexByte(tag, '[')
	if open <= 0 || open == len(tag)-2 {
		return "", "", errors.New("invalid pid")
	}
	for _, c := range tag[open+1 : len(tag)-1] {
		if c < '0' || c > '9' {
			return "", "", errors.New("invalid pid")
		}
	}
	return tag[:open], tag[open+1 : len(tag)-1], nil
}

func resolveTime(raw string, wall time.Time, ctx TimeContext) model.Timestamp {
	t := model.Timestamp{Raw: raw, Quality: model.TimeWallOnly}
	year := ctx.Year
	inferred := false
	if year == 0 && ctx.Reference != nil && ctx.Location != nil {
		ref := ctx.Reference.In(ctx.Location)
		best := time.Duration(1<<63 - 1)
		for candidate := ref.Year() - 1; candidate <= ref.Year()+1; candidate++ {
			v := time.Date(candidate, wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), 0, ctx.Location)
			if v.Month() != wall.Month() || v.Day() != wall.Day() {
				continue
			}
			d := v.Sub(ref)
			if d < 0 {
				d = -d
			}
			if d < best {
				best, year = d, candidate
			}
		}
		inferred = year != 0
	}
	if year == 0 {
		return t
	}
	t.Year = year
	if ctx.Location == nil {
		t.Quality = model.TimeYearWithoutZone
		return t
	}
	v := time.Date(year, wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), 0, ctx.Location)
	if v.Month() != wall.Month() || v.Day() != wall.Day() || v.Hour() != wall.Hour() {
		// This can occur on a nonexistent local time during a DST jump.
		t.Quality = model.TimeYearWithoutZone
		return t
	}
	t.Value = &v
	t.Zone = ctx.Location.String()
	if inferred {
		t.Quality = model.TimeInferredYearAndZone
	} else {
		t.Quality = model.TimeConfiguredYearAndZone
	}
	return t
}
