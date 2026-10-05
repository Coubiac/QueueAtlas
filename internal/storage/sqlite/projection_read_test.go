package sqlite

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/correlation"
)

func TestCurrentProjectionReconstructsCompleteSnapshotAndCopiesOutput(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	var seed []correlation.Fact
	for _, name := range []string{"11-filter-reinjection", "09-bounce", "06-noqueue-client-reject"} {
		seed = append(seed, storeCorrelationCorpus(t, s, name, name, "trusted", true, false)...)
	}
	scope := queueScope(seed)
	scope.UnqueuedInstances = []string{"trusted"}
	facts, err := s.CorrelationFacts(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	opts := correlation.LinkOptions{Window: time.Minute, SMTPBindings: []correlation.SMTPBinding{{FromInstance: "trusted", Relay: "127.0.0.1[127.0.0.1]:10024", ToInstance: "trusted"}}}
	want, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "foreign", "foreign-instance", true, false)
	for i, j := 0, len(scope.Queues)-1; i < j; i, j = i+1, j-1 {
		scope.Queues[i], scope.Queues[j] = scope.Queues[j], scope.Queues[i]
	}
	got, found, err := s.CurrentProjection(ctx, scope, 100)
	if err != nil || !found || !reflect.DeepEqual(got, want) || len(got.Links) != 2 || len(got.Prequeue.Sessions) != 1 {
		t.Fatal("persisted revision changed derived proof", err)
	}
	got.LinkOptions.SMTPBindings[0].ToInstance = "changed"
	got.Queues[0].Key.Generation = 900
	*got.Links[0].From = correlation.QueueInstanceKey{}
	again, found, err := s.CurrentProjection(ctx, scope, 100)
	if err != nil || !found || !reflect.DeepEqual(again, want) {
		t.Fatal("returned output changed manifest", err)
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
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatal("reopen reconstruction", err)
	}
}

func TestCurrentProjectionRejectsLateFactsAndRequiresExplicitReinstallation(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	seed := storeCorrelationCorpus(t, s, "11-filter-reinjection", "live", "trusted", true, false)
	scope := queueScope(seed)
	facts, err := s.CorrelationFacts(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	opts := correlation.LinkOptions{Window: time.Minute}
	initial, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "late-archive", "trusted", false, false)
	got, found, err := s.CurrentProjection(ctx, scope, 100)
	if !errors.Is(err, ErrProjectionStale) || found || !reflect.DeepEqual(got, correlation.Projection{}) {
		t.Fatal("stale output accepted", err)
	}
	if rev, n := installedRevision(t, s, scope); rev != initial.Revision || n != len(facts) {
		t.Fatal("read silently recalculated manifest")
	}
	facts, err = s.CorrelationFacts(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	got, found, err = s.CurrentProjection(ctx, scope, 100)
	if err != nil || !found || !reflect.DeepEqual(got, rebuilt) || len(got.Unresolved) != 2 || got.Revision == initial.Revision {
		t.Fatal("late uncertainty lost", err)
	}
}

func TestCurrentProjectionRejectsMalformedManifestWithoutPartialData(t *testing.T) {
	for _, test := range []struct {
		name, sql string
		want      error
	}{
		{"count mismatch", `UPDATE projection_revisions SET fact_count=fact_count+1`, ErrProjectionStoredManifest},
		{"membership removed", `DELETE FROM projection_revision_facts WHERE raw_record_id=(SELECT min(raw_record_id) FROM projection_revision_facts)`, ErrProjectionStoredManifest},
		{"revision changed", `UPDATE projection_revisions SET revision='0000000000000000000000000000000000000000000000000000000000000000'`, ErrProjectionStoredManifest},
		{"input changed", `UPDATE projection_revisions SET input_revision='0000000000000000000000000000000000000000000000000000000000000000'`, ErrProjectionStale},
		{"scope bytes changed", `UPDATE projection_scope_parts SET instance=CAST('synthetic-private@example.org' AS BLOB)`, ErrProjectionStoredManifest},
		{"scope ordinal gap", `UPDATE projection_scope_parts SET ordinal=63 WHERE ordinal=0`, ErrProjectionStoredManifest},
		{"binding ordinal gap", `UPDATE projection_revision_bindings SET ordinal=63`, ErrProjectionStoredManifest},
		{"binding changed", `UPDATE projection_revision_bindings SET relay=CAST('synthetic-private@example.org' AS BLOB)`, ErrProjectionStoredManifest},
		{"conversion sanitized", `PRAGMA ignore_check_constraints=ON; UPDATE projection_revisions SET link_window_ns='synthetic-private@example.org'; PRAGMA ignore_check_constraints=OFF`, ErrProjectionStoredManifest},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _ := openTestStore(t)
			ctx := context.Background()
			seed := storeCorrelationCorpus(t, s, "11-filter-reinjection", "live", "trusted", true, false)
			scope := queueScope(seed)
			facts, err := s.CorrelationFacts(ctx, scope, 100)
			if err != nil {
				t.Fatal(err)
			}
			opts := correlation.LinkOptions{Window: time.Minute, SMTPBindings: []correlation.SMTPBinding{{FromInstance: "trusted", Relay: "127.0.0.1[127.0.0.1]:10024", ToInstance: "trusted"}}}
			if _, err := s.InstallProjection(ctx, scope, facts, 100, opts); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(test.sql); err != nil {
				t.Fatal(err)
			}
			got, found, err := s.CurrentProjection(ctx, scope, 100)
			if !errors.Is(err, test.want) || found || !reflect.DeepEqual(got, correlation.Projection{}) || strings.Contains(err.Error(), "synthetic-private") {
				t.Fatal("malformed output escaped", err)
			}
		})
	}
}

