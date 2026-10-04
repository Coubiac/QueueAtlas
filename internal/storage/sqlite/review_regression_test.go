package sqlite

import (
	"context"
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/parser/postfix"
)

func TestForeignSchemaObjectsRefusedBeforeWAL(t *testing.T) {
	for _, seedSQL := range []string{
		`CREATE TABLE sqliteApp (id INTEGER)`,
		`CREATE VIEW another_app AS SELECT 1 AS id`,
		`PRAGMA user_version = -1`,
	} {
		t.Run(seedSQL, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "foreign.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.Exec(seedSQL); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0600); err != nil {
				t.Fatal(err)
			}
			s, err := Open(context.Background(), path)
			if err == nil {
				s.Close()
				t.Fatal("foreign or invalid schema accepted")
			}
			if err := migrate(context.Background(), db, 0); err == nil {
				t.Fatal("transactional migration accepted foreign or invalid schema")
			}
			var mode string
			var version int
			if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
				t.Fatal(err)
			}
			if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
				t.Fatal(err)
			}
			wantVersion := 0
			if seedSQL == `PRAGMA user_version = -1` {
				wantVersion = -1
			}
			if mode != "delete" || version != wantVersion {
				t.Fatal("refusal mutated persistent pragmas", mode, version)
			}
			var tables int
			if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'sources'`).Scan(&tables); err != nil || tables != 0 {
				t.Fatal("refusal added schema", tables, err)
			}
		})
	}
}

func TestTimestampNanosecondProjectionNeverWraps(t *testing.T) {
	for _, stamp := range []time.Time{
		time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Unix(0, math.MinInt64), time.Unix(0, math.MaxInt64),
		time.Unix(0, math.MinInt64).Add(-time.Nanosecond),
		time.Unix(0, math.MaxInt64).Add(time.Nanosecond),
	} {
		t.Run(stamp.Format(time.RFC3339Nano), func(t *testing.T) {
			s, _ := openTestStore(t)
			batch := testBatch()
			line := "<134>1 " + stamp.Format(time.RFC3339Nano) + " mx.example.org postfix/qmgr 1 - - ABC123: removed\n"
			batch.Records[0].Raw, batch.Records[0].End = []byte(line), int64(len(line))
			batch.Checkpoints[0].Offset = int64(len(line))
			o := postfix.Parse([]byte(line), postfix.Options{SourceID: batch.Source.ID})
			if o.ParseError != "" || o.Timestamp.Value == nil {
				t.Fatal("valid RFC5424 timestamp not parsed", o)
			}
			batch.Records[0].Observation = o
			if err := s.Commit(context.Background(), batch); err != nil {
				t.Fatal(err)
			}
			var ns sql.NullInt64
			var raw, quality, zone string
			var year int
			if err := s.db.QueryRow(`SELECT time_utc_ns, time_raw, time_quality, time_year, time_zone FROM events`).Scan(&ns, &raw, &quality, &year, &zone); err != nil {
				t.Fatal(err)
			}
			exact := time.Unix(0, stamp.UnixNano()).Equal(stamp)
			if ns.Valid != exact || exact && !time.Unix(0, ns.Int64).Equal(stamp) || raw != o.Timestamp.Raw || quality != string(o.Timestamp.Quality) || year != o.Timestamp.Year || zone != o.Timestamp.Zone {
				t.Fatal("wrapped or lost timestamp", ns, raw, quality, year, zone)
			}
		})
	}
}

func TestOpenRejectsInsecureWAL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	s, path := openTestStore(t)
	if err := s.Commit(context.Background(), testBatch()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path+"-wal", 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path+"-wal", 0600)
	// Confirm the driver itself does not repair a preexisting readable WAL.
	// Keeping the first store open preserves a live, valid WAL for the probe.
	probe, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := probe.PingContext(context.Background()); err != nil {
		probe.Close()
		t.Fatal(err)
	}
	winfo, err := os.Stat(path + "-wal")
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
	if err != nil || winfo.Mode().Perm() != 0644 {
		t.Fatal("test requires unrepaired WAL permissions", winfo, err)
	}
	opened, err := Open(context.Background(), path)
	if err == nil {
		opened.Close()
		t.Fatal("readable-by-others WAL accepted")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("test requires private main database", info, err)
	}
}

func TestOpenRejectsInsecureOtherSidecars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	for _, suffix := range []string{"-shm", "-journal"} {
		t.Run(suffix, func(t *testing.T) {
			s, path := openTestStore(t)
			if err := s.Commit(context.Background(), testBatch()); err != nil {
				t.Fatal(err)
			}
			if suffix == "-journal" {
				if err := os.WriteFile(path+suffix, []byte("synthetic cold journal"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Chmod(path+suffix, 0644); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(path+suffix, 0600)
			opened, err := Open(context.Background(), path)
			if err == nil {
				opened.Close()
				t.Fatal("readable-by-others sidecar accepted")
			}
		})
	}
}

func TestOpenCanceledBeforeCreatingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.db")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if s, err := Open(ctx, path); err == nil {
		s.Close()
		t.Fatal("canceled open accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("canceled open created database", err)
	}
}
