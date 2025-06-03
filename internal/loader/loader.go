package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
)

// LoadFile parses the given Go source file and returns its AST.
func LoadFile(fset *token.FileSet, filename string) (*ast.File, error) {
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}
	return file, nil
}
