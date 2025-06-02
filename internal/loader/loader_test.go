package loader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile_Success(t *testing.T) {
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

	fileAST, err := LoadFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if fileAST == nil {
		t.Fatal("LoadFile returned nil AST")
	}
	if fileAST.Name.Name != "main" {
		t.Errorf("Expected package name 'main', got '%s'", fileAST.Name.Name)
	}
}

func TestLoadFile_NonExistentFile(t *testing.T) {
	_, err := LoadFile("non_existent_file.go")
	if err == nil {
		t.Fatal("LoadFile should have failed for a non-existent file, but it did not")
	}
	// Check if the error message is somewhat informative, though exact message depends on os
	if !strings.Contains(err.Error(), "no such file or directory") && !strings.Contains(err.Error(), "cannot find the file") {
		t.Logf("Warning: Error message might not be as expected: %v", err)
	}
}

func TestLoadFile_InvalidGoSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.go")
	content := `
package main
func main() { fmt.Println("Hello" // Missing closing parenthesis
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	_, err := LoadFile(tmpFile)
	if err == nil {
		t.Fatal("LoadFile should have failed for a file with syntax errors, but it did not")
	}
	// Error message from parser usually contains line number and expected token
	if !strings.Contains(err.Error(), "expected ')'") && !strings.Contains(err.Error(), "expected declaration") { // depends on parser error detail
		t.Logf("Warning: Syntax error message might not be as expected: %v", err)
	}
}