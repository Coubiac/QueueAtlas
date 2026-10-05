package importfile

import (
	"context"
	"database/sql"
	"errors"

	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/parser/postfix"
	"github.com/Coubiac/mailtrace/internal/parser/syslog"
	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/source/file"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

const pipelineFirst = "Dec 31 23:59:58 mx-declared postfix/qmgr[1]: A1B2C3: from=<alice@invalid.example>, size=42, nrcpt=1 (queue active)\n"
const pipelineDelivery = "Jan  1 00:00:01 mx-declared postfix/smtp[2]: A1B2C3: to=<bob@invalid.example>, relay=remote.invalid[192.0.2.1]:25, dsn=2.0.0, status=sent (250 accepted)\r\n"
const pipelineRemoved = "<22>1 2026-01-01T00:00:02+01:00 mx-declared postfix/qmgr 3 - - A1B2C3: removed\n"
const pipelinePayload = pipelineFirst + pipelineDelivery + pipelineDelivery + pipelineRemoved

func pipelineNormalizer(year int, calls *int) file.Normalize {
	return func(raw []byte) model.Observation {
		*calls++
		opts := postfix.Options{SourceID: "caller-parser-id", Time: syslog.TimeContext{Year: year}}
		if year != 0 {
			opts.Time.Location = time.UTC
		}
		return postfix.Parse(raw, opts)
	}
}

type pipelineFact struct {
	SourceID, OriginID, Raw, Host, Instance, Quality, TimeRaw, Zone, Kind, QueueID, ParseError string
	Start, End                                                                                 int64
	Year                                                                                       int
	UTC                                                                                        sql.NullInt64
	Sender, Recipient, Status                                                                  sql.NullString
}

func pipelineFacts(t *testing.T, db *sql.DB) []pipelineFact {
	t.Helper()
	rows, err := db.Query("SELECT r.source_id,r.generation_id,r.start_offset,r.end_offset,r.raw,e.host,e.instance,e.time_quality,e.time_raw,e.time_year,e.time_zone,e.time_utc_ns,e.kind,e.queue_id,e.sender,e.recipient,e.status,e.parse_error FROM raw_records r JOIN events e ON e.raw_record_id=r.id ORDER BY r.source_id,r.generation_id,r.start_offset")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var facts []pipelineFact
	for rows.Next() {
		var f pipelineFact
		if err := rows.Scan(&f.SourceID, &f.OriginID, &f.Start, &f.End, &f.Raw, &f.Host, &f.Instance, &f.Quality, &f.TimeRaw, &f.Year, &f.Zone, &f.UTC, &f.Kind, &f.QueueID, &f.Sender, &f.Recipient, &f.Status, &f.ParseError); err != nil {
			t.Fatal(err)
		}
		facts = append(facts, f)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return facts
}

func pipelineStore(t *testing.T) (*sqlite.Store, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic-pipeline.db")
	s, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return s, db
}

func TestImportPipelineRealParserDurableRestartPreservesTimeHypotheses(t *testing.T) {
	for _, zipped := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "gzip"}[zipped], func(t *testing.T) {
			s, db := pipelineStore(t)
			data := []byte(pipelinePayload)
			if zipped {
				data = compressedFixture(t, pipelinePayload, 1, "synthetic")
			}
			cfg := AttemptConfig{Identity: source.Identity{ID: "archive", Kind: "import", Name: "synthetic archive", TrustedHost: "trusted-mx"}, RunID: 101,
				Path: inputFixture(t, data), Prepare: PrepareOptions{Gzip: zipped, Limits: unlimitedFixtureLimits(), TempDir: t.TempDir()}}
			calls := 0
			a, err := NewAttempt(cfg, s, pipelineNormalizer(0, &calls))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(attemptContext(t))
			defer cancel()
			sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
				err := s.Commit(ctx, b)
				if err == nil && len(b.Records) != 0 {
					cancel()
				}
				return err
			})
			if err := a.Run(ctx, sink); !errors.Is(err, context.Canceled) || calls != 1 {
				t.Fatal(err, calls)
			}
			run := attemptRunState(t, s, a)
			if run.LastOffset != int64(len(pipelineFirst)) || run.Status != source.ImportRunning {
				t.Fatal(run)
			}
			first := pipelineFacts(t, db)
			if len(first) != 1 || first[0].UTC.Valid || first[0].Year != 0 || first[0].Quality != string(model.TimeWallOnly) {
				t.Fatal("missing year was invented", first)
			}
			assertAttemptCleanup(t, cfg.Prepare.TempDir)
			// New object has no earlier pending state, and its parsing context
			// changes. The first durable observation must remain untouched.
			restarted, err := NewAttempt(cfg, s, pipelineNormalizer(2027, &calls))
			if err != nil {
				t.Fatal(err)
			}
			if err := restarted.Run(attemptContext(t), s); err != nil || calls != 4 {
				t.Fatal(err, calls)
			}
			facts := pipelineFacts(t, db)
			if len(facts) != 4 || facts[0] != first[0] {
				t.Fatal("resume replaced first fact", facts)
			}
			offset := int64(0)
			for i, raw := range []string{pipelineFirst, pipelineDelivery, pipelineDelivery, pipelineRemoved} {
				f := facts[i]
				if f.SourceID != "archive" || f.Host != "mx-declared" || f.Instance != "trusted-mx" || f.QueueID != "A1B2C3" ||
					f.Start != offset || f.End != offset+int64(len(raw)) || f.Raw != raw || f.ParseError != "" {
					t.Fatal("raw/search/identity lost", i, f)
				}
				offset = f.End
			}
			if !facts[0].Sender.Valid || facts[0].Sender.String != "alice@invalid.example" || facts[0].Recipient.Valid {
				t.Fatal(facts[0])
			}
			for _, f := range facts[1:3] {
				when := time.Date(2027, 1, 1, 0, 0, 1, 0, time.UTC).UnixNano()
				if f.Kind != string(model.KindDelivery) || f.Recipient.String != "bob@invalid.example" || f.Status.String != "sent" ||
					f.Quality != string(model.TimeConfiguredYearAndZone) || f.Year != 2027 || f.Zone != "UTC" || !f.UTC.Valid || f.UTC.Int64 != when {
					t.Fatal("delivery hypothesis lost", f)
				}
			}
			if facts[1].Start == facts[2].Start || facts[3].Kind != string(model.KindRemoved) || facts[3].Quality != string(model.TimeExplicitOffset) ||
				facts[3].Year != 2026 || facts[3].Zone != "+01:00" || facts[3].UTC.Int64 != time.Date(2025, 12, 31, 23, 0, 2, 0, time.UTC).UnixNano() {
				t.Fatal("identical lines collapsed or explicit time overwritten", facts)
			}
			// Renamed/recompressed new attempt reuses the proven EOF checkpoint
			// without reinterpreting old events under another year hypothesis.
			cfg.RunID = 202
			cfg.Path = inputFixture(t, compressedFixture(t, pipelinePayload, 9, "renamed"))
			cfg.Prepare.Gzip = true
			again, err := NewAttempt(cfg, s, func([]byte) model.Observation {
				t.Fatal("reimport parsed completed content")
				return model.Observation{}
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := again.Run(attemptContext(t), s); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(pipelineFacts(t, db), facts) {
				t.Fatal("reimport changed durable parser facts")
			}
			assertAttemptCleanup(t, cfg.Prepare.TempDir)
		})
	}
}
