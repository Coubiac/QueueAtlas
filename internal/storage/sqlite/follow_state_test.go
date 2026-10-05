package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
)

func assertFollowState(t *testing.T, s *Store, want source.FollowState) {
	t.Helper()
	ctx := context.Background()
	for _, read := range []func() (source.OriginPage, error){
		func() (source.OriginPage, error) {
			return s.FileOrigins(ctx, source.OriginQuery{SourceID: "mail", Device: "1", Inode: "2", Limit: 1})
		},
		func() (source.OriginPage, error) {
			return s.FileOriginsByPath(ctx, source.OriginPathQuery{SourceID: "mail", Path: "/var/log/mail.log", Limit: 1})
		},
	} {
		page, err := read()
		if err != nil || len(page.States) != 1 || page.States[0].Origin.ID != "gen-1" || page.States[0].FollowState != want {
			t.Fatal("follow state page", page, err, "want", want)
		}
	}
}

func followBatch(from, to source.FollowState) source.Batch {
	return source.Batch{Source: testBatch().Source, FollowTransitions: []source.FollowTransition{{OriginID: "gen-1", From: from, To: to}}}
}

func TestFollowTransitionsAcquisitionRetirementRetryAndReopen(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	b.FollowTransitions = followBatch(source.FollowUnknown, source.FollowFollowing).FollowTransitions
	for range 2 {
		if err := s.Commit(ctx, b); err != nil {
			t.Fatal("acquisition/retry", err)
		}
	}
	assertFollowState(t, s, source.FollowFollowing)
	// Re-registering metadata and replaying observations do not reset the state.
	b.FollowTransitions = nil
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	assertFollowState(t, s, source.FollowFollowing)
	for range 2 {
		if err := s.Commit(ctx, followBatch(source.FollowFollowing, source.FollowRetired)); err != nil {
			t.Fatal("retirement/retry", err)
		}
	}
	assertFollowState(t, s, source.FollowRetired)
	if err := s.Commit(ctx, followBatch(source.FollowUnknown, source.FollowFollowing)); !errors.Is(err, ErrFollowStateConflict) {
		t.Fatal("stale acquisition revived retired state", err)
	}
	if err := s.Commit(ctx, followBatch(source.FollowRetired, source.FollowFollowing)); err != nil {
		t.Fatal("explicit reacquisition", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	assertFollowState(t, reopened, source.FollowFollowing)
	position, found, err := reopened.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !found || position != b.Checkpoints[0] || count(t, reopened, "raw_records") != 1 || count(t, reopened, "events") != 1 {
		t.Fatal("transitions changed provenance/checkpoint", position, found, err)
	}
}

func TestFollowTransitionsConflictScopeAndRollback(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	b.FollowTransitions = followBatch(source.FollowUnknown, source.FollowFollowing).FollowTransitions
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	foreign := b.Origins[0]
	foreign.ID = "foreign"
	if err := s.Commit(ctx, source.Batch{Source: source.Identity{ID: "other", Kind: "file", Name: "other"}, Origins: []source.Origin{foreign}}); err != nil {
		t.Fatal(err)
	}
	for _, originID := range []string{"missing", "foreign"} {
		batch := followBatch(source.FollowFollowing, source.FollowRetired)
		batch.FollowTransitions = append(batch.FollowTransitions, source.FollowTransition{OriginID: originID, From: source.FollowUnknown, To: source.FollowFollowing})
		if err := s.Commit(ctx, batch); !errors.Is(err, ErrFollowStateConflict) {
			t.Fatal("missing/foreign origin accepted", originID, err)
		}
		assertFollowState(t, s, source.FollowFollowing)
	}
	var foreignState source.FollowState
	if err := s.db.QueryRow(`SELECT follow_state FROM file_generations WHERE id = 'foreign'`).Scan(&foreignState); err != nil || foreignState != source.FollowUnknown {
		t.Fatal("foreign state changed", foreignState, err)
	}
	// The state change and new observation precede a valid checkpoint update,
	// then a failing second checkpoint. All four must roll back together.
	batch := followBatch(source.FollowFollowing, source.FollowRetired)
	start := b.Checkpoints[0].Offset
	batch.Records = []source.Record{{OriginID: "gen-1", Start: start, End: start + 5, Raw: []byte("late\n")}}
	batch.Checkpoints = []source.Position{{OriginID: "gen-1", Offset: start + 5, AnchorHash: "new-anchor"}, {OriginID: "missing", Offset: 0}}
	if err := s.Commit(ctx, batch); err == nil {
		t.Fatal("missing checkpoint origin committed")
	}
	assertFollowState(t, s, source.FollowFollowing)
	position, found, err := s.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !found || position != b.Checkpoints[0] || count(t, s, "raw_records") != 1 || count(t, s, "events") != 1 {
		t.Fatal("failed transaction left checkpoint/observations", position, found, err)
	}
	batch.Checkpoints = batch.Checkpoints[:1]
	if err := s.Commit(ctx, batch); err != nil {
		t.Fatal("corrected transaction", err)
	}
	assertFollowState(t, s, source.FollowRetired)
	position, found, err = s.Checkpoint(ctx, "mail", "gen-1")
	if err != nil || !found || position != batch.Checkpoints[0] || count(t, s, "raw_records") != 2 || count(t, s, "events") != 2 {
		t.Fatal("state/checkpoint/observations not committed together", position, found, err)
	}
}

func TestFollowTransitionsRejectInvalidChangesAndConstrainStoredValues(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	if err := s.Commit(ctx, testBatch()); err != nil {
		t.Fatal(err)
	}
	for _, transitions := range [][]source.FollowTransition{
		{{OriginID: "gen-1", From: source.FollowUnknown, To: source.FollowRetired}},
		{{OriginID: "gen-1", From: source.FollowFollowing, To: source.FollowUnknown}},
		{{OriginID: "gen-1", From: source.FollowFollowing, To: source.FollowFollowing}},
		{{OriginID: "gen-1", From: -1, To: source.FollowFollowing}},
		{{OriginID: "gen-1", From: source.FollowUnknown, To: 99}},
		{{From: source.FollowUnknown, To: source.FollowFollowing}},
		{{OriginID: "gen-1", From: source.FollowUnknown, To: source.FollowFollowing}, {OriginID: "gen-1", From: source.FollowFollowing, To: source.FollowRetired}},
	} {
		batch := source.Batch{Source: testBatch().Source, FollowTransitions: transitions}
		if err := s.Commit(ctx, batch); err == nil {
			t.Fatal("invalid transition committed", transitions)
		}
		assertFollowState(t, s, source.FollowUnknown)
	}
	batch := followBatch(source.FollowUnknown, source.FollowFollowing)
	batch.Source.Kind = "syslog"
	if err := s.Commit(ctx, batch); err == nil {
		t.Fatal("non-file follow transition accepted")
	}
	if _, err := s.db.Exec(`UPDATE file_generations SET follow_state = 99`); err == nil {
		t.Fatal("stored state constraint not enforced")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Commit(cancelled, followBatch(source.FollowUnknown, source.FollowFollowing)); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled transition committed", err)
	}
	assertFollowState(t, s, source.FollowUnknown)
}

func seedV1FollowStore(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(schemaV1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations VALUES(1, 1); PRAGMA user_version = 1; PRAGMA foreign_keys = ON;`); err != nil {
		t.Fatal(err)
	}
	// A batch without transitions is compatible with the unchanged v1 schema.
	if err := (&Store{db: db}).Commit(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	return db, path
}

func TestFollowStateMigrationV1PreservesUnknownAndExistingData(t *testing.T) {
	legacy, path := seedV1FollowStore(t)
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	assertFollowState(t, s, source.FollowUnknown)
	position, found, err := s.Checkpoint(context.Background(), "mail", "gen-1")
	if err != nil || !found || position != testBatch().Checkpoints[0] || count(t, s, "raw_records") != 1 || count(t, s, "events") != 1 || count(t, s, "schema_migrations") != 4 {
		t.Fatal("v1 data lost or changed", position, found, err)
	}
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 4 {
		t.Fatal("migration version", version, err)
	}
}

func TestFollowStateMigrationFailureRollsBackDDLAndHistory(t *testing.T) {
	legacy, path := seedV1FollowStore(t)
	if _, err := legacy.Exec(`CREATE TRIGGER stop_v2 BEFORE INSERT ON schema_migrations
		WHEN NEW.version = 2 BEGIN SELECT RAISE(ABORT, 'synthetic migration failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(context.Background(), path); err == nil {
		s.Close()
		t.Fatal("failed migration opened store")
	}
	check, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	var version, columns, history int
	if err := check.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := check.QueryRow(`SELECT count(*) FROM pragma_table_info('file_generations') WHERE name = 'follow_state'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if err := check.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if version != 1 || columns != 0 || history != 1 {
		t.Fatal("migration failure left partial schema", version, columns, history)
	}
}

func TestFollowStateMigrationRejectsMissingV2History(t *testing.T) {
	s, path := openTestStore(t)
	if _, err := s.db.Exec(`DELETE FROM schema_migrations WHERE version = 2`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := Open(context.Background(), path); err == nil {
		reopened.Close()
		t.Fatal("missing v2 history accepted")
	} else if !strings.Contains(err.Error(), "invalid schema migration history") {
		t.Fatal("unexpected history refusal", err)
	}
}
