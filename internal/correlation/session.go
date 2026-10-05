package correlation

import (
	"strings"

	"github.com/Coubiac/mailtrace/internal/model"
)

type SessionReason string

const (
	SessionBoundaryUnproven SessionReason = "boundary_unproven"
	SessionClientMismatch   SessionReason = "client_mismatch"
	SessionUndated          SessionReason = "undated"
	SessionTimeConflict     SessionReason = "time_conflict"
)

// PrequeueSession is a source/origin-scoped candidate between explicit native
// boundaries, not a PID identity, accepted-queue link or log coverage certificate.
type PrequeueSession struct {
	Instance, ProcessID, Client string
	Connect, Disconnect         FactRef
	Attempts                    []FactRef
	HasNonExplicitTime          bool
	CoverageUnproven            bool
}

type UnassignedPrequeue struct {
	Ref    FactRef
	Reason SessionReason
}

type PrequeueSessions struct {
	Prequeue   PrequeuePartition // all reports and other facts remain available
	Sessions   []PrequeueSession
	Unassigned []UnassignedPrequeue
}

type sessionActor struct{ Instance, SourceID, OriginID, PID string }
type pendingSession struct {
	Connect  FactRef
	Client   string
	Attempts []FactRef
	Reason   SessionReason
}

// BuildPrequeueSessions attaches reports only to closed candidate windows with
// matching native clients and usable date hypotheses. One doubtful report makes
// the window unassigned. Origins stay independent; no accepted queue is attached.
func BuildPrequeueSessions(facts []Fact, limit int) (PrequeueSessions, error) {
	prequeue, err := BuildPrequeue(facts, limit)
	if err != nil {
		return PrequeueSessions{}, err
	}
	lookup := make(map[FactRef]Fact, len(facts))
	refs := make([]FactRef, 0, len(facts))
	for _, f := range facts {
		lookup[f.Ref] = f
		refs = append(refs, f.Ref)
	}
	sortRefs(refs)
	attempts := make(map[FactRef]PrequeueAttempt, len(prequeue.Attempts))
	for _, a := range prequeue.Attempts {
		attempts[a.Ref] = a
	}
	active := make(map[sessionActor]*pendingSession)
	unassigned := make(map[FactRef]SessionReason)
	sessions := make(map[FactRef]PrequeueSession)
	refuse := func(p *pendingSession, reason SessionReason) {
		for _, ref := range p.Attempts {
			unassigned[ref] = reason
		}
	}
	for _, ref := range refs {
		f := lookup[ref]
		o := f.Observation
		actor := sessionActor{f.Instance, ref.SourceID, ref.OriginID, o.PID}
		if o.Service == "smtpd" && o.PID != "" && (o.Kind == model.KindConnect || o.Kind == model.KindDisconnect) {
			client := boundaryClient(o)
			if o.Kind == model.KindConnect {
				if old := active[actor]; old != nil {
					refuse(old, SessionBoundaryUnproven)
				}
				p := &pendingSession{Connect: ref, Client: client}
				if client == "" {
					p.Reason = SessionBoundaryUnproven
				}
				active[actor] = p
			} else if p := active[actor]; p != nil {
				delete(active, actor)
				reason := p.Reason
				if reason == "" && (client == "" || client != p.Client) {
					reason = SessionClientMismatch
				}
				nonExplicit := false
				if reason == "" {
					reason, nonExplicit = sessionDates(p, ref, lookup)
				}
				if reason != "" {
					refuse(p, reason)
				} else if len(p.Attempts) > 0 {
					sessions[p.Connect] = PrequeueSession{
						Instance: f.Instance, ProcessID: o.PID, Client: client,
						Connect: p.Connect, Disconnect: ref, Attempts: p.Attempts,
						HasNonExplicitTime: nonExplicit, CoverageUnproven: true,
					}
				}
			}
			continue
		}
		if a, ok := attempts[ref]; ok {
			p := active[actor]
			if p == nil || o.PID == "" {
				unassigned[ref] = SessionBoundaryUnproven
				continue
			}
			p.Attempts = append(p.Attempts, ref)
			if client := reportClient(a); p.Reason == "" && (client == "" || client != p.Client) {
				p.Reason = SessionClientMismatch
			}
		}
	}
	for _, p := range active {
		refuse(p, SessionBoundaryUnproven)
	}
	out := PrequeueSessions{Prequeue: prequeue}
	// Fixed physical display order, never map iteration or guessed chronology.
	for _, ref := range refs {
		if session, ok := sessions[ref]; ok {
			out.Sessions = append(out.Sessions, session)
		}
		if reason, ok := unassigned[ref]; ok {
			out.Unassigned = append(out.Unassigned, UnassignedPrequeue{ref, reason})
		}
	}
	return out, nil
}

func boundaryClient(o model.Observation) string {
	if o.NoQueue || o.QueueID != "" || o.ParseError != "" || len(o.Message) > model.MaxLineBytes {
		return ""
	}
	var prefix string
	switch o.Kind {
	case model.KindConnect:
		prefix = "connect from "
	case model.KindDisconnect:
		prefix = "disconnect from "
	default:
		return ""
	}
	if !strings.HasPrefix(o.Message, prefix) {
		return ""
	}
	value := strings.TrimPrefix(o.Message, prefix)
	if o.Kind == model.KindDisconnect {
		if end := strings.IndexByte(value, ' '); end >= 0 {
			value = value[:end]
		}
	}
	return literalClient(value)
}

func reportClient(a PrequeueAttempt) string {
	text := strings.TrimSpace(strings.TrimPrefix(a.Message, a.NativeAction+":"))
	from := strings.Index(text, " from ")
	if from < 1 {
		return ""
	}
	switch text[:from] {
	case "CONNECT", "HELO", "EHLO", "MAIL", "RCPT", "DATA", "END-OF-MESSAGE":
	default:
		return ""
	}
	text = text[from+6:]
	end := strings.Index(text, ": ")
	if end < 1 {
		return ""
	}
	return literalClient(text[:end])
}

func literalClient(value string) string {
	// Preserve the exact server-reported host[address] token, no DNS lookup or
	// canonicalisation. Unsupported formats stay unassigned.
	open := strings.IndexByte(value, '[')
	if open < 1 || open >= len(value)-2 || !strings.HasSuffix(value, "]") || strings.ContainsAny(value, " \t\r\n;\x00") {
		return ""
	}
	return value
}

func sessionDates(p *pendingSession, end FactRef, lookup map[FactRef]Fact) (SessionReason, bool) {
	startDate := lookup[p.Connect].Observation.Timestamp
	endDate := lookup[end].Observation.Timestamp
	if !usableDate(startDate) || !usableDate(endDate) {
		return SessionUndated, false
	}
	if !endDate.Value.After(*startDate.Value) {
		return SessionTimeConflict, false
	}
	nonExplicit := startDate.Quality != model.TimeExplicitOffset || endDate.Quality != model.TimeExplicitOffset
	for _, ref := range p.Attempts {
		date := lookup[ref].Observation.Timestamp
		if !usableDate(date) {
			return SessionUndated, false
		}
		if date.Value.Before(*startDate.Value) || date.Value.After(*endDate.Value) {
			return SessionTimeConflict, false
		}
		nonExplicit = nonExplicit || date.Quality != model.TimeExplicitOffset
	}
	return "", nonExplicit
}
