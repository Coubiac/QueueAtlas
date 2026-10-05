package correlation

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/parser/postfix"
	"github.com/Coubiac/mailtrace/internal/parser/syslog"
)

func sessionFacts(lines []string, source string) []Fact {
	var out []Fact
	var offset int64
	for _, line := range lines {
		end := offset + int64(len(line)+1)
		o := postfix.Parse([]byte(line), postfix.Options{SourceID: source, Time: syslog.TimeContext{Year: 2026, Location: time.UTC}})
		out = append(out, Fact{FactRef{source, "synthetic-session", offset, end}, "trusted", o})
		offset = end
	}
	return out
}

func sessionLines() []string {
	return []string{
		"Oct  3 12:05:00 mx postfix/smtpd[1151]: connect from bad.example.net[192.0.2.15]",
		"Oct  3 12:05:01 mx postfix/smtpd[1151]: NOQUEUE: reject: RCPT from bad.example.net[192.0.2.15]: 550 denied; from=<a@example.org> to=<b@example.org> proto=ESMTP helo=<bad>",
		"Oct  3 12:05:02 mx postfix/smtpd[1151]: NOQUEUE: reject_warning: RCPT from bad.example.net[192.0.2.15]: 550 warning; from=<a@example.org> to=<c@example.org> proto=ESMTP helo=<bad>",
		"Oct  3 12:05:03 mx postfix/smtpd[1151]: disconnect from bad.example.net[192.0.2.15] ehlo=1 rcpt=0/2 quit=1 commands=3/5",
	}
}

func TestPrequeueSessionsRequireNativeBoundariesAndKeepEveryReport(t *testing.T) {
	facts := sessionFacts(sessionLines(), "live")
	out, err := BuildPrequeueSessions(facts, 100)
	if err != nil || len(out.Sessions) != 1 || len(out.Unassigned) != 0 || len(out.Prequeue.Attempts) != 2 || len(out.Prequeue.Other) != 2 {
		t.Fatalf("session: %#v %v", out, err)
	}
	s := out.Sessions[0]
	if s.Connect != facts[0].Ref || s.Disconnect != facts[3].Ref || !reflect.DeepEqual(s.Attempts, []FactRef{facts[1].Ref, facts[2].Ref}) || s.Client != "bad.example.net[192.0.2.15]" || s.Instance != "trusted" || !s.CoverageUnproven || !s.HasNonExplicitTime {
		t.Fatalf("evidence/reserves: %#v", s)
	}
	if out.Prequeue.Attempts[0].Disposition != PrequeueRejected || out.Prequeue.Attempts[1].Disposition != PrequeueWarning {
		t.Fatal("warning changed during session assignment")
	}
	rng := rand.New(rand.NewSource(98))
	for i := 0; i < 25; i++ {
		shuffled := append([]Fact(nil), facts...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		got, err := BuildPrequeueSessions(shuffled, 100)
		if err != nil || !reflect.DeepEqual(got, out) {
			t.Fatalf("permutation%d: %v", i, err)
		}
	}
	out.Sessions[0].Attempts[0] = FactRef{}
	*out.Prequeue.Attempts[0].Timestamp.Value = out.Prequeue.Attempts[0].Timestamp.Value.AddDate(1, 0, 0)
	if facts[1].Ref.SourceID == "" || facts[1].Observation.Timestamp.Value.Year() != 2026 {
		t.Fatal("candidate aliases facts")
	}
}

func TestPrequeueSessionsNeverJoinPIDReuseOriginsOrAcceptedQueue(t *testing.T) {
	lines := sessionLines()
	for _, line := range sessionLines() {
		lines = append(lines, strings.Replace(line, "12:05:", "12:15:", 1))
	}
	facts := append(sessionFacts(lines, "live"), sessionFacts(lines, "archive")...)
	out, err := BuildPrequeueSessions(facts, 100)
	if err != nil || len(out.Sessions) != 4 || len(out.Unassigned) != 0 || len(out.Prequeue.Attempts) != 8 {
		t.Fatalf("recycled PID/source: %#v %v", out, err)
	}
	for _, s := range out.Sessions {
		if len(s.Attempts) != 2 || !s.CoverageUnproven {
			t.Fatalf("sessions collapsed: %#v", s)
		}
	}
	// The closed window can contain a queued smtpd fact, without attributing
	// the NOQUEUE recipient to that queue (or projecting a session/queue link).
	lines = sessionLines()
	lines = append(lines[:3], "Oct  3 12:05:02 mx postfix/smtpd[1151]: ABC123: client=bad.example.net[192.0.2.15]", lines[3])
	facts = sessionFacts(lines, "live")
	out, err = BuildPrequeueSessions(facts, 100)
	if err != nil || len(out.Sessions) != 1 || len(out.Prequeue.Queued) != 1 || len(out.Sessions[0].Attempts) != 2 || len(out.Prequeue.Queued[0].Streams[0].Timed) != 1 {
		t.Fatalf("accepted queue assignment: %#v %v", out, err)
	}
}

func TestPrequeueSessionsQuarantineDoubtfulWholeWindow(t *testing.T) {
	cases := []struct {
		name   string
		edit   func([]Fact)
		reason SessionReason
	}{
		{"mismatched report", func(f []Fact) {
			f[2].Observation.Message = strings.ReplaceAll(f[2].Observation.Message, "192.0.2.15", "192.0.2.99")
		}, SessionClientMismatch},
		{"mismatched end", func(f []Fact) {
			f[3].Observation.Message = strings.ReplaceAll(f[3].Observation.Message, "192.0.2.15", "192.0.2.99")
		}, SessionClientMismatch},
		{"missing native start", func(f []Fact) { f[0].Observation.Message = "" }, SessionBoundaryUnproven},
		{"unknown date", func(f []Fact) {
			f[2].Observation.Timestamp.Value = nil
			f[2].Observation.Timestamp.Quality = model.TimeWallOnly
		}, SessionUndated},
		{"unsupported date quality", func(f []Fact) { f[0].Observation.Timestamp.Quality = model.TimeWallOnly }, SessionUndated},
		{"report after end", func(f []Fact) {
			d := f[3].Observation.Timestamp.Value.Add(time.Second)
			f[2].Observation.Timestamp.Value = &d
		}, SessionTimeConflict},
		{"end equal start", func(f []Fact) { d := *f[0].Observation.Timestamp.Value; f[3].Observation.Timestamp.Value = &d }, SessionTimeConflict},
		{"unsupported stage", func(f []Fact) {
			f[2].Observation.Message = strings.Replace(f[2].Observation.Message, "RCPT from", "FAKE from", 1)
		}, SessionClientMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := sessionFacts(sessionLines(), "live")
			tc.edit(facts)
			out, err := BuildPrequeueSessions(facts, 100)
			if err != nil || len(out.Sessions) != 0 || len(out.Unassigned) != 2 || len(out.Prequeue.Attempts) != 2 {
				t.Fatalf("partial confident window: %#v %v", out, err)
			}
			for _, a := range out.Unassigned {
				if a.Reason != tc.reason {
					t.Fatalf("reason: %#v", out.Unassigned)
				}
			}
		})
	}
}

