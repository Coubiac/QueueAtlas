// Package postfix turns one syslog record into one observation. It does not
// correlate records, assign a queue generation, or infer a delivery result.
package postfix

import (
	"regexp"
	"strings"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/parser/syslog"
)

type Options struct {
	SourceID string
	Time     syslog.TimeContext
}

var rejectMetadata = regexp.MustCompile(`^from=<([^<>]*)> to=<([^<>]*)> proto=([A-Za-z0-9_-]+)(?: helo=(?:<([^<>]*)>|([^\s;]+)))?$`)

var supported = map[string]bool{
	"smtpd": true, "pickup": true, "cleanup": true, "qmgr": true,
	"smtp": true, "lmtp": true, "local": true, "pipe": true,
	"bounce": true, "virtual": true,
}

var deliveryService = map[string]bool{
	"smtp": true, "lmtp": true, "local": true, "pipe": true, "virtual": true,
}

// Parse returns KindUnknown for any unrecognised or malformed line. All input
// is bounded by the syslog parser before field extraction or regex matching.
func Parse(line []byte, opts Options) model.Observation {
	e := syslog.Parse(line, opts.Time)
	o := model.Observation{
		Raw: e.Raw, SourceID: opts.SourceID, Timestamp: e.Timestamp,
		Host: e.Host, Program: e.Program, Service: e.Service, PID: e.PID,
		Kind: model.KindUnknown, ParseError: e.Error,
	}
	if e.Error != "" || !supported[e.Service] {
		return o
	}
	message := e.Message
	if strings.HasPrefix(message, "NOQUEUE: ") {
		o.NoQueue = true
		message = strings.TrimPrefix(message, "NOQUEUE: ")
	} else if i := strings.Index(message, ": "); i > 0 && validQueueID(message[:i]) {
		o.QueueID = message[:i]
		message = message[i+2:]
	}
	o.Message = message
	if o.NoQueue {
		if strings.HasPrefix(message, "reject:") || strings.HasPrefix(message, "reject_warning:") {
			o.Kind = model.KindReject
		}
		parseRejectFields(&o, message)
		return o
	}
	if e.Service == "smtpd" {
		switch {
		case strings.HasPrefix(message, "connect from "):
			o.Kind = model.KindConnect
		case strings.HasPrefix(message, "disconnect from "):
			o.Kind = model.KindDisconnect
		case strings.HasPrefix(message, "reject:") || strings.HasPrefix(message, "reject_warning:"):
			o.Kind = model.KindReject
		}
	}
	if o.QueueID == "" {
		if o.Kind == model.KindReject {
			parseRejectFields(&o, message)
		}
		return o
	}
	if e.Service == "qmgr" && message == "removed" {
		o.Kind = model.KindRemoved
		return o
	}
	if e.Service == "bounce" {
		o.Kind = model.KindBounce
		const notification = "sender non-delivery notification: "
		if strings.HasPrefix(message, notification) {
			id := strings.TrimPrefix(message, notification)
			if validQueueID(id) {
				setField(&o, "notification_queue_id", id)
			}
		}
	} else {
		o.Kind = model.KindMessage
	}
	if e.Service == "pickup" || e.Service == "smtpd" {
		parseSpaceFields(&o, message)
	} else {
		parseCommaFields(&o, message)
	}
	if deliveryService[e.Service] && o.Present["to"] && o.Present["status"] {
		o.Kind = model.KindDelivery
	}
	return o
}

func validQueueID(v string) bool {
	if len(v) < 3 || len(v) > 32 {
		return false
	}
	hasDigit, allUpperHex := false, true
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c >= '0' && c <= '9' {
			hasDigit = true
		} else if c < 'A' || c > 'Z' {
			if c < 'a' || c > 'z' {
				return false
			}
		}
		if c < 'A' || c > 'F' {
			allUpperHex = false
		}
	}
	return hasDigit || allUpperHex
}

func setField(o *model.Observation, key, value string) {
	if o.Fields == nil {
		o.Fields = make(map[string]string)
		o.Present = make(map[string]bool)
	}
	if len(o.Fields) >= 32 && !o.Present[key] {
		return
	}
	if o.Present[key] {
		return // a second key in untrusted text must not replace an observed value
	}
	// The enclosing brackets belong to addresses/IDs, never to a delivery
	// reply. Its punctuation must survive unchanged for later projections.
	if key != "reply" {
		value = trimAngle(value)
	}
	o.Fields[key] = value
	o.Present[key] = true
}

func trimAngle(v string) string {
	if len(v) >= 2 && v[0] == '<' && v[len(v)-1] == '>' {
		return v[1 : len(v)-1]
	}
	return v
}

