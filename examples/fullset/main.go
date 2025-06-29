package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/podhmo/goat"
)

// stringPtr is a helper function to get a pointer to a string.
func stringPtr(s string) *string {
	return &s
}

//go:generate goat emit -run run -initializer NewOptions main.go

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

// NewOptions initializes Options with default values and enum constraints.
// This function will be "interpreted" by the goat tool.
func NewOptions() *Options {
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
		ConfigFile:                  goat.Default("config.json"),
		Pattern:                     goat.Default("*.go"),
		EnableFeatureX:              goat.Default(true),
		HostIP:                      goat.Default(net.ParseIP("127.0.0.1")),
		ExistingFieldToMakeOptional: goat.Default(stringPtr("was set by default")),
	}
}

// run is the core logic for this CLI tool.
// It receives the parsed and validated options.
// This function's doc comment is used as the main help text for the command.
func run(ctx context.Context, opts *Options) error {
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

// This main function was auto-generated by goat.
func main() {
	ctx := context.Background()
	isFlagExplicitlySet := make(map[string]bool)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `fullset - run is the core logic for this CLI tool.
         It receives the parsed and validated options.
         This function`+"`"+`s doc comment is used as the main help text for the command.

Usage:
  fullset [flags]

Flags:
  --name                            string   Name of the person to greet. This is a mandatory field. (default: "World") (env: SIMPLE_NAME)
  --age                             int      Age of the person. This is an optional field. (env: SIMPLE_AGE)
  --log-level                       string   LogLevel for the application output.
                                           It can be one of: debug, info, warning, error. (default: "info") (env: SIMPLE_LOG_LEVEL) (allowed: "debug", "info", "warning", "error")
  --features                        strings  Features to enable, provided as a comma-separated list.
                                           Example: --features feat1,feat2 (required) (env: SIMPLE_FEATURES)
  --output-dir                      string   OutputDir for any generated files or reports.
                                           Defaults to "output" if not specified by the user. (default: "output")
  --mode                            string   Mode of operation for the tool, affecting its behavior. (default: "standard") (env: SIMPLE_MODE) (allowed: "standard", "turbo", "eco")
  --super-verbose                   bool     Enable extra verbose output. (env: SIMPLE_SUPER_VERBOSE)
  --optional-toggle                 bool     An optional boolean flag with no default, should be nil if not set.
  --config-file                     string   Path to a configuration file. Must exist. (env:"FULLSET_CONFIG_FILE") (default: "config.json") (env: FULLSET_CONFIG_FILE)
  --pattern                         string   A glob pattern for input files. (env:"FULLSET_PATTERN") (default: "*.go") (env: FULLSET_PATTERN)
  --no-enable-feature-x             bool     Enable Feature X by default. Use --no-enable-feature-x to disable. (env:"FULLSET_FEATURE_X") (env: FULLSET_FEATURE_X)
  --host-ip                         ip       The host IP address for the service. (env:"FULLSET_HOST_IP") (required) (env: FULLSET_HOST_IP)
  --existing-field-to-make-optional string   Example of an existing field made optional. (env:"FULLSET_OPTIONAL_EXISTING") (env: FULLSET_OPTIONAL_EXISTING)

  -h, --help                                Show this help message and exit
`)
	}

	// 1. Create Options using the initializer function.
	options := NewOptions()

	// 2. Override with environment variable values.
	// This section assumes 'options' is already initialized.

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
			slog.WarnContext(ctx, "Could not parse environment variable as *int for option", "envVar", "SIMPLE_AGE", "option", "Age", "value", val, "error", err)
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
			slog.WarnContext(ctx, "Could not parse environment variable as bool for option", "envVar", "SIMPLE_SUPER_VERBOSE", "option", "SuperVerbose", "value", val, "error", err)
		}
	}

	if val, ok := os.LookupEnv("FULLSET_CONFIG_FILE"); ok {
		options.ConfigFile = val
	}

	if val, ok := os.LookupEnv("FULLSET_PATTERN"); ok {
		options.Pattern = val
	}

	if val, ok := os.LookupEnv("FULLSET_FEATURE_X"); ok {

		if v, err := strconv.ParseBool(val); err == nil {
			options.EnableFeatureX = v
		} else {
			slog.WarnContext(ctx, "Could not parse environment variable as bool for option", "envVar", "FULLSET_FEATURE_X", "option", "EnableFeatureX", "value", val, "error", err)
		}
	}

	if val, ok := os.LookupEnv("FULLSET_HOST_IP"); ok {

		err := (&options.HostIP).UnmarshalText([]byte(val))
		if err != nil {
			slog.WarnContext(ctx, "Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "FULLSET_HOST_IP", "option", "host-ip", "value", val, "error", err)
		}
	}

	if val, ok := os.LookupEnv("FULLSET_OPTIONAL_EXISTING"); ok {

		if options.ExistingFieldToMakeOptional == nil {
			options.ExistingFieldToMakeOptional = new(string)
		}
		*options.ExistingFieldToMakeOptional = val
	}

	// 3. Set flags.
	flag.StringVar(&options.Name, "name", options.Name, "Name of the person to greet. This is a mandatory field." /* Original Default: World, Env: SIMPLE_NAME */)
	isAgeNilInitially := options.Age == nil
	var tempAgeVal int
	var defaultAgeValForFlag int
	if options.Age != nil {
		defaultAgeValForFlag = *options.Age
	}
	if isAgeNilInitially {
		flag.IntVar(&tempAgeVal, "age", 0, "Age of the person. This is an optional field." /* Env: SIMPLE_AGE */)
	} else {
		flag.IntVar(options.Age, "age", defaultAgeValForFlag, "Age of the person. This is an optional field." /* Env: SIMPLE_AGE */)
	}
	flag.StringVar(&options.LogLevel, "log-level", options.LogLevel, `LogLevel for the application output.
It can be one of: debug, info, warning, error.` /* Original Default: info, Env: SIMPLE_LOG_LEVEL */)
	flag.StringVar(&options.OutputDir, "output-dir", options.OutputDir, `OutputDir for any generated files or reports.
Defaults to "output" if not specified by the user.` /* Original Default: output, Env:  */)
	flag.StringVar(&options.Mode, "mode", options.Mode, "Mode of operation for the tool, affecting its behavior." /* Original Default: standard, Env: SIMPLE_MODE */)
	flag.BoolVar(&options.SuperVerbose, "super-verbose", options.SuperVerbose, "Enable extra verbose output." /* Env: SIMPLE_SUPER_VERBOSE */)
	isOptionalToggleNilInitially := options.OptionalToggle == nil
	var tempOptionalToggleVal bool
	var defaultOptionalToggleValForFlag bool
	if options.OptionalToggle != nil {
		defaultOptionalToggleValForFlag = *options.OptionalToggle
	}
	if isOptionalToggleNilInitially {
		flag.BoolVar(&tempOptionalToggleVal, "optional-toggle", false, "An optional boolean flag with no default, should be nil if not set.")
	} else {
		flag.BoolVar(options.OptionalToggle, "optional-toggle", defaultOptionalToggleValForFlag, "An optional boolean flag with no default, should be nil if not set.")
	}
	flag.StringVar(&options.ConfigFile, "config-file", options.ConfigFile, "Path to a configuration file. Must exist. (env:\"FULLSET_CONFIG_FILE\")" /* Original Default: config.json, Env: FULLSET_CONFIG_FILE */)
	flag.StringVar(&options.Pattern, "pattern", options.Pattern, "A glob pattern for input files. (env:\"FULLSET_PATTERN\")" /* Original Default: *.go, Env: FULLSET_PATTERN */)
	var EnableFeatureX_NoFlagIsPresent bool
	flag.BoolVar(&EnableFeatureX_NoFlagIsPresent, "no-enable-feature-x", false, "Set enable-feature-x to false")
	flag.TextVar(&options.HostIP, "host-ip", options.HostIP, "The host IP address for the service. (env:\"FULLSET_HOST_IP\")" /* Env: FULLSET_HOST_IP */)
	isExistingFieldToMakeOptionalNilInitially := options.ExistingFieldToMakeOptional == nil
	var tempExistingFieldToMakeOptionalVal string
	var defaultExistingFieldToMakeOptionalValForFlag string
	if options.ExistingFieldToMakeOptional != nil {
		defaultExistingFieldToMakeOptionalValForFlag = *options.ExistingFieldToMakeOptional
	}
	if isExistingFieldToMakeOptionalNilInitially {
		flag.StringVar(&tempExistingFieldToMakeOptionalVal, "existing-field-to-make-optional", "", "Example of an existing field made optional. (env:\"FULLSET_OPTIONAL_EXISTING\")" /* Env: FULLSET_OPTIONAL_EXISTING */)
	} else {
		flag.StringVar(options.ExistingFieldToMakeOptional, "existing-field-to-make-optional", defaultExistingFieldToMakeOptionalValForFlag, "Example of an existing field made optional. (env:\"FULLSET_OPTIONAL_EXISTING\")" /* Env: FULLSET_OPTIONAL_EXISTING */)
	}

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	if EnableFeatureX_NoFlagIsPresent {
		options.EnableFeatureX = false
	}

	// 6. Assign values for initially nil pointers if flags were explicitly set
	if isAgeNilInitially && isFlagExplicitlySet["age"] {
		options.Age = &tempAgeVal
	}
	if isOptionalToggleNilInitially && isFlagExplicitlySet["optional-toggle"] {
		options.OptionalToggle = &tempOptionalToggleVal
	}
	if isExistingFieldToMakeOptionalNilInitially && isFlagExplicitlySet["existing-field-to-make-optional"] {
		options.ExistingFieldToMakeOptional = &tempExistingFieldToMakeOptionalVal
	}

	// 5. Perform required checks (excluding booleans).

	initialDefaultName := "World"
	envNameWasSet := false
	if _, ok := os.LookupEnv("SIMPLE_NAME"); ok {
		envNameWasSet = true
	}
	if options.Name == initialDefaultName && !isFlagExplicitlySet["name"] && !envNameWasSet {
		slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "name", "envVar", "SIMPLE_NAME", "option", "Name")
		os.Exit(1)
	}

	initialDefaultLogLevel := "info"
	envLogLevelWasSet := false
	if _, ok := os.LookupEnv("SIMPLE_LOG_LEVEL"); ok {
		envLogLevelWasSet = true
	}
	if options.LogLevel == initialDefaultLogLevel && !isFlagExplicitlySet["log-level"] && !envLogLevelWasSet {
		slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "log-level", "envVar", "SIMPLE_LOG_LEVEL", "option", "LogLevel")
		os.Exit(1)
	}

	isValidChoice_LogLevel := false
	allowedChoices_LogLevel := []string{"debug", "info", "warning", "error"}

	currentValue_LogLevelStr := fmt.Sprintf("%v", options.LogLevel)
	isValidChoice_LogLevel = slices.Contains(allowedChoices_LogLevel, currentValue_LogLevelStr)

	if !isValidChoice_LogLevel {
		var currentValueForMsg interface{} = options.LogLevel // options.OptName
		slog.ErrorContext(ctx, "Invalid value for flag", errors.New("Invalid value for flag"), "flag", "log-level", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_LogLevel, ", "))
		os.Exit(1)
	}

	initialDefaultOutputDir := "output"
	envOutputDirWasSet := false
	if options.OutputDir == initialDefaultOutputDir && !isFlagExplicitlySet["output-dir"] && !envOutputDirWasSet {
		slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "output-dir", "option", "OutputDir")
		os.Exit(1)
	}

	initialDefaultMode := "standard"
	envModeWasSet := false
	if _, ok := os.LookupEnv("SIMPLE_MODE"); ok {
		envModeWasSet = true
	}
	if options.Mode == initialDefaultMode && !isFlagExplicitlySet["mode"] && !envModeWasSet {
		slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "mode", "envVar", "SIMPLE_MODE", "option", "Mode")
		os.Exit(1)
	}

	isValidChoice_Mode := false
	allowedChoices_Mode := []string{"standard", "turbo", "eco"}

	currentValue_ModeStr := fmt.Sprintf("%v", options.Mode)
	isValidChoice_Mode = slices.Contains(allowedChoices_Mode, currentValue_ModeStr)

	if !isValidChoice_Mode {
		var currentValueForMsg interface{} = options.Mode // options.OptName
		slog.ErrorContext(ctx, "Invalid value for flag", errors.New("Invalid value for flag"), "flag", "mode", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_Mode, ", "))
		os.Exit(1)
	}

	initialDefaultConfigFile := "config.json"
	envConfigFileWasSet := false
	if _, ok := os.LookupEnv("FULLSET_CONFIG_FILE"); ok {
		envConfigFileWasSet = true
	}
	if options.ConfigFile == initialDefaultConfigFile && !isFlagExplicitlySet["config-file"] && !envConfigFileWasSet {
		slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "config-file", "envVar", "FULLSET_CONFIG_FILE", "option", "ConfigFile")
		os.Exit(1)
	}

	initialDefaultPattern := "*.go"
	envPatternWasSet := false
	if _, ok := os.LookupEnv("FULLSET_PATTERN"); ok {
		envPatternWasSet = true
	}
	if options.Pattern == initialDefaultPattern && !isFlagExplicitlySet["pattern"] && !envPatternWasSet {
		slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "pattern", "envVar", "FULLSET_PATTERN", "option", "Pattern")
		os.Exit(1)
	}
	if err := run(ctx, options); err != nil {

		slog.ErrorContext(ctx, "Runtime error", "error", err)
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
