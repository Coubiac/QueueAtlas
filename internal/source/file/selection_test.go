package file

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

type originReaderFunc func(context.Context, source.OriginQuery) (source.OriginPage, error)

func (r originReaderFunc) FileOrigins(ctx context.Context, q source.OriginQuery) (source.OriginPage, error) {
	return r(ctx, q)
}

func (r originReaderFunc) Checkpoint(context.Context, string, string) (source.Position, bool, error) {
	return source.Position{}, false, errors.New("unexpected separate checkpoint read")
}

func selectionStates(statuses ...ResumeStatus) []source.OriginState {
	states := make([]source.OriginState, len(statuses))
	for i, status := range statuses {
		id := fmt.Sprintf("gen-%04d", i)
		states[i] = source.OriginState{
			Origin:     source.Origin{ID: id, Device: "1", Inode: "2", Fingerprint: string(status)},
			Checkpoint: &source.Position{OriginID: id, Offset: int64(i + 1)},
		}
		if status == ResumeRestartZero {
			states[i].Checkpoint.Offset = 0
		}
	}
	return states
}

func selectionQuery() source.OriginQuery {
	return source.OriginQuery{SourceID: "source-1", Device: "1", Inode: "2"}
}

func statusVerifier(s source.OriginState) (ResumeCheck, error) {
	return ResumeCheck{Status: ResumeStatus(s.Origin.Fingerprint)}, nil
}

func pagedOrigins(t *testing.T, states []source.OriginState, pageSize int, calls *int) originReaderFunc {
	t.Helper()
	return func(_ context.Context, q source.OriginQuery) (source.OriginPage, error) {
		(*calls)++
		if q.SourceID != "source-1" || q.Device != "1" || q.Inode != "2" || q.Limit < 1 || q.Limit > source.MaxOriginPageSize {
			t.Fatalf("unexpected query: %+v", q)
		}
		start := 0
		for start < len(states) && states[start].Origin.ID <= q.AfterID {
			start++
		}
		end := min(len(states), start+min(q.Limit, pageSize))
		page := source.OriginPage{States: states[start:end]}
		if end < len(states) {
			page.NextID = states[end-1].Origin.ID
		}
		return page, nil
	}
}

func TestResumeSelectionAcrossPages(t *testing.T) {
	for _, tc := range []struct {
		name   string
		states []ResumeStatus
		want   SelectionStatus
	}{
		{"absent", nil, SelectionAbsent},
		{"different", []ResumeStatus{ResumeDifferent, ResumeDifferent}, SelectionDifferent},
		{"unique after different", []ResumeStatus{ResumeDifferent, ResumeMatch, ResumeDifferent}, SelectionUnique},
		{"unique before different", []ResumeStatus{ResumeMatch, ResumeDifferent}, SelectionUnique},
		{"ambiguous", []ResumeStatus{ResumeMatch, ResumeDifferent, ResumeMatch}, SelectionAmbiguous},
		{"incomplete alone", []ResumeStatus{ResumeInsufficient}, SelectionInsufficient},
		{"incomplete after match", []ResumeStatus{ResumeMatch, ResumeInsufficient}, SelectionInsufficient},
		{"incomplete before match", []ResumeStatus{ResumeInsufficient, ResumeMatch}, SelectionInsufficient},
		{"incomplete and different", []ResumeStatus{ResumeDifferent, ResumeInsufficient}, SelectionInsufficient},
		{"zero after different", []ResumeStatus{ResumeDifferent, ResumeRestartZero}, SelectionRestartZero},
		{"zero then incomplete", []ResumeStatus{ResumeRestartZero, ResumeInsufficient}, SelectionInsufficient},
		{"incomplete then zero", []ResumeStatus{ResumeInsufficient, ResumeRestartZero}, SelectionInsufficient},
		{"two zero candidates", []ResumeStatus{ResumeRestartZero, ResumeRestartZero}, SelectionAmbiguous},
		{"zero then positive", []ResumeStatus{ResumeRestartZero, ResumeMatch}, SelectionAmbiguous},
		{"positive then zero", []ResumeStatus{ResumeMatch, ResumeRestartZero}, SelectionAmbiguous},
	} {
		t.Run(tc.name, func(t *testing.T) {
			states := selectionStates(tc.states...)
			calls := 0
			got, err := scanResume(context.Background(), pagedOrigins(t, states, 1, &calls), selectionQuery(), statusVerifier)
			if err != nil || got.Status != tc.want || got.Examined != len(states) {
				t.Fatalf("selection: %+v, %v", got, err)
			}
			if tc.want == SelectionUnique || tc.want == SelectionRestartZero {
				wantFingerprint := ResumeMatch
				if tc.want == SelectionRestartZero {
					wantFingerprint = ResumeRestartZero
				}
				if got.Candidate == nil || got.Candidate.Origin.Fingerprint != string(wantFingerprint) {
					t.Fatalf("missing matching candidate: %+v", got)
				}
				position := got.Candidate.Checkpoint.Offset
				for _, s := range states {
					s.Checkpoint.Offset = 99
				}
				if got.Candidate.Checkpoint.Offset != position {
					t.Fatal("borrowed checkpoint escaped reader")
				}
			} else if got.Candidate != nil {
				t.Fatal("nonunique result exposed a candidate")
			}
			if calls != max(1, len(states)) {
				t.Fatalf("page calls = %d", calls)
			}
		})
	}
}

