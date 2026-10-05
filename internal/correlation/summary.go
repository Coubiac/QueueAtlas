package correlation

// RecipientCounts counts exact observed addresses by latest observed result.
// Sent is a transport report; Delivered is a recognised local mailbox report.
// Neither counts recipients missing from the snapshot or certifies a journey.
type RecipientCounts struct {
	Unknown, Sent, Delivered, Deferred, Bounced int
}

type SummaryReserve string

const (
	ReserveCoverageUnproven      SummaryReserve = "coverage_unproven"
	ReserveReceiptNotObserved    SummaryReserve = "receipt_not_observed"
	ReserveRemovalNotObserved    SummaryReserve = "removal_not_observed"
	ReserveNonExplicitTime       SummaryReserve = "non_explicit_time"
	ReserveCrossStreamUncertain  SummaryReserve = "cross_stream_uncertain"
	ReserveNoRecipientsObserved  SummaryReserve = "no_recipients_observed"
	ReserveAddressUnspecified    SummaryReserve = "address_unspecified"
	ReserveLatestOrderUncertain  SummaryReserve = "latest_order_uncertain"
	ReserveUnknownResult         SummaryReserve = "unknown_result"
	ReserveUnprojectedDeliveries SummaryReserve = "unprojected_deliveries"
)

type GenerationSummary struct {
	Queue             GenerationRecipients
	Counts            RecipientCounts
	ExpirationReports int
	Reserves          []SummaryReserve
}

type SummaryPartition struct {
	Queues     []GenerationSummary
	Unresolved []UnresolvedStream
	Other      []FactRef
}

// BuildSummaries counts bounded candidate projections without promoting a
// partial snapshot into a complete/global delivery verdict. This API has no
// log-coverage proof input: every summary therefore retains coverage_unproven,
// even with receipt, removal and matching qmgr nrcpt. Reserves have fixed order.
func BuildSummaries(facts []Fact, limit int) (SummaryPartition, error) {
	partition, err := BuildRecipients(facts, limit)
	if err != nil {
		return SummaryPartition{}, err
	}
	out := SummaryPartition{Unresolved: partition.Unresolved, Other: partition.Other}
	for _, queue := range partition.Queues {
		summary := GenerationSummary{Queue: queue, ExpirationReports: len(queue.Expirations)}
		summary.Reserves = append(summary.Reserves, ReserveCoverageUnproven)
		if !queue.Generation.ReceiptObserved {
			summary.Reserves = append(summary.Reserves, ReserveReceiptNotObserved)
		}
		if queue.Generation.Removed == nil {
			summary.Reserves = append(summary.Reserves, ReserveRemovalNotObserved)
		}
		if queue.Generation.HasNonExplicitTime {
			summary.Reserves = append(summary.Reserves, ReserveNonExplicitTime)
		}
		if queue.Generation.CrossStreamUncertain {
			summary.Reserves = append(summary.Reserves, ReserveCrossStreamUncertain)
		}
		if len(queue.Recipients) == 0 {
			summary.Reserves = append(summary.Reserves, ReserveNoRecipientsObserved)
		}
		var unspecified, uncertain bool
		for _, recipient := range queue.Recipients {
			unspecified = unspecified || recipient.AddressUnspecified
			uncertain = uncertain || recipient.OrderUncertain
			switch recipient.ObservedStatus {
			case DeliverySent:
				summary.Counts.Sent++
			case DeliveryDelivered:
				summary.Counts.Delivered++
			case DeliveryDeferred:
				summary.Counts.Deferred++
			case DeliveryBounced:
				summary.Counts.Bounced++
			default:
				summary.Counts.Unknown++
			}
		}
		if unspecified {
			summary.Reserves = append(summary.Reserves, ReserveAddressUnspecified)
		}
		if uncertain {
			summary.Reserves = append(summary.Reserves, ReserveLatestOrderUncertain)
		}
		if summary.Counts.Unknown > 0 {
			summary.Reserves = append(summary.Reserves, ReserveUnknownResult)
		}
		if len(queue.UnprojectedDeliveries) > 0 {
			summary.Reserves = append(summary.Reserves, ReserveUnprojectedDeliveries)
		}
		out.Queues = append(out.Queues, summary)
	}
	return out, nil
}
