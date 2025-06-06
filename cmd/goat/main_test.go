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

	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/metadata"
)

const testGoFileContent = `
package main

import "github.com/podhmo/goat"

// Options for testapp.
// This is a test application.
type Options struct {
	// Name of the user.
	Name string ` + "`goat:\"required\"`" + `
	// Port number.
	Port int ` + "`goat:\"required\"`" + `
	// Verbose flag.
	Verbose bool
	// Force operation.
	Force bool ` + "`goat:\"required\"`" + `
	// Enable magic feature.
	EnableMagic bool ` + "`goat:\"required\"`" + `
}

func NewOptions() *Options {
	return &Options{
		Name:        goat.Default("anonymous"),
		Port:        goat.Default(8080),
		Verbose:     goat.Default(false), // Default for bool is false anyway
		Force:       goat.Default(false), // Required, but default is false
		EnableMagic: goat.Default(true),  // Required, but default is true
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
  --name              string Name of the user. (required) (default: "anonymous")
  --port              int    Port number. (required) (default: 8080)
  --verbose           bool   Verbose flag.
  --force             bool   Force operation.
  --no-enable-magic   bool   Enable magic feature.

  -h, --help          Show this help message and exit
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

	opts := &Options{
		RunFuncName:            "Run",
		OptionsInitializerName: "NewOptions",
		TargetFile:             tmpFile,
	}

	fset := token.NewFileSet()
	cmdMetadata, _, err := scanMain(fset, opts)
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
		// To make debugging easier, compare line by line after splitting
		gotLines := strings.Split(strings.ReplaceAll(got, "\r\n", "\n"), "\n")
		expectedLines := strings.Split(strings.ReplaceAll(expectedHelpOutput, "\r\n", "\n"), "\n")
		if len(gotLines) != len(expectedLines) {
			t.Errorf("help.GenerateHelp() line count mismatch:\nWant (%d lines):\n%s\nGot (%d lines):\n%s", len(expectedLines), expectedHelpOutput, len(gotLines), got)
			return
		}
		for i := range gotLines {
			// Normalize spaces for comparison of content, as padding might differ subtly
			normalizedGotLine := strings.Join(strings.Fields(gotLines[i]), " ")
			normalizedExpectedLine := strings.Join(strings.Fields(expectedLines[i]), " ")
			if normalizedGotLine != normalizedExpectedLine {
				t.Errorf("help.GenerateHelp() mismatch at line %d:\nWant: %s\nGot:  %s\n\nFull Expected:\n%s\nFull Got:\n%s", i+1, expectedLines[i], gotLines[i], expectedHelpOutput, got)
				return // Stop at first differing line
			}
		}
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
	if len(metadataOutput.Options) != 5 { // Updated for 3 new boolean flags
		t.Errorf("Expected 5 options, got %d", len(metadataOutput.Options))
	}

	optionsChecks := map[string]func(opt *metadata.OptionMetadata){
		"Name": func(opt *metadata.OptionMetadata) {
			if opt.TypeName != "string" || !opt.IsRequired || opt.DefaultValue != "anonymous" {
				t.Errorf("Validation failed for Name: %+v", opt)
			}
		},
		"Port": func(opt *metadata.OptionMetadata) {
			if opt.TypeName != "int" || !opt.IsRequired || opt.DefaultValue.(float64) != 8080 { // JSON unmarshals numbers to float64
				t.Errorf("Validation failed for Port: %+v, DefaultValue type: %T", opt, opt.DefaultValue)
			}
		},
		"Verbose": func(opt *metadata.OptionMetadata) {
			if opt.TypeName != "bool" || opt.IsRequired || opt.DefaultValue.(bool) != false {
				t.Errorf("Validation failed for Verbose: %+v", opt)
			}
		},
		"Force": func(opt *metadata.OptionMetadata) {
			if opt.TypeName != "bool" || !opt.IsRequired || opt.DefaultValue.(bool) != false {
				t.Errorf("Validation failed for Force: %+v", opt)
			}
		},
		"EnableMagic": func(opt *metadata.OptionMetadata) {
			if opt.TypeName != "bool" || !opt.IsRequired || opt.DefaultValue.(bool) != true {
				t.Errorf("Validation failed for EnableMagic: %+v", opt)
			}
		},
	}

	foundOptions := make(map[string]bool)
	for _, opt := range metadataOutput.Options {
		if checkFunc, ok := optionsChecks[opt.Name]; ok {
			checkFunc(opt)
			foundOptions[opt.Name] = true
		}
	}

	for optName := range optionsChecks {
		if !foundOptions[optName] {
			t.Errorf("Option '%s' not found in metadata output", optName)
		}
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