func TestResumeSelectionCandidateLimit(t *testing.T) {
	for _, size := range []int{MaxResumeCandidates, MaxResumeCandidates + 1} {
		statuses := make([]ResumeStatus, size)
		for i := range statuses {
			statuses[i] = ResumeDifferent
		}
		statuses[0] = ResumeMatch
		calls := 0
		got, err := scanResume(context.Background(), pagedOrigins(t, selectionStates(statuses...), 100, &calls), selectionQuery(), statusVerifier)
		want := SelectionUnique
		if size > MaxResumeCandidates {
			want = SelectionLimit
		}
		if err != nil || got.Status != want || got.Examined != MaxResumeCandidates || calls != 10 {
			t.Fatalf("size %d: %+v, %v, calls %d", size, got, err, calls)
		}
		if want == SelectionLimit && got.Candidate != nil {
			t.Fatal("limited scan selected a candidate")
		}
	}
}

func TestResumeSelectionRejectsInvalidPages(t *testing.T) {
	for _, kind := range []string{"oversized", "duplicate", "unsorted", "empty continuation", "wrong cursor", "wrong identity", "repeated page"} {
		t.Run(kind, func(t *testing.T) {
			states := selectionStates(ResumeDifferent, ResumeMatch)
			page := source.OriginPage{States: states}
			switch kind {
			case "oversized":
				page.States = make([]source.OriginState, source.MaxOriginPageSize+1)
			case "duplicate":
				page.States[1].Origin.ID = page.States[0].Origin.ID
			case "unsorted":
				page.States[0], page.States[1] = page.States[1], page.States[0]
			case "empty continuation":
				page.States, page.NextID = nil, "next"
			case "wrong cursor":
				page.NextID = "wrong"
			case "wrong identity":
				page.States[0].Origin.Device = "other"
			case "repeated page":
				page.NextID = page.States[1].Origin.ID
			}
			reader := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) { return page, nil })
			got, err := scanResume(context.Background(), reader, selectionQuery(), statusVerifier)
			if !errors.Is(err, ErrInvalidOriginPage) || got.Candidate != nil {
				t.Fatalf("selection %+v, error %v", got, err)
			}
		})
	}
}

func TestResumeSelectionCancellationAndErrors(t *testing.T) {
	boom := errors.New("reader failed")
	for _, kind := range []string{"reader", "verify", "between pages", "during final check"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			states := selectionStates(ResumeMatch, ResumeDifferent)
			pages := pagedOrigins(t, states, 1, &calls)
			reader := originReaderFunc(func(ctx context.Context, q source.OriginQuery) (source.OriginPage, error) {
				if kind == "reader" && calls == 1 {
					return source.OriginPage{}, boom
				}
				p, err := pages(ctx, q)
				if kind == "between pages" && calls == 2 {
					cancel()
				}
				return p, err
			})
			verify := func(s source.OriginState) (ResumeCheck, error) {
				if kind == "verify" && s.Origin.ID == states[1].Origin.ID {
					return ResumeCheck{}, boom
				}
				if kind == "during final check" && s.Origin.ID == states[1].Origin.ID {
					cancel()
				}
				return statusVerifier(s)
			}
			got, err := scanResume(ctx, reader, selectionQuery(), verify)
			want := boom
			if kind == "between pages" || kind == "during final check" {
				want = context.Canceled
			}
			if !errors.Is(err, want) || got.Candidate != nil {
				t.Fatalf("selection %+v, error %v", got, err)
			}
		})
	}
}

func TestSelectResumeInputs(t *testing.T) {
	f, _ := testRegularFile(t, "line\n")
	reader := originReaderFunc(func(context.Context, source.OriginQuery) (source.OriginPage, error) {
		t.Fatal("unexpected state query")
		return source.OriginPage{}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SelectResume(ctx, f, "source-1", reader); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := SelectResume(context.Background(), nil, "source-1", reader); err == nil {
		t.Fatal("nil file accepted")
	}
	if _, err := SelectResume(context.Background(), f, "", reader); err == nil {
		t.Fatal("empty source accepted")
	}
	if _, err := SelectResume(context.Background(), f, "source-1", nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	if runtime.GOOS != "linux" {
		got, err := SelectResume(context.Background(), f, "source-1", reader)
		if err != nil || got.Status != SelectionInsufficient || got.Candidate != nil {
			t.Fatalf("unsupported identity: %+v, %v", got, err)
		}
	}
}