func TestPrequeueSessionsDoNotUseAbsentOverlappingOrInvalidBoundaries(t *testing.T) {
	for _, bounds := range [][2]int{{1, 4}, {0, 3}, {1, 3}} {
		facts := sessionFacts(sessionLines(), "live")[bounds[0]:bounds[1]]
		out, err := BuildPrequeueSessions(facts, 100)
		if err != nil || len(out.Sessions) != 0 || len(out.Unassigned) != 2 {
			t.Fatalf("absent boundary: %#v %v", out, err)
		}
		for _, a := range out.Unassigned {
			if a.Reason != SessionBoundaryUnproven {
				t.Fatalf("missing boundary reason: %#v", a)
			}
		}
	}
	lines := sessionLines()
	lines = append(lines[:2], lines[0], lines[2], lines[3])
	facts := sessionFacts(lines, "live")
	out, err := BuildPrequeueSessions(facts, 100)
	if err != nil || len(out.Unassigned) != 1 || out.Unassigned[0].Ref != facts[1].Ref || len(out.Sessions) != 1 || !reflect.DeepEqual(out.Sessions[0].Attempts, []FactRef{facts[3].Ref}) {
		t.Fatalf("overlapping starts joined: %#v %v", out, err)
	}
	for _, tc := range []struct {
		facts []Fact
		limit int
		want  error
	}{
		{facts, 1, ErrPartitionLimit},
		{facts, 0, ErrPartitionLimit},
		{append(facts, facts[0]), 100, ErrPartitionOverlap},
	} {
		if got, err := BuildPrequeueSessions(tc.facts, tc.limit); err != tc.want || !reflect.DeepEqual(got, PrequeueSessions{}) {
			t.Fatalf("partial refusal: %#v %v", got, err)
		}
	}
}
