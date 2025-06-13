package astutils

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"strconv"
	"strings"
)

// EvalResult holds the result of an evaluation.
// If Value is not nil, it's a directly evaluated value.
// If IdentifierName is not empty, it means the expression was an identifier
// that needs further resolution (e.g., by a loader).
// PkgName is for qualified identifiers like pkg.MyEnum.
type EvalResult struct {
	Value          any
	IdentifierName string
	PkgName        string // Added for qualified identifiers
}

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

// EvaluateArg evaluates the argument of a function call.
// It returns an EvalResult containing the evaluated argument or identifier information.
// It currently supports basic literals (strings, integers, floats, chars),
// identifiers like true, false, nil, unary expressions for negative numbers, and selector expressions.
func EvaluateArg(arg ast.Expr) EvalResult {
	switch v := arg.(type) {
	case *ast.BasicLit:
		switch v.Kind {
		case token.INT:
			i, err := strconv.ParseInt(v.Value, 0, 64)
			if err == nil {
				return EvalResult{Value: i}
			}
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: failed to parse integer literal %s: %v", v.Value, err))
		case token.FLOAT:
			f, err := strconv.ParseFloat(v.Value, 64)
			if err == nil {
				return EvalResult{Value: f}
			}
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: failed to parse float literal %s: %v", v.Value, err))
		case token.STRING:
			s, err := strconv.Unquote(v.Value)
			if err == nil {
				return EvalResult{Value: s}
			}
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: failed to unquote string literal %s: %v", v.Value, err))
		case token.CHAR:
			s, err := strconv.Unquote(v.Value) // char is rune, often represented as string in Go
			if err == nil && len(s) == 1 {
				return EvalResult{Value: []rune(s)[0]}
			}
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: failed to unquote or invalid char literal %s: %v", v.Value, err))
		default:
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: unsupported basic literal type: %s, returning as raw string", v.Kind))
			return EvalResult{Value: v.Value} // return raw token value as string
		}
		// If parsing failed for supported types, return empty EvalResult
		return EvalResult{}
	case *ast.Ident:
		switch v.Name {
		case "true":
			return EvalResult{Value: true}
		case "false":
			return EvalResult{Value: false}
		case "nil":
			return EvalResult{Value: nil}
		default:
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: identified identifier %s for further resolution", v.Name))
			return EvalResult{IdentifierName: v.Name}
		}
	case *ast.UnaryExpr:
		if v.Op == token.SUB {
			evalRes := EvaluateArg(v.X) // Recursively call EvaluateArg
			if evalRes.Value != nil {
				switch val := evalRes.Value.(type) {
				case int64:
					return EvalResult{Value: -val}
				case float64:
					return EvalResult{Value: -val}
				// Add other numeric types if needed, e.g. int from a const
				case int: // Check if this case is reachable given current literal parsing
					return EvalResult{Value: -val}
				default:
					slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: unary minus operator can only be applied to known numeric types, got %T for value of %s", evalRes.Value, ExprToTypeName(v.X)))
					return EvalResult{}
				}
			}
			// If evalRes.Value is nil, it might be an identifier or unresolved.
			// For now, we don't support unary ops on unresolved identifiers (e.g. -MyConst)
			// unless MyConst resolves to a number. This would require more advanced resolution.
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: unary minus operand %s (type %T) could not be resolved to a value or is an identifier", ExprToTypeName(v.X), v.X))
			return EvalResult{}
		}
		slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: unsupported unary expression operator: %s", v.Op))
		return EvalResult{}
	case *ast.SelectorExpr: // e.g. pkg.MyConst
		if xIdent, okX := v.X.(*ast.Ident); okX { // Should be a package name
			// v.Sel is already *ast.Ident, no type assertion needed here.
			selIdent := v.Sel
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: identified selector %s.%s for further resolution", xIdent.Name, selIdent.Name))
			return EvalResult{IdentifierName: selIdent.Name, PkgName: xIdent.Name}
		}
		slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: unsupported selector expression, expected X to be *ast.Ident but got %T for X in X.Sel", v.X))
		return EvalResult{}
	default:
		slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateArg: argument is not a basic literal, identifier, unary or selector expression, got %T", arg))
		return EvalResult{}
	}
}

// EvaluateSliceArg evaluates a slice argument of a function call.
// It returns an EvalResult. If the argument is a literal slice of simple values,
// EvalResult.Value will contain []any. If it's an identifier (qualified or not)
// that refers to a slice, EvalResult.IdentifierName (and optionally PkgName) will be set.
func EvaluateSliceArg(arg ast.Expr) EvalResult {
	switch v := arg.(type) {
	case *ast.CompositeLit:
		// This is a composite literal, like []string{"a", "b"}
		results := make([]any, 0, len(v.Elts)) // Initialize to empty slice, not nil
		for _, elt := range v.Elts {
			evalRes := EvaluateArg(elt) // Use the updated EvaluateArg
			if evalRes.Value != nil {
				results = append(results, evalRes.Value)
			} else if evalRes.IdentifierName != "" {
				// Element is an identifier, this slice is not purely literal.
				slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateSliceArg: composite literal element %s (pkg %s) is an identifier, direct slice evaluation cannot fully resolve here.", evalRes.IdentifierName, evalRes.PkgName))
				return EvalResult{} // Not a simple literal slice.
			} else {
				// Element could not be evaluated to a value and is not an identifier.
				slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateSliceArg: failed to evaluate element %s of composite literal to a value and it's not an identifier.", ExprToTypeName(elt)))
				return EvalResult{}
			}
		}
		return EvalResult{Value: results}
	case *ast.Ident:
		// This could be an identifier for a slice variable, requires further resolution
		slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateSliceArg: argument is an identifier %s, requires loader resolution", v.Name))
		return EvalResult{IdentifierName: v.Name}
	case *ast.SelectorExpr: // e.g. pkg.MyEnumSlice
		if xIdent, okX := v.X.(*ast.Ident); okX { // Package name
			// v.Sel is already *ast.Ident, no type assertion needed here.
			selIdent := v.Sel
			slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateSliceArg: argument is a selector %s.%s, requires loader resolution", xIdent.Name, selIdent.Name))
			return EvalResult{IdentifierName: selIdent.Name, PkgName: xIdent.Name}
		}
		slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateSliceArg: unsupported selector expression for slice, expected X to be *ast.Ident but got %T for X in X.Sel", v.X))
		return EvalResult{}
	default:
		slog.InfoContext(context.Background(), fmt.Sprintf("EvaluateSliceArg: argument is not a composite literal, identifier, or selector, got %T", arg))
		return EvalResult{}
	}
}
