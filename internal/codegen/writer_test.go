package codegen_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp" // Added for regexes
	"strings" // Added for strings
	"testing"
	"go/format" // Added for format.Source in normalizeCode
	"bytes"     // Added for log capture
	"log/slog"  // Added for slog

	"github.com/podhmo/goat/internal/codegen"
	"github.com/stretchr/testify/assert" // Added for assertions
	// Assuming normalizeCode is in the same package or adjust import
)

// Helper functions (copied from main_generator_test.go as writer_test.go is in a different package)
var (
	lineCommentRegex = regexp.MustCompile(`//.*`)
	// whitespaceRegex matches all whitespace characters, including newlines.
	// It's used to replace any sequence of whitespace with a single space.
	whitespaceRegex = regexp.MustCompile(`\s+`)
)

// normalizeForContains prepares a code snippet for robust substring checking.
// It removes comments, replaces various whitespace with single spaces, and trims.
func normalizeForContains(snippet string) string {
	// Remove Go line comments first to prevent // from becoming part of a word.
	var noCommentsLines []string
	for _, line := range strings.Split(snippet, "\n") {
		if idx := strings.Index(line, "//"); idx != -1 {
			noCommentsLines = append(noCommentsLines, line[:idx])
		} else {
			noCommentsLines = append(noCommentsLines, line)
		}
	}
	processed := strings.Join(noCommentsLines, " ") // Join with space to process as a single "line"

	// Replace tabs with spaces first to ensure uniform space characters.
	processed = strings.ReplaceAll(processed, "\t", " ")
	// Compact all sequences of whitespace (now including newlines replaced by spaces) into a single space.
	processed = whitespaceRegex.ReplaceAllString(processed, " ")
	return strings.TrimSpace(processed)
}

// normalizeCode formats the actual generated Go code string.
func normalizeCode(t *testing.T, code string) string {
	t.Helper()
	formatted, err := format.Source([]byte(code))
	if err != nil {
		// If go/format.Source fails on the actual generated code, it's a critical error.
		t.Fatalf("Failed to format actual generated code: %v\nOriginal code:\n%s", err, code)
	}
	// After gofmt, further normalize for robust comparison (remove comments, compact whitespace)
	return normalizeForContains(string(formatted))
}
// End of helper functions

// createTempFile creates a temporary Go file with the given initial content.
// It returns the path to the temporary file.
func createTempFile(t *testing.T, initialContent string) string {
	t.Helper()
	tempDir := t.TempDir() // Auto-cleaned up
	tempFile, err := os.Create(filepath.Join(tempDir, "test_program.go"))
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tempFile.Close()

	if _, err := tempFile.WriteString(initialContent); err != nil {
		t.Fatalf("Failed to write initial content to temp file: %v", err)
	}
	return tempFile.Name()
}

// findMainFuncDecl finds the *ast.FuncDecl and its token.Position for the main function.
// Returns nil if not found.
func findMainFuncDecl(fset *token.FileSet, fileAst *ast.File) (*ast.FuncDecl, *token.Position) {
	for _, decl := range fileAst.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
			pos := fset.Position(fn.Pos()) // Get the position of 'func' keyword
			return fn, &pos
		}
	}
	return nil, nil
}

func TestWriteMain_ReplaceExistingMain(t *testing.T) {
	initialContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, old world!")
}
`
	newMainContent := `
func main() {
	fmt.Println("Hello, new world!")
	fmt.Println("This is the new main.")
}`
	expectedContentAfterWrite := `package main

import "fmt"

func main() {
	fmt.Println("Hello, new world!")
	fmt.Println("This is the new main.")
}
`
	tempFilePath := createTempFile(t, initialContent)
	fset := token.NewFileSet()

	fileAst, err := parser.ParseFile(fset, tempFilePath, []byte(initialContent), parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse initial content: %v", err)
	}

	_, mainFuncPos := findMainFuncDecl(fset, fileAst)
	if mainFuncPos == nil {
		t.Fatal("Test setup error: main function not found in initial content.")
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, mainFuncPos, 0)
	if err != nil {
		t.Fatalf("WriteMain failed: %v", err)
	}

	modifiedContentBytes, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	if normalizeCode(t, string(modifiedContentBytes)) != normalizeCode(t, expectedContentAfterWrite) {
		t.Errorf("Content mismatch.\nExpected:\n%s\n\nGot:\n%s",
			normalizeCode(t, expectedContentAfterWrite),
			normalizeCode(t, string(modifiedContentBytes)))
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "WriteMain: start")
	assert.Contains(t, logOutput, "\tMain function position provided, attempting to replace existing main")
	assert.Contains(t, logOutput, "\tProcessing (goimports) the generated code")
	assert.Contains(t, logOutput, "WriteMain: end")
}

func TestWriteMain_ReplaceMainWithComments(t *testing.T) {
	initialContent := `package main // Package comment

import "fmt" // Import related comment

