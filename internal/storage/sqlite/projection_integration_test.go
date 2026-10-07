package sqlite

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
)

func TestProjectionPersistedEqualDateConflictSurvivesInsertionOrderAndReopen(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "postfix", "07-deferred-then-sent.log"))
	if err != nil {
		t.Fatal(err)
	}
	// Change the native log before parsing, keeping durable raw and timestamps
	// consistent. The ordinary corpus still exercises the later successful retry.
	sentPrefix := []byte("Oct  3 12:16:01 mx-a postfix/smtp[1164]:")
	if bytes.Count(raw, sentPrefix) != 1 {
		t.Fatal("expected exactly one synthetic sent report")
	}
	raw = bytes.Replace(raw, sentPrefix, []byte("Oct  3 12:06:01 mx-a postfix/smtp[1164]:"), 1)
	ctx := context.Background()
	var baseline correlation.Projection
	for _, reverse := range []bool{false, true} {
		name := "forward"
		if reverse {
			name = "reverse"
		}
		t.Run(name, func(t *testing.T) {
			s, path := openTestStore(t)
			seed := storeCorrelationRaw(t, s, raw, "equal-date", "synthetic", "trusted", true, reverse)
			if len(seed) != 6 {
				t.Fatal("expected six synthetic native records")
			}
			scope := queueScope(seed)
			facts, err := s.CorrelationFacts(ctx, scope, 100)
			if err != nil || !reflect.DeepEqual(facts, seed) {
				t.Fatalf("stored facts differ from native input: %v", err)
			}
			// Verify that this changes real database IDs, not just an input slice
			// that was canonicalised before insertion. Provenance remains identical.
			var deferredID, sentID int64
			for i, dst := range []*int64{&deferredID, &sentID} {
				ref := seed[2+i*2].Ref
				if err := s.db.QueryRowContext(ctx, `SELECT id FROM raw_records
					WHERE source_id=? AND generation_id=? AND start_offset=?`,
					ref.SourceID, ref.OriginID, ref.Start).Scan(dst); err != nil {
					t.Fatal(err)
				}
			}
			if deferredID == sentID || (deferredID > sentID) != reverse {
				t.Fatal("test did not invert stored insertion IDs")
			}
			installed, err := s.InstallProjection(ctx, scope, facts, 100, correlation.LinkOptions{Window: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			if len(installed.Queues) != 1 || len(installed.Unresolved) != 0 {
				t.Fatal("equal-date reports did not retain one candidate generation")
			}
			summary := installed.Queues[0].Summary
			if !summary.Queue.Generation.ReceiptObserved || summary.Queue.Generation.Removed == nil {
				t.Fatal("synthetic receipt/removal evidence lost")
			}
			if len(summary.Queue.Recipients) != 1 {
				t.Fatal("recipient evidence lost")
			}
			recipient := summary.Queue.Recipients[0]
			wantRefs := []correlation.FactRef{seed[2].Ref, seed[4].Ref}
			if recipient.Address != "bob@example.net" || recipient.ObservedStatus != correlation.DeliveryUnknown ||
				!recipient.OrderUncertain || len(recipient.Attempts) != 2 || !reflect.DeepEqual(recipient.Latest, wantRefs) {
				t.Fatalf("conflict was resolved or evidence lost: %+v", recipient)
			}
			wantDeliveries := []correlation.Delivery{
				{Recipient: "bob@example.net", NativeStatus: "deferred", Status: correlation.DeliveryDeferred,
					Scope: correlation.ScopeSMTPPeer, Relay: correlation.Field{Value: "none", Present: true},
					DSN:   correlation.Field{Value: "4.4.1", Present: true},
					Reply: correlation.Field{Value: "connect to mx.example.net[203.0.113.27]:25: Connection refused", Present: true}},
				{Recipient: "bob@example.net", NativeStatus: "sent", Status: correlation.DeliverySent,
					Scope: correlation.ScopeSMTPPeer, Relay: correlation.Field{Value: "mx.example.net[203.0.113.27]:25", Present: true},
					DSN:   correlation.Field{Value: "2.0.0", Present: true},
					Reply: correlation.Field{Value: "250 2.0.0 queued as REMOTE07", Present: true}},
			}
			wantTime := time.Date(2026, 10, 3, 12, 6, 1, 0, time.UTC)
			for i, attempt := range recipient.Attempts {
				if attempt.Ref != wantRefs[i] || !reflect.DeepEqual(attempt.Delivery, wantDeliveries[i]) ||
					attempt.Timestamp.Value == nil || !attempt.Timestamp.Value.Equal(wantTime) {
					t.Fatalf("native attempt %d changed: %+v", i, attempt)
				}
			}
			wantReserves := []correlation.SummaryReserve{correlation.ReserveCoverageUnproven, correlation.ReserveNonExplicitTime,
				correlation.ReserveLatestOrderUncertain, correlation.ReserveUnknownResult}
			if summary.Counts != (correlation.RecipientCounts{Unknown: 1}) || !reflect.DeepEqual(summary.Reserves, wantReserves) {
				t.Fatalf("summary promoted conflicting reports: %+v", summary)
			}
			if reverse {
				if !reflect.DeepEqual(installed, baseline) {
					t.Fatal("insertion IDs changed projection, revisions or references")
				}
			} else {
				baseline = installed
			}
			got, found, err := s.CurrentProjection(ctx, scope, 100)
			if err != nil || !found || !reflect.DeepEqual(got, installed) {
				t.Fatalf("current reconstruction changed conflict: %v", err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			got, found, err = reopened.CurrentProjection(ctx, scope, 100)
			if err != nil || !found || !reflect.DeepEqual(got, installed) {
				t.Fatalf("reopen changed conflict: %v", err)
			}
		})
	}
}
