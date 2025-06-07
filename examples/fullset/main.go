package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"net"

	"github.com/podhmo/goat"
)

// stringPtr is a helper function to get a pointer to a string.
func stringPtr(s string) *string {
	return &s
}

//go:generate goat emit -run FullsetRun -initializer NewFullsetOptions main.go

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

	// An optional boolean flag with no default, should be nil if not set.
	OptionalToggle *bool
	// Path to a configuration file. Must exist. (env:"FULLSET_CONFIG_FILE")
	ConfigFile string `env:"FULLSET_CONFIG_FILE"`
	// A glob pattern for input files. (env:"FULLSET_PATTERN")
	Pattern string `env:"FULLSET_PATTERN"`
	// Enable Feature X by default. Use --no-enable-feature-x to disable. (env:"FULLSET_FEATURE_X")
	EnableFeatureX bool `env:"FULLSET_FEATURE_X"`
	// The host IP address for the service. (env:"FULLSET_HOST_IP")
	HostIP net.IP `env:"FULLSET_HOST_IP"`
	// Example of an existing field made optional. (env:"FULLSET_OPTIONAL_EXISTING")
	ExistingFieldToMakeOptional *string `env:"FULLSET_OPTIONAL_EXISTING"`
}

// NewFullsetOptions initializes Options with default values and enum constraints.
// This function will be "interpreted" by the goat tool.
func NewFullsetOptions() *Options {
	return &Options{
		// Existing fields from original
		Name:      goat.Default("World"),
		LogLevel:  goat.Default("info", goat.Enum([]string{"debug", "info", "warning", "error"})),
		OutputDir: goat.Default("output"),
		Mode:      goat.Default("standard", goat.Enum([]string{"standard", "turbo", "eco"})),
		// Age is optional (pointer) and has no default here. It remains *int.
		// Features is []string, handled by flag package. Env var should work.
		// SuperVerbose is a bool, defaults to false (zero value for bool).

		// New fields from first subtask, adapted for current subtask
		// OptionalToggle is *bool and should not have goat.Default()
		// TODO: The goat tool fails to parse the following line for ConfigFile.
		// TODO: It reports an error: "in call to goat.Default, type string of goat.File(...) does not match []T (cannot infer T)".
		// TODO: This suggests an issue with how goat's parser handles goat.File when nested in goat.Default, potentially misinterpreting goat.File's return type.
		ConfigFile: goat.Default("config.json", goat.File("config.json", goat.MustExist())),
		// TODO: The goat tool fails to parse the following line for Pattern.
		// TODO: It reports an error: "in call to goat.Default, type string of goat.File(...) does not match []T (cannot infer T)".
		// TODO: This suggests an issue with how goat's parser handles goat.File when nested in goat.Default, potentially misinterpreting goat.File's return type.
		Pattern:    goat.Default("*.go", goat.File("*.go", goat.GlobPattern())),
		EnableFeatureX: goat.Default(true),
		HostIP:     goat.Default(net.ParseIP("127.0.0.1")),
		ExistingFieldToMakeOptional: goat.Default(stringPtr("was set by default")),
	}
}

