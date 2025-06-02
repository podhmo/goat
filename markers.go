// Package goat provides marker functions used by the `goat` tool
// to understand user intentions for CLI option generation.
// These functions are intended to be used in the user's `main.go` file
// within the Options struct's initializer function (e.g. NewOptions).
// The `goat` tool parses the AST and identifies calls to these functions
// to extract default values, enum choices, etc.
// These functions themselves have minimal runtime behavior, typically
// just returning their input, as their primary purpose is static analysis.
package goat

// Enum marks a field as having a set of allowed values.
// The `goat` tool's interpreter will extract these `values`.
// It is used for analysis purposes only and returns the passed `values` as is at runtime.
// Type parameter T can be any type, but typically string, int, or other simple types
// suitable for command-line options.
func Enum[T any](values []T) []T {
	return values
}

// Default sets a default value for a field.
// The `goat` tool's interpreter will extract this `defaultValue`.
// It can optionally take an `enumConstraint` which is typically the result of a call to `Enum()`.
// If `enumConstraint` is provided and is a non-empty slice, its first element
// (which should be a slice of allowed values from `Enum()`) will be used for enum validation.
// It is used for analysis purposes only and returns the passed `defaultValue` as is at runtime.
// Type parameter T can be any type suitable for a default value.
func Default[T any](defaultValue T, enumConstraint ...[]T) T {
	// The enumConstraint argument is primarily for the static analyzer.
	// The analyzer will look for calls to goat.Enum() passed here.
	// At runtime, this function simply returns the defaultValue.
	return defaultValue
}