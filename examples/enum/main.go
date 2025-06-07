package main

import (
	"fmt"
	"os"

	"github.com/podhmo/goat"
	"github.com/podhmo/goat/examples/enum/customtypes"
)

//go:generate goat emit -run Run -initializer NewOptions main.go

func main() {
	opts := NewOptions()
	if err := goat.Run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
}

// Run is the main execution logic for the enum example CLI.
// It prints the selected enum values.
func Run(opts Options) error {
	fmt.Printf("Selected Local Enum: %s", opts.LocalEnumField)
	fmt.Printf("Selected Imported Enum: %s", opts.ImportedEnumField)

	if opts.OptionalImportedEnumField != nil {
		fmt.Printf("Selected Optional Imported Enum: %s", *opts.OptionalImportedEnumField)
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
