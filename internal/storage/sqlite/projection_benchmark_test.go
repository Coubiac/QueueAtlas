package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/parser/postfix"
	"github.com/Coubiac/QueueAtlas/internal/parser/syslog"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

type projectionBenchmarkProfile struct {
	name           string
	queues, cycles int
}

// The same native synthetic input feeds all three operations. Dense and wide
// profiles have equal fact/output counts but different numbers of scope parts.
// This regular retry workload is not a representative production distribution.
var projectionBenchmarkProfiles = []projectionBenchmarkProfile{
	{"facts16_scope4", 4, 1},
	{"facts1024_scope16", 16, 16},
	{"facts4096_scope1", 1, 1024},
	{"facts4096_scope64", 64, 16},
}

func projectionBenchmarkInput(b *testing.B, profile projectionBenchmarkProfile) (source.Batch, []correlation.Fact, CorrelationScope) {
	b.Helper()
	count := profile.queues * profile.cycles * 4
	if profile.queues < 1 || profile.queues > MaxCorrelationScopeParts || profile.cycles < 1 || count > correlation.MaxPartitionFacts {
		b.Fatal("benchmark profile exceeds reconstruction bounds")
	}
	const sourceID, originID, instance = "bench-projection", "native-log", "trusted"
	start := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	batch := source.Batch{Source: source.Identity{ID: sourceID, Kind: "file", Name: "synthetic projection benchmark", TrustedHost: instance},
		Origins: []source.Origin{{ID: originID, Path: "/synthetic/projection.log", Fingerprint: "synthetic-projection", FirstSeen: start}}}
	opts := postfix.Options{SourceID: sourceID, Time: syslog.TimeContext{Year: 2026, Location: time.UTC}}
	facts := make([]correlation.Fact, 0, count)
	scope := CorrelationScope{}
	var offset int64
	for cycle := 0; cycle < profile.cycles; cycle++ {
		for queue := 0; queue < profile.queues; queue++ {
			id := fmt.Sprintf("A%09X", queue+1)
			if cycle == 0 {
				scope.Queues = append(scope.Queues, correlation.QueueKey{Instance: instance, QueueID: id})
			}
			for step, report := range []struct{ service, message string }{
				{"qmgr", "from=<alice@example.org>, size=1100, nrcpt=1 (queue active)"},
				{"smtp", "to=<bob@example.net>, relay=none, dsn=4.4.1, status=deferred (Connection refused)"},
				{"smtp", "to=<bob@example.net>, relay=mx.example.net[203.0.113.27]:25, dsn=2.0.0, status=sent (250 2.0.0 accepted)"},
				{"qmgr", "removed"},
			} {
				at := start.Add(time.Duration((cycle*profile.queues+queue)*4+step) * time.Second)
				raw := []byte(fmt.Sprintf("%s mx-a postfix/%s[1234]: %s: %s\n", at.Format("Jan _2 15:04:05"), report.service, id, report.message))
				end := offset + int64(len(raw))
				observation := postfix.Parse(raw, opts)
				if observation.ParseError != "" || observation.QueueID != id || observation.Timestamp.Value == nil || !observation.Timestamp.Value.Equal(at) {
					b.Fatal("invalid native benchmark observation")
				}
				batch.Records = append(batch.Records, source.Record{OriginID: originID, Start: offset, End: end, Raw: raw, ReadAt: at, Observation: observation})
				facts = append(facts, correlation.Fact{Ref: correlation.FactRef{SourceID: sourceID, OriginID: originID, Start: offset, End: end}, Instance: instance, Observation: observation})
				offset = end
			}
		}
	}
	batch.Checkpoints = []source.Position{{OriginID: originID, Offset: offset, AnchorHash: "synthetic-tail"}}
	return batch, facts, scope
}

