//go:generate goat main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
)

type options struct {
	Version    bool   // Print version information
	Help       bool   // Show help message
	ConfigFile string // Path to the configuration file
}

func main() {
	isFlagExplicitlySet := make(map[string]bool)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `main - 

Usage:
  main [flags]

Flags:
  --version     bool     Print version information
  --help        bool     Show help message
  --config-file string   Path to the configuration file (required)

  -h, --help            Show this help message and exit
`)
	}

	var options = &options{}

	// 1. Create Options with default values.

	options.Version = false

	options.Help = false

	options.ConfigFile = ""

	// 2. Override with environment variable values.

	// 3. Set flags.

	flag.BoolVar(&options.Version, "version", options.Version, "Print version information")

	flag.BoolVar(&options.Help, "help", options.Help, "Show help message")

	flag.StringVar(&options.ConfigFile, "config-file", options.ConfigFile, "Path to the configuration file")

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	// Handle special case for required bools defaulting to true with 'no-<flag>'

	// 5. Perform required checks (excluding booleans).

	// A string is required. It must not be its original default if the flag wasn't set and env var wasn't set.
	// If default was empty: must not be empty.
	// If default was non-empty: must not be that specific non-empty value.
	initialDefaultConfigFile := ""
	envConfigFileWasSet := false

	if options.ConfigFile == initialDefaultConfigFile && !isFlagExplicitlySet["config-file"] && !envConfigFileWasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "config-file", "option", "ConfigFile")
		os.Exit(1)
	}

	// End of range .Options for required checks
	// End of if .HasOptions

	err := run(*options)

	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}

func run(options options) error {
	return json.NewEncoder(os.Stdout).Encode(options)
}
