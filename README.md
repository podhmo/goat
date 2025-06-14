# goat 🐐

`goat` is a Go command-line tool with several subcommands that work with `go generate` to automate the creation of command-line interfaces from your Go `main.go` (or other specified) files.

## Overview

The core idea is to define a `run(options MyOptions) error` function and an `Options` struct in your Go program. `goat`'s subcommands can then parse this structure, along with special marker functions like `goat.Default()` and `goat.Enum()`, to either generate CLI boilerplate, inspect metadata, or display help messages.

The main subcommands are:
*   `emit`: Modifies the target Go file to include CLI argument parsing and execution logic. This is the primary command for `go:generate`.
*   `scan`: Outputs the extracted command metadata as JSON. Useful for debugging or integration with other tools.
*   `help-message`: Prints the generated help message for the CLI to stdout.
*   `init`: A placeholder for future functionality (e.g., scaffolding a new `goat`-compatible file).

>[!IMPORTANT]
>🚧This project is currently in the **early stages of development**.

## Features (Planned & In Progress)

*   **Automatic CLI generation:** Parses `Options` struct fields (name, type, comments, tags) to create CLI flags.
*   **Help message generation:** Creates comprehensive help messages based on comments and option attributes.
*   **Environment variable loading:** Reads option values from environment variables specified in struct tags (e.g., `env:"MY_VAR"`).
*   **Required flags:** Non-pointer fields in the `Options` struct are treated as required.
*   **Custom Option Types:** Supports fields implementing `encoding.TextUnmarshaler` and `encoding.TextMarshaler` for custom parsing logic and default value representation (via `flag.TextVar`).
*   **AST-based:** Operates directly on the Go Abstract Syntax Tree, avoiding reflection at runtime for the generated CLI.
*   **`go generate` integration:** Designed to be invoked via `//go:generate goat emit ...` comments for the `emit` subcommand.

## Usage

In your `main.go`:

```go
package main

import (
	"fmt"
	"log"

	"github.com/podhmo/goat" // Import goat marker package
)

//go:generate goat emit -run RunApp -initializer NewAppOptions main.go

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

## Marker Functions

`goat` utilizes special marker functions within your options initializer to provide metadata for CLI generation. These functions are typically used as the right-hand side of an assignment to a field in your options struct.

*   `goat.Default(value interface{}, options ...interface{}) interface{}`: Specifies a default value for an option. The first argument is the default value itself. Optional subsequent arguments can be other markers like `goat.Enum`.
*   `goat.Enum(allowed []string) interface{}`: Restricts the allowed values for a string option to the provided list.

### Subcommands

*   **`emit`**
    *   Syntax: `goat emit [flags] <target_gofile.go>`
    *   This is the primary command, typically used with `go generate`. It parses the target Go file, analyzes the specified run function and options initializer, and then rewrites the `main()` function in the target file to include CLI argument parsing, help message generation, and execution of your run function.
    *   Key flags:
        *   `-run <FunctionName>`: Specifies the name of the main function to be executed (e.g., `RunApp`). (Default: "run")
        *   `-initializer <FunctionName>`: Specifies the name of the function that initializes the options struct (e.g., `NewAppOptions`). (Optional)

*   **`scan`**
    *   Syntax: `goat scan [flags] <target_gofile.go>`
    *   This command parses and analyzes the target Go file (similar to `emit`) but instead of rewriting the file, it outputs the extracted command metadata as a JSON object to stdout. This can be useful for debugging or for other tools to consume.
    *   Key flags:
        *   `-run <FunctionName>`: (Default: "run")
        *   `-initializer <FunctionName>`: (Optional)

*   **`help-message`**
    *   Syntax: `goat help-message [flags] <target_gofile.go>`
    *   This command parses and analyzes the target Go file and then prints the generated help message for the CLI to stdout, based on the options struct and comments.
    *   Key flags:
        *   `-run <FunctionName>`: (Default: "run")
        *   `-initializer <FunctionName>`: (Optional)

*   **`init`**
    *   Syntax: `goat init`
    *   Currently, this command is a placeholder and will print a "TODO: init subcommand" message. Future functionality might involve scaffolding a new `goat`-compatible `main.go` file or project structure.

## Development

To build the `goat` tool:
```bash
go build -o goat_tool cmd/goat/main.go
```
(Or use `make build` if a Makefile exists and has a build target for the tool)

To run tests:
```bash
go test ./...
```
(Or use `make test` if a Makefile exists and has a test target)

## Contributing

Contributions are welcome! Please feel free to open an issue to discuss a bug or a new feature, or submit a pull request.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
