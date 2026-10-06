package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
)

// Dense equal criteria with all foreign-instance events ordered before matching
// ones exposes filtering work for indexes that lack an instance prefix. This is
// one synthetic distribution, not a representative mail workload or an SLA.
func benchmarkSearchStore(b *testing.B, size int) *Store {
	b.Helper()
	s, err := Open(context.Background(), filepath.Join(b.TempDir(), "search.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { s.Close() })
	start := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	for group, instance := range []string{"foreign", "trusted"} {
		id := "bench-" + instance
		batch := source.Batch{Source: source.Identity{ID: id, Kind: "file", Name: "synthetic benchmark", TrustedHost: instance},
			Origins: []source.Origin{{ID: id, Path: "/synthetic/" + id, Fingerprint: id, FirstSeen: start}}}
		var offset int64
		for i := group * size / 2; i < (group+1)*size/2; i++ {
			at := start.Add(time.Duration(i) * time.Second)
			raw := []byte(fmt.Sprintf("synthetic benchmark event %d\n", i))
			end := offset + int64(len(raw))
			o := model.Observation{SourceID: id, QueueID: "BENCHQ", Kind: model.KindDelivery,
				Timestamp: model.Timestamp{Value: &at, Quality: model.TimeExplicitOffset},
				Fields:    map[string]string{"from": "sender@example.org", "to": "recipient@example.net", "message-id": "bench@example.org", "status": "sent"},
				Present:   map[string]bool{"from": true, "to": true, "message-id": true, "status": true}}
			batch.Records = append(batch.Records, source.Record{OriginID: id, Start: offset, End: end, Raw: raw, ReadAt: at, Observation: o})
			offset = end
		}
		batch.Checkpoints = []source.Position{{OriginID: id, Offset: offset, AnchorHash: "synthetic"}}
		if err := s.Commit(context.Background(), batch); err != nil {
			b.Fatal(err)
		}
	}
	return s
}

func BenchmarkSearchEventsIndexed(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("rows%d", size), func(b *testing.B) {
			s := benchmarkSearchStore(b, size)
			for _, criterion := range []struct {
				field SearchField
				value string
			}{
				{SearchSender, "sender@example.org"}, {SearchRecipient, "recipient@example.net"},
				{SearchQueueID, "BENCHQ"}, {SearchMessageID, "bench@example.org"},
				{SearchSenderDomain, "example.org"}, {SearchRecipientDomain, "example.net"},
			} {
				for _, position := range []string{"first", "seek"} {
					b.Run(string(criterion.field)+"/"+position, func(b *testing.B) {
						query := eventSearchQuery(criterion.field, criterion.value)
						query.Limit = 100
						if position == "seek" {
							_, _, revision, err := eventSearchSelection(query)
							if err != nil {
								b.Fatal(err)
							}
							// IDs start at 1; event index size*3/4 has ID+1. Position
							// lies halfway through the trusted half, after foreign rows.
							index := size * 3 / 4
							query.After = &SearchCursor{QueryRevision: revision, TimeNS: query.From.Add(time.Duration(index) * time.Second).UnixNano(), RowID: int64(index + 1)}
						}
						ctx := context.Background()
						warm, err := s.SearchEvents(ctx, query)
						if err != nil || len(warm.Hits) != 100 || warm.Next == nil {
							b.Fatal("benchmark page shape", err)
						}
						b.ReportAllocs()
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							page, err := s.SearchEvents(ctx, query)
							if err != nil || len(page.Hits) != 100 || page.Next == nil {
								b.Fatal("benchmark page shape", err)
							}
						}
					})
				}
			}
		})
	}
}

// Fixture/reset/verification are excluded; the timed operation is the actual
// v5->v6 migration including indexes, backfill, history/version and commit.
func BenchmarkSearchDomainMigration(b *testing.B) {
	for _, size := range []int{1000, 10000} {
		b.Run(fmt.Sprintf("rows%d", size), func(b *testing.B) {
			b.StopTimer()
			s := benchmarkSearchStore(b, size)
			ctx := context.Background()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := s.db.Exec(`DROP TABLE event_search_domains; DELETE FROM schema_migrations WHERE version=6; PRAGMA user_version=5;`); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				err := migrate(ctx, s.db, 0)
				b.StopTimer()
				if err != nil {
					b.Fatal(err)
				}
				var count, version int
				if err := s.db.QueryRow(`SELECT count(*) FROM event_search_domains`).Scan(&count); err != nil || count != size {
					b.Fatal("benchmark backfill count", err)
				}
				if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 6 {
					b.Fatal("benchmark version", err)
				}
			}
		})
	}
}
