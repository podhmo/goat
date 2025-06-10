package loader

import "fmt"

// PackageNotFoundError indicates that a package could not be found.
type PackageNotFoundError struct {
	Path string
}

func (e *PackageNotFoundError) Error() string {
	return fmt.Sprintf("package %q not found", e.Path)
}

// ParseError indicates an error during parsing of a Go source file.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse %s: %v", e.Path, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}
