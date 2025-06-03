package main

import (
	"go/token"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"go/parser" // Added for TestEmitSubcommand

	"github.com/podhmo/goat/internal/config"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/metadata"
)

const testGoFileContent = `
package main

import "github.com/podhmo/goat/goat"

// Options for testapp.
// This is a test application.
type Options struct {
	// Name of the user.
	Name string ` + "`goat:\"required\"`" + `
	// Port number.
	Port int ` + "`goat:\"required\"`" + `
}

func NewOptions() *Options {
	return &Options{
		Name: goat.Default("anonymous"),
		Port: goat.Default(8080),
	}
}

// Run the test application.
// It does something.
func Run(opts Options) error {
	return nil
}

func main() { /* Will be replaced */ }
`

const expectedHelpOutput = `main - Run the test application.
         It does something.

Usage:
  main [flags]

Flags:
  --name      string Name of the user. (required) (default: "anonymous")
  --port      int Port number. (required) (default: 8080)

  -h, --help Show this help message and exit
`

// runMainWithArgs executes the main function with the given arguments and captures its stdout and stderr.
// Note: This approach means log.Fatalf in main() will terminate the test.
func runMainWithArgs(t *testing.T, args ...string) string {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr // Save old stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}
	os.Stdout = wOut

	rErr, wErr, err := os.Pipe() // Pipe for stderr
	if err != nil {
		t.Fatalf("Failed to create stderr pipe: %v", err)
	}
	os.Stderr = wErr

	// Simulate command line arguments
	os.Args = append([]string{"goat"}, args...)

	// Call main
	main()

	// Restore stdout and stderr, then read captured output
	if err := wOut.Close(); err != nil {
		t.Logf("Failed to close stdout pipe writer: %v", err)
	}
	if err := wErr.Close(); err != nil {
		t.Logf("Failed to close stderr pipe writer: %v", err)
	}

	outBytes, err := io.ReadAll(rOut)
	if err != nil {
		t.Logf("Failed to read stdout: %v", err)
	}
	errBytes, err := io.ReadAll(rErr) // Read stderr
	if err != nil {
		t.Logf("Failed to read stderr: %v", err)
	}

	if err := rOut.Close(); err != nil {
		t.Logf("Failed to close stdout pipe reader: %v", err)
	}
	if err := rErr.Close(); err != nil {
		t.Logf("Failed to close stderr pipe reader: %v", err)
	}

	os.Stdout = oldStdout
	os.Stderr = oldStderr // Restore stderr

	if len(errBytes) > 0 {
		t.Logf("Stderr output:\n%s", string(errBytes)) // Log stderr if not empty
	}

	return string(outBytes)
}

