package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

var (
	ErrCandidateNotFound = errors.New("candidate not found")
	ErrCandidateStale    = errors.New("candidate revision changed")
)

// NativeValue preserves exact address bytes, including empty/invalid UTF-8.
// Encoding is utf8 or base64 (standard padded encoding); never normalize a value.
type NativeValue struct {
	Encoding string `json:"encoding"`
	Value    string `json:"value"`
}

type DetailRecipient struct {
	Address            NativeValue                `json:"address"`
	ObservedStatus     correlation.DeliveryStatus `json:"observed_status"`
	OrderUncertain     bool                       `json:"order_uncertain"`
	AddressUnspecified bool                       `json:"address_unspecified"`
	AttemptCount       int                        `json:"attempt_count"`
	Latest             []SearchRef                `json:"latest"`
}

type DetailResponse struct {
	Candidate                SearchCandidate   `json:"candidate"`
	CoverageUnproven         bool              `json:"coverage_unproven"`
	First                    SearchRef         `json:"first"`
	Removed                  *SearchRef        `json:"removed"`
	ReceiptObserved          bool              `json:"receipt_observed"`
	HasNonExplicitTime       bool              `json:"has_non_explicit_time"`
	CrossStreamUncertain     bool              `json:"cross_stream_uncertain"`
	Recipients               []DetailRecipient `json:"recipients"`
	UnprojectedDeliveryCount int               `json:"unprojected_delivery_count"`
}

func buildQueueProjection(ctx context.Context, facts []correlation.Fact, limit int) (correlation.Projection, error) {
	if err := ctx.Err(); err != nil {
		return correlation.Projection{}, err
	}
	projection, err := correlation.BuildProjection(facts, limit, correlation.LinkOptions{Window: time.Minute})
	if err != nil {
		return correlation.Projection{}, err
	}
	if err := ctx.Err(); err != nil {
		return correlation.Projection{}, err
	}
	return projection, nil
}

func searchCandidate(queue correlation.ProjectedQueue) (SearchCandidate, error) {
	key, counts := queue.Key, queue.Summary.Counts
	id, err := EncodeCandidateID(key)
	if err != nil {
		return SearchCandidate{}, err
	}
	return SearchCandidate{ID: id, Revision: key.Revision, Instance: key.Instance, QueueID: key.QueueID, Generation: key.Generation,
		Counts:            SearchCounts{counts.Unknown, counts.Sent, counts.Delivered, counts.Deferred, counts.Bounced},
		ExpirationReports: queue.Summary.ExpirationReports, Reserves: append([]correlation.SummaryReserve(nil), queue.Summary.Reserves...)}, nil
}

func (h *searchHandler) detail(ctx context.Context, key correlation.QueueInstanceKey) (DetailResponse, error) {
	if err := ctx.Err(); err != nil {
		return DetailResponse{}, err
	}
	scope := sqlite.CorrelationScope{Queues: []correlation.QueueKey{{Instance: key.Instance, QueueID: key.QueueID}}}
	facts, err := h.reader.CorrelationFacts(ctx, scope, h.options.FactLimit)
	if err != nil {
		return DetailResponse{}, err
	}
	if err := ctx.Err(); err != nil {
		return DetailResponse{}, err
	}
	if len(facts) == 0 {
		return DetailResponse{}, ErrCandidateNotFound
	}
	for _, fact := range facts {
		if fact.Instance != key.Instance || fact.Observation.QueueID != key.QueueID || fact.Observation.NoQueue {
			return DetailResponse{}, sqlite.ErrSearchStoredHit
		}
	}
	projection, err := buildQueueProjection(ctx, facts, h.options.FactLimit)
	if err != nil {
		return DetailResponse{}, err
	}
	if projection.Revision != key.Revision {
		return DetailResponse{}, ErrCandidateStale
	}
	for _, queue := range projection.Queues {
		if queue.Key == key {
			return detailResponse(queue)
		}
	}
	return DetailResponse{}, ErrCandidateNotFound
}

func wireFactRef(ref correlation.FactRef) (SearchRef, error) {
	if !searchText(ref.SourceID, 1024) || !searchText(ref.OriginID, 1024) || ref.Start < 0 || ref.End <= ref.Start {
		return SearchRef{}, sqlite.ErrSearchStoredHit
	}
	return SearchRef{ref.SourceID, ref.OriginID, ref.Start, ref.End}, nil
}

func detailResponse(queue correlation.ProjectedQueue) (DetailResponse, error) {
	candidate, err := searchCandidate(queue)
	if err != nil {
		return DetailResponse{}, err
	}
	g := queue.Summary.Queue.Generation
	first, err := wireFactRef(g.First)
	if err != nil {
		return DetailResponse{}, err
	}
	out := DetailResponse{Candidate: candidate, CoverageUnproven: true, First: first, ReceiptObserved: g.ReceiptObserved,
		HasNonExplicitTime: g.HasNonExplicitTime, CrossStreamUncertain: g.CrossStreamUncertain, Recipients: []DetailRecipient{},
		UnprojectedDeliveryCount: len(queue.Summary.Queue.UnprojectedDeliveries)}
	if g.Removed != nil {
		removed, err := wireFactRef(*g.Removed)
		if err != nil {
			return DetailResponse{}, err
		}
		out.Removed = &removed
	}
	for _, recipient := range queue.Summary.Queue.Recipients {
		if len(recipient.Address) > 1024 {
			return DetailResponse{}, sqlite.ErrSearchStoredHit
		}
		address := NativeValue{Encoding: "utf8", Value: recipient.Address}
		if !utf8.ValidString(recipient.Address) {
			address = NativeValue{Encoding: "base64", Value: base64.StdEncoding.EncodeToString([]byte(recipient.Address))}
		}
		item := DetailRecipient{Address: address, ObservedStatus: recipient.ObservedStatus, OrderUncertain: recipient.OrderUncertain,
			AddressUnspecified: recipient.AddressUnspecified, AttemptCount: len(recipient.Attempts), Latest: []SearchRef{}}
		for _, ref := range recipient.Latest {
			latest, err := wireFactRef(ref)
			if err != nil {
				return DetailResponse{}, err
			}
			item.Latest = append(item.Latest, latest)
		}
		out.Recipients = append(out.Recipients, item)
	}
	return out, nil
}
