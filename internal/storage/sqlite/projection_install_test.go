package sqlite

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/correlation"
)

func installedRevision(t *testing.T, s *Store, scope CorrelationScope) (string, int) {
	t.Helper()
	_, digest := canonicalProjectionScope(scope)
	var revision string
	var count int
	if err := s.db.QueryRow(`SELECT r.revision, r.fact_count FROM projection_scopes s JOIN projection_revisions r ON r.id = s.current_revision_id WHERE s.scope_sha256 = ?`, digest).Scan(&revision, &count); err != nil {
		t.Fatal(err)
	}
	return revision, count
}

func TestInstallProjectionPreservesCompleteManifestAndReplacesOneScope(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	var all []correlation.Fact
	for _, input := range []struct {
		name, source string
		dated        bool
	}{
		{"11-filter-reinjection", "live", true}, {"11-filter-reinjection", "archive", false},
		{"06-noqueue-client-reject", "prequeue", true},
	} {
		all = append(all, storeCorrelationCorpus(t, s, input.name, input.source, "trusted", input.dated, false)...)
	}
	scope := queueScope(all)
	scope.UnqueuedInstances = []string{"trusted"}
	facts, err := s.CorrelationFacts(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	opts := correlation.LinkOptions{Window: time.Minute, SMTPBindings: []correlation.SMTPBinding{{FromInstance: "trusted", Relay: "127.0.0.1[127.0.0.1]:10024", ToInstance: "trusted"}}}
	want, err := correlation.BuildProjection(facts, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil || !reflect.DeepEqual(got, want) || len(got.Unresolved) != 2 {
		t.Fatal("projection changed", err)
	}
	if revision, n := installedRevision(t, s, scope); revision != got.Revision || n != len(facts) || count(t, s, "projection_revision_facts") != len(facts) {
		t.Fatal("missing facts including reserves")
	}
	var missing int
	if err := s.db.QueryRow(`SELECT count(*) FROM raw_records r JOIN events e ON e.raw_record_id = r.id LEFT JOIN projection_revision_facts p ON p.raw_record_id = r.id WHERE p.raw_record_id IS NULL`).Scan(&missing); err != nil || missing != 0 {
		t.Fatal("input membership incomplete", err)
	}
	foreign := CorrelationScope{Queues: []correlation.QueueKey{{Instance: "foreign", QueueID: "ABSENT"}}}
	other, err := s.InstallProjection(ctx, foreign, nil, 100, opts)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(scope.Queues)-1; i < j; i, j = i+1, j-1 {
		scope.Queues[i], scope.Queues[j] = scope.Queues[j], scope.Queues[i]
	}
	for i, j := 0, len(facts)-1; i < j; i, j = i+1, j-1 {
		facts[i], facts[j] = facts[j], facts[i]
	}
	again, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil || again.Revision != got.Revision || count(t, s, "projection_scopes") != 2 || count(t, s, "projection_revisions") != 2 {
		t.Fatal("equivalent order duplicated scope", err)
	}
	opts.Window = 2 * time.Minute
	replacement, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if err != nil || replacement.Revision == got.Revision || replacement.InputRevision != got.InputRevision {
		t.Fatal("options did not version replacement", err)
	}
	if rev, _ := installedRevision(t, s, foreign); rev != other.Revision {
		t.Fatal("foreign scope replaced")
	}
	if count(t, s, "projection_revisions") != 2 || count(t, s, "projection_revision_facts") != len(facts) {
		t.Fatal("old manifest retained or proof lost")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if rev, n := installedRevision(t, reopened, scope); rev != replacement.Revision || n != len(facts) {
		t.Fatal("reopen lost manifest")
	}
}

func TestInstallProjectionRefusesStaleOrIncompleteInputsWithoutReplacing(t *testing.T) {
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
	changed := append([]correlation.Fact(nil), facts...)
	changed[0].Observation.Message += " changed synthetic observation"
	for _, input := range [][]correlation.Fact{facts[:len(facts)-1], changed} {
		got, err := s.InstallProjection(ctx, scope, input, 100, opts)
		if !errors.Is(err, ErrProjectionStale) || !reflect.DeepEqual(got, correlation.Projection{}) {
			t.Fatal("stale partial returned", err)
		}
	}
	storeCorrelationCorpus(t, s, "11-filter-reinjection", "late-archive", "trusted", false, false)
	got, err := s.InstallProjection(ctx, scope, facts, 100, opts)
	if !errors.Is(err, ErrProjectionStale) || got.Revision != "" {
		t.Fatal("late import accepted as old snapshot", err)
	}
	if rev, n := installedRevision(t, s, scope); rev != initial.Revision || n != len(facts) {
		t.Fatal("stale write altered old manifest")
	}
	current, err := s.CorrelationFacts(ctx, scope, 100)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := s.InstallProjection(ctx, scope, current, 100, opts)
	if err != nil || replacement.Revision == initial.Revision || len(replacement.Unresolved) != 2 {
		t.Fatal("fresh late import did not retain uncertainty", err)
	}
}

func TestInstallProjectionFailureRollsBackOldAndNewScopes(t *testing.T) {
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
	// Abort after scope replacement/revision/bindings and the first membership.
	if _, err := s.db.Exec(`CREATE TRIGGER fail_manifest BEFORE INSERT ON projection_revision_facts WHEN (SELECT count(*) FROM projection_revision_facts WHERE revision_id = NEW.revision_id) = 1 BEGIN SELECT RAISE(ABORT,'synthetic-private@example.org'); END`); err != nil {
		t.Fatal(err)
	}
	opts.Window = 2 * time.Minute
	for _, selection := range []CorrelationScope{scope, {Queues: scope.Queues, UnqueuedInstances: []string{"trusted"}}} {
		got, err := s.InstallProjection(ctx, selection, facts, 100, opts)
		if !errors.Is(err, ErrProjectionInstall) || strings.Contains(err.Error(), "synthetic-private") || got.Revision != "" {
			t.Fatal("failed transaction leaked or returned projection", err)
		}
		if rev, n := installedRevision(t, s, scope); rev != initial.Revision || n != len(facts) {
			t.Fatal("old current damaged")
		}
		if count(t, s, "projection_scopes") != 1 || count(t, s, "projection_revisions") != 1 || count(t, s, "projection_revision_facts") != len(facts) {
			t.Fatal("partial replacement survived")
		}
	}
	var bad int
	if err := s.db.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&bad); err != nil || bad != 0 {
		t.Fatal("rollback damaged FKs", err)
	}
}

func TestInstallProjectionEmptyBinaryScopeOptionsAndCanonicalFraming(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	instance := "instance" + string([]byte{0xff})
	relay := "relay" + string([]byte{0xfe})
	scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: instance, QueueID: "ID"}}, UnqueuedInstances: []string{instance}}
	opts := correlation.LinkOptions{Window: time.Minute, SMTPBindings: []correlation.SMTPBinding{{FromInstance: instance, Relay: relay, ToInstance: instance + "target"}}}
	projection, err := s.InstallProjection(ctx, scope, nil, 1, opts)
	if err != nil || projection.Revision == "" {
		t.Fatal("empty declared scope failed", err)
	}
	var storedInstance, storedRelay, storedTarget []byte
	if err := s.db.QueryRow(`SELECT instance FROM projection_scope_parts WHERE kind='queue'`).Scan(&storedInstance); err != nil || !bytes.Equal(storedInstance, []byte(instance)) {
		t.Fatal("scope bytes changed", err)
	}
	if err := s.db.QueryRow(`SELECT relay,to_instance FROM projection_revision_bindings`).Scan(&storedRelay, &storedTarget); err != nil || !bytes.Equal(storedRelay, []byte(relay)) || !bytes.Equal(storedTarget, []byte(instance+"target")) {
		t.Fatal("options bytes changed", err)
	}
	if _, n := installedRevision(t, s, scope); n != 0 || count(t, s, "projection_revision_facts") != 0 {
		t.Fatal("empty scope fabricated facts")
	}
	_, a := canonicalProjectionScope(CorrelationScope{Queues: []correlation.QueueKey{{Instance: "ab", QueueID: "c"}}})
	_, b := canonicalProjectionScope(CorrelationScope{Queues: []correlation.QueueKey{{Instance: "a", QueueID: "bc"}}})
	_, c := canonicalProjectionScope(CorrelationScope{UnqueuedInstances: []string{"ab"}})
	if a == b || a == c {
		t.Fatal("scope framing/kind collided")
	}
}

