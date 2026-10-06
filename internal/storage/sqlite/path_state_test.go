package sqlite

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

func TestFileOriginsByPathReopenPaginationAndExactScope(t *testing.T) {
	s, databasePath := openTestStore(t)
	ctx := context.Background()
	when := time.Date(2026, 10, 4, 0, 0, 0, 123, time.UTC)
	path := "/var/log/mail_%'.log"
	origins := []source.Origin{
		{ID: "gen-a", Path: path, Device: "1", Inode: "2", Fingerprint: "prefix-a", FirstSeen: when.Add(2 * time.Hour)},
		{ID: "gen-b", Path: path, Device: "3", Inode: "4", Fingerprint: "prefix-b", FirstSeen: when.Add(time.Hour)},
		{ID: "gen-c", Path: path, Fingerprint: "prefix-c", FirstSeen: when},
		{ID: "other-pattern", Path: "/var/log/mail_Z'.log", Device: "1", Inode: "2", Fingerprint: "pattern", FirstSeen: when},
		{ID: "other-case", Path: strings.ToUpper(path), Device: "1", Inode: "2", Fingerprint: "case", FirstSeen: when},
		{ID: "other-normalization", Path: "/var/log/./mail_%'.log", Device: "1", Inode: "2", Fingerprint: "normalization", FirstSeen: when},
	}
	positions := []source.Position{
		{OriginID: "gen-b", Offset: 0, AnchorHash: "zero-anchor"},
		{OriginID: "gen-c", Offset: 37, AnchorHash: "positive-anchor"},
	}
	if err := s.Commit(ctx, source.Batch{Source: source.Identity{ID: "mail", Kind: "file", Name: "mail"}, Origins: origins, Checkpoints: positions}); err != nil {
		t.Fatal(err)
	}
	foreign := source.Origin{ID: "foreign", Path: path, Device: "1", Inode: "2", Fingerprint: "foreign", FirstSeen: when}
	foreignPosition := source.Position{OriginID: foreign.ID, Offset: 99, AnchorHash: "foreign-anchor"}
	if err := s.Commit(ctx, source.Batch{Source: source.Identity{ID: "other", Kind: "file", Name: "other"}, Origins: []source.Origin{foreign}, Checkpoints: []source.Position{foreignPosition}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var reader source.PathStateReader = reopened
	query := source.OriginPathQuery{SourceID: "mail", Path: path, Limit: 1}
	for i := range 3 {
		page, err := reader.FileOriginsByPath(ctx, query)
		if err != nil || len(page.States) != 1 || !reflect.DeepEqual(page.States[0].Origin, origins[i]) {
			t.Fatalf("page %d metadata: %+v, %v", i, page, err)
		}
		checkpoint := page.States[0].Checkpoint
		if i == 0 {
			if checkpoint != nil {
				t.Fatal("absent checkpoint became present", checkpoint)
			}
		} else if checkpoint == nil || *checkpoint != positions[i-1] {
			t.Fatal("zero/positive checkpoint changed", checkpoint)
		}
		if i < 2 && page.NextID != origins[i].ID || i == 2 && page.NextID != "" {
			t.Fatal("page cursor", i, page.NextID)
		}
		query.AfterID = page.NextID
	}
	query.Limit = source.MaxOriginPageSize
	for _, cursor := range []string{"gen-bZ", "gen-b' OR 1=1 --"} {
		query.AfterID = cursor
		page, err := reader.FileOriginsByPath(ctx, query)
		if err != nil || len(page.States) != 1 || page.States[0].Origin.ID != "gen-c" || page.NextID != "" {
			t.Fatal("exclusive literal cursor need not exist", page, err)
		}
	}
	query.AfterID = "zzzz"
	page, err := reader.FileOriginsByPath(ctx, query)
	if err != nil || len(page.States) != 0 || page.NextID != "" {
		t.Fatal("cursor past last origin", page, err)
	}
	query.SourceID, query.AfterID = "other", ""
	page, err = reader.FileOriginsByPath(ctx, query)
	if err != nil || len(page.States) != 1 || !reflect.DeepEqual(page.States[0].Origin, foreign) || page.States[0].Checkpoint == nil || *page.States[0].Checkpoint != foreignPosition || page.NextID != "" {
		t.Fatal("foreign source isolation", page, err)
	}
	for _, q := range []source.OriginPathQuery{
		{SourceID: "mail' OR 1=1 --", Path: path, Limit: 10},
		{SourceID: "mail", Path: path + "' OR 1=1 --", Limit: 10},
		{SourceID: "mail", Path: "/missing", Limit: 10},
	} {
		page, err := reader.FileOriginsByPath(ctx, q)
		if err != nil || len(page.States) != 0 || page.NextID != "" {
			t.Fatal("missing or literal scope returned origins", page, err)
		}
	}
	// The read contract exposes persisted strings; verification is a later step.
	query.SourceID, query.AfterID = "mail", ""
	page, err = reader.FileOriginsByPath(ctx, query)
	if err != nil || len(page.States) != 3 || page.NextID != "" || page.States[2].Origin.Device != "" {
		t.Fatal("full page imposed physical identity or anchor validation", page, err)
	}
}

func TestFileOriginsByPathRejectsInvalidQueriesAndHonorsCancellation(t *testing.T) {
	s, _ := openTestStore(t)
	query := source.OriginPathQuery{SourceID: "mail", Path: "/var/log/mail.log", Limit: 1}
	for _, invalid := range []source.OriginPathQuery{
		{Path: query.Path, Limit: 1},
		{SourceID: query.SourceID, Limit: 1},
		{SourceID: query.SourceID, Path: query.Path, Limit: 0},
		{SourceID: query.SourceID, Path: query.Path, Limit: -1},
		{SourceID: query.SourceID, Path: query.Path, Limit: source.MaxOriginPageSize + 1},
	} {
		if page, err := s.FileOriginsByPath(context.Background(), invalid); err == nil || !reflect.DeepEqual(page, source.OriginPage{}) {
			t.Fatal("invalid query returned a page", page, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if page, err := s.FileOriginsByPath(ctx, query); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(page, source.OriginPage{}) {
		t.Fatal("cancelled query returned a page", page, err)
	}
}
