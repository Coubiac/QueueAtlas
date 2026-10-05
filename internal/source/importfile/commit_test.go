package importfile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Coubiac/mailtrace/internal/model"
	"github.com/Coubiac/mailtrace/internal/source"
	"github.com/Coubiac/mailtrace/internal/source/file"
	"github.com/Coubiac/mailtrace/internal/storage/sqlite"
)

type importSinkFunc func(context.Context, source.Batch) error

func (f importSinkFunc) Commit(ctx context.Context, b source.Batch) error { return f(ctx, b) }

func registeredIngestor(t *testing.T, payload string, zipped bool, normalize file.Normalize) (*Ingestor, *sqlite.Store, string) {
	t.Helper()
	p, identity, run, cp := importIngestorFixture(t, payload, zipped, 0)
	path := filepath.Join(t.TempDir(), "synthetic.db")
	s, err := sqlite.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Commit(context.Background(), source.Batch{Source: identity,
		Origins:     []source.Origin{{ID: cp.OriginID, Path: run.Path, Fingerprint: "sha256:" + run.Content.SHA256, FirstSeen: run.CreatedAt}},
		Checkpoints: []source.Position{cp}, ImportChange: &source.ImportChange{Target: run}}); err != nil {
		t.Fatal(err)
	}
	r, err := NewIngestor(context.Background(), p, identity, run, cp, normalize)
	if err != nil {
		t.Fatal(err)
	}
	return r, s, path
}

func assertAcknowledgedImport(t *testing.T, s *sqlite.Store, r *Ingestor) {
	t.Helper()
	got, found, err := s.ImportRun(context.Background(), r.identity.ID, r.run.ID)
	if err != nil || !found || !reflect.DeepEqual(got, r.RunState()) {
		t.Fatal("run state diverged", got, found, err, r.RunState())
	}
	cp, found, err := s.Checkpoint(context.Background(), r.identity.ID, r.position.OriginID)
	if err != nil || !found || cp != r.Position() {
		t.Fatal("checkpoint diverged", cp, found, err, r.Position())
	}
}

func TestImportCommitNextAtomicLostACKAndFiniteCompletion(t *testing.T) {
	for _, zipped := range []bool{false, true} {
		for _, lostACK := range []bool{false, true} {
			t.Run(strings.Join([]string{map[bool]string{true: "gzip", false: "plain"}[zipped], map[bool]string{true: "lost_ack", false: "before_write"}[lostACK]}, "/"), func(t *testing.T) {
				normalized := 0
				r, s, path := registeredIngestor(t, "same\nsame\n", zipped, func(raw []byte) model.Observation {
					normalized++
					return model.Observation{Raw: string(raw), Kind: model.KindUnknown, SourceID: "untrusted"}
				})
				failure := errors.New("synthetic commit failure")
				for step := 0; step < 3; step++ {
					beforeRun, beforeCP := r.RunState(), r.Position()
					var first []byte
					calls := 0
					sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
						data, err := json.Marshal(b)
						if err != nil {
							t.Fatal(err)
						}
						calls++
						if calls == 1 {
							first = data
							if lostACK {
								if err := s.Commit(ctx, b); err != nil {
									t.Fatal(err)
								}
							}
							return failure
						}
						if string(data) != string(first) {
							t.Fatal("retry changed records, normalization, timestamps or expected state")
						}
						return s.Commit(ctx, b)
					})
					if err := r.CommitNext(context.Background(), sink); err != failure ||
						!reflect.DeepEqual(r.RunState(), beforeRun) || r.Position() != beforeCP {
						t.Fatal("failed commit acknowledged or advanced", err)
					}
					canceled, cancel := context.WithCancel(context.Background())
					cancel()
					if err := r.CommitNext(canceled, sink); err != context.Canceled || calls != 1 {
						t.Fatal("canceled retry called sink", err, calls)
					}
					want := error(nil)
					if step == 2 {
						want = io.EOF
					}
					if err := r.CommitNext(context.Background(), sink); err != want || calls != 2 {
						t.Fatal("retry result", err, want, calls)
					}
					assertAcknowledgedImport(t, s, r)
					if normalized != min(step+1, 2) {
						t.Fatal("normalized twice or skipped line", normalized)
					}
				}
				if err := r.CommitNext(context.Background(), importSinkFunc(func(context.Context, source.Batch) error {
					t.Fatal("terminal call wrote again")
					return nil
				})); err != io.EOF {
					t.Fatal(err)
				}
				if err := s.Close(); err != nil {
					t.Fatal(err)
				}
				reopened, err := sqlite.Open(context.Background(), path)
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				assertAcknowledgedImport(t, reopened, r)
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				for _, table := range []string{"raw_records", "events"} {
					var count int
					if err := db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 2 {
						t.Fatal("duplicate or missing physical records", table, count, err)
					}
				}
			})
		}
	}
}

