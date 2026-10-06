package sqlite

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/correlation"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

// Simulate the future purge inside one transaction. No public deletion API is
// delivered in this lot; referenced projections are tested by the migration.
func simulatePurgedRecord(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRow(`SELECT id FROM raw_records ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := rememberPurgedRecord(ctx, tx, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM raw_records WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestPurgedRecordReplaySurvivesReopenAndKeepsNamespacesDistinct(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	cp, found, err := s.Checkpoint(ctx, b.Source.ID, b.Records[0].OriginID)
	if err != nil || !found {
		t.Fatal("checkpoint absent", err)
	}
	simulatePurgedRecord(t, s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Parsed data and wall-clock ReadAt are not the physical content commitment.
	b.Records[0].ReadAt = b.Records[0].ReadAt.Add(time.Hour)
	b.Records[0].Observation.QueueID = "changed-projection-input"
	for range 2 {
		if err := s.Commit(ctx, b); err != nil {
			t.Fatal("same-byte retry", err)
		}
	}
	if count(t, s, "raw_records") != 0 || count(t, s, "events") != 0 || count(t, s, "event_search_domains") != 0 || count(t, s, "purged_records") != 1 {
		t.Fatal("retry resurrected deleted facts")
	}
	after, found, err := s.Checkpoint(ctx, b.Source.ID, b.Records[0].OriginID)
	if err != nil || !found || after != cp {
		t.Fatal("checkpoint changed", err)
	}
	for i, sourceID := range []string{"mail", "other-source"} {
		fresh := testBatch()
		fresh.Source.ID = sourceID
		fresh.Records[0].Observation.SourceID = sourceID
		fresh.Origins[0].ID = []string{"other-origin", "third-origin"}[i]
		fresh.Records[0].OriginID = fresh.Origins[0].ID
		fresh.Checkpoints[0].OriginID = fresh.Origins[0].ID
		if err := s.Commit(ctx, fresh); err != nil {
			t.Fatal("new physical namespace rejected", err)
		}
	}
	if count(t, s, "raw_records") != 2 || count(t, s, "purged_records") != 1 {
		t.Fatal("physical origins were merged")
	}
}

func TestPurgedRecordCollisionRollsBackWholeBatch(t *testing.T) {
	for _, field := range []string{"raw", "read-error", "end"} {
		t.Run(field, func(t *testing.T) {
			s, _ := openTestStore(t)
			ctx := context.Background()
			b := testBatch()
			if err := s.Commit(ctx, b); err != nil {
				t.Fatal(err)
			}
			simulatePurgedRecord(t, s)
			bad := b.Records[0]
			switch field {
			case "raw":
				bad.Raw = append([]byte(nil), bad.Raw...)
				bad.Raw[0] = 'X'
			case "read-error":
				bad.Error = "synthetic-private-collision"
			case "end":
				bad.End++
			}
			fresh := b.Records[0]
			fresh.Start, fresh.End = 1000, 1000+int64(len(fresh.Raw))
			b.Records = []source.Record{fresh, bad}
			b.Source.Name = "synthetic-uncommitted-name"
			b.Checkpoints[0].Offset = fresh.End
			err := s.Commit(ctx, b)
			if !errors.Is(err, ErrPurgedRecordCollision) || err.Error() != ErrPurgedRecordCollision.Error() {
				t.Fatal("collision not fixed", err)
			}
			if count(t, s, "raw_records") != 0 || count(t, s, "events") != 0 || count(t, s, "purged_records") != 1 {
				t.Fatal("failed batch changed facts")
			}
			cp, found, err := s.Checkpoint(ctx, "mail", "gen-1")
			if err != nil || !found || cp != testBatch().Checkpoints[0] {
				t.Fatal("failed batch advanced checkpoint", err)
			}
			var name string
			if err := s.db.QueryRow(`SELECT name FROM sources WHERE id='mail'`).Scan(&name); err != nil || name != "mail log" {
				t.Fatal("failed batch changed source", err)
			}
		})
	}
}

func TestPurgedRecordRememberRequiresCoveredCheckpointAndIsTransactional(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	if err := s.Commit(ctx, testBatch()); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`DELETE FROM checkpoints`,
		`INSERT INTO checkpoints VALUES('mail','gen-1',0,'anchor',0)`,
		`UPDATE checkpoints SET offset=10000,anchor_hash=''`,
		`UPDATE checkpoints SET offset='synthetic-private-offset',anchor_hash='anchor'`,
	} {
		if _, err := s.db.Exec(sql); err != nil {
			t.Fatal(err)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = rememberPurgedRecord(ctx, tx, 1)
		tx.Rollback()
		if !errors.Is(err, ErrPurgedRecordState) || err.Error() != ErrPurgedRecordState.Error() {
			t.Fatal("invalid checkpoint accepted", err)
		}
	}
	if _, err := s.db.Exec(`UPDATE checkpoints SET offset=10000,anchor_hash='anchor'`); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := rememberPurgedRecord(ctx, tx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`DELETE FROM raw_records WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if count(t, s, "purged_records") != 0 || count(t, s, "events") != 1 || count(t, s, "raw_records") != 1 {
		t.Fatal("rollback retained marker or deleted facts")
	}
	simulatePurgedRecord(t, s)
	if _, err := s.db.Exec(`UPDATE purged_records SET end_offset=end_offset+1`); err == nil {
		t.Fatal("marker was mutable")
	}
	if _, err := s.db.Exec(`INSERT INTO purged_records VALUES('mail','gen-1',1000,1001,'not-a-blob')`); err == nil {
		t.Fatal("invalid digest accepted")
	}
}

func TestPurgedRecordCompletedImportRetryDoesNotResurrectFacts(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	attached := prepareImportFixture(t, s, "same\n")
	last := importProgressBatch(attached, "same\n", source.ImportComplete)
	if err := s.Commit(ctx, last); err != nil {
		t.Fatal(err)
	}
	simulatePurgedRecord(t, s)
	if err := s.Commit(ctx, last); err != nil {
		t.Fatal("lost ACK retry rejected", err)
	}
	assertImportPosition(t, s, last.ImportChange.Target, 0)
	last.Records[0].Raw = []byte("evil\n")
	if err := s.Commit(ctx, last); !errors.Is(err, ErrPurgedRecordCollision) {
		t.Fatal("import retry collision accepted", err)
	}
	assertImportPosition(t, s, last.ImportChange.Target, 0)
	if count(t, s, "purged_records") != 1 || count(t, s, "import_runs") != 1 {
		t.Fatal("retry changed import metadata")
	}
}

func TestPurgedRecordMalformedStoredDigestFailsWithoutPrivateError(t *testing.T) {
	s, _ := openTestStore(t)
	ctx := context.Background()
	b := testBatch()
	if err := s.Commit(ctx, b); err != nil {
		t.Fatal(err)
	}
	simulatePurgedRecord(t, s)
	if _, err := s.db.Exec(`DROP TRIGGER purged_records_immutable; PRAGMA ignore_check_constraints=ON; UPDATE purged_records SET digest=x'01'; PRAGMA ignore_check_constraints=OFF;`); err != nil {
		t.Fatal(err)
	}
	err := s.Commit(ctx, b)
	if !errors.Is(err, ErrPurgedRecordState) || err.Error() != ErrPurgedRecordState.Error() {
		t.Fatal("malformed commitment accepted", err)
	}
	if count(t, s, "raw_records") != 0 {
		t.Fatal("malformed commitment resurrected a fact")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.Commit(ctx, b); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
}

func TestPurgedRecordMigrationV6PreservesProjectionAndRollsBack(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "reopen", true: "rollback"}[fail], func(t *testing.T) {
			s, path := openTestStore(t)
			ctx := context.Background()
			if err := s.Commit(ctx, testBatch()); err != nil {
				t.Fatal(err)
			}
			scope := CorrelationScope{Queues: []correlation.QueueKey{{Instance: "mx-a", QueueID: "ABC123"}}}
			facts, err := s.CorrelationFacts(ctx, scope, 10)
			if err != nil {
				t.Fatal(err)
			}
			want, err := s.InstallProjection(ctx, scope, facts, 10, correlation.LinkOptions{Window: time.Minute})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(`DROP TABLE purged_records; DELETE FROM schema_migrations WHERE version=7; PRAGMA user_version=6;`); err != nil {
				t.Fatal(err)
			}
			if fail {
				if _, err := s.db.Exec(`CREATE TRIGGER stop_v7 BEFORE INSERT ON schema_migrations WHEN NEW.version=7 BEGIN SELECT RAISE(ABORT,'synthetic v7 failure'); END;`); err != nil {
					t.Fatal(err)
				}
				if err := migrate(ctx, s.db, 7); err == nil {
					t.Fatal("failed migration accepted")
				}
				var n, version int
				if err := s.db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name LIKE 'purged_records%'`).Scan(&n); err != nil || n != 0 {
					t.Fatal("partial DDL survived", err)
				}
				if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 6 || count(t, s, "schema_migrations") != 6 {
					t.Fatal("partial migration advanced history", err)
				}
			} else {
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
				s, err = Open(ctx, path)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				if count(t, s, "purged_records") != 0 || count(t, s, "schema_migrations") != schemaVersion {
					t.Fatal("migration invented purged records")
				}
			}
			got, found, err := s.CurrentProjection(ctx, scope, 10)
			if err != nil || !found || !reflect.DeepEqual(want, got) {
				t.Fatal("migration changed projection", err)
			}
			cp, found, err := s.Checkpoint(ctx, "mail", "gen-1")
			if err != nil || !found || cp != testBatch().Checkpoints[0] || count(t, s, "raw_records") != 1 || count(t, s, "event_search_domains") != 1 {
				t.Fatal("migration lost facts/checkpoint", err)
			}
		})
	}
}
