//go:build ignore
// The line above ensures this file is not part of the normal build.

// This file is solely for demonstrating/triggering `go generate`.
// You would typically place the `//go:generate` directive directly in `main.go`.
// Having it here is just for organizational clarity in this example.

package main

// To run generation for this example, navigate to the `examples/simple` directory
// and run `go generate`.
// You might need to build the `goat` tool first and ensure it's in your PATH
// or provide a relative path to it.
//
// Example if `goat` is in project root as `goat_tool`:
// //go:generate ../../goat_tool -run RunSimpleApp -initializer NewSimpleOptions main.go
//
// If `goat` is in PATH:
// //go:generate goat -run RunSimpleApp -initializer NewSimpleOptions main.go
//
// The directive is usually placed in the `main.go` file itself, like this:
//
// package main
//
// //go:generate goat -run RunSimpleApp -initializer NewSimpleOptions main.go
//
// import ( ... )
// ... rest of main.go