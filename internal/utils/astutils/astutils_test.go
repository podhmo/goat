package astutils

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func parseAndFindFirstFuncArgType(t *testing.T, code string, funcName string) ast.Expr {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}
	var targetExpr ast.Expr
	ast.Inspect(f, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
			if fn.Type.Params != nil && len(fn.Type.Params.List) > 0 {
				targetExpr = fn.Type.Params.List[0].Type
				return false
			}
		}
		return true
	})
	if targetExpr == nil {
		t.Fatalf("Could not find func %s or its first argument type", funcName)
	}
	return targetExpr
}

func TestExprToTypeName(t *testing.T) {
	testCases := []struct {
		name     string
		code     string
		funcName string
		expected string
	}{
		{"Ident", `package main; type MyType string; func T(a MyType){}`, "T", "MyType"},
		{"StarExpr", `package main; type MyType string; func T(a *MyType){}`, "T", "*MyType"},
		{"SelectorExpr", `package main; import "io"; func T(a io.Reader){}`, "T", "io.Reader"},
		{"ArrayTypeSlice", `package main; type MyType string; func T(a []MyType){}`, "T", "[]MyType"},
		{"ArrayTypePointerSlice", `package main; type MyType string; func T(a []*MyType){}`, "T", "[]*MyType"},
		{"MapType", `package main; func T(a map[string]int){}`, "T", "map[string]int"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expr := parseAndFindFirstFuncArgType(t, tc.code, tc.funcName)
			actual := ExprToTypeName(expr)
			if actual != tc.expected {
				t.Errorf("Expected type name '%s', got '%s'", tc.expected, actual)
			}
		})
	}
}

func TestIsPointerType(t *testing.T) {
	codeIsPtr := `package main; type MyType int; func PtrFunc(a *MyType){}`
	codeIsNotPtr := `package main; type MyType int; func NonPtrFunc(a MyType){}`

	exprIsPtr := parseAndFindFirstFuncArgType(t, codeIsPtr, "PtrFunc")
	if !IsPointerType(exprIsPtr) {
		t.Error("Expected IsPointerType to be true for *MyType")
	}

	exprIsNotPtr := parseAndFindFirstFuncArgType(t, codeIsNotPtr, "NonPtrFunc")
	if IsPointerType(exprIsNotPtr) {
		t.Error("Expected IsPointerType to be false for MyType")
	}
}

func parseAndFindFirstCallExprFun(t *testing.T, code string, targetVar string) ast.Expr {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("Failed to parse code: %v", err)
	}
	var targetExpr ast.Expr
	ast.Inspect(f, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			if len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
				if ident, ok := assign.Lhs[0].(*ast.Ident); ok && ident.Name == targetVar {
					if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
						targetExpr = call.Fun
						return false
					}
				}
			}
		}
		return true
	})
	if targetExpr == nil {
		t.Fatalf("Could not find call expression assigned to %s", targetVar)
	}
	return targetExpr
}

func TestGetFullFunctionName(t *testing.T) {
	testCases := []struct {
		name         string
		code         string
		varToInspect string // Variable whose assigned CallExpr.Fun we inspect
		expectedName string
		expectedPkg  string
	}{
		{"LocalFunc", `package main; func local() {}; func T() { x := local() }`, "x", "local", ""},
		{"PkgFunc", `package main; import p "pkg.com/lib"; func T() { y := p.Remote() }`, "y", "Remote", "p"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callFunExpr := parseAndFindFirstCallExprFun(t, tc.code, tc.varToInspect)
			actualName, actualPkg := GetFullFunctionName(callFunExpr)
			if actualName != tc.expectedName || actualPkg != tc.expectedPkg {
				t.Errorf("Expected (%s, %s), got (%s, %s)", tc.expectedName, tc.expectedPkg, actualName, actualPkg)
			}
		})
	}
}

func parseFileForImports(t *testing.T, code string) *ast.File {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "testimports.go", code, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("Failed to parse imports: %v", err)
	}
	return f
}

func TestGetImportPath(t *testing.T) {
	code := `
package main

import (
	"fmt"
	myio "io"
	"os"
	_ "github.com/lib/pq" // blank import
	. "github.com/onsi/ginkgo" // dot import
	custom_path "github.com/custom/module/v2"
)
`
	fileAst := parseFileForImports(t, code)
	testCases := []struct {
		alias    string
		expected string
	}{
		{"fmt", "fmt"},
		{"myio", "io"},
		{"os", "os"},
		{"pq", "github.com/lib/pq"},          // Assumes alias matches last part if Name is nil
		{"ginkgo", "github.com/onsi/ginkgo"}, // for dot import, alias is package name
		{"custom_path", "github.com/custom/module/v2"},
		{"nonexistent", ""},
		{"", ""}, // local or builtin
	}

	for _, tc := range testCases {
		t.Run(tc.alias, func(t *testing.T) {
			actual := GetImportPath(fileAst, tc.alias)
			if actual != tc.expected {
				t.Errorf("For alias '%s', expected import path '%s', got '%s'", tc.alias, tc.expected, actual)
			}
		})
	}
}

func parseExpr(t *testing.T, exprStr string) ast.Expr {
	expr, err := parser.ParseExpr(exprStr)
	if err != nil {
		t.Fatalf("Failed to parse expr '%s': %v", exprStr, err)
	}
	return expr
}

func TestEvaluateArg(t *testing.T) {
	testCases := []struct {
		name     string
		exprStr  string
		expected any
	}{
		{"Int", "123", int64(123)},
		{"String", `"hello"`, "hello"},
		{"Float", "123.45", 123.45},
		{"True", "true", true},
		{"False", "false", false},
		{"Nil", "nil", nil},
		// TODO: Add char, negative numbers, etc.
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			expr := parseExpr(t, tc.exprStr)
			actual := EvaluateArg(expr)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("For expr '%s', expected %v (type %T), got %v (type %T)",
					tc.exprStr, tc.expected, tc.expected, actual, actual)
			}
		})
	}
}

func TestEvaluateSliceArg(t *testing.T) {
	testCases := []struct {
		name     string
		exprStr  string
		expected []any
	}{
		{"StringSlice", `[]string{"a", "b", "c"}`, []any{"a", "b", "c"}},
		{"IntSlice", `[]int{1, 2, 3}`, []any{int64(1), int64(2), int64(3)}},
		{"MixedSliceNotDirectlySupportedByBasicLit", `[]any{"a", 1}`, nil}, // EvaluateArg handles elements individually
		{"EmptySlice", `[]string{}`, []any{}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exprNode, err := parser.ParseExpr(tc.exprStr) // Use ParseExpr for slice literals
			if err != nil {
				t.Fatalf("Failed to parse expr %s: %v", tc.exprStr, err)
			}
			actual := EvaluateSliceArg(exprNode)
			if tc.name == "MixedSliceNotDirectlySupportedByBasicLit" { // special case for current limitations
				if len(actual) != 2 || actual[0] != "a" || actual[1] != int64(1) {
					t.Errorf("For expr '%s', expected evaluated elements, got %v", tc.exprStr, actual)
				}
				return
			}

			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("For expr '%s', expected %v, got %v", tc.exprStr, tc.expected, actual)
			}
		})
	}
}
