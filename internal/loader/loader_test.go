package loader

import (
	"bytes" // Added for log capture
	"go/token"
	"log/slog" // Added for slog
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert" // Added for assertions
)

func TestLoadFile_Success(t *testing.T) {
	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	// Create a temporary Go file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	content := `
package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	fset := token.NewFileSet()
	fileAST, err := LoadFile(fset, tmpFile, 0)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if fileAST == nil {
		t.Fatal("LoadFile returned nil AST")
	}
	if fileAST.Name.Name != "main" {
		t.Errorf("Expected package name 'main', got '%s'", fileAST.Name.Name)
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "LoadFile: start")
	assert.Contains(t, logOutput, "LoadFile: end")
}

func TestLoadFile_NonExistentFile(t *testing.T) {
	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	fset := token.NewFileSet()
	_, err := LoadFile(fset, "non_existent_file.go", 0)
	if err == nil {
		t.Fatal("LoadFile should have failed for a non-existent file, but it did not")
	}
	// Check if the error message is somewhat informative, though exact message depends on os
	if !strings.Contains(err.Error(), "no such file or directory") && !strings.Contains(err.Error(), "cannot find the file") {
		t.Logf("Warning: Error message might not be as expected: %v", err)
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "LoadFile: start")
	assert.Contains(t, logOutput, "LoadFile: end (error)")
}

func TestLoadFile_InvalidGoSyntax(t *testing.T) {
	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.go")
	content := `
package main
func main() { fmt.Println("Hello" // Missing closing parenthesis
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	fset := token.NewFileSet()
	_, err := LoadFile(fset, tmpFile, 0)
	if err == nil {
		t.Fatal("LoadFile should have failed for a file with syntax errors, but it did not")
	}
	// Error message from parser usually contains line number and expected token
	if !strings.Contains(err.Error(), "expected ')'") && !strings.Contains(err.Error(), "expected declaration") { // depends on parser error detail
		t.Logf("Warning: Syntax error message might not be as expected: %v", err)
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "LoadFile: start")
	assert.Contains(t, logOutput, "LoadFile: end (error)")
}
