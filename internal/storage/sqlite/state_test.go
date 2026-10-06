package sqlite

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestFileOriginsAbsentState(t *testing.T) {
	s, _ := openTestStore(t)
	var reader source.StateReader = s
	page, err := reader.FileOrigins(context.Background(), source.OriginQuery{SourceID: "mail", Device: "1", Inode: "2", Limit: 1})
	if err != nil || len(page.States) != 0 || page.NextID != "" {
		t.Fatalf("absent origins: %+v, %v", page, err)
	}
	if p, ok, err := reader.Checkpoint(context.Background(), "mail", "missing"); err != nil || ok || p != (source.Position{}) {
		t.Fatalf("absent checkpoint: %+v, %v, %v", p, ok, err)
	}
}

func TestFileOriginsReopenPaginationAndSourceIsolation(t *testing.T) {
	s, path := openTestStore(t)
	ctx := context.Background()
	when := time.Date(2026, 10, 3, 12, 0, 0, 123, time.UTC)
	origins := []source.Origin{
		{ID: "gen-a", Path: "/var/log/mail.log.2", Device: "1", Inode: "2", Fingerprint: "prefix-a", FirstSeen: when},
		{ID: "gen-b", Path: "/var/log/mail.log.1", Device: "1", Inode: "2", Fingerprint: "prefix-b", FirstSeen: when.Add(time.Hour)},
		{ID: "gen-c", Path: "/var/log/mail.log", Device: "1", Inode: "2", Fingerprint: "prefix-c", FirstSeen: when.Add(2 * time.Hour)},
		{ID: "other-device", Path: "/var/log/other.log", Device: "9", Inode: "2", Fingerprint: "other", FirstSeen: when},
		{ID: "other-inode", Path: "/var/log/another.log", Device: "1", Inode: "9", Fingerprint: "another", FirstSeen: when},
	}
	positions := []source.Position{
		{OriginID: "gen-b", Offset: 0, AnchorHash: "zero-anchor"},
		{OriginID: "gen-c", Offset: 37, AnchorHash: "tail-anchor"},
	}
	if err := s.Commit(ctx, source.Batch{
		Source: source.Identity{ID: "mail", Kind: "file", Name: "mail"}, Origins: origins, Checkpoints: positions,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Commit(ctx, source.Batch{
		Source:      source.Identity{ID: "other", Kind: "file", Name: "other"},
		Origins:     []source.Origin{{ID: "foreign-origin", Path: "/var/log/foreign.log", Device: "1", Inode: "2", Fingerprint: "foreign", FirstSeen: when}},
		Checkpoints: []source.Position{{OriginID: "foreign-origin", Offset: 99, AnchorHash: "foreign-anchor"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var reader source.StateReader = reopened
	query := source.OriginQuery{SourceID: "mail", Device: "1", Inode: "2", Limit: 1}
	for i := range 3 {
		page, err := reader.FileOrigins(ctx, query)
		if err != nil || len(page.States) != 1 {
			t.Fatalf("page %d: %+v, %v", i, page, err)
		}
		state := page.States[0]
		if !reflect.DeepEqual(state.Origin, origins[i]) {
			t.Fatalf("origin metadata changed after reopen: %+v, want %+v", state.Origin, origins[i])
		}
		if i == 0 {
			if state.Checkpoint != nil {
				t.Fatalf("origin without checkpoint acquired one: %+v", state.Checkpoint)
			}
		} else if state.Checkpoint == nil || *state.Checkpoint != positions[i-1] {
			t.Fatalf("checkpoint %d: %+v, want %+v", i, state.Checkpoint, positions[i-1])
		}
		if i < 2 && page.NextID != origins[i].ID || i == 2 && page.NextID != "" {
			t.Fatalf("page %d next cursor = %q", i, page.NextID)
		}
		query.AfterID = page.NextID
	}
	// A foreign origin ID must not leak its position through another source.
	if p, ok, err := reader.Checkpoint(ctx, "mail", "foreign-origin"); err != nil || ok || p != (source.Position{}) {
		t.Fatalf("foreign checkpoint leaked: %+v, %v, %v", p, ok, err)
	}
	query = source.OriginQuery{SourceID: "other", Device: "1", Inode: "2", Limit: 10}
	page, err := reader.FileOrigins(ctx, query)
	if err != nil || len(page.States) != 1 || page.States[0].Origin.ID != "foreign-origin" || page.States[0].Checkpoint == nil || page.States[0].Checkpoint.Offset != 99 || page.NextID != "" {
		t.Fatalf("other source's page: %+v, %v", page, err)
	}
}

func TestFileOriginsRejectInvalidQueriesAndHonorCancellation(t *testing.T) {
	s, _ := openTestStore(t)
	query := source.OriginQuery{SourceID: "mail", Device: "1", Inode: "2", Limit: 1}
	for _, invalid := range []source.OriginQuery{
		{Device: "1", Inode: "2", Limit: 1},
		{SourceID: "mail", Inode: "2", Limit: 1},
		{SourceID: "mail", Device: "1", Limit: 1},
		{SourceID: "mail", Device: "1", Inode: "2", Limit: 0},
		{SourceID: "mail", Device: "1", Inode: "2", Limit: source.MaxOriginPageSize + 1},
	} {
		if _, err := s.FileOrigins(context.Background(), invalid); err == nil {
			t.Fatalf("invalid query accepted: %+v", invalid)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.FileOrigins(ctx, query); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query: %v", err)
	}
}
