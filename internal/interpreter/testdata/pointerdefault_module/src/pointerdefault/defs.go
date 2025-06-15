package pointerdefault

import "github.com/podhmo/goat"

// Config demonstrates a struct with a pointer field having a default value
// set using goat.Default and a helper function that returns a pointer.
type Config struct {
	// MyStringPtr is a pointer to a string, with a default.
	MyStringPtr *string `goat:"ptr-field" help:"A pointer to a string with a default value."`
	AnotherPtr  *int    `goat:"another-ptr-field" help:"Another pointer field, this one to an int."`
}

// stringPtr is a helper function to get a pointer to a string.
// This mimics how users might provide pointer defaults.
func stringPtr(s string) *string {
	return &s
}

// intPtr is a helper function to get a pointer to an int.
func intPtr(i int) *int {
	return &i
}

// DefaultConfig is an initializer function that would typically be used by goat.
// The .(*string) cast is idiomatic for how goat.Default is used,
// though for the interpreter's static analysis of goat.Default itself,
// the cast isn't strictly what's analyzed (the arguments to goat.Default are).
func DefaultConfig() *Config {
	return &Config{
		MyStringPtr: goat.Default(stringPtr("expected_default")).(*string),
		AnotherPtr:  goat.Default(intPtr(123)).(*int),
	}
}

// Run is a dummy run function to make this package loadable by the interpreter
// if it expects a run function to be present.
func Run(cfg *Config) error {
	// Do nothing
	return nil
}
