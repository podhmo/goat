package main

import (
	"fmt"
	"log"
	"os"

	"github.com/podhmo/goat/goat" // Assuming goat markers are in this path
)

//go:generate goat -run RunSimpleApp -initializer NewSimpleOptions main.go
// An alternative could be:
//go:generate ../../goat_tool -run RunSimpleApp -initializer NewSimpleOptions main.go
// (if goat_tool is built to project root)

// SimpleOptions defines the command line options for this simple example tool.
// This tool demonstrates the basic capabilities of goat for CLI generation.
type SimpleOptions struct {
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

// NewSimpleOptions initializes SimpleOptions with default values and enum constraints.
// This function will be "interpreted" by the goat tool.
func NewSimpleOptions() *SimpleOptions {
	return &SimpleOptions{
		Name:      goat.Default("World"), // Default name
		LogLevel:  goat.Default("info", goat.Enum([]string{"debug", "info", "warning", "error"})),
		OutputDir: goat.Default("output"),
		Mode:      goat.Enum([]string{"standard", "turbo", "eco"}), // Enum without explicit default
		// Age is optional (pointer) and has no default here.
		// Features is a slice, will be handled by flag package (e.g. multiple --features flag or comma sep)
		// SuperVerbose is a bool, defaults to false (zero value for bool)
	}
}

// RunSimpleApp is the core logic for this simple CLI tool.
// It receives the parsed and validated options.
// This function's doc comment is used as the main help text for the command.
func RunSimpleApp(opts SimpleOptions) error {
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

	// Example of returning an error
	if opts.Name == "ErrorTrigger" {
		return fmt.Errorf("the name 'ErrorTrigger' is not allowed")
	}

	return nil
}

// The main function will be overwritten by the `goat` tool.
// For development purposes, you can have a simple main that calls your run function.
func main() {
	log.Println("Original main: This will be replaced by goat.")
	// Example of how you might run it manually during development:
	// opts := NewSimpleOptions()
	// if err := RunSimpleApp(*opts); err != nil {
	// 	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	// 	os.Exit(1)
	// }
	//
	// Or, to simulate generated main slightly more closely:
	//
	// You would need to parse flags manually here if you want to test that aspect
	// without running `go generate` yet. For simplicity, we just use defaults.
	fmt.Println("Simulating execution with default options before `go generate`:")
	defaultOpts := NewSimpleOptions()
	if err := RunSimpleApp(*defaultOpts); err != nil {
		fmt.Fprintf(os.Stderr, "Application error: %v\n", err)
		os.Exit(1)
	}
}