func TestCurrentProjectionAbsentInvalidatedAndEmptyAreDistinct(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: "absent", QueueID: "ABSENT"}}}
	got, found, err := s.CurrentProjection(ctx, scope, 10)
	if err != nil || found || got.Revision != "" {
		t.Fatal("absent scope invented manifest", err)
	}
	want, err := s.InstallProjection(ctx, scope, nil, 10, correlation.LinkOptions{Window: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	got, found, err = s.CurrentProjection(ctx, scope, 10)
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatal("valid empty scope treated absent", err)
	}
	if _, err := s.db.Exec(`UPDATE projection_scopes SET current_revision_id=NULL`); err != nil {
		t.Fatal(err)
	}
	got, found, err = s.CurrentProjection(ctx, scope, 10)
	if err != nil || found || got.Revision != "" || count(t, s, "projection_revisions") != 1 {
		t.Fatal("invalidated manifest displayed or mutated", err)
	}
}

func TestCurrentProjectionLimitsCancellationAndBinaryOptions(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	instance := "binary" + string([]byte{0xff})
	scope := CorrelationScope{UnqueuedInstances: []string{instance}}
	opts := correlation.LinkOptions{Window: time.Minute, SMTPBindings: []correlation.SMTPBinding{{FromInstance: instance, Relay: "relay" + string([]byte{0xfe}), ToInstance: instance + "target"}}}
	want, err := s.InstallProjection(ctx, scope, nil, 1, opts)
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := s.CurrentProjection(ctx, scope, 1)
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatal("binary options altered", err)
	}
	for _, limit := range []int{0, correlation.MaxPartitionFacts + 1} {
		got, found, err = s.CurrentProjection(ctx, scope, limit)
		if !errors.Is(err, correlation.ErrPartitionLimit) || found || got.Revision != "" {
			t.Fatal("limit", err)
		}
	}
	if got, found, err = s.CurrentProjection(ctx, CorrelationScope{}, 1); !errors.Is(err, ErrCorrelationScope) || found || got.Revision != "" {
		t.Fatal("scope", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, found, err = s.CurrentProjection(canceled, scope, 1); !errors.Is(err, context.Canceled) || found || got.Revision != "" {
		t.Fatal("cancellation", err)
	}
	seed := storeCorrelationCorpus(t, s, "11-filter-reinjection", "live", "trusted", true, false)
	queue := queueScope(seed)
	facts, err := s.CorrelationFacts(ctx, queue, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallProjection(ctx, queue, facts, 100, correlation.LinkOptions{Window: time.Minute}); err != nil {
		t.Fatal(err)
	}
	if got, found, err = s.CurrentProjection(ctx, queue, len(facts)-1); !errors.Is(err, correlation.ErrPartitionLimit) || found || got.Revision != "" {
		t.Fatal("truncated read returned projection", err)
	}
}

func TestProjectionManifestReadsShareSnapshotAcrossConcurrentCommit(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	seed := storeCorrelationCorpus(t, s, "11-filter-reinjection", "live", "trusted", true, false)
	scope := queueScope(seed)
	facts, err := s.CorrelationFacts(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	opts := correlation.LinkOptions{Window: time.Minute}
	old, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var currentID, scopeID int64
	if err := tx.QueryRowContext(ctx, `SELECT id,current_revision_id FROM projection_scopes`).Scan(&scopeID, &currentID); err != nil {
		t.Fatal(err)
	}
	storeCorrelationCorpus(t, other, "11-filter-reinjection", "late-archive", "trusted", false, false)
	newFacts, err := other.CorrelationFacts(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.InstallProjection(ctx, scope, newFacts, 100, opts); err != nil {
		t.Fatal(err)
	}
	parts, _ := canonicalProjectionScope(scope)
	if err := readProjectionScope(ctx, tx, scopeID, parts); err != nil {
		t.Fatal(err)
	}
	members, err := readProjectionMembers(ctx, tx, currentID, len(facts))
	if err != nil || len(members) != len(facts) {
		t.Fatal("manifest changed inside read snapshot", err)
	}
	before, _, err := correlationFacts(ctx, tx, scope, 100)
	if err != nil || !reflect.DeepEqual(before, facts) {
		t.Fatal("facts crossed snapshots", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.CurrentProjection(ctx, scope, 100)
	if err != nil || !found || got.Revision == old.Revision || len(got.Unresolved) != 2 {
		t.Fatal("next transaction missed concurrent revision", err)
	}
}
