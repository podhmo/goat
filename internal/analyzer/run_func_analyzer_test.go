package analyzer

import (
	"go/ast"
	// "go/parser" // No longer used directly in this file
	// "go/token"  // No longer used directly in this file
	"strings"
	"testing"
)

// parseSingleFileAst is defined in options_analyzer_test.go (or a shared test utility file)
// and is accessible to other test files in the same package.

func TestAnalyzeRunFunc_Simple(t *testing.T) {
	content := `
package main

// MyRun is the main logic.
// It does important things.
func MyRun(opts Options) error { return nil }
`
	_, fileAst := parseSingleFileAst(t, content) // fset is not used directly by AnalyzeRunFunc, but helper provides it
	runFuncInfo, doc, err := AnalyzeRunFunc([]*ast.File{fileAst}, "MyRun")

	if err != nil {
		t.Fatalf("AnalyzeRunFunc failed: %v", err)
	}
	if runFuncInfo == nil {
		t.Fatal("runFuncInfo is nil")
	}

	expectedDoc := "MyRun is the main logic.\nIt does important things."
	if doc != expectedDoc {
		t.Errorf("Expected doc comment '%s', got '%s'", expectedDoc, doc)
	}
	if runFuncInfo.Name != "MyRun" {
		t.Errorf("Expected func name 'MyRun', got '%s'", runFuncInfo.Name)
	}
	if runFuncInfo.OptionsArgName != "opts" {
		t.Errorf("Expected options arg name 'opts', got '%s'", runFuncInfo.OptionsArgName)
	}
	if runFuncInfo.OptionsArgType != "Options" {
		t.Errorf("Expected options arg type 'Options', got '%s'", runFuncInfo.OptionsArgType)
	}
}

func TestAnalyzeRunFunc_WithContext(t *testing.T) {
	content := `
package main

import "context"

// RunWithCtx executes with context.
func RunWithCtx(ctx context.Context, appOpts AppOptions) error { return nil }
`
	_, fileAst := parseSingleFileAst(t, content)
	runFuncInfo, doc, err := AnalyzeRunFunc([]*ast.File{fileAst}, "RunWithCtx")

	if err != nil {
		t.Fatalf("AnalyzeRunFunc failed: %v", err)
	}
	if runFuncInfo == nil {
		t.Fatal("runFuncInfo is nil")
	}

	expectedDoc := "RunWithCtx executes with context."
	if doc != expectedDoc {
		t.Errorf("Expected doc comment '%s', got '%s'", expectedDoc, doc)
	}
	if runFuncInfo.Name != "RunWithCtx" {
		t.Errorf("Expected func name 'RunWithCtx', got '%s'", runFuncInfo.Name)
	}
	if runFuncInfo.ContextArgName != "ctx" {
		t.Errorf("Expected context arg name 'ctx', got '%s'", runFuncInfo.ContextArgName)
	}
	if runFuncInfo.ContextArgType != "context.Context" {
		t.Errorf("Expected context arg type 'context.Context', got '%s'", runFuncInfo.ContextArgType)
	}
	if runFuncInfo.OptionsArgName != "appOpts" {
		t.Errorf("Expected options arg name 'appOpts', got '%s'", runFuncInfo.OptionsArgName)
	}
	if runFuncInfo.OptionsArgType != "AppOptions" {
		t.Errorf("Expected options arg type 'AppOptions', got '%s'", runFuncInfo.OptionsArgType)
	}
}

func TestAnalyzeRunFunc_NotFound(t *testing.T) {
	content := `package main; func SomeOtherFunc() {}`
	_, fileAst := parseSingleFileAst(t, content)
	_, _, err := AnalyzeRunFunc([]*ast.File{fileAst}, "NonExistentRun")
	if err == nil {
		t.Fatal("AnalyzeRunFunc should have failed for a non-existent function")
	}
	if !strings.Contains(err.Error(), "NonExistentRun' not found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestAnalyzeRunFunc_InvalidSignature(t *testing.T) {
	content := `package main; func MyRun() error { return nil }` // No params
	_, fileAst := parseSingleFileAst(t, content)
	_, _, err := AnalyzeRunFunc([]*ast.File{fileAst}, "MyRun")
	if err == nil {
		t.Fatal("AnalyzeRunFunc should have failed for invalid signature")
	}
	if !strings.Contains(err.Error(), "unexpected signature") {
		t.Errorf("Unexpected error message for invalid signature: %v", err)
	}
}
