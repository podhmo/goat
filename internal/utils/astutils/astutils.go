package astutils

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"strconv"
	"strings"
)

// ExprToTypeName converts an ast.Expr (representing a type) to its string representation.
func ExprToTypeName(expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	// This is a simplified version. For full accuracy, `go/types` might be needed,
	// but the project aims to avoid it.
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr: // For types like `pkg.Type`
		return fmt.Sprintf("%s.%s", ExprToTypeName(t.X), t.Sel.Name)
	case *ast.StarExpr: // For pointer types like `*Type`
		return "*" + ExprToTypeName(t.X)
	case *ast.ArrayType: // For slice/array types like `[]Type` or `[5]Type`
		// TODO: Differentiate array and slice if necessary
		return "[]" + ExprToTypeName(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", ExprToTypeName(t.Key), ExprToTypeName(t.Value))
	// TODO: Add other types as needed (FuncType, ChanType, InterfaceType, etc.)
	default:
		return fmt.Sprintf("<unsupported_type_expr: %T>", expr)
	}
}

// IsPointerType checks if an ast.Expr represents a pointer type.
func IsPointerType(expr ast.Expr) bool {
	_, ok := expr.(*ast.StarExpr)
	return ok
}

// GetFullFunctionName extracts package alias and function name from a call expression's Fun field.
// Example: for `pkg.MyFunc()`, returns ("MyFunc", "pkg"). For `MyFunc()`, returns ("MyFunc", "").
func GetFullFunctionName(funExpr ast.Expr) (name string, pkgAlias string) {
	switch f := funExpr.(type) {
	case *ast.Ident: // Local function call
		return f.Name, ""
	case *ast.SelectorExpr: // Package function call (e.g. goat.Default)
		if xIdent, ok := f.X.(*ast.Ident); ok {
			return f.Sel.Name, xIdent.Name
		}
	}
	return "", ""
}

// GetImportPath returns the import path for a given alias (import name) in the file.
// Supports blank imports (_), dot imports (.), and normal/aliased imports.
func GetImportPath(file *ast.File, alias string) string {
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			switch imp.Name.Name {
			case "_":
				// blank import: alias is the last part of the import path
				if alias == lastPathPart(path) {
					return path
				}
			case ".":
				// dot import: alias is the last part of the import path
				if alias == lastPathPart(path) {
					return path
				}
			default:
				// explicit alias
				if alias == imp.Name.Name {
					return path
				}
			}
		} else {
			// normal import: alias is the last part of the import path
			if alias == lastPathPart(path) {
				return path
			}
		}
	}
	return ""
}

// lastPathPart returns the last element of a slash-separated path.
func lastPathPart(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

// EvaluateArg tries to evaluate an AST expression (typically a function argument)
// to a Go literal value. It supports basic literals.
// Returns the evaluated value or nil if not a simple literal.
func EvaluateArg(arg ast.Expr) any {
	switch v := arg.(type) {
	case *ast.BasicLit:
		switch v.Kind {
		case token.INT:
			i, err := strconv.ParseInt(v.Value, 0, 64)
			if err == nil {
				return i
			}
		case token.FLOAT:
			f, err := strconv.ParseFloat(v.Value, 64)
			if err == nil {
				return f
			}
		case token.STRING:
			s, err := strconv.Unquote(v.Value)
			if err == nil {
				return s
			}
		case token.CHAR:
			s, err := strconv.Unquote(v.Value) // char is rune, often represented as string in Go
			if err == nil && len(s) == 1 {
				return []rune(s)[0]
			}
		}
		log.Printf("EvaluateArg: unhandled basic literal kind %v for value %s", v.Kind, v.Value)
		return v.Value // return raw token value as string
	case *ast.Ident:
		// Could be a predefined constant like `true`, `false`, `nil`
		switch v.Name {
		case "true":
			return true
		case "false":
			return false
		case "nil":
			return nil
		default:
			// Assume it's a constant and return its name.
			return v.Name
		}
	case *ast.SelectorExpr: // e.g. customtypes.OptionX
		// For now, assume X is an *ast.Ident (package name)
		// v.X is ast.Expr (e.g. 'customtypes')
		// v.Sel is *ast.Ident (e.g. 'OptionX')
		if pkgIdent, ok := v.X.(*ast.Ident); ok {
			// This is the case we want to handle: Pkg.Const
			// v.Sel should be non-nil for valid code.
			if v.Sel != nil {
				return v.Sel.Name // "OptionX"
			}
			// If v.Sel is nil, it's malformed, log and return nil.
			log.Printf("EvaluateArg: Selector expression %s has a nil Sel identifier", pkgIdent.Name)
		} else {
			// v.X is not a simple identifier. e.g., myStruct.Field.EnumVal
			// We are not handling this for now. Log it.
			selName := "<nil_sel>"
			if v.Sel != nil {
				selName = v.Sel.Name
			}
			log.Printf("EvaluateArg: unhandled selector expression with non-Ident X (%T = %s) and Sel (%s)", v.X, ExprToTypeName(v.X), selName)
		}
	case *ast.UnaryExpr:
		if v.Op == token.SUB {
			// Handle negative numbers
			if xVal := EvaluateArg(v.X); xVal != nil {
				switch num := xVal.(type) {
				case int64:
					return -num
				case float64:
					return -num
				// Add other numeric types if needed
				default:
					log.Printf("EvaluateArg: unhandled unary minus for type %T", xVal)
				}
			}
		}
		log.Printf("EvaluateArg: unhandled unary operator %s", v.Op)
	// TODO: Add *ast.CompositeLit for simple slice/map literals if needed directly as default
	default:
		log.Printf("EvaluateArg: unhandled expression type %T", arg)
	}
	return nil
}

// EvaluateSliceArg tries to evaluate an AST expression (typically an argument to goat.Enum)
// that should be a slice literal (e.g., []string{"a", "b"}) into a []any.
func EvaluateSliceArg(arg ast.Expr) []any {
	compLit, ok := arg.(*ast.CompositeLit)
	if !ok {
		log.Printf("EvaluateSliceArg: argument is not a composite literal, got %T", arg)
		return []any{}
	}
	results := make([]any, 0, len(compLit.Elts))
	for _, elt := range compLit.Elts {
		val := EvaluateArg(elt)
		if val != nil {
			results = append(results, val)
		} else {
			log.Printf("EvaluateSliceArg: could not evaluate element %T in slice", elt)
		}
	}
	return results
}
