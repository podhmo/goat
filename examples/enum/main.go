package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

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

var MyLocalEnumValues = []string{
	string(LocalA),
	string(LocalB),
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
		LocalEnumField:            goat.Default(LocalA, goat.Enum(MyLocalEnumValues)),                           // Changed
		ImportedEnumField:         goat.Default(customtypes.OptionX, goat.Enum(customtypes.MyCustomEnumValues)), // Changed
		OptionalImportedEnumField: goat.Enum((*customtypes.MyCustomEnum)(nil), []customtypes.MyCustomEnum{customtypes.OptionX, customtypes.OptionY}),
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
  --local-enum-field             mylocalenum LocalEnumField demonstrates a locally defined enum. (default: "local-a") (env: ENUM_LOCAL_ENUM) (allowed: "local-a", "local-b")
  --imported-enum-field          mycustomenum ImportedEnumField demonstrates an enum imported from another package. (default: "option-x") (env: ENUM_IMPORTED_ENUM) (allowed: "option-x", "option-y", "option-z")
  --optional-imported-enum-field mycustomenum OptionalImportedEnumField demonstrates an optional enum (pointer type)
                                        imported from another package. (env: ENUM_OPTIONAL_IMPORTED_ENUM) (allowed: "option-x", "option-y")

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

		// This handles non-pointer named types with an underlying kind of string (e.g., string-based enums)
		options.LocalEnumField = MyLocalEnum(val)

	}

	if val, ok := os.LookupEnv("ENUM_IMPORTED_ENUM"); ok {

		// This handles non-pointer named types with an underlying kind of string (e.g., string-based enums)
		options.ImportedEnumField = customtypes.MyCustomEnum(val)

	}

	if val, ok := os.LookupEnv("ENUM_OPTIONAL_IMPORTED_ENUM"); ok {

		// This handles pointer to named types with an underlying kind of string (e.g., *MyEnum)
		typedVal := customtypes.MyCustomEnum(val)
		options.OptionalImportedEnumField = &typedVal

	}

	// End of range .Options for env vars

	// 3. Set flags.

	// Handles *MyCustomEnum if EnumValues are present and not TextUnmarshaler
	// Ensure the field is initialized if nil, as flag.Var needs a non-nil flag.Value.
	// The initializer (NewOptions) would have run. If it set a non-nil default, that's used.
	// If the default from NewOptions was nil (as in our case for OptionalImportedEnumField),
	// then we must initialize it here before passing to flag.Var.
	if options.OptionalImportedEnumField == nil {
		options.OptionalImportedEnumField = new(customtypes.MyCustomEnum)
	}
	flag.Var(options.OptionalImportedEnumField, "optional-imported-enum-field", `OptionalImportedEnumField demonstrates an optional enum (pointer type)
imported from another package.`)

	// End of range .Options for flags

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	// Handle special case for required bools defaulting to true with 'no-<flag>'

	// 5. Perform required checks (excluding booleans).

	isValidChoice_LocalEnumField := false
	allowedChoices_LocalEnumField := []string{"local-a", "local-b"}

	currentValue_LocalEnumFieldStr := fmt.Sprintf("%v", options.LocalEnumField)
	isValidChoice_LocalEnumField = slices.Contains(allowedChoices_LocalEnumField, currentValue_LocalEnumFieldStr)

	if !isValidChoice_LocalEnumField {
		var currentValueForMsg interface{} = options.LocalEnumField

		slog.ErrorContext(context.Background(), "Invalid value for flag", errors.New("Invalid value for flag"), "flag", "local-enum-field", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_LocalEnumField, ", "))
		os.Exit(1)
	}

	isValidChoice_ImportedEnumField := false
	allowedChoices_ImportedEnumField := []string{"option-x", "option-y", "option-z"}

	currentValue_ImportedEnumFieldStr := fmt.Sprintf("%v", options.ImportedEnumField)
	isValidChoice_ImportedEnumField = slices.Contains(allowedChoices_ImportedEnumField, currentValue_ImportedEnumFieldStr)

	if !isValidChoice_ImportedEnumField {
		var currentValueForMsg interface{} = options.ImportedEnumField

		slog.ErrorContext(context.Background(), "Invalid value for flag", errors.New("Invalid value for flag"), "flag", "imported-enum-field", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_ImportedEnumField, ", "))
		os.Exit(1)
	}

	isValidChoice_OptionalImportedEnumField := false
	allowedChoices_OptionalImportedEnumField := []string{"option-x", "option-y"}

	// Catches other pointer enums, e.g. *MyCustomEnum
	if options.OptionalImportedEnumField != nil {
		currentValue_OptionalImportedEnumFieldStr := fmt.Sprintf("%v", *options.OptionalImportedEnumField)
		isValidChoice_OptionalImportedEnumField = slices.Contains(allowedChoices_OptionalImportedEnumField, currentValue_OptionalImportedEnumFieldStr)
	} else { // Field is nil

		// For optional pointer enums, nil is a valid state (means not provided).
		// If EnumValues are defined, it implies that if a value IS provided, it must be one of them.
		// If it's nil, it hasn't been provided, so it's "valid" in terms of choice.
		isValidChoice_OptionalImportedEnumField = true

	}

	if !isValidChoice_OptionalImportedEnumField {
		var currentValueForMsg interface{} = options.OptionalImportedEnumField

		if options.OptionalImportedEnumField != nil {
			currentValueForMsg = *options.OptionalImportedEnumField
		}
		// If nil, currentValueForMsg remains options.OptionalImportedEnumField (which will print as <nil>)

		slog.ErrorContext(context.Background(), "Invalid value for flag", errors.New("Invalid value for flag"), "flag", "optional-imported-enum-field", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_OptionalImportedEnumField, ", "))
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
	err = run(*options)

	if err != nil {
		slog.ErrorContext(context.Background(), "Runtime error", "error", err)
		os.Exit(1)
	}
}