func checkProjectionBenchmarkShape(b *testing.B, p correlation.Projection, profile projectionBenchmarkProfile) {
	b.Helper()
	if len(p.Queues) != profile.queues*profile.cycles || len(p.Unresolved) != 0 || len(p.Other) != 0 || len(p.Links) != 0 || len(p.Revision) != 64 {
		b.Fatal("benchmark did not reconstruct the expected generations")
	}
	for _, queue := range p.Queues {
		summary := queue.Summary
		if len(summary.Queue.Generation.Facts) != 4 || !summary.Queue.Generation.ReceiptObserved || summary.Queue.Generation.Removed == nil ||
			len(summary.Queue.Recipients) != 1 || summary.Counts != (correlation.RecipientCounts{Sent: 1}) ||
			!reflect.DeepEqual(summary.Reserves, []correlation.SummaryReserve{correlation.ReserveCoverageUnproven, correlation.ReserveNonExplicitTime}) {
			b.Fatal("benchmark generation lost evidence or reserves")
		}
		recipient := summary.Queue.Recipients[0]
		if recipient.Address != "bob@example.net" || len(recipient.Attempts) != 2 || len(recipient.Latest) != 1 || recipient.OrderUncertain || recipient.ObservedStatus != correlation.DeliverySent {
			b.Fatal("benchmark retry shape changed")
		}
	}
}

func benchmarkProjectionOperation(b *testing.B, operation string) {
	for _, profile := range projectionBenchmarkProfiles {
		b.Run(profile.name, func(b *testing.B) {
			b.StopTimer()
			ctx := context.Background()
			batch, facts, scope := projectionBenchmarkInput(b, profile)
			options := correlation.LinkOptions{Window: time.Minute}
			want, err := correlation.BuildProjection(facts, correlation.MaxPartitionFacts, options)
			if err != nil {
				b.Fatal(err)
			}
			checkProjectionBenchmarkShape(b, want, profile)
			var call func() (correlation.Projection, error)
			if operation == "build" {
				call = func() (correlation.Projection, error) {
					return correlation.BuildProjection(facts, correlation.MaxPartitionFacts, options)
				}
			} else {
				s, err := Open(ctx, filepath.Join(b.TempDir(), "projection.db"))
				if err != nil {
					b.Fatal(err)
				}
				b.Cleanup(func() { s.Close() })
				if err := s.Commit(ctx, batch); err != nil {
					b.Fatal(err)
				}
				stored, err := s.CorrelationFacts(ctx, scope, correlation.MaxPartitionFacts)
				if err != nil || !reflect.DeepEqual(stored, facts) {
					b.Fatal("benchmark native facts changed in storage", err)
				}
				// Install once before timing: subsequent installs replace a current
				// manifest; reads reconstruct it. Neither operation sees a cold DB.
				installed, err := s.InstallProjection(ctx, scope, stored, correlation.MaxPartitionFacts, options)
				if err != nil || !reflect.DeepEqual(installed, want) {
					b.Fatal("benchmark installation changed reconstruction", err)
				}
				if operation == "install" {
					call = func() (correlation.Projection, error) {
						return s.InstallProjection(ctx, scope, stored, correlation.MaxPartitionFacts, options)
					}
				} else {
					call = func() (correlation.Projection, error) {
						p, found, err := s.CurrentProjection(ctx, scope, correlation.MaxPartitionFacts)
						if err == nil && !found {
							return p, fmt.Errorf("benchmark manifest absent")
						}
						return p, err
					}
				}
			}
			warm, err := call()
			if err != nil || !reflect.DeepEqual(warm, want) {
				b.Fatal("benchmark warmup changed projection", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			b.StartTimer()
			for i := 0; i < b.N; i++ {
				p, err := call()
				if err != nil || p.Revision != want.Revision || len(p.Queues) != len(want.Queues) {
					b.Fatal("benchmark operation failed or changed revision", err)
				}
			}
			b.StopTimer()
		})
	}
}

func BenchmarkProjectionBuild(b *testing.B)   { benchmarkProjectionOperation(b, "build") }
func BenchmarkProjectionInstall(b *testing.B) { benchmarkProjectionOperation(b, "install") }
func BenchmarkProjectionCurrent(b *testing.B) { benchmarkProjectionOperation(b, "current") }
