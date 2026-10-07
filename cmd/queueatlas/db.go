package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/config"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

const dbUsage = "Usage: queueatlas db stats --config <path>\n"

// Keep the output contract separate from the storage struct so adding a library
// field never silently expands what this diagnostic prints.
type dbStatsOutput struct {
	SchemaVersion int    `json:"schema_version"`
	SQLiteVersion string `json:"sqlite_version"`
	JournalMode   string `json:"journal_mode"`
	PageSize      int64  `json:"page_size"`
	PageCount     int64  `json:"page_count"`
	FreePageCount int64  `json:"free_page_count"`
}

func dbCommand(args []string, stdout, stderr io.Writer) int {
	if (len(args) == 1 && isHelp(args[0])) ||
		(len(args) == 2 && args[0] == "stats" && isHelp(args[1])) {
		return writeOutput(dbUsage, stdout, stderr)
	}
	if len(args) != 3 || args[0] != "stats" || args[1] != "--config" || strings.TrimSpace(args[2]) == "" {
		fmt.Fprint(stderr, dbUsage)
		return 2
	}
	c, err := config.Load(args[2])
	if err != nil {
		if errors.Is(err, config.ErrInvalid) {
			fmt.Fprintf(stderr, "queueatlas: %v\n", err)
			return 2
		}
		fmt.Fprintln(stderr, "queueatlas: cannot read configuration")
		return 1
	}
	// Bound the database phase cooperatively, including pool/SQLite waits. This
	// does not impose a hard deadline on configuration or filesystem IO.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	d, err := sqlite.OpenDiagnostics(ctx, c.Storage.Path)
	if err != nil {
		return dbFailure(err, "cannot open database for diagnostics", stderr)
	}
	m, readErr := d.Metadata(ctx)
	closeErr := d.Close()
	if readErr != nil {
		return dbFailure(readErr, "cannot read database diagnostic metadata", stderr)
	}
	if closeErr != nil {
		fmt.Fprintln(stderr, "queueatlas: cannot close database diagnostic connection")
		return 1
	}
	output, err := json.Marshal(dbStatsOutput{
		SchemaVersion: m.SchemaVersion, SQLiteVersion: m.SQLiteVersion,
		JournalMode: m.JournalMode, PageSize: m.PageSize,
		PageCount: m.PageCount, FreePageCount: m.FreePageCount,
	})
	if err != nil {
		fmt.Fprintln(stderr, "queueatlas: cannot encode database diagnostic metadata")
		return 1
	}
	return writeOutput(string(output)+"\n", stdout, stderr)
}

func isHelp(arg string) bool { return arg == "--help" || arg == "-h" }

func dbFailure(err error, fallback string, stderr io.Writer) int {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		fmt.Fprintln(stderr, "queueatlas: database diagnostic timed out")
	case errors.Is(err, sqlite.ErrDiagnosticsSchema):
		fmt.Fprintln(stderr, "queueatlas: database schema is not supported for diagnostics")
	default:
		fmt.Fprintln(stderr, "queueatlas: "+fallback)
	}
	return 1
}
