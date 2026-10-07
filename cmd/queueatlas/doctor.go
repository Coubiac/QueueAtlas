package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Coubiac/QueueAtlas/internal/config"
	"github.com/Coubiac/QueueAtlas/internal/storage/sqlite"
)

const doctorUsage = "Usage: queueatlas doctor --config <path>\n"

func doctorCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && isHelp(args[0]) {
		return writeOutput(doctorUsage, stdout, stderr)
	}
	if len(args) != 2 || args[0] != "--config" || strings.TrimSpace(args[1]) == "" {
		fmt.Fprint(stderr, doctorUsage)
		return 2
	}
	c, err := config.Load(args[1])
	if err != nil {
		if errors.Is(err, config.ErrInvalid) {
			fmt.Fprintf(stderr, "queueatlas: %v\n", err)
			return 2
		}
		fmt.Fprintln(stderr, "queueatlas: cannot read configuration")
		return 1
	}
	// Only the database phase has a cooperative timeout. Configuration and
	// filesystem IO are not covered by a hard deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	d, err := sqlite.OpenDiagnostics(ctx, c.Storage.Path)
	if err != nil {
		return dbFailure(err, "cannot open database for diagnostics", stderr)
	}
	if err := d.Close(); err != nil {
		fmt.Fprintln(stderr, "queueatlas: cannot close database diagnostic connection")
		return 1
	}
	// These fixed statuses attest only to the loader and schema compatibility
	// checks. Do not imply database integrity or service/deployment readiness.
	return writeOutput("{\"configuration\":\"valid\",\"database\":\"compatible\"}\n", stdout, stderr)
}
