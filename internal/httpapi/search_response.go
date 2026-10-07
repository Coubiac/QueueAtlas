package httpapi

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

// SearchResponse paginates matched events. A repeated candidate may occur within
// or across pages. Each candidate revision covers its exact full queue scope,
// not a global message or stable cross-page snapshot. Raw observations are never JSON.
type SearchResponse struct {
	From             time.Time     `json:"from"`
	Until            time.Time     `json:"until"`
	Limit            int           `json:"limit"`
	CoverageUnproven bool          `json:"coverage_unproven"`
	Matches          []SearchMatch `json:"matches"`
	NextCursor       string        `json:"next_cursor,omitempty"`
}

type SearchRef struct {
	SourceID string `json:"source_id"`
	OriginID string `json:"origin_id"`
	Start    int64  `json:"start,string"`
	End      int64  `json:"end,string"`
}

type SearchMatch struct {
	Ref                 SearchRef                       `json:"ref"`
	At                  time.Time                       `json:"at"`
	TimeQuality         model.TimeQuality               `json:"time_quality"`
	Kind                model.Kind                      `json:"kind"`
	NoQueue             bool                            `json:"no_queue"`
	Candidate           *SearchCandidate                `json:"candidate"`
	UnresolvedReason    correlation.GenerationReason    `json:"unresolved_reason,omitempty"`
	PrequeueDisposition correlation.PrequeueDisposition `json:"prequeue_disposition,omitempty"`
}

type SearchCounts struct {
	Unknown   int `json:"unknown"`
	Sent      int `json:"sent"`
	Delivered int `json:"delivered"`
	Deferred  int `json:"deferred"`
	Bounced   int `json:"bounced"`
}

type SearchCandidate struct {
	Revision          string                       `json:"revision"`
	Instance          string                       `json:"instance"`
	QueueID           string                       `json:"queue_id"`
	Generation        int                          `json:"generation"`
	Counts            SearchCounts                 `json:"counts"`
	ExpirationReports int                          `json:"expiration_reports"`
	Reserves          []correlation.SummaryReserve `json:"reserves"`
}

func searchScope(hits []sqlite.SearchHit, query sqlite.SearchQuery) (sqlite.CorrelationScope, error) {
	scope := sqlite.CorrelationScope{}
	queues := make(map[correlation.QueueKey]bool)
	unqueued := false
	seen := make(map[correlation.FactRef]bool)
	for _, hit := range hits {
		if seen[hit.Ref] || hit.Instance != query.Instance || !searchText(hit.Ref.SourceID, 1024) || !searchText(hit.Ref.OriginID, 1024) ||
			hit.Ref.Start < 0 || hit.Ref.End <= hit.Ref.Start || !searchInstantFits(hit.At) || hit.At.Before(query.From) || !hit.At.Before(query.Until) {
			return sqlite.CorrelationScope{}, sqlite.ErrSearchStoredHit
		}
		seen[hit.Ref] = true
		if hit.NoQueue || hit.QueueID == "" {
			if !unqueued {
				scope.UnqueuedInstances = []string{query.Instance}
				unqueued = true
			}
		} else {
			if !searchText(hit.QueueID, 32) {
				return sqlite.CorrelationScope{}, sqlite.ErrSearchStoredHit
			}
			key := correlation.QueueKey{Instance: hit.Instance, QueueID: hit.QueueID}
			if !queues[key] {
				scope.Queues = append(scope.Queues, key)
				queues[key] = true
			}
		}
		if len(scope.Queues)+len(scope.UnqueuedInstances) > sqlite.MaxCorrelationScopeParts {
			return sqlite.CorrelationScope{}, sqlite.ErrCorrelationScope
		}
	}
	return scope, nil
}

func searchText(value string, max int) bool {
	return value != "" && len(value) <= max && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n\t")
}

func searchMatches(ctx context.Context, hits []sqlite.SearchHit, facts []correlation.Fact, limit int) ([]SearchMatch, error) {
	if len(facts) > limit {
		return nil, correlation.ErrPartitionLimit
	}
	byRef := make(map[correlation.FactRef]correlation.Fact, len(facts))
	groups := make(map[correlation.QueueKey][]correlation.Fact)
	for _, fact := range facts {
		byRef[fact.Ref] = fact
		key := correlation.QueueKey{Instance: fact.Instance, QueueID: fact.Observation.QueueID}
		if fact.Observation.NoQueue {
			key.QueueID = ""
		}
		groups[key] = append(groups[key], fact)
	}
	candidates := make(map[correlation.FactRef]*SearchCandidate)
	unresolved := make(map[correlation.FactRef]correlation.GenerationReason)
	prequeue := make(map[correlation.FactRef]correlation.PrequeueDisposition)
	// Per-queue revisions do not depend on unrelated hits in this page. Every
	// origin, cycle and undated fact of that exact queue still participates.
	for _, group := range groups {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		projection, err := correlation.BuildProjection(group, limit, correlation.LinkOptions{Window: time.Minute})
		if err != nil {
			return nil, err
		}
		for _, queue := range projection.Queues {
			key, counts := queue.Key, queue.Summary.Counts
			if !searchText(key.Instance, 1024) || !searchText(key.QueueID, 32) {
				return nil, sqlite.ErrSearchStoredHit
			}
			candidate := &SearchCandidate{Revision: key.Revision, Instance: key.Instance, QueueID: key.QueueID, Generation: key.Generation,
				Counts:            SearchCounts{counts.Unknown, counts.Sent, counts.Delivered, counts.Deferred, counts.Bounced},
				ExpirationReports: queue.Summary.ExpirationReports, Reserves: append([]correlation.SummaryReserve(nil), queue.Summary.Reserves...)}
			for _, ref := range queue.Summary.Queue.Generation.Facts {
				candidates[ref] = candidate
			}
		}
		for _, stream := range projection.Unresolved {
			for _, ref := range stream.Facts {
				unresolved[ref] = stream.Reason
			}
		}
		for _, attempt := range projection.Prequeue.Prequeue.Attempts {
			prequeue[attempt.Ref] = attempt.Disposition
		}
	}
	out := make([]SearchMatch, 0, len(hits))
	for _, hit := range hits {
		fact, found := byRef[hit.Ref]
		o := fact.Observation
		if !found || fact.Instance != hit.Instance || o.QueueID != hit.QueueID || o.NoQueue != hit.NoQueue || o.Kind != hit.Kind || o.Timestamp.Quality != hit.TimeQuality ||
			o.Timestamp.Value == nil || !o.Timestamp.Value.Equal(hit.At) || !searchText(string(hit.Kind), 64) || !searchText(string(hit.TimeQuality), 64) {
			return nil, sqlite.ErrSearchStoredHit // e.g. retention between the two reads: no partial answer.
		}
		ref := hit.Ref
		out = append(out, SearchMatch{Ref: SearchRef{ref.SourceID, ref.OriginID, ref.Start, ref.End}, At: hit.At.UTC(),
			TimeQuality: hit.TimeQuality, Kind: hit.Kind, NoQueue: hit.NoQueue, Candidate: candidates[ref], UnresolvedReason: unresolved[ref], PrequeueDisposition: prequeue[ref]})
	}
	return out, nil
}
