package codegen_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/codegen"
	// Assuming normalizeCode is in the same package or adjust import
)

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

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, mainFuncPos)
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

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, mainFuncPos)
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
	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, nil)
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

	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, mainFuncPos)
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

	// Parsing an empty string might result in a non-nil fileAst with no decls.
	fileAst, err := parser.ParseFile(fset, tempFilePath, nil, parser.ParseComments) // Pass nil for []byte for empty
	if err != nil {
		t.Fatalf("Failed to parse initial (empty) content: %v", err)
	}

	// mainFuncPos is nil as the file is empty
	err = codegen.WriteMain(tempFilePath, fset, fileAst, newMainContent, nil)
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
}