// This is a doc comment for old main.
func main() {
	// Inner comment
	fmt.Println("Hello, old world!")
}

// Another function
func helper() {}
`
	newMainContent := `
// This is a doc comment for new main.
func main() {
	// New inner comment
	fmt.Println("Hello, new world!")
}`
	// The doc comment of the old main is replaced along with the function itself.
	// Other comments (package, import, other functions) should remain.
	expectedContentAfterWrite := `package main // Package comment

import "fmt" // Import related comment

// This is a doc comment for new main.
func main() {
	// New inner comment
	fmt.Println("Hello, new world!")
}

// Another function
func helper() {}
`
	tempFilePath := createTempFile(t, initialContent)
	fset := token.NewFileSet()

	fileAst, err := parser.ParseFile(fset, tempFilePath, []byte(initialContent), parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse initial content: %v", err)
	}

	_, mainFuncPos := findMainFuncDecl(fset, fileAst)
	if mainFuncPos == nil {
		t.Fatal("Test setup error: main function not found in initial content.")
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, mainFuncPos, 0)
	if err != nil {
		t.Fatalf("WriteMain failed: %v", err)
	}

	modifiedContentBytes, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	if normalizeCode(t, string(modifiedContentBytes)) != normalizeCode(t, expectedContentAfterWrite) {
		t.Errorf("Content mismatch.\nExpected:\n%s\n\nGot:\n%s",
			normalizeCode(t, expectedContentAfterWrite),
			normalizeCode(t, string(modifiedContentBytes)))
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "WriteMain: start")
	assert.Contains(t, logOutput, "\tMain function node found, proceeding with line-by-line replacement")
	assert.Contains(t, logOutput, "WriteMain: end")
}

func TestWriteMain_AppendNewMain(t *testing.T) {
	initialContent := `package main

import "log"

func someOtherFunc() {
	log.Println("This file has no main yet.")
}
`
	newMainContent := `
func main() {
	log.Println("This is the new main, appended.")
}`
	// Expected: initial content, a newline (if not present), then new main content, then formatted.
	expectedContentAfterWrite := `package main

import "log"

func someOtherFunc() {
	log.Println("This file has no main yet.")
}

func main() {
	log.Println("This is the new main, appended.")
}
`
	tempFilePath := createTempFile(t, initialContent)
	fset := token.NewFileSet()
	// Parse the file to get fileAst, even though mainFuncPos will be nil
	fileAst, err := parser.ParseFile(fset, tempFilePath, []byte(initialContent), parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse initial content: %v", err)
	}

	// mainFuncPos is nil because main is not found
	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, nil, 0)
	if err != nil {
		t.Fatalf("WriteMain failed: %v", err)
	}

	modifiedContentBytes, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	normalizedModified := normalizeCode(t, string(modifiedContentBytes))
	normalizedExpected := normalizeCode(t, expectedContentAfterWrite)

	if normalizedModified != normalizedExpected {
		fmt.Println("GOT:\n", string(modifiedContentBytes)) // print raw for easier diff
		fmt.Println("EXPECTED:\n", expectedContentAfterWrite)
		t.Errorf("Content mismatch.\nExpected (normalized):\n%s\n\nGot (normalized):\n%s",
			normalizedExpected,
			normalizedModified)
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "WriteMain: start")
	assert.Contains(t, logOutput, "\tMain function position not provided, appending new content")
	assert.Contains(t, logOutput, "WriteMain: end")
}

func TestWriteMain_LineByLine_PreserveFormatting(t *testing.T) {
	initialContent := `package main

import "fmt"

// Comment before main line 1
// Comment before main line 2

// Another comment block
// with multiple lines.

func main() {
	// Old main content
	// To be replaced completely.
	fmt.Println("Old Main Content Here")
	// Another line in old main.
}

// Comment immediately after main's original closing brace.
// Another comment after main.

var GlobalVarAfterMain = "test value"

// Func after main
func AnotherFunctionAfterMain() {
	fmt.Println("This is another function, after main.")
}

// Trailing comment at EOF.
`
	newMainContent := `func main() {
	fmt.Println("New Main Content Here")
	// New main has its own comments.
}`
	// Expected content after WriteMain, assuming gofmt formatting.
	// format.Source in WriteMain will handle gofmt.
	expectedContentAfterWrite := `package main

import "fmt"

// Comment before main line 1
// Comment before main line 2

// Another comment block
// with multiple lines.

func main() {
	fmt.Println("New Main Content Here")
	// New main has its own comments.
}

// Comment immediately after main's original closing brace.
// Another comment after main.

var GlobalVarAfterMain = "test value"

// Func after main
func AnotherFunctionAfterMain() {
	fmt.Println("This is another function, after main.")
}

