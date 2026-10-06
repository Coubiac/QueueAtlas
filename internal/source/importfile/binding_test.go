package importfile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/model"
	"github.com/Coubiac/QueueAtlas/internal/source"
)

type checkpointFunc func(context.Context, string, string) (source.Position, bool, error)

func (f checkpointFunc) Checkpoint(ctx context.Context, id, origin string) (source.Position, bool, error) {
	return f(ctx, id, origin)
}

func beginBindingRun(t *testing.T, sink source.Sink, identity source.Identity, run source.ImportRun, id int64) source.ImportRun {
	t.Helper()
	run.ID, run.Content, run.LastOffset = id, nil, 0
	run.Status, run.CompletedAt = source.ImportRunning, nil
	if err := sink.Commit(context.Background(), source.Batch{Source: identity, ImportChange: &source.ImportChange{Target: run}}); err != nil {
		t.Fatal(err)
	}
	return run
}

func TestImportBindingNewContentAtomicAssociationRetry(t *testing.T) {
	for _, lostACK := range []bool{false, true} {
		t.Run(map[bool]string{true: "lost_ack", false: "before_write"}[lostACK], func(t *testing.T) {
			count := 0
			previous, s, _ := registeredIngestor(t, "other\n", false, func(raw []byte) model.Observation { return model.Observation{Raw: string(raw)} })
			p, identity, run, wantCP := importIngestorFixture(t, "same\nsame\n", true, 0)
			run = beginBindingRun(t, s, identity, run, 202)
			binding, err := PrepareBinding(context.Background(), p, identity, run, s, func(raw []byte) model.Observation {
				count++
				return model.Observation{Raw: string(raw)}
			})
			if err != nil || count != 0 || binding == nil {
				t.Fatal("proof", binding, err, count)
			}
			current, found, err := s.ImportRun(context.Background(), identity.ID, run.ID)
			if err != nil || !found || !reflect.DeepEqual(current, run) {
				t.Fatal("preparation wrote manifest", current, err)
			}
			if _, found, err := s.Checkpoint(context.Background(), identity.ID, wantCP.OriginID); err != nil || found {
				t.Fatal("preparation registered zero checkpoint", found, err)
			}
			failure := errors.New("synthetic association ACK failure")
			calls := 0
			var first []byte
			sink := importSinkFunc(func(ctx context.Context, b source.Batch) error {
				calls++
				data, _ := json.Marshal(b)
				if len(b.Records) != 0 || len(b.Checkpoints) != 1 || b.Checkpoints[0] != wantCP {
					t.Fatal("association parsed or skipped content")
				}
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
					t.Fatal("association retry batch changed")
				}
				return s.Commit(ctx, b)
			})
			if r, err := binding.Commit(context.Background(), sink); err != failure || r != nil || binding.pending == nil || count != 0 {
				t.Fatal("failed association exposed ingestor", r, err)
			}
			canceled, cancel := context.WithCancel(context.Background())
			cancel()
			if r, err := binding.Commit(canceled, sink); err != context.Canceled || r != nil || calls != 1 {
				t.Fatal("canceled association wrote", r, err, calls)
			}
			r, err := binding.Commit(context.Background(), sink)
			if err != nil || r == nil || calls != 2 || r.Position() != wantCP || count != 0 {
				t.Fatal("association ACK", r, err, calls, count)
			}
			assertAcknowledgedImport(t, s, r)
			again, err := binding.Commit(context.Background(), sink)
			if err != nil || again != r || calls != 2 {
				t.Fatal("association re-committed", err, calls)
			}
			if err := r.CommitNext(context.Background(), s); err != nil || count != 1 || r.Position().Offset != 5 {
				t.Fatal("first record after association", err, count)
			}
			assertAcknowledgedImport(t, s, r)
			assertAcknowledgedImport(t, s, previous)
		})
	}
}

