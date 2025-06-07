//go:generate goat main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

// Options defines the command line options.
type Options struct {
	Version    bool   // Print version information
	Help       bool   // Show help message
	ConfigFile string // Path to the configuration file
}

func main() {
	isFlagExplicitlySet := make(map[string]bool)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `m/hello - run is the actual command logic.

Usage:
  m/hello [flags]

Flags:
  --version     bool     Print version information
  --help        bool     Show help message
  --config-file string   Path to the configuration file (required)

  -h, --help            Show this help message and exit
`)
	}

	var options *Options

	// 1. Create Options with default values (no initializer function provided).
	options = new(Options) // options is now a valid pointer to a zeroed struct

	// The following block populates the fields of the options struct.
	// This logic is only executed if no InitializerFunc is provided.

	options.Version = false

	options.Help = false

	options.ConfigFile = ""

	// End of range .Options (for non-initializer case)
	// End of if/else .RunFunc.InitializerFunc for options assignment

	// 2. Override with environment variable values.
	// This section assumes 'options' is already initialized.

	// End of range .Options for env vars

	// 3. Set flags.

	flag.BoolVar(&options.Version, "version", options.Version, "Print version information")

	flag.BoolVar(&options.Help, "help", options.Help, "Show help message")

	flag.StringVar(&options.ConfigFile, "config-file", options.ConfigFile, "Path to the configuration file")

	// End of range .Options for flags

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	// Handle special case for required bools defaulting to true with 'no-<flag>'

	// 5. Perform required checks (excluding booleans).

	initialDefaultConfigFile := ""
	envConfigFileWasSet := false

	if options.ConfigFile == initialDefaultConfigFile && !isFlagExplicitlySet["config-file"] && !envConfigFileWasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "config-file", "option", "ConfigFile")
		os.Exit(1)
	}

	// End of range .Options for required checks
	// End of if .RunFunc.OptionsArgTypeNameStripped (options handling block)

	var err error

	// Run function expects an options argument
	err = run(*options)

	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}

// run is the actual command logic.
func run(opts Options) error { // Parameter type changed to Options, name to opts
	return json.NewEncoder(os.Stdout).Encode(opts)
}
