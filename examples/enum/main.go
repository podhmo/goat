package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/podhmo/goat"
	"github.com/podhmo/goat/examples/enum/customtypes"
)

//go:generate goat emit -run run -initializer NewOptions main.go

// MyLocalEnum is a locally defined enum type.
type MyLocalEnum string

const (
	// LocalA is a possible value for MyLocalEnum.
	LocalA MyLocalEnum = "local-a"
	// LocalB is a possible value for MyLocalEnum.
	LocalB MyLocalEnum = "local-b"
)

// GetLocalEnumsAsStrings returns the string representations of MyLocalEnum options.
func GetLocalEnumsAsStrings() []string {
	return []string{string(LocalA), string(LocalB)}
}

// Options defines the command line options for the enum example.
type Options struct {
	// LocalEnumField demonstrates a locally defined enum.
	LocalEnumField MyLocalEnum `env:"ENUM_LOCAL_ENUM"`

	// ImportedEnumField demonstrates an enum imported from another package.
	ImportedEnumField customtypes.MyCustomEnum `env:"ENUM_IMPORTED_ENUM"`

	// OptionalImportedEnumField demonstrates an optional enum (pointer type)
	// imported from another package.
	OptionalImportedEnumField *customtypes.MyCustomEnum `env:"ENUM_OPTIONAL_IMPORTED_ENUM"`
}

// NewOptions initializes Options with default values and enum constraints.
func NewOptions() *Options {
	return &Options{
		LocalEnumField: goat.Default(LocalA, goat.Enum(GetLocalEnumsAsStrings())),
		// TODO: Since it's a generic function, there should be no need to cast it to a string type.If there are reasons why it is not possible, I want to resolve them.
		ImportedEnumField:         goat.Default(customtypes.OptionX, goat.Enum(customtypes.GetCustomEnumOptionsAsStrings())),
		OptionalImportedEnumField: goat.Enum(nil, []string{string(customtypes.OptionX), string(customtypes.OptionY)}),
	}
}

// run is the main execution logic for the enum example CLI.
// It prints the selected enum values.
func run(opts Options) error {
	fmt.Printf("Selected Local Enum: %s\n", opts.LocalEnumField)
	fmt.Printf("Selected Imported Enum: %s\n", opts.ImportedEnumField)

	if opts.OptionalImportedEnumField != nil {
		fmt.Printf("Selected Optional Imported Enum: %s\n", *opts.OptionalImportedEnumField)
	} else {
		fmt.Println("Optional Imported Enum: not set")
	}

	// Example of accessing original enum values if needed
	if opts.ImportedEnumField == customtypes.OptionX {
		fmt.Println("INFO: ImportedEnumField is OptionX")
	}

	// Example to trigger an error for testing (e.g. if a certain combination is invalid)
	// if opts.LocalEnumField == LocalA && opts.ImportedEnumField == customtypes.OptionY {
	// 	return fmt.Errorf("combination of LocalA and OptionY is not allowed")
	// }

	return nil
}

func main() {
	isFlagExplicitlySet := make(map[string]bool)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `enum - run is the main execution logic for the enum example CLI.
         It prints the selected enum values.

Usage:
  enum [flags]

Flags:
  --local-enum-field             mylocalenum LocalEnumField demonstrates a locally defined enum. (required) (env: ENUM_LOCAL_ENUM)
  --imported-enum-field          mycustomenum ImportedEnumField demonstrates an enum imported from another package. (required) (env: ENUM_IMPORTED_ENUM)
  --optional-imported-enum-field mycustomenum OptionalImportedEnumField demonstrates an optional enum (pointer type)
                                        imported from another package. (env: ENUM_OPTIONAL_IMPORTED_ENUM)

  -h, --help                             Show this help message and exit
`)
	}

	var options *Options

	// 1. Create Options using the initializer function.
	options = NewOptions()
	// End of if/else .RunFunc.InitializerFunc for options assignment

	// 2. Override with environment variable values.
	// This section assumes 'options' is already initialized.

	if val, ok := os.LookupEnv("ENUM_LOCAL_ENUM"); ok {

	}

	if val, ok := os.LookupEnv("ENUM_IMPORTED_ENUM"); ok {

	}

	if val, ok := os.LookupEnv("ENUM_OPTIONAL_IMPORTED_ENUM"); ok {

	}

	// End of range .Options for env vars

	// 3. Set flags.

	// End of range .Options for flags

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	// Handle special case for required bools defaulting to true with 'no-<flag>'

	// 5. Perform required checks (excluding booleans).

	// End of range .Options for required checks

	// TODO: Implement runtime validation for file options based on metadata:
	// - Check for opt.FileMustExist (e.g., using os.Stat)
	// - Handle opt.FileGlobPattern (e.g., using filepath.Glob)
	// Currently, these attributes are parsed but not enforced at runtime by the generated CLI.
	// End of if .RunFunc.OptionsArgTypeNameStripped (options handling block)

	var err error

	// Run function expects an options argument
	err = run(*options)

	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}
