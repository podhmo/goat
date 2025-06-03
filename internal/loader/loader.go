package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/build"
	// "go/parser" // Removed duplicate
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// LoadFile parses the given Go source file and returns its AST.
func LoadFile(fset *token.FileSet, filename string) (*ast.File, error) {
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}
	return file, nil
}

// LoadPackageFiles loads and parses all Go files in the package specified by importPath.
// It prioritizes files containing typeNameHint in their names.
func LoadPackageFiles(fset *token.FileSet, importPath string, typeNameHint string) ([]*ast.File, error) {
	pkg, err := build.Default.Import(importPath, ".", build.FindOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to find package %q: %w", importPath, err)
	}

	files, err := os.ReadDir(pkg.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", pkg.Dir, err)
	}

	var priorityFiles []string
	var otherFiles []string
	lowerTypeNameHint := strings.ToLower(typeNameHint)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}

		lowerFileName := strings.ToLower(file.Name())
		fullPath := filepath.Join(pkg.Dir, file.Name())

		if typeNameHint != "" && strings.Contains(lowerFileName, lowerTypeNameHint) {
			priorityFiles = append(priorityFiles, fullPath)
		} else {
			otherFiles = append(otherFiles, fullPath)
		}
	}

	allFiles := append(priorityFiles, otherFiles...)
	parsedFiles := make([]*ast.File, 0, len(allFiles))

	for _, filePath := range allFiles {
		fileAst, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("failed to parse file %q: %w", filePath, err)
		}
		parsedFiles = append(parsedFiles, fileAst)
	}

	return parsedFiles, nil
}