func TestImportCommitNextEmptyPartialAndPositiveRestart(t *testing.T) {
	for _, payload := range []string{"", "partial", "line\npartial", "line\nnext\n"} {
		t.Run(payload, func(t *testing.T) {
			count := 0
			r, s, _ := registeredIngestor(t, payload, false, func(raw []byte) model.Observation {
				count++
				return model.Observation{Raw: string(raw)}
			})
			if strings.HasPrefix(payload, "line\n") {
				if err := r.CommitNext(context.Background(), s); err != nil {
					t.Fatal(err)
				}
				// A fresh constructor at the durable position must neither replay
				// the first record nor reopen the original input path.
				fresh, err := NewIngestor(context.Background(), r.prepared, r.identity, r.RunState(), r.Position(), r.normalize)
				if err != nil {
					t.Fatal(err)
				}
				r = fresh
				if payload == "line\nnext\n" {
					if err := r.CommitNext(context.Background(), s); err != nil {
						t.Fatal(err)
					}
				}
			}
			want, status := error(io.EOF), source.ImportComplete
			if r.run.Content.TrailingPartial {
				want, status = ErrImportPartial, source.ImportFailed
			}
			failure := errors.New("terminal ACK lost")
			var first []byte
			calls := 0
			sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
				if len(b.Records) != 0 || len(b.Checkpoints) != 0 {
					t.Fatal("EOF manufactured physical record/checkpoint")
				}
				data, _ := json.Marshal(b)
				calls++
				if calls == 1 {
					first = data
					if err := s.Commit(ctx, b); err != nil {
						t.Fatal(err)
					}
					return failure
				}
				if string(first) != string(data) {
					t.Fatal("terminal retry timestamp/state changed")
				}
				return s.Commit(ctx, b)
			})
			if err := r.CommitNext(context.Background(), sink); err != failure || r.run.Status != source.ImportRunning {
				t.Fatal("ambiguous terminal commit advanced", err)
			}
			if err := r.CommitNext(context.Background(), sink); err != want || r.run.Status != status || count != strings.Count(payload, "\n") {
				t.Fatal("terminal outcome", err, want, r.RunState(), count)
			}
			assertAcknowledgedImport(t, s, r)
			stamp := *r.RunState().CompletedAt
			exposed := r.RunState()
			*exposed.CompletedAt = exposed.CreatedAt.AddDate(1, 0, 0)
			if !r.RunState().CompletedAt.Equal(stamp) {
				t.Fatal("terminal timestamp pointer aliased")
			}
		})
	}
}