// HasExactStatusReply checks the first native status field using the same bounded
// field boundaries as Parse, without address/ID normalisation. It supports
// conservative projections of legacy observations whose reply was normalised;
// a later status-looking fragment cannot replace the first field's evidence.
func HasExactStatusReply(message, status, reply string) bool {
	if len(message) > model.MaxLineBytes || len(status) > model.MaxLineBytes ||
		len(reply) > model.MaxLineBytes || len(status)+len(reply)+3 > model.MaxLineBytes {
		return false
	}
	for _, part := range topLevelParts(message, 32) {
		part = strings.TrimSpace(part)
		i := strings.IndexByte(part, '=')
		if i > 0 && part[:i] == "status" {
			return strings.TrimSpace(part[i+1:]) == status+" ("+reply+")"
		}
	}
	return false
}

func parseCommaFields(o *model.Observation, message string) {
	for _, part := range topLevelParts(message, 32) {
		part = strings.TrimSpace(part)
		i := strings.IndexByte(part, '=')
		if i <= 0 {
			continue
		}
		key := part[:i]
		if !validKey(key) {
			continue
		}
		value := strings.TrimSpace(part[i+1:])
		if key == "status" {
			if open := strings.Index(value, " ("); open >= 0 {
				setField(o, "status", value[:open])
				reply := value[open+2:]
				if strings.HasSuffix(reply, ")") {
					reply = reply[:len(reply)-1]
				}
				setField(o, "reply", reply)
				break // delivery status and its remote reply are the final field
			}
			setField(o, key, value)
			break
		}
		setField(o, key, value)
	}
}

func topLevelParts(v string, limit int) []string {
	parts := make([]string, 0, 8)
	start, paren, angle, square := 0, 0, 0, 0
	quoted, escaped := false, false
	for i := 0; i < len(v) && len(parts) < limit-1; i++ {
		switch c := v[i]; {
		case escaped:
			escaped = false
		case quoted && c == '\\':
			escaped = true
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '(':
			paren++
		case c == ')' && paren > 0:
			paren--
		case c == '<':
			angle++
		case c == '>' && angle > 0:
			angle--
		case c == '[':
			square++
		case c == ']' && square > 0:
			square--
		case c == ',' && paren == 0 && angle == 0 && square == 0:
			parts = append(parts, v[start:i])
			start = i + 1
		}
	}
	parts = append(parts, v[start:])
	return parts
}

func validKey(key string) bool {
	if key == "" || len(key) > 32 {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && (c >= '0' && c <= '9' || c == '_' || c == '-') {
			continue
		}
		return false
	}
	return true
}

func parseSpaceFields(o *model.Observation, message string) {
	// A quoted local-part may contain spaces, semicolons and angle brackets.
	// Split only outside quotes and the enclosing address, so hostile sender
	// text cannot become another observed key. Escaped quotes stay in the value.
	start := 0
	quoted, escaped, angle := false, false, false
	for i := 0; i <= len(message); i++ {
		if i < len(message) {
			c := message[i]
			switch {
			case escaped:
				escaped = false
				continue
			case quoted && c == '\\':
				escaped = true
				continue
			case c == '"':
				quoted = !quoted
				continue
			case !quoted && c == '<':
				angle = true
				continue
			case !quoted && c == '>':
				angle = false
				continue
			}
			if quoted || angle || strings.IndexByte(" \t\r\n\v\f;", c) < 0 {
				continue
			}
		}
		if quoted || angle {
			return // incomplete value: preserve raw, never extract its inner keys
		}
		part := strings.TrimSuffix(message[start:i], ",")
		start = i + 1
		equal := strings.IndexByte(part, '=')
		if equal <= 0 || equal == len(part)-1 || !validKey(part[:equal]) {
			continue
		}
		setField(o, part[:equal], part[equal+1:])
		if len(o.Fields) >= 32 {
			return
		}
	}
}

func parseRejectFields(o *model.Observation, message string) {
	// Postfix appends this metadata after the free-form rejection text.
	// If the delimiter appears twice, a forged field may be present; preserve
	// the raw line without asserting a recipient or sender.
	if strings.Count(message, "; from=<") != 1 {
		return
	}
	i := strings.Index(message, "; from=<")
	parts := rejectMetadata.FindStringSubmatch(message[i+2:])
	if parts == nil {
		return
	}
	setField(o, "from", parts[1])
	setField(o, "to", parts[2])
	setField(o, "proto", parts[3])
	if parts[4] != "" {
		setField(o, "helo", parts[4])
	} else if parts[5] != "" {
		setField(o, "helo", parts[5])
	}
}
