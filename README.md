# goat 🐐

`goat` is a Go command-line tool that works with `go generate` to automate the creation of command-line interfaces from your Go `main.go` (or other specified) files.

## Overview

The core idea is to define a `run(options MyOptions) error` function and an `Options` struct in your Go program. `goat` will then parse this structure, along with special marker functions like `goat.Default()` and `goat.Enum()`, to generate the necessary CLI boilerplate (flag parsing, help messages, environment variable loading, etc.) directly into your `main()` function.

This project is currently in the **early stages of development**.

## Features (Planned & In Progress)

*   **Automatic CLI generation:** Parses `Options` struct fields (name, type, comments, tags) to create CLI flags.
*   **Help message generation:** Creates comprehensive help messages based on comments and option attributes.
*   **Default values:** Supports default values via `goat.Default()` marker function.
*   **Enum validation:** Supports enum-like restricted values via `goat.Enum()` marker function.
*   **Environment variable loading:** Reads option values from environment variables specified in struct tags (e.g., `env:"MY_VAR"`).
*   **Required flags:** Non-pointer fields in the `Options` struct are treated as required.
*   **AST-based:** Operates directly on the Go Abstract Syntax Tree, avoiding reflection at runtime for the generated CLI.
*   **`go generate` integration:** Designed to be invoked via `//go:generate goat ...` comments.

## Tentative Usage

In your `main.go`:

```go
package main

import (
	"fmt"
	"log"

	"github.com/podhmo/goat/goat" // Import goat marker package
)

//go:generate goat -run RunApp -initializer NewAppOptions main.go

// AppOptions defines the command-line options for our application.
// This application serves as a demonstration of goat's capabilities.
type AppOptions struct {
	// Name of the user to greet. This is a very important field.
	UserName string `env:"APP_USER_NAME"`

	// Port for the server to listen on.
	Port int `env:"APP_PORT"`

	// Verbose enables verbose logging. Optional.
	Verbose *bool `env:"APP_VERBOSE"`

	// LogLevel sets the logging level for the application.
	LogLevel string `env:"APP_LOG_LEVEL"`

	// Mode of operation.
	Mode string
}

// NewAppOptions creates a new AppOptions with default values and enum constraints.
func NewAppOptions() *AppOptions {
	return &AppOptions{
		UserName: goat.Default("Guest"),
		Port:     goat.Default(8080),
		LogLevel: goat.Default("info", goat.Enum([]string{"debug", "info", "warn", "error"})),
		Mode:     goat.Enum([]string{"dev", "prod"}),
	}
}

// RunApp is the main application logic.
// It receives configured options and executes the core functionality.
func RunApp(opts AppOptions) error {
	log.Printf("Running app with options: %+v\n", opts)
	fmt.Printf("Hello, %s!\n", opts.UserName)
	if opts.Verbose != nil && *opts.Verbose {
		fmt.Println("Verbose mode enabled.")
	}
	fmt.Printf("Server would run on port: %d\n", opts.Port)
	fmt.Printf("Log level: %s\n", opts.LogLevel)
	fmt.Printf("Mode: %s\n", opts.Mode)
	return nil
}

// main function will be generated/overwritten by goat.
// You can have a simple main for development before generation.
func main() {
	// This content will be replaced by goat.
	// For local development, you might manually call:
	//
	// opts := NewAppOptions()
	// if err := RunApp(*opts); err != nil {
	// 	 log.Fatal(err)
	// }
	log.Println("Original main.go - This will be replaced by goat.")
}

```

Then run:

```bash
go generate
go build -o myapp
./myapp --help
```

This would (ideally) produce a CLI tool with flags derived from `AppOptions`.

## Development

(Details on building `goat` itself, running tests, etc. will go here.)

```bash
# To build the goat tool itself
# cd cmd/goat
# go build -o ../../goat_tool # builds to project root as goat_tool
```

## Contributing

(Contribution guidelines will go here.)

## License

(License information will go here, e.g., MIT License.)