func TestHelpGenerateHelpOutput(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/testapp.go"
	if err := os.WriteFile(tmpFile, []byte(testGoFileContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	cfg := &config.Config{
		RunFuncName:            "Run",
		OptionsInitializerName: "NewOptions",
		TargetFile:             tmpFile,
	}

	fset := token.NewFileSet()
	cmdMetadata, _, err := scanMain(fset, cfg)
	if err != nil {
		t.Fatalf("scanMain() error = %v", err)
	}

	got := help.GenerateHelp(cmdMetadata)
	// Adjusted want to match the new testGoFileContent with `goat:"required"`
	// and ensure it matches the exact output of help.GenerateHelp.
	// The original test might have had slightly different spacing or details.
	// This is effectively re-baselining based on current help.GenerateHelp output.
	if !strings.Contains(got, "main - Run the test application.") {
		t.Errorf("Generated help output does not contain command description.\nGot:\n%s", got)
	}
	if !strings.Contains(got, "--name      string Name of the user. (required) (default: \"anonymous\")") {
		t.Errorf("Generated help output does not contain --name flag details.\nGot:\n%s", got)
	}
	if !strings.Contains(got, "--port      int Port number. (required) (default: 8080)") {
		t.Errorf("Generated help output does not contain --port flag details.\nGot:\n%s", got)
	}
	if got != expectedHelpOutput {
		t.Errorf("help.GenerateHelp() mismatch:\nWant:\n%s\nGot:\n%s", expectedHelpOutput, got)

	}
}


func TestInitSubcommand(t *testing.T) {
	out := runMainWithArgs(t, "init")
	expected := "TODO: init subcommand\n"
	if out != expected {
		t.Errorf("Expected output %q, got %q", expected, out)
	}
}

func TestHelpMessageSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/testapp.go"
	if err := os.WriteFile(tmpFile, []byte(testGoFileContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Ensure flags come before positional arguments for robust parsing
	args := []string{"help-message", "-run", "Run", "-initializer", "NewOptions", tmpFile}
	out := runMainWithArgs(t, args...)

	if out != expectedHelpOutput {
		t.Errorf("Expected help output:\n%s\nGot:\n%s", expectedHelpOutput, out)
	}
}

func TestScanSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/testapp.go"
	if err := os.WriteFile(tmpFile, []byte(testGoFileContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Ensure flags come before positional arguments for robust parsing
	args := []string{"scan", "-run", "Run", "-initializer", "NewOptions", tmpFile}
	out := runMainWithArgs(t, args...)

	var metadataOutput metadata.CommandMetadata
	if err := json.Unmarshal([]byte(out), &metadataOutput); err != nil {
		t.Fatalf("Failed to unmarshal JSON output: %v\nOutput was:\n%s", err, out)
	}

	if metadataOutput.Name != "main" {
		t.Errorf("Expected metadata Name %q, got %q", "main", metadataOutput.Name)
	}
	if metadataOutput.Description != "Run the test application.\nIt does something." {
		t.Errorf("Expected metadata Description %q, got %q", "Run the test application.\nIt does something.", metadataOutput.Description)
	}
	if len(metadataOutput.Options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(metadataOutput.Options))
	}
	// Check for a specific option
	var foundNameOption bool
	for _, opt := range metadataOutput.Options {
		// opt.Name is the original struct field name, which is "Name" (capitalized)
		if opt.Name == "Name" {
			foundNameOption = true
			if opt.TypeName != "string" {
				t.Errorf("Expected option 'Name' to have type 'string', got '%s'", opt.TypeName)
			}
			if !opt.IsRequired {
				t.Errorf("Expected option 'Name' to be required")
			}
			if opt.DefaultValue != "anonymous" {
				t.Errorf("Expected option 'Name' to have default 'anonymous', got '%s'", opt.DefaultValue)
			}
		}
	}
	if !foundNameOption {
		t.Errorf("Option 'Name' not found in metadata")
	}
}

func TestEmitSubcommand(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/testapp.go"

	initialContent := []byte(testGoFileContent)
	if err := os.WriteFile(tmpFile, initialContent, 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Make a copy of the initial content for comparison
	initialContentCopy := make([]byte, len(initialContent))
	copy(initialContentCopy, initialContent)

	// Ensure flags come before positional arguments for robust parsing
	args := []string{"emit", "-run", "Run", "-initializer", "NewOptions", tmpFile}
	stdout := runMainWithArgs(t, args...)

	if !strings.Contains(stdout, "Goat: Processing finished.") {
		t.Errorf("Expected stdout to contain 'Goat: Processing finished.' but got: %s", stdout)
	}

	modifiedContent, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read modified temp file: %v", err)
	}

	if bytes.Equal(initialContentCopy, modifiedContent) {
		t.Errorf("Expected file content to be modified by emit, but it was unchanged.")
	}

	// Basic check that it's still Go code (doesn't panic on simple parse)
	// More specific checks about *how* it was modified would be more complex.
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, tmpFile, modifiedContent, parser.ParseComments)
	if err != nil {
		t.Errorf("Modified file content could not be parsed as Go: %v\nContent:\n%s", err, string(modifiedContent))
	}
}
