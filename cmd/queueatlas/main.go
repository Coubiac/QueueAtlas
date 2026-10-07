// QueueAtlas provides the command-line entry point for the application.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Coubiac/QueueAtlas/internal/config"
)

// Set by release builds with -ldflags '-X main.version=<label>'. A plain build
// stays explicitly marked as a development build rather than inventing a tag.
var version = "dev"

const usage = "Usage: queueatlas version\n       queueatlas check-config --config <path>\n       queueatlas db stats --config <path>\n       queueatlas --help\n"
const checkConfigUsage = "Usage: queueatlas check-config --config <path>\n"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "db" {
		return dbCommand(args[1:], stdout, stderr)
	}
	if len(args) > 0 && args[0] == "check-config" {
		return checkConfig(args[1:], stdout, stderr)
	}
	if len(args) != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var output string
	switch args[0] {
	case "version":
		output = fmt.Sprintf("QueueAtlas %s\n", version)
	case "--help", "-h":
		output = usage
	default:
		// Do not echo arbitrary arguments into diagnostics.
		fmt.Fprint(stderr, usage)
		return 2
	}
	return writeOutput(output, stdout, stderr)
}

func checkConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		return writeOutput(checkConfigUsage, stdout, stderr)
	}
	if len(args) != 2 || args[0] != "--config" || strings.TrimSpace(args[1]) == "" {
		fmt.Fprint(stderr, checkConfigUsage)
		return 2
	}
	if _, err := config.Load(args[1]); err != nil {
		if errors.Is(err, config.ErrInvalid) {
			// The loader's field/rule diagnostics contain no supplied values.
			fmt.Fprintf(stderr, "queueatlas: %v\n", err)
			return 2
		}
		fmt.Fprintln(stderr, "queueatlas: cannot read configuration")
		return 1
	}
	return writeOutput("Configuration valid\n", stdout, stderr)
}

func writeOutput(output string, stdout, stderr io.Writer) int {
	if _, err := io.WriteString(stdout, output); err != nil {
		fmt.Fprintln(stderr, "queueatlas: cannot write output")
		return 1
	}
	return 0
}