func TestImportCommitNextCancellationOversizeAndStagedProofFailure(t *testing.T) {
	t.Run("cancel_from_normalizer", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		count := 0
		r, s, _ := registeredIngestor(t, "first\nnext\n", false, func(raw []byte) model.Observation {
			count++
			cancel()
			raw[0] = 'X' // source-owned raw record must not be altered
			return model.Observation{Raw: string(raw)}
		})
		calls := 0
		sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
			calls++
			if string(b.Records[0].Raw) != "first\n" || b.Records[0].Observation.SourceID != r.identity.ID {
				t.Fatal("normalizer changed provenance")
			}
			return s.Commit(ctx, b)
		})
		if err := r.CommitNext(ctx, sink); err != context.Canceled || calls != 0 || count != 1 || r.position.Offset != 0 {
			t.Fatal("cancellation lost batch", err, calls, count)
		}
		if err := r.CommitNext(context.Background(), sink); err != nil || calls != 1 || count != 1 || r.position.Offset != 6 {
			t.Fatal("cancellation retry skipped or normalized twice", err, calls, count)
		}
		assertAcknowledgedImport(t, s, r)
	})
	t.Run("oversize", func(t *testing.T) {
		payload := strings.Repeat("x", model.MaxLineBytes+17) + "\n"
		r, s, _ := registeredIngestor(t, payload, false, func([]byte) model.Observation {
			t.Fatal("oversized line reached parser")
			return model.Observation{}
		})
		sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
			record := b.Records[0]
			if len(record.Raw) != model.MaxLineBytes || record.End != int64(len(payload)) ||
				record.Error == "" || record.Observation.ParseError != record.Error || record.Observation.Kind != model.KindUnknown {
				t.Fatal("oversized provenance lost", record.End, len(record.Raw))
			}
			want, err := file.CaptureAnchor(r.prepared.file, record.End)
			if err != nil || b.Checkpoints[0].AnchorHash != want.String() {
				t.Fatal("anchor not over full physical line", err)
			}
			return s.Commit(ctx, b)
		})
		if err := r.CommitNext(context.Background(), sink); err != nil {
			t.Fatal(err)
		}
		assertAcknowledgedImport(t, s, r)
	})
	t.Run("proof_failure_keeps_consumed_record", func(t *testing.T) {
		p, identity, run, cp := importIngestorFixture(t, "first\nnext\n", false, 0)
		count := 0
		r, err := NewIngestor(context.Background(), p, identity, run, cp, func([]byte) model.Observation { count++; return model.Observation{} })
		if err != nil {
			t.Fatal(err)
		}
		// Inject a closed proof descriptor after a complete record is consumed.
		// The production copy contract forbids this; refusal must not skip it.
		closed, _, err := file.OpenLog(context.Background(), run.Path)
		if err != nil {
			t.Fatal(err)
		}
		closed.Close()
		original := p.file
		p.file = closed
		defer func() { p.file = original }()
		r.lines, _ = file.NewLineReader(strings.NewReader("first\nnext\n"), 0)
		sink := importSinkFunc(func(context.Context, source.Batch) error { t.Fatal("failed proof committed"); return nil })
		if err := r.CommitNext(context.Background(), nil); err == nil || r.staged != nil {
			t.Fatal("nil sink consumed input")
		}
		for range 2 {
			if err := r.CommitNext(context.Background(), sink); err == nil ||
				r.staged == nil || string(r.staged.Raw) != "first\n" || count != 0 || r.position != cp {
				t.Fatal("proof failure skipped consumed line", err, r.staged, count)
			}
		}
		// Restore the held copy; the staged first record, then next record,
		// must be emitted in order without another physical reader pass.
		p.file = original
		var raws []string
		ack := importSinkFunc(func(_ context.Context, b source.Batch) error {
			raws = append(raws, string(b.Records[0].Raw))
			return nil
		})
		for range 2 {
			if err := r.CommitNext(context.Background(), ack); err != nil {
				t.Fatal(err)
			}
		}
		if strings.Join(raws, "") != "first\nnext\n" || count != 2 {
			t.Fatal(raws, count)
		}
	})
}

func TestImportCommitNextSinkSentinelsDoNotAcknowledgeCompletion(t *testing.T) {
	for _, payload := range []string{"line\n", "", "partial"} {
		for _, failure := range []error{io.EOF, ErrImportPartial} {
			for _, committed := range []bool{false, true} {
				t.Run(payload+"/"+failure.Error()+"/"+map[bool]string{true: "lost_ack", false: "before_write"}[committed], func(t *testing.T) {
					r, s, _ := registeredIngestor(t, payload, false, func(raw []byte) model.Observation { return model.Observation{Raw: string(raw)} })
					var data []byte
					before := r.RunState()
					if err := r.CommitNext(context.Background(), importSinkFunc(func(ctx context.Context, b source.Batch) error {
						data, _ = json.Marshal(b)
						if committed {
							if err := s.Commit(ctx, b); err != nil {
								t.Fatal(err)
							}
						}
						return failure
					})); err != failure || !reflect.DeepEqual(r.RunState(), before) || r.pending == nil {
						t.Fatal("sink sentinel falsely acknowledged completion", err, r.RunState())
					}
					err := r.CommitNext(context.Background(), importSinkFunc(func(ctx context.Context, b source.Batch) error {
						retry, _ := json.Marshal(b)
						if string(data) != string(retry) {
							t.Fatal("sentinel retry changed batch")
						}
						return s.Commit(ctx, b)
					}))
					want := error(nil)
					if payload == "" {
						want = io.EOF
					}
					if payload == "partial" {
						want = ErrImportPartial
					}
					if err != want {
						t.Fatal(err, want)
					}
					assertAcknowledgedImport(t, s, r)
				})
			}
		}
	}
}
