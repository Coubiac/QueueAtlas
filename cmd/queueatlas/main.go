// QueueAtlas provides the command-line entry point for the application.
package main

import (
	"fmt"
	"io"
	"os"
)

// Set by release builds with -ldflags '-X main.version=<label>'. A plain build
// stays explicitly marked as a development build rather than inventing a tag.
var version = "dev"

const usage = "Usage: queueatlas version\n       queueatlas --help\n"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
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
	if _, err := io.WriteString(stdout, output); err != nil {
		fmt.Fprintln(stderr, "queueatlas: cannot write output")
		return 1
	}
	return 0
}
