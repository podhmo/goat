package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/build"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadFile parses the given Go source file and returns its AST.
func LoadFile(fset *token.FileSet, filename string, baseIndent int) (*ast.File, error) {
	slog.Debug(strings.Repeat("\t", baseIndent)+"LoadFile: start", "filename", filename)
	file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		slog.Debug(strings.Repeat("\t", baseIndent)+"LoadFile: end (error)")
		return nil, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}
	slog.Debug(strings.Repeat("\t", baseIndent)+"LoadFile: end")
	return file, nil
}

// LoadPackageFiles loads and parses all Go files in the package specified by path.
// path can be an import path or an absolute directory path.
// It prioritizes files containing typeNameHint in their names.
// The typeNameHint is now called currentGoFile for clarity, but serves a similar purpose.
func LoadPackageFiles(fset *token.FileSet, path string, currentGoFile string, baseIndent int) ([]*ast.File, error) {
	slog.Debug(strings.Repeat("\t", baseIndent)+"LoadPackageFiles: start", "dirPath", path, "currentGoFile", currentGoFile)
	var dirToScan string
	var pkgNameForLogging string // For logging purposes, use the original path

	if filepath.IsAbs(path) {
		slog.Debug(strings.Repeat("\t", baseIndent+1)+"Path is absolute", "path", path)
		dirToScan = path
		pkgNameForLogging = path // Use the abs path for logging if it's a dir
		// Check if it's actually a directory
		info, err := os.Stat(dirToScan)
		if err != nil {
			slog.Debug(strings.Repeat("\t", baseIndent)+"LoadPackageFiles: end (error stat-ing path)")
			return nil, fmt.Errorf("failed to stat path %q: %w", dirToScan, err)
		}
		if !info.IsDir() {
			slog.Debug(strings.Repeat("\t", baseIndent)+"LoadPackageFiles: end (path is not a directory)")
			return nil, fmt.Errorf("absolute path %q is not a directory", dirToScan)
		}
	} else {
		slog.Debug(strings.Repeat("\t", baseIndent+1)+"Path is import path", "path", path)
		pkgNameForLogging = path
		pkgBuildInfo, err := build.Default.Import(path, ".", build.FindOnly)
		if err != nil {
			slog.Debug(strings.Repeat("\t", baseIndent)+"LoadPackageFiles: end (error importing package)")
			return nil, fmt.Errorf("failed to find package %q using build.Import: %w", path, err)
		}
		dirToScan = pkgBuildInfo.Dir
		slog.Debug(strings.Repeat("\t", baseIndent+1)+"Resolved import path to directory", "dirToScan", dirToScan)
	}

	files, err := os.ReadDir(dirToScan)
	if err != nil {
		slog.Debug(strings.Repeat("\t", baseIndent)+"LoadPackageFiles: end (error reading directory)")
		return nil, fmt.Errorf("failed to read directory %q (derived from path %q): %w", dirToScan, pkgNameForLogging, err)
	}
	slog.Debug(strings.Repeat("\t", baseIndent+1)+"Read directory contents", "numFiles", len(files))

	var priorityFiles []string
	var otherFiles []string
	lowerCurrentGoFileHint := ""
	if currentGoFile != "" { // typeNameHint is now currentGoFile
		lowerCurrentGoFileHint = strings.ToLower(filepath.Base(currentGoFile)) // Use Base to match only filename part
		slog.Debug(strings.Repeat("\t", baseIndent+1)+"Using hint for prioritizing files", "hint", lowerCurrentGoFileHint)
	}


	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}

		lowerFileName := strings.ToLower(file.Name())
		fullPath := filepath.Join(dirToScan, file.Name())

		// Prioritize the currentGoFile itself if a hint is given
		if lowerCurrentGoFileHint != "" && lowerFileName == lowerCurrentGoFileHint {
			priorityFiles = append([]string{fullPath}, priorityFiles...) // Prepend to make it highest priority
			slog.Debug(strings.Repeat("\t", baseIndent+2)+"Prioritized file (matches currentGoFile hint)", "path", fullPath)
		} else {
			otherFiles = append(otherFiles, fullPath)
		}
	}

	allFiles := append(priorityFiles, otherFiles...)
	parsedFiles := make([]*ast.File, 0, len(allFiles))
	slog.Debug(strings.Repeat("\t", baseIndent+1)+"Processing files for parsing", "numAllFiles", len(allFiles))

	for i, filePath := range allFiles {
		slog.Debug(strings.Repeat("\t", baseIndent+2)+"Parsing file", "index", i, "path", filePath)
		fileAst, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
		if err != nil {
			slog.Debug(strings.Repeat("\t", baseIndent)+"LoadPackageFiles: end (error parsing file)")
			return nil, fmt.Errorf("failed to parse file %q: %w", filePath, err)
		}
		slog.Debug(strings.Repeat("\t", baseIndent+2)+"Loaded file", "path", filePath) // Matches requested internal log
		parsedFiles = append(parsedFiles, fileAst)
	}

	slog.Debug(strings.Repeat("\t", baseIndent)+"LoadPackageFiles: end")
	return parsedFiles, nil
}