func TestInstallProjectionLimitsInvalidConfigurationAndCancellation(t *testing.T) {
	s, _ := openTestStore(t)
	scope := CorrelationScope{UnqueuedInstances: []string{"trusted"}}
	opts := correlation.LinkOptions{Window: time.Minute}
	for _, limit := range []int{0, correlation.MaxPartitionFacts + 1} {
		got, err := s.InstallProjection(context.Background(), scope, nil, limit, opts)
		if !errors.Is(err, correlation.ErrPartitionLimit) || got.Revision != "" {
			t.Fatal("invalid limit", err)
		}
	}
	if got, err := s.InstallProjection(context.Background(), CorrelationScope{}, nil, 1, opts); !errors.Is(err, ErrCorrelationScope) || got.Revision != "" {
		t.Fatal("invalid scope", err)
	}
	if got, err := s.InstallProjection(context.Background(), scope, nil, 1, correlation.LinkOptions{}); err == nil || got.Revision != "" {
		t.Fatal("invalid options accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := s.InstallProjection(ctx, scope, nil, 1, opts); !errors.Is(err, context.Canceled) || got.Revision != "" {
		t.Fatal("cancellation", err)
	}
	if count(t, s, "projection_scopes") != 0 {
		t.Fatal("refusal wrote manifest")
	}
}

func TestProjectionWriteLockReservesBeforeReadEvenWithoutMatchingRow(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	other, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.db.Exec(`PRAGMA busy_timeout=0`); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := projectionWriteLock(ctx, tx); err != nil {
		t.Fatal(err)
	}
	if err := other.Commit(ctx, testBatch()); err == nil || !strings.Contains(err.Error(), "SQLITE_BUSY") {
		t.Fatal("another writer committed while manifest lock held", err)
	}
	if count(t, other, "raw_records") != 0 {
		t.Fatal("busy write left partial facts")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := other.Commit(ctx, testBatch()); err != nil {
		t.Fatal("lock not released", err)
	}
}
