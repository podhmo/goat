package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/podhmo/goat"
)

//go:generate goat emit -run run -initializer newOptions main.go

// Options defines the command line options for this simple example tool.
// This tool demonstrates the basic capabilities of goat for CLI generation.
type Options struct {
	// Name of the person to greet. This is a mandatory field.
	Name string `env:"SIMPLE_NAME"`

	// Age of the person. This is an optional field.
	Age *int `env:"SIMPLE_AGE"`

	// LogLevel for the application output.
	// It can be one of: debug, info, warning, error.
	LogLevel string `env:"SIMPLE_LOG_LEVEL"`

	// Features to enable, provided as a comma-separated list.
	// Example: --features feat1,feat2
	Features []string `env:"SIMPLE_FEATURES"`

	// OutputDir for any generated files or reports.
	// Defaults to "output" if not specified by the user.
	OutputDir string

	// Mode of operation for the tool, affecting its behavior.
	Mode string `env:"SIMPLE_MODE"`

	// Enable extra verbose output.
	SuperVerbose bool `env:"SIMPLE_SUPER_VERBOSE"`
}

// newOptions initializes SimpleOptions with default values and enum constraints.
// This function will be "interpreted" by the goat tool.
func newOptions() *Options {
	return &Options{
		Name:      goat.Default("World"),
		LogLevel:  goat.Default("info", goat.Enum([]string{"debug", "info", "warning", "error"})),
		OutputDir: goat.Default("output"),
		Mode:      goat.Enum([]string{"standard", "turbo", "eco"}),
		// Age is optional (pointer) and has no default here.
		// Features is a slice, will be handled by flag package (e.g. multiple --features flag or comma sep)
		// SuperVerbose is a bool, defaults to false (zero value for bool)
	}
}

// run is the core logic for this simple CLI tool.
// It receives the parsed and validated options.
// This function's doc comment is used as the main help text for the command.
func run(opts Options) error {
	fmt.Printf("Hello, %s!\n", opts.Name)

	if opts.Age != nil {
		fmt.Printf("You are %d years old.\n", *opts.Age)
	} else {
		fmt.Println("Your age was not provided.")
	}

	fmt.Printf("Log Level: %s\n", opts.LogLevel)
	fmt.Printf("Output Directory: %s\n", opts.OutputDir)
	fmt.Printf("Mode: %s\n", opts.Mode)

	if len(opts.Features) > 0 {
		fmt.Printf("Enabled features: %v\n", opts.Features)
	} else {
		fmt.Println("No special features enabled.")
	}

	if opts.SuperVerbose {
		fmt.Println("Super verbose mode is ON!")
	}

	if opts.Name == "ErrorTrigger" {
		return fmt.Errorf("the name 'ErrorTrigger' is not allowed")
	}

	return nil
}