// Trailing comment at EOF.
`
	tempFilePath := createTempFile(t, initialContent)
	fset := token.NewFileSet()

	// Provide initial content bytes to ParseFile, as it might be modified by WriteMain
	initialBytes, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read initial temp file content: %v", err)
	}

	fileAst, err := parser.ParseFile(fset, tempFilePath, initialBytes, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse initial content: %v", err)
	}

	mainFuncNode, mainFuncPos := findMainFuncDecl(fset, fileAst)
	if mainFuncNode == nil || mainFuncPos == nil {
		t.Fatal("Test setup error: main function not found in initial content.")
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, mainFuncPos, 0)
	if err != nil {
		t.Fatalf("WriteMain failed: %v", err)
	}

	modifiedContentBytes, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	// Normalize both expected and actual for a robust comparison
	// (handles gofmt differences, trailing newlines consistently)
	normalizedGot := normalizeCode(t, string(modifiedContentBytes))
	normalizedExpected := normalizeCode(t, expectedContentAfterWrite)

	if normalizedGot != normalizedExpected {
		// For easier debugging, print raw strings if they differ
		t.Logf("RAW EXPECTED:\n%s\n", expectedContentAfterWrite)
		t.Logf("RAW GOT:\n%s\n", string(modifiedContentBytes))
		t.Errorf("Content mismatch after WriteMain.\nEXPECTED (normalized):\n%s\nGOT (normalized):\n%s",
			normalizedExpected, normalizedGot)
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "WriteMain: start")
	assert.Contains(t, logOutput, "\tMain function node found, proceeding with line-by-line replacement")
	assert.Contains(t, logOutput, "WriteMain: end")
}

func TestWriteMain_FileWithOtherDeclarations(t *testing.T) {
	initialContent := `package main

import "fmt"

var GlobalVar = "initial value"

const MyConstant = 123

func utilityFunc() {
	fmt.Println("This is a utility function.")
}

type MyStruct struct {
	Field int
}

func main() {
	fmt.Println("Old main referring to", GlobalVar, MyConstant)
	utilityFunc()
}

func anotherUtility() {
	fmt.Println("Another utility.")
}
`
	newMainContent := `
func main() {
	fmt.Println("New main logic here.")
	fmt.Println(GlobalVar, MyConstant) // It can still refer to globals
}`
	expectedContentAfterWrite := `package main

import "fmt"

var GlobalVar = "initial value"

const MyConstant = 123

func utilityFunc() {
	fmt.Println("This is a utility function.")
}

type MyStruct struct {
	Field int
}

func main() {
	fmt.Println("New main logic here.")
	fmt.Println(GlobalVar, MyConstant) // It can still refer to globals
}

func anotherUtility() {
	fmt.Println("Another utility.")
}
`
	tempFilePath := createTempFile(t, initialContent)
	fset := token.NewFileSet()

	fileAst, err := parser.ParseFile(fset, tempFilePath, []byte(initialContent), parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse initial content: %v", err)
	}

	_, mainFuncPos := findMainFuncDecl(fset, fileAst)
	if mainFuncPos == nil {
		t.Fatal("Test setup error: main function not found in initial content.")
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, mainFuncPos, 0)
	if err != nil {
		t.Fatalf("WriteMain failed: %v", err)
	}

	modifiedContentBytes, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	if normalizeCode(t, string(modifiedContentBytes)) != normalizeCode(t, expectedContentAfterWrite) {
		t.Errorf("Content mismatch.\nExpected:\n%s\n\nGot:\n%s",
			normalizeCode(t, expectedContentAfterWrite),
			normalizeCode(t, string(modifiedContentBytes)))
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "WriteMain: start")
	assert.Contains(t, logOutput, "WriteMain: end")
}

func TestWriteMain_EmptyFileInput(t *testing.T) {
	initialContent := ""
	newMainContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello from a new file!")
}`
	// Expected is just the new main content, formatted.
	expectedContentAfterWrite := newMainContent

	tempFilePath := createTempFile(t, initialContent)
	fset := token.NewFileSet()
	var fileAst *ast.File = nil
	var err error

	if initialContent != "" {
		// Parsing an empty string might result in a non-nil fileAst with no decls.
		fileAst, err = parser.ParseFile(fset, tempFilePath, []byte(initialContent), parser.ParseComments)
		if err != nil {
			t.Fatalf("Failed to parse initial content: %v", err)
		}
	}
	// If initialContent is empty, fileAst remains nil.
	// mainFuncPos is nil as the file is empty or main is not present.

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, nil, 0)
	if err != nil {
		t.Fatalf("WriteMain failed: %v", err)
	}

	modifiedContentBytes, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read modified file: %v", err)
	}

	normalizedModified := normalizeCode(t, string(modifiedContentBytes))
	normalizedExpected := normalizeCode(t, expectedContentAfterWrite)

	if normalizedModified != normalizedExpected {
		t.Errorf("Content mismatch.\nExpected:\n%s\n\nGot:\n%s",
			normalizedExpected,
			normalizedModified)
	}

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "WriteMain: start")
	assert.Contains(t, logOutput, "\tMain function position not provided, appending new content")
	assert.Contains(t, logOutput, "WriteMain: end")
}
