package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/build"
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

// LoadPackageFiles loads and parses all Go files in the package specified by path.
// path can be an import path or an absolute directory path.
// It prioritizes files containing typeNameHint in their names.
func LoadPackageFiles(fset *token.FileSet, path string, typeNameHint string) ([]*ast.File, error) {
	var dirToScan string
	var pkgNameForLogging string // For logging purposes, use the original path

	if filepath.IsAbs(path) {
		dirToScan = path
		pkgNameForLogging = path // Use the abs path for logging if it's a dir
		// Check if it's actually a directory
		info, err := os.Stat(dirToScan)
		if err != nil {
			return nil, fmt.Errorf("failed to stat path %q: %w", dirToScan, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("absolute path %q is not a directory", dirToScan)
		}
	} else {
		// Assume it's an import path
		pkgNameForLogging = path
		pkgBuildInfo, err := build.Default.Import(path, ".", build.FindOnly)
		if err != nil {
			return nil, fmt.Errorf("failed to find package %q using build.Import: %w", path, err)
		}
		dirToScan = pkgBuildInfo.Dir
	}

	files, err := os.ReadDir(dirToScan)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q (derived from path %q): %w", dirToScan, pkgNameForLogging, err)
	}

	var priorityFiles []string
	var otherFiles []string
	lowerTypeNameHint := strings.ToLower(typeNameHint)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}

		lowerFileName := strings.ToLower(file.Name())
		fullPath := filepath.Join(dirToScan, file.Name()) // Changed pkg.Dir to dirToScan

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

// FindModuleRoot searches upwards from the given filePath for a go.mod file.
// It returns the path to the directory containing the go.mod file.
// If no go.mod is found, it returns an error.
func FindModuleRoot(filePath string) (string, error) {
	dir := filepath.Dir(filePath)
	if dir == "" || dir == "." || dir == "/" { // Reached root or invalid path
		return "", fmt.Errorf("go.mod not found for path %s", filePath)
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil // Found go.mod
		}

		parentDir := filepath.Dir(dir)
		if parentDir == dir { // Reached root
			break
		}
		dir = parentDir
	}
	return "", fmt.Errorf("go.mod not found upwards from %s", filePath)
}

// GetModuleName parses a go.mod file and returns the module name.
func GetModuleName(moduleRootPath string) (string, error) {
	goModPath := filepath.Join(moduleRootPath, "go.mod")
	modBytes, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("could not read go.mod at %s: %w", goModPath, err)
	}

	// Using a simplified parser for module path. For full parsing, use golang.org/x/mod/modfile.
	// Example: module example.com/mymodule
	var modulePath string
	for _, line := range strings.Split(string(modBytes), "\n") {
		if strings.HasPrefix(line, "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			break
		}
	}

	if modulePath == "" {
		return "", fmt.Errorf("module directive not found in %s", goModPath)
	}
	return modulePath, nil
}
