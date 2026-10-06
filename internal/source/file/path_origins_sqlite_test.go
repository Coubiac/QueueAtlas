package file

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/source"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

func TestLoadPathOriginsWithSQLiteAndReducedFinalPage(t *testing.T) {
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	states := pathOriginStates(source.MaxOriginPageSize + 1)
	batch := source.Batch{Source: source.Identity{ID: "mail", Kind: "file", Name: "synthetic"}}
	for _, state := range states {
		state.Origin.FirstSeen = time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
		batch.Origins = append(batch.Origins, state.Origin)
		batch.Checkpoints = append(batch.Checkpoints, *state.Checkpoint)
	}
	if err := store.Commit(ctx, batch); err != nil {
		t.Fatal(err)
	}
	result, err := LoadPathOrigins(ctx, "mail", "/synthetic/mail.log", store, len(states))
	if err != nil || result.Status != PathOriginsComplete || result.Examined != len(states) || len(result.States) != len(states) {
		t.Fatal("SQLite completed scan", result.Status, result.Examined, err)
	}
	for i, state := range result.States {
		if state.Origin.ID != states[i].Origin.ID || state.Checkpoint == nil || *state.Checkpoint != *states[i].Checkpoint {
			t.Fatal("SQLite scan lost or changed a checkpoint", i, state)
		}
	}
	result, err = LoadPathOrigins(ctx, "mail", "/synthetic/mail.log", store, len(states)-1)
	if err != nil || result.Status != PathOriginsLimit || result.Examined != len(states)-1 || result.States != nil {
		t.Fatal("SQLite incomplete scan became usable", result.Status, result.Examined, err)
	}
}
