package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/podhmo/goat"
)

//go:generate ../../goat_temp_tool emit -run RunWithConvention main.go
//go:generate ../../goat_temp_tool emit -run RunWithoutAnyInit main.go

type OptionsWithConvention struct {
	Message string
	Count   int
}

// NewOptionsWithConvention is a conventional initializer
func NewOptionsWithConvention() *OptionsWithConvention {
	return &OptionsWithConvention{
		Message: goat.Default("Conventional Hello"),
		Count:   goat.Default(100),
	}
}

func RunWithConvention(opts OptionsWithConvention) error {
	fmt.Printf("RunWithConvention: %s, %d\n", opts.Message, opts.Count)
	return nil
}

type OptionsWithoutAnyInit struct {
	Name    string
	Value   float64
	Enabled bool
}

// No initializer for OptionsWithoutAnyInit

func RunWithoutAnyInit(opts OptionsWithoutAnyInit) error {
	fmt.Printf("RunWithoutAnyInit: %s, %f, %t\n", opts.Name, opts.Value, opts.Enabled)
	return nil
}

func main() {
	isFlagExplicitlySet := make(map[string]bool)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `examples/noinit -

Usage:
  examples/noinit [flags]

Flags:
  --name      string    (required)
  --value     float64   (required)
  --enabled   bool

  -h, --help          Show this help message and exit
`)
	}

	var options *OptionsWithoutAnyInit

	// 1. Create Options with default values (no initializer function provided).
	options = new(OptionsWithoutAnyInit) // options is now a valid pointer to a zeroed struct

	// The following block populates the fields of the options struct.
	// This logic is only executed if no InitializerFunc is provided.

	options.Name = ""

	options.Enabled = false

	// End of range .Options (for non-initializer case)
	// End of if/else .RunFunc.InitializerFunc for options assignment

	// 2. Override with environment variable values.
	// This section assumes 'options' is already initialized.

	// End of range .Options for env vars

	// 3. Set flags.

	flag.StringVar(&options.Name, "name", options.Name, "")

	flag.BoolVar(&options.Enabled, "enabled", options.Enabled, "")

	// End of range .Options for flags

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	// Handle special case for required bools defaulting to true with 'no-<flag>'

	// 5. Perform required checks (excluding booleans).

	initialDefaultName := ""
	envNameWasSet := false

	if options.Name == initialDefaultName && !isFlagExplicitlySet["name"] && !envNameWasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "name", "option", "Name")
		os.Exit(1)
	}

	// End of range .Options for required checks

	// TODO: Implement runtime validation for file options based on metadata:
	// - Check for opt.FileMustExist (e.g., using os.Stat)
	// - Handle opt.FileGlobPattern (e.g., using filepath.Glob)
	// Currently, these attributes are parsed but not enforced at runtime by the generated CLI.
	// End of if .RunFunc.OptionsArgTypeNameStripped (options handling block)

	var err error

	// Run function expects an options argument
	err = RunWithoutAnyInit(*options)

	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}
