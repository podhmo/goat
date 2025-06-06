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

	flag.BoolVar(&options.Version, "version", false, "Print version information")

	flag.BoolVar(&options.Help, "help", false, "Show help message")

	flag.StringVar(&options.ConfigFile, "config-file", "", "Path to the configuration file")

	flag.Parse()

	if options.ConfigFile == "" {
		slog.Error("Missing required flag", "flag", "config-file")
		os.Exit(1)
	}

	err := run(*options)

	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}

func run(options options) error {
	return json.NewEncoder(os.Stdout).Encode(options)
}
