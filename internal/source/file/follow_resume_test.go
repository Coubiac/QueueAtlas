package file

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Coubiac/mailtrace/internal/source"
)

func TestPrepareFollowResumeValidatesBeforeStateOrFilesystem(t *testing.T) {
	for _, kind := range []string{"ID", "kind", "name", "relative", "reader", "normalizer", "origins zero", "origins max", "entries zero", "entries max", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			identity, path := fileSourceIdentity(), filepath.Join(t.TempDir(), "absent", "mail.log")
			reader := source.PathStateReader(pathReaderFunc(func(context.Context, source.OriginPathQuery) (source.OriginPage, error) {
				t.Fatal("invalid preparation read state")
				return source.OriginPage{}, nil
			}))
			normalize, limits := Normalize(testNormalizer), FollowResumeLimits{Origins: 1, Entries: 1}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "ID":
				identity.ID = ""
			case "kind":
				identity.Kind = "syslog"
			case "name":
				identity.Name = ""
			case "relative":
				path = "mail.log"
			case "reader":
				reader = nil
			case "normalizer":
				normalize = nil
			case "origins zero":
				limits.Origins = 0
			case "origins max":
				limits.Origins = MaxPathOrigins + 1
			case "entries zero":
				limits.Entries = 0
			case "entries max":
				limits.Entries = MaxFollowLocationEntries + 1
			case "canceled":
				cancel()
			}
			result, err := PrepareFollowResume(ctx, identity, path, reader, normalize, limits)
			if err == nil || result != (FollowResume{}) || kind == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatal("invalid preparation", result, err)
			}
		})
	}
}

func TestPrepareFollowResumeClassifiesBeforeFilesystem(t *testing.T) {
	for _, kind := range []string{"empty", "retired", "unknown", "invalid", "capacity", "limit", "reader error", "reader cancellation"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "directory-not-created", "mail.log")
			states := pathOriginStates(3)
			for i := range states {
				states[i].Origin.Path = path
				states[i].FollowState = source.FollowFollowing
			}
			limits := FollowResumeLimits{Origins: 3, Entries: 1}
			boom := errors.New("synthetic state read error")
			var want error
			absent := false
			switch kind {
			case "empty":
				states = nil
				absent = true
			case "retired":
				for i := range states {
					states[i].FollowState = source.FollowRetired
				}
				absent = true
			case "unknown":
				states[2].FollowState = source.FollowUnknown
				want = ErrUnknownFollowState
			case "invalid":
				states[2].FollowState = 3
				want = ErrInvalidFollowState
			case "capacity":
				want = ErrRotationCapacity
			case "limit":
				limits.Origins = 2
			case "reader error":
				want = boom
			case "reader cancellation":
				want = context.Canceled
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			reader := pathReaderFunc(func(ctx context.Context, q source.OriginPathQuery) (source.OriginPage, error) {
				calls++
				if q.SourceID != fileSourceIdentity().ID || q.Path != path || q.Limit != limits.Origins || q.AfterID != "" {
					t.Fatal("incorrect preparation query", q)
				}
				if kind == "reader error" {
					return source.OriginPage{}, boom
				}
				if kind == "reader cancellation" {
					cancel()
					return source.OriginPage{}, nil
				}
				page := source.OriginPage{States: states[:min(len(states), q.Limit)]}
				if len(states) > q.Limit {
					page.NextID = page.States[len(page.States)-1].Origin.ID
				}
				return page, nil
			})
			result, err := PrepareFollowResume(ctx, fileSourceIdentity(), path, reader, testNormalizer, limits)
			if calls != 1 {
				t.Fatal("unexpected state page count", calls)
			}
			if absent {
				if err != nil || result != (FollowResume{Status: FollowResumeAbsent}) {
					t.Fatal("absent opened filesystem or returned a set", result, err)
				}
				return
			}
			if result != (FollowResume{}) {
				t.Fatal("blocked preparation returned partial result", result)
			}
			if kind == "limit" {
				var decision *ResumeDecisionError
				if !errors.As(err, &decision) || decision.Status != SelectionLimit {
					t.Fatal("state budget diagnostic", err)
				}
				return
			}
			if !errors.Is(err, want) {
				t.Fatal("state diagnostic", err, want)
			}
		})
	}
}
