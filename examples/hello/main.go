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

func main() { /* generated code will replace this */
	var options = &options{}

	flag.BoolVar(&options.Version, "Version", false, "Print version information")
	flag.BoolVar(&options.Help, "Help", false, "Show help message")
	flag.StringVar(&options.ConfigFile, "ConfigFile", "", "Path to the configuration file")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `main - 

Usage:
  main [flags]

Flags:
  --version     bool Print version information (required)
  --help        bool Show help message (required)
  --config-file string Path to the configuration file (required)

  -h, --help   Show this help message and exit
`)
	}

	flag.Parse()

	if options.ConfigFile == "" {
		slog.Error("Missing required flag", "flag", "ConfigFile")
		os.Exit(1)
	}

	if err := run(*options); err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}

func run(options options) error {
	return json.NewEncoder(os.Stdout).Encode(options)
}
