//go:generate goat main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	// TODO: generate this main function using `goat` tool

	var options options
	flag.BoolVar(&options.Version, "version", false, "Print version information")
	flag.BoolVar(&options.Help, "help", false, "Show help message")
	flag.StringVar(&options.ConfigFile, "config", "", "Path to the configuration file")

	flag.Parse()

	if err := run(options); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type options struct {
	Version    bool   // Print version information
	Help       bool   // Show help message
	ConfigFile string // Path to the configuration file
}

func run(options options) error {
	return json.NewEncoder(os.Stdout).Encode(options)
}