// FullsetRun is the core logic for this CLI tool.
// It receives the parsed and validated options.
// This function's doc comment is used as the main help text for the command.
func FullsetRun(opts Options) error {
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

	// Print new fields from first subtask
	if opts.OptionalToggle != nil {
		fmt.Printf("OptionalToggle: %t\n", *opts.OptionalToggle)
	} else {
		fmt.Println("OptionalToggle: not set")
	}
	fmt.Printf("ConfigFile: %s\n", opts.ConfigFile)
	fmt.Printf("Pattern: %s\n", opts.Pattern)
	fmt.Printf("EnableFeatureX: %t\n", opts.EnableFeatureX)
	fmt.Printf("Host IP: %s\n", opts.HostIP)
	if opts.ExistingFieldToMakeOptional != nil {
		fmt.Printf("ExistingFieldToMakeOptional: %s\n", *opts.ExistingFieldToMakeOptional)
	} else {
		fmt.Println("ExistingFieldToMakeOptional: not set")
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
  --name          string   Name of the person to greet. This is a mandatory field. (default: "World") (env: SIMPLE_NAME)
  --age           int      Age of the person. This is an optional field. (env: SIMPLE_AGE)
  --log-level     string   LogLevel for the application output.
                         It can be one of: debug, info, warning, error. (default: "info") (env: SIMPLE_LOG_LEVEL) (allowed: "debug", "info", "warning", "error")
  --features      strings  Features to enable, provided as a comma-separated list.
                         Example: --features feat1,feat2 (required) (env: SIMPLE_FEATURES)
  --output-dir    string   OutputDir for any generated files or reports.
                         Defaults to "output" if not specified by the user. (default: "output")
  --mode          string   Mode of operation for the tool, affecting its behavior. (required) (env: SIMPLE_MODE) (allowed: "standard", "turbo", "eco")
  --super-verbose bool     Enable extra verbose output. (env: SIMPLE_SUPER_VERBOSE)

  -h, --help              Show this help message and exit
`)
	}

	var options = &Options{}

	// 1. Create Options with default values.

	options.Name = "World"

	options.Age = new(int)

	options.LogLevel = "info"

	options.OutputDir = "output"

	options.Mode = ""

	options.SuperVerbose = false

	// 2. Override with environment variable values.

	if val, ok := os.LookupEnv("SIMPLE_NAME"); ok {

		options.Name = val

	}

	if val, ok := os.LookupEnv("SIMPLE_AGE"); ok {

		if options.Age == nil {
			options.Age = new(int)
		}
		if v, err := strconv.Atoi(val); err == nil {
			*options.Age = v
		} else {
			slog.Warn("Could not parse environment variable as *int for option", "envVar", "SIMPLE_AGE", "option", "Age", "value", val, "error", err)
		}

	}

	if val, ok := os.LookupEnv("SIMPLE_LOG_LEVEL"); ok {

		options.LogLevel = val

	}

	if val, ok := os.LookupEnv("SIMPLE_FEATURES"); ok {
		options.Features = strings.Split(val, ",")
	}

	if val, ok := os.LookupEnv("SIMPLE_MODE"); ok {

		options.Mode = val

	}

	if val, ok := os.LookupEnv("SIMPLE_SUPER_VERBOSE"); ok {

		if v, err := strconv.ParseBool(val); err == nil {
			options.SuperVerbose = v
		} else {
			slog.Warn("Could not parse environment variable as bool for option", "envVar", "SIMPLE_SUPER_VERBOSE", "option", "SuperVerbose", "value", val, "error", err)
		}

	}

	// 3. Set flags.

	flag.StringVar(&options.Name, "name", options.Name, "Name of the person to greet. This is a mandatory field." /* Original Default: World, Env: SIMPLE_NAME */)

	var defaultAgeValForFlag int
	if options.Age != nil {
		defaultAgeValForFlag = *options.Age
	}
	if options.Age == nil {
		options.Age = new(int)
	}
	flag.IntVar(options.Age, "age", defaultAgeValForFlag, "Age of the person. This is an optional field." /* Env: SIMPLE_AGE */)

	flag.StringVar(&options.LogLevel, "log-level", options.LogLevel, `LogLevel for the application output.
It can be one of: debug, info, warning, error.` /* Original Default: info, Env: SIMPLE_LOG_LEVEL */)

	flag.StringVar(&options.OutputDir, "output-dir", options.OutputDir, `OutputDir for any generated files or reports.
Defaults to "output" if not specified by the user.` /* Original Default: output, Env:  */)

	flag.StringVar(&options.Mode, "mode", options.Mode, "Mode of operation for the tool, affecting its behavior." /* Env: SIMPLE_MODE */)

	flag.BoolVar(&options.SuperVerbose, "super-verbose", options.SuperVerbose, "Enable extra verbose output." /* Env: SIMPLE_SUPER_VERBOSE */)

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	// Handle special case for required bools defaulting to true with 'no-<flag>'

	// 5. Perform required checks (excluding booleans).

	// A string is required. It must not be its original default if the flag wasn't set and env var wasn't set.
	// If default was empty: must not be empty.
	// If default was non-empty: must not be that specific non-empty value.
	initialDefaultName := "World"
	envNameWasSet := false

	if _, ok := os.LookupEnv("SIMPLE_NAME"); ok {
		envNameWasSet = true
	}

	if options.Name == initialDefaultName && !isFlagExplicitlySet["name"] && !envNameWasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "name", "envVar", "SIMPLE_NAME", "option", "Name")
		os.Exit(1)
	}

	// A string is required. It must not be its original default if the flag wasn't set and env var wasn't set.
	// If default was empty: must not be empty.
	// If default was non-empty: must not be that specific non-empty value.
	initialDefaultLogLevel := "info"
	envLogLevelWasSet := false

	if _, ok := os.LookupEnv("SIMPLE_LOG_LEVEL"); ok {
		envLogLevelWasSet = true
	}

	if options.LogLevel == initialDefaultLogLevel && !isFlagExplicitlySet["log-level"] && !envLogLevelWasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "log-level", "envVar", "SIMPLE_LOG_LEVEL", "option", "LogLevel")
		os.Exit(1)
	}

	isValidChoice_LogLevel := false
	allowedChoices_LogLevel := []string{"debug", "info", "warning", "error"}

	// Handle non-pointer types for enum
	currentValue_LogLevelStr := fmt.Sprintf("%v", options.LogLevel)
	isValidChoice_LogLevel = slices.Contains(allowedChoices_LogLevel, currentValue_LogLevelStr)

	if !isValidChoice_LogLevel {
		var currentValueForMsg interface{} = options.LogLevel

		slog.Error("Invalid value for flag", "flag", "log-level", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_LogLevel, ", "))
		os.Exit(1)
	}

	// A string is required. It must not be its original default if the flag wasn't set and env var wasn't set.
	// If default was empty: must not be empty.
	// If default was non-empty: must not be that specific non-empty value.
	initialDefaultOutputDir := "output"
	envOutputDirWasSet := false

	if options.OutputDir == initialDefaultOutputDir && !isFlagExplicitlySet["output-dir"] && !envOutputDirWasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "output-dir", "option", "OutputDir")
		os.Exit(1)
	}

	// A string is required. It must not be its original default if the flag wasn't set and env var wasn't set.
	// If default was empty: must not be empty.
	// If default was non-empty: must not be that specific non-empty value.
	initialDefaultMode := ""
	envModeWasSet := false

	if _, ok := os.LookupEnv("SIMPLE_MODE"); ok {
		envModeWasSet = true
	}

	if options.Mode == initialDefaultMode && !isFlagExplicitlySet["mode"] && !envModeWasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "mode", "envVar", "SIMPLE_MODE", "option", "Mode")
		os.Exit(1)
	}

	isValidChoice_Mode := false
	allowedChoices_Mode := []string{"standard", "turbo", "eco"}

	// Handle non-pointer types for enum
	currentValue_ModeStr := fmt.Sprintf("%v", options.Mode)
	isValidChoice_Mode = slices.Contains(allowedChoices_Mode, currentValue_ModeStr)

	if !isValidChoice_Mode {
		var currentValueForMsg interface{} = options.Mode

		slog.Error("Invalid value for flag", "flag", "mode", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_Mode, ", "))
		os.Exit(1)
	}

	// End of range .Options for required checks
	// End of if .HasOptions

	err := FullsetRun(*options)

	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}

// ConfigureOptions would be options for a hypothetical 'configure' command.
type ConfigureOptions struct {
	Name    string `env:"CONFIGURE_NAME"`
	Verbose bool   `env:"CONFIGURE_VERBOSE"`
	Mode    string `env:"CONFIGURE_MODE"`
	Port    int    `env:"CONFIGURE_PORT"`
}

// NewConfigureOptions provides default values for ConfigureOptions.
func NewConfigureOptions() *ConfigureOptions {
	return &ConfigureOptions{
		Name:    "DefaultFullsetName",
		Verbose: true,
		// Mode and Port will use their zero values (empty string, 0)
		// or rely on struct tags for other defaults if those were supported by the initializer directly.
		// For this example, explicit initialization is sufficient.
	}
}
