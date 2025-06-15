package interpreter

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"strconv" // Added strconv import

	"github.com/podhmo/goat/internal/loader"
	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/astutils"
)

// InterpretInitializer analyzes the AST of an options initializer function (e.g., NewOptions)
// to extract default values and enum choices by "interpreting" calls to goat.Default() and goat.Enum().
// It modifies the passed cmdMetadata.Options directly.
func InterpretInitializer(
	ctx context.Context,
	fileAst *ast.File,
	optionsStructName string,
	initializerFuncName string,
	options []*metadata.OptionMetadata,
	markerPkgImportPath string, // e.g., "github.com/podhmo/goat"
	currentPkgPath string, // Import path of the package being processed
	loader *loader.Loader, // Loader to resolve identifiers
) error {
	var initializerFunc *ast.FuncDecl
	ast.Inspect(fileAst, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == initializerFuncName {
			initializerFunc = fn
			return false
		}
		return true
	})

	if initializerFunc == nil {
		return fmt.Errorf("initializer function '%s' not found", initializerFuncName)
	}

	if initializerFunc.Body == nil {
		return fmt.Errorf("initializer function '%s' has no body", initializerFuncName)
	}

	optionsMap := make(map[string]*metadata.OptionMetadata)
	for _, opt := range options {
		optionsMap[opt.Name] = opt
	}

	// TODO: This is a very simplified interpreter.
	// It should handle variable assignments that eventually set fields on the options struct.
	// For now, let's assume direct assignment like:
	// return &Options{ Field: goat.Default(...), ... }
	// or
	// opts := &Options{}
	// opts.Field = goat.Default(...)
	// return opts

	slog.InfoContext(ctx, fmt.Sprintf("Interpreting initializer: %s", initializerFuncName))

	ast.Inspect(initializerFunc.Body, func(n ast.Node) bool {
		switch stmtNode := n.(type) {
		case *ast.AssignStmt: // e.g. options.Field = goat.Default(...) or var x = goat.Default(...)
			// We need to trace assignments to see if they end up in an OptionMetadata field
			// This example focuses on direct assignments to struct fields.
			// E.g., `opt.MyField = goat.Default("value")`
			if len(stmtNode.Lhs) == 1 && len(stmtNode.Rhs) == 1 {
				if selExpr, ok := stmtNode.Lhs[0].(*ast.SelectorExpr); ok {
					// Assuming selExpr.X is the options struct variable, selExpr.Sel is the field name
					fieldName := selExpr.Sel.Name
					if optMeta, exists := optionsMap[fieldName]; exists {
						slog.InfoContext(ctx, fmt.Sprintf("Found assignment to options field: %s", fieldName))
						extractMarkerInfo(ctx, stmtNode.Rhs[0], optMeta, fileAst, markerPkgImportPath, loader, currentPkgPath)
					}
				}
			}

		case *ast.ReturnStmt: // e.g. return &Options{ Field: goat.Default(...) }
			if len(stmtNode.Results) == 1 {
				actualExpr := stmtNode.Results[0]
				if unaryExpr, ok := actualExpr.(*ast.UnaryExpr); ok && unaryExpr.Op == token.AND {
					actualExpr = unaryExpr.X
				}

				if compLit, ok := actualExpr.(*ast.CompositeLit); ok {
					// Check if this composite literal is for our Options struct
					// This requires resolving compLit.Type to optionsStructName, which can be complex.
					// For a simpler start, assume if it's a struct literal in NewOptions, it's the one.
					slog.InfoContext(ctx, fmt.Sprintf("Found return composite literal in %s", initializerFuncName))
					for _, elt := range compLit.Elts {
						if kvExpr, ok := elt.(*ast.KeyValueExpr); ok {
							if keyIdent, ok := kvExpr.Key.(*ast.Ident); ok {
								fieldName := keyIdent.Name
								if optMeta, exists := optionsMap[fieldName]; exists {
									extractMarkerInfo(ctx, kvExpr.Value, optMeta, fileAst, markerPkgImportPath, loader, currentPkgPath)
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	return nil
}

// extractMarkerInfo extracts default value and enum choices from a marker function call.
func extractMarkerInfo(
	ctx context.Context,
	valueExpr ast.Expr,
	optMeta *metadata.OptionMetadata,
	fileAst *ast.File,
	markerPkgImportPath string,
	loader *loader.Loader,
	currentPkgPath string,
) {
	callExpr, ok := valueExpr.(*ast.CallExpr)
	if !ok {
		// Value is not a function call, could be a direct literal (TODO: handle direct literals as defaults)
		// Check if it's an identifier that needs resolution (e.g. o.MyEnum = MyEnumValues)
		// Corrected: Pass ctx to EvaluateArg
		evalRes := astutils.EvaluateArg(ctx, valueExpr)
		if evalRes.IdentifierName != "" {
			// This is an attempt to handle cases like `FieldName: MyEnumVariable`
			// where MyEnumVariable itself is a slice. This is complex because optMeta.Type
			// might not be known yet to confirm it's a slice type.
			// For now, we log and might need a separate mechanism or rely on `goat.Enum(MyEnumVariable)`.
			slog.InfoContext(ctx, fmt.Sprintf("  Field %s is assigned an identifier '%s' (pkg '%s') directly. If this is an enum, use goat.Enum(%s) or ensure type information is available for resolution.", optMeta.Name, evalRes.IdentifierName, evalRes.PkgName, evalRes.IdentifierName))
		} else if evalRes.Value != nil {
			// It's a literal value assigned directly, e.g. `FieldName: "defaultValue"`
			// This could be a default value.
			// We need to be careful not to overwrite defaults set by goat.Default() if that's preferred.
			// For now, let's assume goat.X markers are the primary source of metadata.
			slog.InfoContext(ctx, fmt.Sprintf("  Field %s is assigned a literal value '%v' directly. This might be a default, but typically use goat.Default() for clarity.", optMeta.Name, evalRes.Value))
		}
		return
	}

	markerFuncName, markerPkgAlias := astutils.GetFullFunctionName(callExpr.Fun)
	actualMarkerPkgPath := astutils.GetImportPath(fileAst, markerPkgAlias)

	// Allow original goat path or the one used in cmd/goat tests via testcmdmodule
	isKnownMarkerPackage := (actualMarkerPkgPath == markerPkgImportPath || // e.g. "github.com/podhmo/goat"
		actualMarkerPkgPath == "testcmdmodule/internal/goat") // For cmd/goat tests

	if !isKnownMarkerPackage {
		slog.InfoContext(ctx, fmt.Sprintf("  Call is to package '%s' (alias '%s'), not the recognized marker package(s) ('%s' or 'testcmdmodule/internal/goat')", actualMarkerPkgPath, markerPkgAlias, markerPkgImportPath))
		return
	}

	switch markerFuncName {
	case "Default":
		slog.InfoContext(ctx, fmt.Sprintf("Interpreting goat.Default for field %s (current Pkg: %s)", optMeta.Name, currentPkgPath))
		if len(callExpr.Args) > 0 {
			// Default value is the first argument
			defaultArgExpr := callExpr.Args[0]
			defaultEvalResult := astutils.EvaluateArg(ctx, defaultArgExpr)

			// Check if the option field itself is a pointer type
			isPointerField := optMeta.IsPointer

			if isPointerField {
				slog.InfoContext(ctx, fmt.Sprintf("  Field %s is a pointer type (TypeName: %s). Attempting to extract underlying default value.", optMeta.Name, optMeta.TypeName))
				// If the field is a pointer, we want the underlying value.
				// Case 1: Argument is a call to a helper function, e.g., stringPtr("value")
				if innerCall, ok := defaultArgExpr.(*ast.CallExpr); ok {
					slog.InfoContext(ctx, fmt.Sprintf("  Default for pointer field %s is via helper call: %s", optMeta.Name, astutils.ExprToTypeName(innerCall.Fun)))
					if len(innerCall.Args) > 0 {
						innerArgEvalResult := astutils.EvaluateArg(ctx, innerCall.Args[0])
						if innerArgEvalResult.IdentifierName == "" { // Literal or directly evaluatable
							optMeta.DefaultValue = innerArgEvalResult.Value
							slog.InfoContext(ctx, fmt.Sprintf("  Default value (from pointer helper call arg): %v for field %s", optMeta.DefaultValue, optMeta.Name))
						} else { // Argument to helper is an identifier
							slog.InfoContext(ctx, fmt.Sprintf("  Argument to pointer helper for field %s is identifier '%s' (pkg '%s'). Attempting resolution.", optMeta.Name, innerArgEvalResult.IdentifierName, innerArgEvalResult.PkgName))
							resolvedVal, success := resolveEvalResultToEnumString(ctx, innerArgEvalResult, loader, currentPkgPath, fileAst) // Using existing resolver
							if success {
								optMeta.DefaultValue = resolvedVal
								slog.InfoContext(ctx, fmt.Sprintf("  Successfully resolved identifier (from pointer helper arg) for field %s: %v", optMeta.Name, optMeta.DefaultValue))
							} else {
								slog.WarnContext(ctx, fmt.Sprintf("  Failed to resolve identifier (from pointer helper arg) '%s' for field %s. DefaultValue will be nil.", innerArgEvalResult.IdentifierName, optMeta.Name))
								optMeta.DefaultValue = nil
							}
						}
					} else {
						slog.WarnContext(ctx, fmt.Sprintf("  Pointer helper call for field %s has no arguments. Using raw eval of the call itself: %v", optMeta.Name, defaultEvalResult.Value))
						optMeta.DefaultValue = defaultEvalResult.Value // Fallback to whatever EvaluateArg made of the call itself
					}
				} else if unaryExpr, ok := defaultArgExpr.(*ast.UnaryExpr); ok && unaryExpr.Op == token.AND {
					// Case 2: Argument is an address-of expression, e.g., &myVar or &"literal" (if "literal" was a const/var)
					slog.InfoContext(ctx, fmt.Sprintf("  Default for pointer field %s is via address-of operator (&). Evaluating inner expression.", optMeta.Name))
					valueInsideAddrOf := astutils.EvaluateArg(ctx, unaryExpr.X)
					if valueInsideAddrOf.IdentifierName == "" { // Literal or directly evaluatable
						optMeta.DefaultValue = valueInsideAddrOf.Value
						slog.InfoContext(ctx, fmt.Sprintf("  Default value (from &expr): %v for field %s", optMeta.DefaultValue, optMeta.Name))
					} else { // Inner part of &expr is an identifier
						slog.InfoContext(ctx, fmt.Sprintf("  Inner expression of & for field %s is identifier '%s' (pkg '%s'). Attempting resolution.", optMeta.Name, valueInsideAddrOf.IdentifierName, valueInsideAddrOf.PkgName))
						resolvedVal, success := resolveEvalResultToEnumString(ctx, valueInsideAddrOf, loader, currentPkgPath, fileAst) // Using existing resolver
						if success {
							optMeta.DefaultValue = resolvedVal
							slog.InfoContext(ctx, fmt.Sprintf("  Successfully resolved identifier (from &expr) for field %s: %v", optMeta.Name, optMeta.DefaultValue))
						} else {
							slog.WarnContext(ctx, fmt.Sprintf("  Failed to resolve identifier (from &expr) '%s' for field %s. DefaultValue will be nil.", valueInsideAddrOf.IdentifierName, optMeta.Name))
							optMeta.DefaultValue = nil
						}
					}
				} else {
					// Case 3: Field is a pointer, but default arg is not a helper call or &expr.
					// It might be a direct variable that is already a pointer, or a literal that EvaluateArg handled.
					// The goal is to store the *dereferenced* value, but if it's just a variable, resolving its pointed-to value here is complex.
					// Trust defaultEvalResult for now, which might be the pointer value itself or what EvaluateArg could determine.
					// This path means we are not explicitly dereferencing a known pointer-creating pattern here.
					slog.InfoContext(ctx, fmt.Sprintf("  Default for pointer field %s is (value: %v, ident: '%s', pkg: '%s'). Using direct evaluation or attempting resolution if identifier.", optMeta.Name, defaultEvalResult.Value, defaultEvalResult.IdentifierName, defaultEvalResult.PkgName))
					if defaultEvalResult.IdentifierName == "" {
						optMeta.DefaultValue = defaultEvalResult.Value // This might be nil if original was `var p *string; ... Default(p)`
						slog.InfoContext(ctx, fmt.Sprintf("  Default value (direct for pointer field): %v for field %s", optMeta.DefaultValue, optMeta.Name))
					} else {
						slog.InfoContext(ctx, fmt.Sprintf("  Default value for pointer field %s is an identifier '%s' (pkg '%s'). Attempting resolution.", optMeta.Name, defaultEvalResult.IdentifierName, defaultEvalResult.PkgName))
						resolvedVal, success := resolveEvalResultToEnumString(ctx, defaultEvalResult, loader, currentPkgPath, fileAst)
						if success {
							optMeta.DefaultValue = resolvedVal // This would be the string value if resolved.
							slog.InfoContext(ctx, fmt.Sprintf("  Successfully resolved identifier for pointer field %s: %v", optMeta.Name, optMeta.DefaultValue))
						} else {
							slog.WarnContext(ctx, fmt.Sprintf("  Failed to resolve identifier '%s' for pointer field %s. DefaultValue will be nil.", defaultEvalResult.IdentifierName, optMeta.Name))
							optMeta.DefaultValue = nil
						}
					}
				}
			} else { // Field is not a pointer type (original logic)
				if defaultEvalResult.IdentifierName == "" { // If it's a literal or directly evaluatable value
					optMeta.DefaultValue = defaultEvalResult.Value
					slog.InfoContext(ctx, fmt.Sprintf("  Default value: %v for field %s", optMeta.DefaultValue, optMeta.Name))
				} else { // Default value is an identifier
					slog.InfoContext(ctx, fmt.Sprintf("  Default value for field %s is an identifier '%s' (pkg '%s'). Attempting resolution.", optMeta.Name, defaultEvalResult.IdentifierName, defaultEvalResult.PkgName))
					resolvedStrVal, success := resolveEvalResultToEnumString(ctx, defaultEvalResult, loader, currentPkgPath, fileAst)
					if success {
						optMeta.DefaultValue = resolvedStrVal
						slog.DebugContext(ctx, fmt.Sprintf("Successfully resolved identifier default value for field %s: %v", optMeta.Name, optMeta.DefaultValue))
					} else {
						slog.DebugContext(ctx, fmt.Sprintf("Failed to resolve identifier default value for '%s' (field %s). DefaultValue will be nil.", defaultEvalResult.IdentifierName, optMeta.Name))
						optMeta.DefaultValue = nil
					}
				}
			}

			// Subsequent args could be an Enum call for enumConstraint
			if len(callExpr.Args) > 1 {
				enumArg := callExpr.Args[1]
				if enumInnerCallExpr, ok := enumArg.(*ast.CallExpr); ok { // goat.Default("val", goat.Enum(MyEnumVar))
					innerFuncName, innerPkgAlias := astutils.GetFullFunctionName(enumInnerCallExpr.Fun)
					resolvedInnerPkgPath := astutils.GetImportPath(fileAst, innerPkgAlias)
					// Check if it's the specific marker package and function name "Enum"
					isGoatEnumCall := (resolvedInnerPkgPath == markerPkgImportPath || resolvedInnerPkgPath == "testcmdmodule/internal/goat") && innerFuncName == "Enum"

					if isGoatEnumCall {
						if len(enumInnerCallExpr.Args) > 0 {
							// Corrected: Pass ctx to EvaluateSliceArg
							evalResult := astutils.EvaluateSliceArg(ctx, enumInnerCallExpr.Args[0])
							extractEnumValuesFromEvalResult(ctx, evalResult, optMeta, fileAst, loader, currentPkgPath, "Default (via goat.Enum)")
						}
					} else {
						slog.DebugContext(ctx, fmt.Sprintf("Second argument to goat.Default for field %s is a call to %s.%s, not goat.Enum. Ignoring for enum constraints.", optMeta.Name, innerPkgAlias, innerFuncName))
					}
				} else { // goat.Default("val", MyEnumVarOrSliceLiteral)
					// Corrected: Pass ctx to EvaluateSliceArg
					enumEvalResult := astutils.EvaluateSliceArg(ctx, enumArg)
					if enumEvalResult.Value != nil {
						if s, ok := enumEvalResult.Value.([]any); ok {
							optMeta.EnumValues = s
							slog.InfoContext(ctx, fmt.Sprintf("  Enum values for Default from direct evaluation: %v", optMeta.EnumValues))
						} else {
							slog.InfoContext(ctx, fmt.Sprintf("  Enum values for Default for field %s from direct evaluation was not []any, but %T", optMeta.Name, enumEvalResult.Value))
						}
					} else if enumEvalResult.IdentifierName != "" {
						slog.InfoContext(ctx, fmt.Sprintf("  Enum constraint for Default for field %s is an identifier '%s' (pkg '%s'). Attempting resolution.", optMeta.Name, enumEvalResult.IdentifierName, enumEvalResult.PkgName))
						// Resolve the identifier for enum values.
						// fileAst is the AST of the file where goat.Default is called.
						// currentPkgPath is the import path of this file.
						extractEnumValuesFromEvalResult(ctx, enumEvalResult, optMeta, fileAst, loader, currentPkgPath, "Default (direct ident)")
					} else {
						// This case handles where enumEvalResult.Value is nil AND enumEvalResult.IdentifierName is empty.
						slog.InfoContext(ctx, fmt.Sprintf("  Enum argument for Default for field %s (type %T) could not be evaluated to a literal slice or a resolvable identifier. EvalResult: %+v", optMeta.Name, enumArg, enumEvalResult))
					}
				}
			}
		}
	case "Enum":
		slog.DebugContext(ctx, fmt.Sprintf("Interpreting goat.Enum for field %s (current Pkg: %s)", optMeta.Name, currentPkgPath))
		var valuesArg ast.Expr
		if len(callExpr.Args) == 1 { // goat.Enum(MyEnumValuesVarOrLiteral)
			valuesArg = callExpr.Args[0]
		} else if len(callExpr.Args) == 2 { // goat.Enum((*MyType)(nil), MyEnumValuesVarOrLiteral)
			// The second argument is the slice of enum values
			valuesArg = callExpr.Args[1]
		} else {
			slog.DebugContext(ctx, fmt.Sprintf("Warning: goat.Enum for field %s called with unexpected number of arguments: %d. Expected 1 or 2.", optMeta.Name, len(callExpr.Args)))
			return // or break, depending on desired error handling
		}

		if valuesArg != nil {
			// Corrected: Pass ctx to EvaluateSliceArg
			evalResult := astutils.EvaluateSliceArg(ctx, valuesArg)

			// Check if EvaluateSliceArg could not resolve valuesArg into a simple slice
			// This happens if valuesArg is a composite literal with identifiers, e.g., []customtypes.MyCustomEnum{customtypes.OptionX}
			if evalResult.Value == nil && evalResult.IdentifierName == "" {
				if compLit, ok := valuesArg.(*ast.CompositeLit); ok {
					slog.DebugContext(ctx, fmt.Sprintf("Enum for field %s is a composite literal. Attempting to resolve elements.", optMeta.Name))
					var resolvedEnumStrings []any
					for _, elt := range compLit.Elts {
						// Corrected: Pass ctx to EvaluateArg
						elementEvalResult := astutils.EvaluateArg(ctx, elt)
						// fileAst is the AST of the file where goat.Enum is called.
						// currentPkgPath is the import path of this file.
						// Pass fileAst as fileAstForContext for resolving package aliases within elt if it's a qualified identifier.
						strVal, success := resolveEvalResultToEnumString(ctx, elementEvalResult, loader, currentPkgPath, fileAst)
						if success {
							resolvedEnumStrings = append(resolvedEnumStrings, strVal)
						} else {
							slog.DebugContext(ctx, fmt.Sprintf("Warning: Could not resolve enum element '%s' for field %s in composite literal. Element EvalResult: %+v", astutils.ExprToTypeName(elt), optMeta.Name, elementEvalResult))
						}
					}
					if len(resolvedEnumStrings) > 0 {
						optMeta.EnumValues = resolvedEnumStrings
						slog.DebugContext(ctx, fmt.Sprintf("Successfully resolved enum values from composite literal for field %s: %v", optMeta.Name, optMeta.EnumValues))
					} else {
						slog.DebugContext(ctx, fmt.Sprintf("Warning: Composite literal for enum field %s did not yield any resolvable string values.", optMeta.Name))
					}
				} else {
					slog.DebugContext(ctx, fmt.Sprintf("Warning: Enum argument for field %s could not be processed as a slice or composite literal. Arg type: %T. EvalResult: %+v", optMeta.Name, valuesArg, evalResult))
				}
			} else {
				// Existing logic: valuesArg was either a literal slice evaluatable by EvaluateSliceArg,
				// or an identifier pointing to a slice (e.g., goat.Enum(MyEnumVariable)).
				extractEnumValuesFromEvalResult(ctx, evalResult, optMeta, fileAst, loader, currentPkgPath, "Enum (direct)")
			}
		}
	case "File":
		slog.DebugContext(ctx, fmt.Sprintf("Interpreting goat.File for field %s", optMeta.Name))
		if len(callExpr.Args) > 0 {
			// Corrected: Pass ctx to EvaluateArg
			fileArgEvalResult := astutils.EvaluateArg(ctx, callExpr.Args[0])
			if fileArgEvalResult.IdentifierName == "" {
				optMeta.DefaultValue = fileArgEvalResult.Value
				slog.InfoContext(ctx, fmt.Sprintf("  Default path: %v", optMeta.DefaultValue))
			} else {
				slog.InfoContext(ctx, fmt.Sprintf("  Default path for field %s is an identifier '%s' (pkg '%s'). Resolution of identifiers for file paths is not yet implemented here. DefaultValue will be nil.", optMeta.Name, fileArgEvalResult.IdentifierName, fileArgEvalResult.PkgName))
				optMeta.DefaultValue = nil
			}
			optMeta.TypeName = "string" // File paths are strings

			// Subsequent args are FileOption calls (e.g., goat.MustExist(), goat.GlobPattern())
			if len(callExpr.Args) > 1 {
				for _, arg := range callExpr.Args[1:] {
					if optionCallExpr, ok := arg.(*ast.CallExpr); ok {
						optionFuncName, optionFuncPkgAlias := astutils.GetFullFunctionName(optionCallExpr.Fun)
						actualOptionFuncPkgPath := astutils.GetImportPath(fileAst, optionFuncPkgAlias)

						if actualOptionFuncPkgPath == markerPkgImportPath { // Ensure it's a goat.Xxx call
							switch optionFuncName {
							case "MustExist":
								optMeta.FileMustExist = true
								slog.InfoContext(ctx, fmt.Sprintf("  FileOption: MustExist"))
							case "GlobPattern":
								optMeta.FileGlobPattern = true
								slog.InfoContext(ctx, fmt.Sprintf("  FileOption: GlobPattern"))
							default:
								slog.InfoContext(ctx, fmt.Sprintf("  Unknown FileOption: %s", optionFuncName))
							}
						}
					}
				}
			}
		}
	default:
		// Not a recognized marker function from the specified package
		slog.DebugContext(ctx, fmt.Sprintf("Not a goat marker function: %s.%s", markerPkgAlias, markerFuncName))
	}
}

// extractEnumValuesFromEvalResult is a helper to resolve enum values from EvalResult.
// It populates optMeta.EnumValues if resolution is successful.

// resolveEvalResultToEnumString takes an EvalResult (typically from astutils.EvaluateArg
// called on an individual enum value like customtypes.OptionX) and resolves it to its
// underlying string value. This is used when enums are defined via composite literals
// with identifiers.
func resolveEvalResultToEnumString(
	ctx context.Context,
	elementEvalResult astutils.EvalResult,
	loader *loader.Loader,
	currentPkgPath string, // Package path where the goat.Enum call is made or where the variable holding the enum is defined
	fileAstForContext *ast.File, // *ast.File where the identifier is used (for resolving local package aliases)
) (string, bool) {
	slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] --- ENTER --- EvalResult: %+v, currentPkgPath: %s", elementEvalResult, currentPkgPath))

	if elementEvalResult.Value != nil {
		if strVal, ok := elementEvalResult.Value.(string); ok {
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Value is direct string: \"%s\"", strVal))
			return strVal, true
		}
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Warning: elementEvalResult.Value is not a string, but %T (%v). Cannot use as enum string.", elementEvalResult.Value, elementEvalResult.Value))
		return "", false
	}

	if elementEvalResult.IdentifierName == "" {
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Value is nil and IdentifierName is empty. EvalResult: %+v", elementEvalResult))
		return "", false
	}

	// IdentifierName is present, resolve it using loader.LookupSymbol
	identName := elementEvalResult.IdentifierName
	pkgAlias := elementEvalResult.PkgName
	targetPkgPath := ""

	if pkgAlias != "" { // Qualified identifier like mypkg.MyConst
		// fileAstForContext is the AST of the file where the identifier (pkgAlias.identName) is used.
		// This context is needed to resolve the pkgAlias to its full import path.
		if fileAstForContext == nil {
			slog.ErrorContext(ctx, "[resolveEvalResultToEnumString] Error: fileAstForContext is nil for a qualified identifier. Cannot resolve package alias.", "identifier", identName, "pkgAlias", pkgAlias)
			return "", false
		}
		resolvedImportPath := astutils.GetImportPath(fileAstForContext, pkgAlias)
		if resolvedImportPath == "" {
			slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Error: Could not resolve import path for package alias.", "pkgAlias", pkgAlias, "identifier", identName, "contextFile", fileAstForContext.Name.Name)
			return "", false
		}
		targetPkgPath = resolvedImportPath
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Resolved package alias '%s' to import path '%s' for identifier '%s'", pkgAlias, targetPkgPath, identName))
	} else { // Unqualified identifier, assume current package context
		targetPkgPath = currentPkgPath
		if targetPkgPath == "" {
			slog.ErrorContext(ctx, "[resolveEvalResultToEnumString] Error: Current package path is empty. Cannot resolve unqualified identifier.", "identifier", identName)
			return "", false
		}
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Identifier '%s' is unqualified, using current package path '%s'", identName, targetPkgPath))
	}

	fullSymbolName := targetPkgPath + ":" + identName
	slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Looking up symbol: '%s'", fullSymbolName))

	symInfo, found := loader.LookupSymbol(fullSymbolName)
	if !found {
		slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Symbol not found in loader cache.", "fullSymbolName", fullSymbolName)
		return "", false
	}

	slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Found symbol in loader.", "fullSymbolName", fullSymbolName, "filePath", symInfo.FilePath, "nodeType", fmt.Sprintf("%T", symInfo.Node))

	valSpec, ok := symInfo.Node.(*ast.ValueSpec)
	if !ok {
		slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Symbol is not an ast.ValueSpec.", "fullSymbolName", fullSymbolName, "actualNodeType", fmt.Sprintf("%T", symInfo.Node))
		return "", false
	}

	// Enhanced Debugging for ValueSpec content
	var specNames []string
	for _, name := range valSpec.Names {
		specNames = append(specNames, name.Name)
	}
	var specValuesStr []string
	if valSpec.Values != nil {
		for _, val := range valSpec.Values {
			specValuesStr = append(specValuesStr, astutils.ExprToTypeName(val)) // Using ExprToTypeName for a compact representation
		}
	}
	slog.DebugContext(ctx, "[resolveEvalResultToEnumString] ValueSpec details:",
		"fullSymbolName", fullSymbolName,
		"identToFind", identName,
		"specNames", specNames,
		"specValues", specValuesStr,
		"specFilePath", symInfo.FilePath,
		"numNamesInSpec", len(valSpec.Names),
		"numValuesInSpec", len(valSpec.Values))

	// Find the specific name in the ValueSpec (e.g., const ( A = "a", B = "b" ), we need B )
	for i, nameIdent := range valSpec.Names {
		if nameIdent.Name == identName {
			slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Matched identName in ValueSpec.", "identName", identName, "index", i)
			if len(valSpec.Values) > i {
				valueNode := valSpec.Values[i]
				if basicLit, ok := valueNode.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
					unquotedVal, errUnquote := strconv.Unquote(basicLit.Value)
					if errUnquote != nil {
						slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Error unquoting string for const/var.", "fullSymbolName", fullSymbolName, "rawValue", basicLit.Value, "error", errUnquote)
						return "", false
					}
					slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Successfully resolved identifier '%s' to string value: \"%s\"", fullSymbolName, unquotedVal))
					return unquotedVal, true
				} else if callExpr, ok := valueNode.(*ast.CallExpr); ok {
					// Handle string conversions like string(MyConstString)
					// Expects callExpr.Fun to be "string" and callExpr.Args[0] to be the const identifier
					if identFun, ok := callExpr.Fun.(*ast.Ident); ok && identFun.Name == "string" && len(callExpr.Args) == 1 {
						slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Value for '%s' is a string cast. Attempting to resolve argument: %s", fullSymbolName, astutils.ExprToTypeName(callExpr.Args[0])))
						// The argument to string() might be another identifier (e.g. string(AnotherConst))
						// We need to evaluate this argument in the context of the *defining* file of the original const/var.
						// symInfo.FilePath gives us the file where `fullSymbolName` is defined.
						// symInfo.PackagePath is the package path for that file.
						// loader.GetAST(symInfo.FilePath) will give the AST for that file.
						argFileAst, argFileAstFound := loader.GetAST(symInfo.FilePath)
						if !argFileAstFound {
							slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Could not get AST for file where original const/var is defined. Cannot resolve string cast argument.", "filePath", symInfo.FilePath, "constVar", fullSymbolName)
							return "", false
						}

						argEvalResult := astutils.EvaluateArg(ctx, callExpr.Args[0])
						// Recursively call resolveEvalResultToEnumString for the argument of string()
						// The context for this recursive call:
						// - elementEvalResult: the result of evaluating the argument (e.g., `AnotherConst`)
						// - loader: same loader
						// - currentPkgPath: package path of the file where `fullSymbolName` (and thus the string cast) is defined (`symInfo.PackagePath`)
						// - fileAstForContext: AST of the file where `fullSymbolName` is defined (`argFileAst`), needed if `AnotherConst` is itself qualified.
						return resolveEvalResultToEnumString(ctx, argEvalResult, loader, symInfo.PackagePath, argFileAst)
					}
					slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Const/var value is a CallExpr but not a recognized string() cast.", "fullSymbolName", fullSymbolName, "call", astutils.ExprToTypeName(callExpr))
					return "", false
				}

				slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Const/var is not a basic string literal or string() cast.", "fullSymbolName", fullSymbolName, "valueNodeType", fmt.Sprintf("%T", valueNode))
				return "", false
			}
			slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Const/var spec has no corresponding value.", "fullSymbolName", fullSymbolName, "nameIndex", i)
			return "", false
		}
	}

	slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Identifier name not found within the resolved ValueSpec's Names.", "fullSymbolName", fullSymbolName, "identName", identName)
	return "", false
}

// resolveConstStringValue searches for a constant `constName` in the given `pkg`
// and returns its string value if it's a basic literal string.
func resolveConstStringValue(ctx context.Context, constName string, pkg *loader.Package, identFile *ast.File) (string, bool) {
	pkgFiles, err := pkg.Files()
	if err != nil {
		slog.DebugContext(ctx, fmt.Sprintf("Error getting files for package '%s' to resolve const '%s': %v", pkg.ImportPath, constName, err))
		return "", false
	}

	for _, fileAst := range pkgFiles {
		// If identFile is provided and not the current file, skip (const must be in the same file as usage for this simple resolution)
		// This is a simplification. Proper resolution would check all files if const is exported, or only current file if unexported.
		// For now, let's assume consts are resolved from anywhere in the package.
		// if identFile != nil && fileAst != identFile {
		// continue
		// }

		var foundVal string
		var declFound bool
		ast.Inspect(fileAst, func(node ast.Node) bool {
			if genDecl, ok := node.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
				for _, spec := range genDecl.Specs {
					if valSpec, ok := spec.(*ast.ValueSpec); ok {
						for i, nameIdent := range valSpec.Names {
							if nameIdent.Name == constName {
								declFound = true // Found the const declaration
								if len(valSpec.Values) > i {
									// Try to evaluate the constant's value
									// We expect it to be a basic literal string.
									// Corrected: Pass ctx to EvaluateArg
									constValEval := astutils.EvaluateArg(ctx, valSpec.Values[i])
									if strVal, ok := constValEval.Value.(string); ok {
										foundVal = strVal
										return false // Stop inspection, value found
									}
									slog.DebugContext(ctx, fmt.Sprintf("Const '%s' in package '%s' is not a direct string literal, actual type %T", constName, pkg.ImportPath, constValEval.Value))
								} else {
									slog.DebugContext(ctx, fmt.Sprintf("Const '%s' in package '%s' has no value", constName, pkg.ImportPath))
								}
								return false // Stop for this const name
							}
						}
					}
				}
			}
			return true // Continue inspection
		})
		if declFound && foundVal != "" {
			return foundVal, true
		}
		if declFound { // Found declaration but not a usable string value
			return "", false
		}
	}
	slog.DebugContext(ctx, fmt.Sprintf("Const '%s' not found in package '%s'", constName, pkg.ImportPath))
	return "", false
}

func extractEnumValuesFromEvalResult(
	ctx context.Context,
	evalResult astutils.EvalResult,
	optMeta *metadata.OptionMetadata,
	fileAst *ast.File, // AST of the current file (where the marker is)
	loader *loader.Loader,
	currentPkgPath string, // Import path of the package where the marker is defined
	markerType string, // For logging context (e.g., "Flag", "Arg", "Enum", "Default")
) {
	if evalResult.Value != nil {
		if s, ok := evalResult.Value.([]any); ok {
			optMeta.EnumValues = s
			slog.DebugContext(ctx, fmt.Sprintf("Enum values for %s (field %s) from literal slice: %v", markerType, optMeta.Name, optMeta.EnumValues))
		} else {
			slog.DebugContext(ctx, fmt.Sprintf("Error: Enum argument for %s (field %s) evaluated to a non-slice value: %T (%v)", markerType, optMeta.Name, evalResult.Value, evalResult.Value))
		}
		return
	}

	if evalResult.IdentifierName != "" {
		slog.DebugContext(ctx, fmt.Sprintf("Enum argument for %s (field %s) is an identifier '%s' (pkg '%s'), attempting loader resolution", markerType, optMeta.Name, evalResult.IdentifierName, evalResult.PkgName))
		targetPkgPath := ""
		if evalResult.PkgName != "" { // Qualified identifier like mypkg.MyEnumVar
			// fileAst is the AST of the file where the goat.Enum marker is called (the "using" file).
			resolvedImportPath := astutils.GetImportPath(fileAst, evalResult.PkgName)
			if resolvedImportPath == "" {
				slog.WarnContext(ctx, "Could not resolve import path for package alias.", "pkgAlias", evalResult.PkgName, "identifier", evalResult.IdentifierName, "contextFile", fileAst.Name.Name, "field", optMeta.Name)
				return
			}
			targetPkgPath = resolvedImportPath
		} else { // Unqualified identifier, assume current package
			targetPkgPath = currentPkgPath
			if targetPkgPath == "" {
				slog.ErrorContext(ctx, "Current package path is empty, cannot resolve unqualified identifier.", "identifier", evalResult.IdentifierName, "field", optMeta.Name)
				return
			}
		}

		fullSymbolName := targetPkgPath + ":" + evalResult.IdentifierName
		slog.DebugContext(ctx, fmt.Sprintf("Looking up symbol for enum variable: '%s'", fullSymbolName))

		symInfo, found := loader.LookupSymbol(fullSymbolName)
		if !found {
			slog.WarnContext(ctx, "Enum variable symbol not found in cache.", "fullSymbolName", fullSymbolName, "field", optMeta.Name)
			return
		}

		slog.DebugContext(ctx, fmt.Sprintf("Found symbol for enum variable '%s'. FilePath: '%s', Node Type: %T", fullSymbolName, symInfo.FilePath, symInfo.Node))

		valSpec, ok := symInfo.Node.(*ast.ValueSpec)
		if !ok {
			slog.WarnContext(ctx, "Symbol for enum variable is not a ValueSpec.", "fullSymbolName", fullSymbolName, "nodeType", fmt.Sprintf("%T", symInfo.Node), "field", optMeta.Name)
			return
		}

		// Find the specific name in the ValueSpec (e.g., var ( A = ..., B = ... ), we need B )
		nameIdx := -1
		for i, nameIdent := range valSpec.Names {
			if nameIdent.Name == evalResult.IdentifierName {
				nameIdx = i
				break
			}
		}

		if nameIdx == -1 {
			slog.ErrorContext(ctx, "Identifier name not found within the resolved ValueSpec's Names (should not happen if LookupSymbol worked).", "fullSymbolName", fullSymbolName, "identifierName", evalResult.IdentifierName, "field", optMeta.Name)
			return
		}

		if len(valSpec.Values) <= nameIdx {
			slog.WarnContext(ctx, "Enum variable ValueSpec has no corresponding initializer value.", "fullSymbolName", fullSymbolName, "nameIndex", nameIdx, "field", optMeta.Name)
			return
		}
		initializerExpr := valSpec.Values[nameIdx]

		if compLit, ok := initializerExpr.(*ast.CompositeLit); ok {
			slog.DebugContext(ctx, fmt.Sprintf("Enum variable '%s' is a composite literal. Processing elements.", fullSymbolName))
			var tempValues []any
			var someElementsFailed bool

			// Get the AST of the file where the enum variable is defined.
			// This is crucial for resolving any package aliases used *within* the composite literal elements.
			definingFileAST, astFound := loader.GetAST(symInfo.FilePath)
			if !astFound {
				slog.ErrorContext(ctx, "AST for defining file of enum variable not found in cache. Cannot resolve composite literal elements accurately.", "filePath", symInfo.FilePath, "fullSymbolName", fullSymbolName, "field", optMeta.Name)
				return // Cannot proceed without the defining file's AST for context
			}

			for _, elt := range compLit.Elts {
				eltStrForLog := astutils.ExprToTypeName(elt) // For logging
				var strVal string
				var success bool

				// Check if elt is string(IDENTIFIER)
				if callExpr, ok := elt.(*ast.CallExpr); ok {
					if funIdent, ok := callExpr.Fun.(*ast.Ident); ok && funIdent.Name == "string" && len(callExpr.Args) == 1 {
						slog.DebugContext(ctx, fmt.Sprintf("Enum element '%s' in var '%s' is a string cast. Resolving argument.", eltStrForLog, fullSymbolName))
						argAstNode := callExpr.Args[0]
						argEvalResult := astutils.EvaluateArg(ctx, argAstNode)
						// Resolve the argument of string() using the defining file's context
						// symInfo.PackagePath is the package where 'fullSymbolName' (the slice var) is defined.
						// definingFileAST is the AST of that file. This context is correct for resolving argEvalResult.
						strVal, success = resolveEvalResultToEnumString(ctx, argEvalResult, loader, symInfo.PackagePath, definingFileAST)
					} else {
						// It's some other CallExpr, try to evaluate it directly and then resolve
						elementEvalResult := astutils.EvaluateArg(ctx, elt)
						strVal, success = resolveEvalResultToEnumString(ctx, elementEvalResult, loader, symInfo.PackagePath, definingFileAST)
					}
				} else {
					// Not a CallExpr, proceed as before
					elementEvalResult := astutils.EvaluateArg(ctx, elt)
					strVal, success = resolveEvalResultToEnumString(ctx, elementEvalResult, loader, symInfo.PackagePath, definingFileAST)
				}

				if success {
					tempValues = append(tempValues, strVal)
					slog.DebugContext(ctx, fmt.Sprintf("Successfully resolved enum element '%s' for var '%s' to: \"%s\"", eltStrForLog, fullSymbolName, strVal))
				} else {
					originalElementEvalResult := astutils.EvaluateArg(ctx, elt) // Re-evaluate for logging if needed, or pass down
					slog.WarnContext(ctx, "Failed to resolve enum element for var.", "element", eltStrForLog, "var", fullSymbolName, "elementEvalResult", fmt.Sprintf("%+v", originalElementEvalResult))
					someElementsFailed = true
				}
			}

			if len(tempValues) > 0 {
				optMeta.EnumValues = tempValues
				slog.DebugContext(ctx, fmt.Sprintf("Successfully resolved enum variable '%s' from composite literal to values: %v", fullSymbolName, optMeta.EnumValues))
			} else if !someElementsFailed {
				slog.DebugContext(ctx, fmt.Sprintf("Enum variable '%s' from composite literal resolved to an empty list (or all elements failed to resolve to strings).", fullSymbolName))
				optMeta.EnumValues = []any{} // Explicitly set to empty if no values but no failures
			}
			if someElementsFailed && len(tempValues) == 0 { // All elements failed
				slog.WarnContext(ctx, "All elements of composite literal for enum variable failed to resolve.", "fullSymbolName", fullSymbolName, "field", optMeta.Name)
			}

		} else {
			// Fallback to original logic if initializer is not a CompositeLit (e.g. alias to another var)
			slog.DebugContext(ctx, fmt.Sprintf("Enum variable '%s' initializer is not a CompositeLit (%T). Attempting fallback with EvaluateSliceArg.", fullSymbolName, initializerExpr))
			resolvedSlice := astutils.EvaluateSliceArg(ctx, initializerExpr) // Pass context
			if resolvedSlice.Value != nil {
				if s, ok := resolvedSlice.Value.([]any); ok {
					optMeta.EnumValues = s
					slog.DebugContext(ctx, fmt.Sprintf("Successfully resolved enum variable '%s' to values (via fallback EvaluateSliceArg): %v", fullSymbolName, optMeta.EnumValues))
				} else {
					slog.WarnContext(ctx, "Enum variable initializer (via fallback) not resolved to []any.", "fullSymbolName", fullSymbolName, "resolvedType", fmt.Sprintf("%T", resolvedSlice.Value), "field", optMeta.Name)
				}
			} else if resolvedSlice.IdentifierName != "" {
				// This means the var was an alias to another var. The current LookupSymbol approach handles one level.
				// If `MyEnumVar = OtherEnumVar`, LookupSymbol gets `MyEnumVar`. If `OtherEnumVar` needs further lookup,
				// that would be transitive. For now, this path indicates that the direct ValueSpec didn't contain a literal.
				slog.WarnContext(ctx, "Enum variable initializer (via fallback) is an alias to another identifier. Transitive resolution is not directly supported by this fallback path; direct LookupSymbol on the original identifier should handle it if it's a var/const.", "fullSymbolName", fullSymbolName, "aliasTo", resolvedSlice.IdentifierName, "aliasPkg", resolvedSlice.PkgName, "field", optMeta.Name)
			} else {
				slog.WarnContext(ctx, "Enum variable initializer (via fallback) does not have a resolvable slice literal or identifier.", "fullSymbolName", fullSymbolName, "initializerType", fmt.Sprintf("%T", initializerExpr), "field", optMeta.Name)
			}
		}
		return
	}

	// Neither Value nor IdentifierName is set in the initial evalResult for the enum variable itself
	slog.WarnContext(ctx, "Enum argument could not be evaluated to a literal slice or a resolvable identifier.", "field", optMeta.Name, "markerType", markerType, "evalResult", fmt.Sprintf("%+v", evalResult))
}
