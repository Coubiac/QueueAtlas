package file

import (
	"context"
	"errors"
	"testing"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestAcquisitionRejectsInvalidStateAndHonorsCancellation(t *testing.T) {
	s := &FileSource{config: Config{Identity: fileSourceIdentity()}}
	sink := sinkFunc(func(context.Context, source.Batch) error {
		t.Fatal("unexpected acquisition commit")
		return nil
	})
	for _, state := range []source.FollowState{-1, 3} {
		if err := s.acquireGeneration(context.Background(), source.OriginState{FollowState: state}, sink); !errors.Is(err, ErrInvalidFollowState) {
			t.Fatal("invalid state accepted", err)
		}
	}
	if err := s.acquireGeneration(context.Background(), source.OriginState{FollowState: source.FollowFollowing}, sink); err != nil {
		t.Fatal("durable acquisition not reused", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, state := range []source.FollowState{source.FollowUnknown, source.FollowFollowing, source.FollowRetired} {
		if err := s.acquireGeneration(ctx, source.OriginState{FollowState: state}, sink); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation ignored", err)
		}
	}
}