func main() {
	isFlagExplicitlySet := make(map[string]bool)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `main - run is the core logic for this simple CLI tool.
         It receives the parsed and validated options.
         This function`+"`"+`s doc comment is used as the main help text for the command.

Usage:
  main [flags]

Flags:
  --name          string   Name of the person to greet. This is a mandatory field. (required) (default: "World") (env: SIMPLE_NAME)
  --age           int      Age of the person. This is an optional field. (env: SIMPLE_AGE)
  --log-level     string   LogLevel for the application output.
                         It can be one of: debug, info, warning, error. (required) (default: "info") (env: SIMPLE_LOG_LEVEL) (allowed: "debug", "info", "warning", "error")
  --features      strings  Features to enable, provided as a comma-separated list.
                         Example: --features feat1,feat2 (required) (env: SIMPLE_FEATURES)
  --output-dir    string   OutputDir for any generated files or reports.
                         Defaults to "output" if not specified by the user. (required) (default: "output")
  --mode          string   Mode of operation for the tool, affecting its behavior. (required) (env: SIMPLE_MODE) (allowed: "standard", "turbo", "eco")
  --super-verbose bool     Enable extra verbose output. (env: SIMPLE_SUPER_VERBOSE)

  -h, --help              Show this help message and exit
`)
	}

	var options = &Options{}

	var defaultName string = "World"

	if val, ok := os.LookupEnv("SIMPLE_NAME"); ok {
		defaultName = val
	}

	flag.StringVar(&options.Name, "name", defaultName, "Name of the person to greet. This is a mandatory field." /* Original Default: World, Env: SIMPLE_NAME */)

	options.Age = new(int)

	flag.IntVar(options.Age, "age", 0, "Age of the person. This is an optional field.")

	var defaultLogLevel string = "info"

	if val, ok := os.LookupEnv("SIMPLE_LOG_LEVEL"); ok {
		defaultLogLevel = val
	}

	flag.StringVar(&options.LogLevel, "log-level", defaultLogLevel, `LogLevel for the application output.
It can be one of: debug, info, warning, error.` /* Original Default: info, Env: SIMPLE_LOG_LEVEL */)

	var defaultOutputDir string = "output"

	flag.StringVar(&options.OutputDir, "output-dir", defaultOutputDir, `OutputDir for any generated files or reports.
Defaults to "output" if not specified by the user.` /* Original Default: output, Env:  */)

	var defaultMode string = ""

	if val, ok := os.LookupEnv("SIMPLE_MODE"); ok {
		defaultMode = val
	}

	flag.StringVar(&options.Mode, "mode", defaultMode, "Mode of operation for the tool, affecting its behavior." /* Env: SIMPLE_MODE */)

	var defaultSuperVerbose bool = false

	if val, ok := os.LookupEnv("SIMPLE_SUPER_VERBOSE"); ok {
		if v, err := strconv.ParseBool(val); err == nil {
			defaultSuperVerbose = v
		} else {
			slog.Warn("Could not parse environment variable as bool for default value", "envVar", "SIMPLE_SUPER_VERBOSE", "value", val, "error", err)
		}
	}

	flag.BoolVar(&options.SuperVerbose, "super-verbose", defaultSuperVerbose, "Enable extra verbose output." /* Env: SIMPLE_SUPER_VERBOSE */)

	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	if !isFlagExplicitlySet["age"] {
		if val, ok := os.LookupEnv("SIMPLE_AGE"); ok {

			if v, err := strconv.Atoi(val); err == nil {
				*options.Age = v
			} else {
				slog.Warn("Could not parse environment variable as *int", "envVar", "SIMPLE_AGE", "value", val, "error", err)
			}

		}
	}

	if options.Name == "" {
		slog.Error("Missing required flag", "flag", "name", "envVar", "SIMPLE_NAME")
		os.Exit(1)
	}

	if options.LogLevel == "" {
		slog.Error("Missing required flag", "flag", "log-level", "envVar", "SIMPLE_LOG_LEVEL")
		os.Exit(1)
	}

	isValidChoice_LogLevel := false
	allowedChoices_LogLevel := []string{"debug", "info", "warning", "error"}
	currentValue_LogLevelStr := fmt.Sprintf("%v", options.LogLevel)
	for _, choice := range allowedChoices_LogLevel {
		if currentValue_LogLevelStr == choice {
			isValidChoice_LogLevel = true
			break
		}
	}
	if !isValidChoice_LogLevel {
		slog.Error("Invalid value for flag", "flag", "log-level", "value", options.LogLevel, "allowedChoices", strings.Join(allowedChoices_LogLevel, ", "))
		os.Exit(1)
	}

	if options.OutputDir == "" {
		slog.Error("Missing required flag", "flag", "output-dir")
		os.Exit(1)
	}

	if options.Mode == "" {
		slog.Error("Missing required flag", "flag", "mode", "envVar", "SIMPLE_MODE")
		os.Exit(1)
	}

	isValidChoice_Mode := false
	allowedChoices_Mode := []string{"standard", "turbo", "eco"}
	currentValue_ModeStr := fmt.Sprintf("%v", options.Mode)
	for _, choice := range allowedChoices_Mode {
		if currentValue_ModeStr == choice {
			isValidChoice_Mode = true
			break
		}
	}
	if !isValidChoice_Mode {
		slog.Error("Invalid value for flag", "flag", "mode", "value", options.Mode, "allowedChoices", strings.Join(allowedChoices_Mode, ", "))
		os.Exit(1)
	}

	err := run(*options)

	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}