// FindModuleRoot searches upwards from the given filePath for a go.mod file.
// It returns the path to the directory containing the go.mod file.
// If no go.mod is found, it returns an error.
func FindModuleRoot(filePath string, baseIndent int) (string, error) {
	slog.Debug(strings.Repeat("\t", baseIndent)+"FindModuleRoot: start", "startPath", filePath)
	dir := filepath.Dir(filePath)
	if dir == "" || dir == "." || dir == "/" { // Reached root or invalid path
		slog.Debug(strings.Repeat("\t", baseIndent)+"FindModuleRoot: end (error - initial path is root or invalid)")
		return "", fmt.Errorf("go.mod not found for path %s", filePath)
	}

	currentPath := dir
	for {
		slog.Debug(strings.Repeat("\t", baseIndent+1)+"Checking path", "path", currentPath)
		goModPath := filepath.Join(currentPath, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			slog.Debug(strings.Repeat("\t", baseIndent+1)+"Found go.mod", "path", currentPath)
			slog.Debug(strings.Repeat("\t", baseIndent)+"FindModuleRoot: end")
			return currentPath, nil // Found go.mod
		}

		parentDir := filepath.Dir(currentPath)
		if parentDir == currentPath { // Reached root
			slog.Debug(strings.Repeat("\t", baseIndent+1) + "Reached root, go.mod not found")
			break
		}
		currentPath = parentDir
	}
	slog.Debug(strings.Repeat("\t", baseIndent)+"FindModuleRoot: end (error - go.mod not found)")
	return "", fmt.Errorf("go.mod not found upwards from %s", filePath)
}

// GetModuleName parses a go.mod file and returns the module name.
func GetModuleName(moduleRootPath string, baseIndent int) (string, error) {
	slog.Debug(strings.Repeat("\t", baseIndent)+"GetModuleName: start", "moduleRootPath", moduleRootPath)
	goModPath := filepath.Join(moduleRootPath, "go.mod")
	slog.Debug(strings.Repeat("\t", baseIndent+1)+"Reading go.mod file", "path", goModPath)
	modBytes, err := os.ReadFile(goModPath)
	if err != nil {
		slog.Debug(strings.Repeat("\t", baseIndent)+"GetModuleName: end (error reading go.mod)")
		return "", fmt.Errorf("could not read go.mod at %s: %w", goModPath, err)
	}

	// Using a simplified parser for module path. For full parsing, use golang.org/x/mod/modfile.
	// Example: module example.com/mymodule
	var modulePath string
	slog.Debug(strings.Repeat("\t", baseIndent+1) + "Parsing go.mod content for module directive")
	for _, line := range strings.Split(string(modBytes), "\n") {
		if strings.HasPrefix(line, "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			slog.Debug(strings.Repeat("\t", baseIndent+2)+"Found module directive", "modulePath", modulePath)
			break
		}
	}

	if modulePath == "" {
		slog.Debug(strings.Repeat("\t", baseIndent)+"GetModuleName: end (error - module directive not found)")
		return "", fmt.Errorf("module directive not found in %s", goModPath)
	}
	slog.Debug(strings.Repeat("\t", baseIndent)+"GetModuleName: end")
	return modulePath, nil
}