func TestImportBindingSharedContentRecompressionAndResume(t *testing.T) {
	r, s, _ := registeredIngestor(t, "same\nsame\n", false, func(raw []byte) model.Observation { return model.Observation{Raw: string(raw)} })
	if err := r.CommitNext(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	// A new explicit attempt with renamed/recompressed identical content starts
	// at the shared checkpoint, not zero, after proving its private copy.
	p, identity, run, _ := importIngestorFixture(t, "same\nsame\n", true, 0)
	run = beginBindingRun(t, s, identity, run, 202)
	count := 0
	normalize := func(raw []byte) model.Observation { count++; return model.Observation{Raw: string(raw)} }
	b, err := PrepareBinding(context.Background(), p, identity, run, s, normalize)
	if err != nil {
		t.Fatal(err)
	}
	next, err := b.Commit(context.Background(), s)
	if err != nil || next.Position() != r.Position() || next.run.Content.OriginID != r.run.Content.OriginID || next.run.Path == r.run.Path {
		t.Fatal("recompressed content not bound to shared proven position", err)
	}
	assertAcknowledgedImport(t, s, next)
	// Already associated resume exposes the ingestor without a manifest write.
	restart, err := PrepareBinding(context.Background(), p, identity, next.RunState(), s, normalize)
	if err != nil {
		t.Fatal(err)
	}
	next, err = restart.Commit(context.Background(), importSinkFunc(func(context.Context, source.Batch) error { t.Fatal("prepared resume wrote association"); return nil }))
	if err != nil {
		t.Fatal(err)
	}
	if err := next.CommitNext(context.Background(), s); err != nil || count != 1 || next.Position().Offset != 10 {
		t.Fatal("shared checkpoint replayed first record", err, count)
	}
	if err := next.CommitNext(context.Background(), s); err != io.EOF {
		t.Fatal(err)
	}
	assertAcknowledgedImport(t, s, next)
	// The older running attempt's offset is stale: do not infer advancement.
	if b, err := PrepareBinding(context.Background(), r.prepared, r.identity, r.RunState(), s, r.normalize); b != nil || err != ErrImportResume {
		t.Fatal("stale run adopted another attempt's checkpoint", b, err)
	}
	// Different source IDs keep independent content origins and zero positions.
	foreign := identity
	foreign.ID = "separate-archive"
	run.SourceID = foreign.ID
	run = beginBindingRun(t, s, foreign, run, 303)
	b, err = PrepareBinding(context.Background(), p, foreign, run, s, normalize)
	if err != nil {
		t.Fatal(err)
	}
	separate, err := b.Commit(context.Background(), s)
	if err != nil || separate.Position().Offset != 0 || separate.Position().OriginID == next.Position().OriginID {
		t.Fatal("cross-source adoption", err)
	}
	assertAcknowledgedImport(t, s, separate)
	// Full-content reimport at EOF can complete without parsing another line.
	run.SourceID = identity.ID
	run = beginBindingRun(t, s, identity, run, 404)
	b, err = PrepareBinding(context.Background(), p, identity, run, s, normalize)
	if err != nil {
		t.Fatal(err)
	}
	done, err := b.Commit(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if err := done.CommitNext(context.Background(), s); err != io.EOF || count != 1 {
		t.Fatal(err, count)
	}
	assertAcknowledgedImport(t, s, done)
}

func TestImportBindingRefusesUnprovenPositionsBeforeCommit(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*source.ImportRun, *source.Position, *bool)
	}{
		{"foreign_origin", func(_ *source.ImportRun, p *source.Position, _ *bool) { p.OriginID = "foreign" }},
		{"opaque_anchor", func(_ *source.ImportRun, p *source.Position, _ *bool) { p.AnchorHash = "opaque" }},
		{"wrong_anchor", func(_ *source.ImportRun, p *source.Position, _ *bool) {
			p.AnchorHash = p.AnchorHash[:len(p.AnchorHash)-1] + "0"
		}},
		{"negative", func(_ *source.ImportRun, p *source.Position, _ *bool) { p.Offset = -1 }},
		{"past_end", func(_ *source.ImportRun, p *source.Position, _ *bool) { p.Offset = 11 }},
		{"prepared_without_checkpoint", func(_ *source.ImportRun, _ *source.Position, found *bool) { *found = false }},
		{"stale_prepared_offset", func(r *source.ImportRun, _ *source.Position, _ *bool) { r.LastOffset = 0 }},
		{"prepared_content_changed", func(r *source.ImportRun, _ *source.Position, _ *bool) { r.Content.Bytes++ }},
		{"terminal", func(r *source.ImportRun, _ *source.Position, _ *bool) { r.Status = source.ImportFailed }},
		{"unprepared_with_offset", func(r *source.ImportRun, _ *source.Position, _ *bool) { r.Content = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, identity, run, cp := importIngestorFixture(t, "same\nsame\n", false, 5)
			found := true
			tc.edit(&run, &cp, &found)
			state := checkpointFunc(func(_ context.Context, id, origin string) (source.Position, bool, error) {
				if id != identity.ID {
					t.Fatal("unscoped lookup")
				}
				return cp, found, nil
			})
			if binding, err := PrepareBinding(context.Background(), p, identity, run, state, func([]byte) model.Observation { t.Fatal("refusal normalized"); return model.Observation{} }); binding != nil || err != ErrImportResume {
				t.Fatal("unproven binding accepted", binding, err)
			}
		})
	}
	p, identity, run, cp := importIngestorFixture(t, "same\nsame\n", false, 0)
	cause := errors.New("synthetic state read failure")
	state := checkpointFunc(func(context.Context, string, string) (source.Position, bool, error) { return cp, false, cause })
	normalize := func([]byte) model.Observation { t.Fatal("state failure normalized"); return model.Observation{} }
	if b, err := PrepareBinding(context.Background(), p, identity, run, state, normalize); b != nil || err != cause {
		t.Fatal(b, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state = checkpointFunc(func(context.Context, string, string) (source.Position, bool, error) {
		t.Fatal("canceled preparation read state")
		return cp, false, nil
	})
	if b, err := PrepareBinding(ctx, p, identity, run, state, normalize); b != nil || err != context.Canceled {
		t.Fatal(b, err)
	}
	if b, err := PrepareBinding(context.Background(), p, identity, run, nil, normalize); b != nil || err != ErrImportResume {
		t.Fatal(b, err)
	}
}
