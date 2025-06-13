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
		evalRes := astutils.EvaluateArg(valueExpr) // Use EvaluateArg for single values
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
			defaultEvalResult := astutils.EvaluateArg(callExpr.Args[0])
			if defaultEvalResult.IdentifierName == "" { // If it's a literal or directly evaluatable value
				optMeta.DefaultValue = defaultEvalResult.Value
				slog.InfoContext(ctx, fmt.Sprintf("  Default value: %v", optMeta.DefaultValue))
			} else { // Default value is an identifier
				slog.InfoContext(ctx, fmt.Sprintf("  Default value for field %s is an identifier '%s' (pkg '%s'). Attempting resolution.", optMeta.Name, defaultEvalResult.IdentifierName, defaultEvalResult.PkgName))
				// defaultEvalResult already contains IdentifierName and PkgName
				// fileAst is the AST of the file where goat.Default is called
				// currentPkgPath is the import path of this file
				resolvedStrVal, success := resolveEvalResultToEnumString(ctx, defaultEvalResult, loader, currentPkgPath, fileAst)
				if success {
					optMeta.DefaultValue = resolvedStrVal
					slog.DebugContext(ctx, fmt.Sprintf("Successfully resolved identifier default value: %v", optMeta.DefaultValue))
				} else {
					slog.DebugContext(ctx, fmt.Sprintf("Failed to resolve identifier default value for '%s'. DefaultValue will be nil.", defaultEvalResult.IdentifierName))
					optMeta.DefaultValue = nil
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
							evalResult := astutils.EvaluateSliceArg(enumInnerCallExpr.Args[0])
							extractEnumValuesFromEvalResult(ctx, evalResult, optMeta, fileAst, loader, currentPkgPath, "Default (via goat.Enum)")
						}
					} else {
						slog.DebugContext(ctx, fmt.Sprintf("Second argument to goat.Default for field %s is a call to %s.%s, not goat.Enum. Ignoring for enum constraints.", optMeta.Name, innerPkgAlias, innerFuncName))
					}
				} else { // goat.Default("val", MyEnumVarOrSliceLiteral)
					enumEvalResult := astutils.EvaluateSliceArg(enumArg)
					if enumEvalResult.Value != nil {
						if s, ok := enumEvalResult.Value.([]any); ok {
							optMeta.EnumValues = s
							slog.InfoContext(ctx, fmt.Sprintf("  Enum values for Default from direct evaluation: %v", optMeta.EnumValues))
						} else {
							slog.InfoContext(ctx, fmt.Sprintf("  Enum values for Default for field %s from direct evaluation was not []any, but %T", optMeta.Name, enumEvalResult.Value))
						}
					} else if enumEvalResult.IdentifierName != "" {
						slog.InfoContext(ctx, fmt.Sprintf("  Enum constraint for Default for field %s is an identifier '%s' (pkg '%s'). Loader resolution for this case is not yet fully implemented in Default.", optMeta.Name, enumEvalResult.IdentifierName, enumEvalResult.PkgName))
						// Per subtask, log that loader resolution for Default's direct identifier enum is not yet fully implemented.
						// If we wanted to implement it, we would call:
						// extractEnumValuesFromEvalResult(ctx, enumEvalResult, optMeta, fileAst, loader, currentPkgPath, "Default (direct ident)")
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
			evalResult := astutils.EvaluateSliceArg(valuesArg)

			// Check if EvaluateSliceArg could not resolve valuesArg into a simple slice
			// This happens if valuesArg is a composite literal with identifiers, e.g., []customtypes.MyCustomEnum{customtypes.OptionX}
			if evalResult.Value == nil && evalResult.IdentifierName == "" {
				if compLit, ok := valuesArg.(*ast.CompositeLit); ok {
					slog.DebugContext(ctx, fmt.Sprintf("Enum for field %s is a composite literal. Attempting to resolve elements.", optMeta.Name))
					var resolvedEnumStrings []any
					for _, elt := range compLit.Elts {
						elementEvalResult := astutils.EvaluateArg(elt) // Evaluate each element
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
			fileArgEvalResult := astutils.EvaluateArg(callExpr.Args[0])
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
	slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] --- ENTER --- EvalResult: %+v", elementEvalResult))

	if elementEvalResult.Value != nil {
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Path A1 (Value is not nil)"))
		if strVal, ok := elementEvalResult.Value.(string); ok {
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Value is direct string: \"%s\"", strVal))
			return strVal, true
		}
		// If Value is not nil but not a string, it's an unexpected type for an enum string.
		if elementEvalResult.Value != nil {
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Warning: elementEvalResult.Value is not a string, but %T (%v). Cannot use as enum string.", elementEvalResult.Value, elementEvalResult.Value))
			return "", false
		}
		// If Value is nil, then IdentifierName must be present. // This comment needs review based on structure
		if elementEvalResult.IdentifierName == "" { // This path is only reachable if Value is non-nil and not a string due to the return above.
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Path A2 (Value is not nil, not string, and IdentifierName is empty) - Error. EvalResult: %+v", elementEvalResult))
			return "", false
		}
		// If Value was non-nil, not a string, and IdentifierName was not empty, it would fall through Block 1.
		// This is an undesirable fallthrough from block A.
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Path A3 (Value is not nil, not string, and IdentifierName is NOT empty) - Potential Fallthrough from Block A. EvalResult: %+v", elementEvalResult))
		// This path should ideally not continue to IdentifierName processing without returning false,
		// as Value was present but unusable. For now, let it fall to the next section.
	}

	// Value is nil path (or Path A3 fallthrough)
	slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Path B (Value is nil or fell through A3). EvalResult: %+v", elementEvalResult))
	if elementEvalResult.IdentifierName == "" {
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Path B1 (IdentifierName is empty). EvalResult: %+v", elementEvalResult))
		return "", false
	}

	// IdentifierName is present.
	slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Path B2 (IdentifierName is NOT empty: '%s'). Processing as identifier.", elementEvalResult.IdentifierName))
	// This 'if' is somewhat redundant if logic flows correctly, but good for explicit block.
	if elementEvalResult.IdentifierName != "" {
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Path B2-MAIN (Executing main logic for identifier '%s')", elementEvalResult.IdentifierName))
		identName := elementEvalResult.IdentifierName
		pkgAlias := elementEvalResult.PkgName
		// slog.DebugContext(ctx, fmt.Sprintf("  [resolveEvalResultToEnumString] Resolving identifier '%s' (pkg alias '%s') from package '%s' using context file '%s'", identName, pkgAlias, currentPkgPath, fileAstForContext.Name.Name)) // Original detailed log
		targetPkgPath := ""
		if pkgAlias != "" { // Qualified identifier like mypkg.MyConst
			resolvedImportPath := astutils.GetImportPath(fileAstForContext, pkgAlias)
			if resolvedImportPath == "" {
				slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Could not resolve import path for package alias '%s' in file '%s' (used for enum element const '%s')", pkgAlias, fileAstForContext.Name.Name, identName))
				return "", false
			}
			targetPkgPath = resolvedImportPath
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Resolved package alias '%s' to import path '%s' for identifier '%s'", pkgAlias, targetPkgPath, identName))
		} else { // Unqualified identifier, assume current package context
			targetPkgPath = currentPkgPath
			if targetPkgPath == "" {
				slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Current package path ('%s') is empty or invalid, cannot resolve unqualified identifier '%s'", currentPkgPath, identName))
				return "", false
			}
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Identifier '%s' is unqualified, using current package path '%s'", identName, targetPkgPath))
		}

		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Attempting to load package: '%s' for const identifier '%s'", targetPkgPath, identName))
		loadedPkgs, err := loader.Load(targetPkgPath)
		if err != nil {
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Failed loading package '%s' for const identifier '%s': %v", targetPkgPath, identName, err))
			return "", false
		}
		if len(loadedPkgs) == 0 {
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: No package found at path '%s' when resolving const identifier '%s'", targetPkgPath, identName))
			return "", false
		}
		pkg := loadedPkgs[0]
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Successfully loaded package '%s' (name: '%s') for const '%s'", pkg.ImportPath, pkg.Name, identName))

		pkgFiles, err := pkg.Files()
		if err != nil {
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Failed getting files for package '%s' to resolve const '%s': %v", pkg.ImportPath, identName, err))
			return "", false
		}

		for _, fileAst := range pkgFiles {
			var foundVal string
			var declFound bool
			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Searching for CONST '%s' in file '%s' of package '%s'", identName, fileAst.Name.Name, pkg.ImportPath))
			ast.Inspect(fileAst, func(node ast.Node) bool {
				if genDecl, ok := node.(*ast.GenDecl); ok && genDecl.Tok == token.CONST {
					for _, spec := range genDecl.Specs {
						if valSpec, ok := spec.(*ast.ValueSpec); ok {
							for i, nameIdentNode := range valSpec.Names {
								if nameIdentNode.Name == identName {
									declFound = true
									slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Found const declaration for '%s' in package '%s', file '%s'", identName, pkg.ImportPath, fileAst.Name.Name))
									if len(valSpec.Values) > i {
										if basicLit, ok := valSpec.Values[i].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
											unquotedVal, errUnquote := strconv.Unquote(basicLit.Value) // Changed to strconv.Unquote
											if errUnquote != nil {
												slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Failed unquoting string for const '%s' in package '%s', raw value '%s': %v", identName, pkg.ImportPath, basicLit.Value, errUnquote))
												return false // Stop inspection for this const
											}
											foundVal = unquotedVal
											slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Successfully resolved identifier '%s' in package '%s' to string value: \"%s\"", identName, pkg.ImportPath, foundVal))
											return false // Stop inspection, value found
										}
										slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Const '%s' in package '%s', file '%s' is not a basic string literal. AST node type %T, value: %s", identName, pkg.ImportPath, fileAst.Name.Name, valSpec.Values[i], astutils.ExprToTypeName(valSpec.Values[i])))
									} else {
										slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Const '%s' in package '%s', file '%s' has no value spec", identName, pkg.ImportPath, fileAst.Name.Name))
									}
									return false // Stop for this const name, whether successful or not
								}
							}
						}
					}
				}
				return true // Continue inspection
			}) // End ast.Inspect

			slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Check before return in file '%s': declFound=%v, foundVal='%s'", fileAst.Name.Name, declFound, foundVal))
			if declFound && foundVal != "" {
				return foundVal, true
			}
			if declFound { // Found declaration but not a usable string value
				slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Warning: Const '%s' in package '%s', file '%s' was found but not resolved to a string.", identName, pkg.ImportPath, fileAst.Name.Name))
				return "", false
			}
		} // End for _, fileAst := range pkgFiles
		slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Const identifier '%s' not found in any file of package '%s' (path searched: '%s')", identName, pkg.ImportPath, targetPkgPath))
		return "", false
	}

	// This point should ideally not be reached if the logic for identifier resolution (Path B2-MAIN) is complete and returns.
	slog.DebugContext(ctx, fmt.Sprintf("[resolveEvalResultToEnumString] Error: Unhandled case or fallthrough AFTER main logic block for identifiers. EvalResult: %+v", elementEvalResult))
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
									constValEval := astutils.EvaluateArg(valSpec.Values[i])
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
			// fileAst is the AST of the file where the goat.Enum marker is called.
			resolvedImportPath := astutils.GetImportPath(fileAst, evalResult.PkgName)
			if resolvedImportPath == "" {
				slog.DebugContext(ctx, fmt.Sprintf("Error: Could not resolve import path for package alias '%s' in file %s (used for enum in %s for field %s)", evalResult.PkgName, fileAst.Name.Name, markerType, optMeta.Name))
				return
			}
			targetPkgPath = resolvedImportPath
		} else { // Unqualified identifier, assume current package
			targetPkgPath = currentPkgPath
			if targetPkgPath == "" {
				slog.DebugContext(ctx, fmt.Sprintf("Error: Current package path is empty, cannot resolve unqualified identifier '%s' (used for enum in %s for field %s)", evalResult.IdentifierName, markerType, optMeta.Name))
				return
			}
		}

		slog.DebugContext(ctx, fmt.Sprintf("Attempting to load package: '%s' for enum identifier '%s' (field %s, marker %s)", targetPkgPath, evalResult.IdentifierName, optMeta.Name, markerType))
		loadedPkgs, err := loader.Load(targetPkgPath)
		if err != nil {
			slog.DebugContext(ctx, fmt.Sprintf("Error loading package '%s' for enum identifier '%s': %v (field %s, marker %s)", targetPkgPath, evalResult.IdentifierName, err, optMeta.Name, markerType))
			return
		}
		if len(loadedPkgs) == 0 {
			slog.DebugContext(ctx, fmt.Sprintf("No package found at path '%s' when resolving enum identifier '%s' (field %s, marker %s)", targetPkgPath, evalResult.IdentifierName, optMeta.Name, markerType))
			return
		}

		// Assuming the first loaded package is the relevant one.
		// For specific import paths, loader.Load should ideally return one package.
		// If targetPkgPath was ".", it might return multiple in some loader implementations,
		// but for Go module structure, "." usually maps to one package.
		pkg := loadedPkgs[0]
		var foundValues []any
		var foundDecl bool // Flag to indicate if the variable declaration was found

		slog.DebugContext(ctx, fmt.Sprintf("Searching for VAR '%s' in package '%s' (loaded from '%s')", evalResult.IdentifierName, pkg.ImportPath, targetPkgPath)) // Use pkg.ImportPath

		// Get the files from the package
		pkgFiles, err := pkg.Files()
		if err != nil {
			slog.DebugContext(ctx, fmt.Sprintf("Error getting files for package '%s': %v", pkg.ImportPath, err))
			return
		}

		for _, loadedFileAst := range pkgFiles { // Iterate through all files in the loaded package
			// Log the file being inspected if needed for detailed debugging:
			// slog.DebugContext(ctx, fmt.Sprintf("    Inspecting file: %s (package %s)", loadedFileAst.Name.Name, pkg.ImportPath))

			ast.Inspect(loadedFileAst, func(node ast.Node) bool {
				if foundDecl { // If already found in a previous file or node, stop.
					return false
				}
				genDecl, ok := node.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.VAR {
					return true // Continue inspection for other nodes
				}

				for _, spec := range genDecl.Specs {
					valSpec, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}

					for i, nameIdent := range valSpec.Names {
						if nameIdent.Name == evalResult.IdentifierName {
							slog.DebugContext(ctx, fmt.Sprintf("Found var declaration for '%s' in package '%s', file '%s'", evalResult.IdentifierName, pkg.ImportPath, loadedFileAst.Name.Name)) // Use pkg.ImportPath
							if len(valSpec.Values) > i {
								initializerExpr := valSpec.Values[i]
								if compLit, ok := initializerExpr.(*ast.CompositeLit); ok {
									// Special handling for []string{string(CONST_A), string(CONST_B), ...}
									var tempValues []any
									// elementsAreResolvable := true // Removed: allow partial success
									someElementsFailed := false // Track if any element fails
									for _, elt := range compLit.Elts {
										if callExpr, okElt := elt.(*ast.CallExpr); okElt {
											eltStrForLog := astutils.ExprToTypeName(elt)
											if funIdent, okFun := callExpr.Fun.(*ast.Ident); okFun && funIdent.Name == "string" && len(callExpr.Args) == 1 {
												arg := callExpr.Args[0]
												var constStrVal string
												var constFound bool

												if constIdent, okConst := arg.(*ast.Ident); okConst {
													slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Processing element %s (string(ConstInSamePkg))", optMeta.Name, evalResult.IdentifierName, eltStrForLog))
													constStrVal, constFound = resolveConstStringValue(ctx, constIdent.Name, pkg, loadedFileAst)
												} else if selExpr, okSel := arg.(*ast.SelectorExpr); okSel {
													slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Processing element %s (string(otherpkg.Const))", optMeta.Name, evalResult.IdentifierName, eltStrForLog))
													if pkgNameIdent, okPkgName := selExpr.X.(*ast.Ident); okPkgName {
														selPkgAlias := pkgNameIdent.Name
														constNameToResolve := selExpr.Sel.Name
														resolvedSelImportPath := astutils.GetImportPath(loadedFileAst, selPkgAlias)
														if resolvedSelImportPath == "" {
															slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Error: Could not resolve import path for package alias '%s' in file '%s' (used in string(%s.%s)) for element %s", optMeta.Name, evalResult.IdentifierName, selPkgAlias, loadedFileAst.Name.Name, selPkgAlias, constNameToResolve, eltStrForLog))
															someElementsFailed = true
															continue // Skip this element
														}
														selPkgs, errSel := loader.Load(resolvedSelImportPath)
														if errSel != nil || len(selPkgs) == 0 {
															slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Error: Could not load package '%s' for resolving const '%s' in string(%s.%s) for element %s: %v", optMeta.Name, evalResult.IdentifierName, resolvedSelImportPath, constNameToResolve, selPkgAlias, constNameToResolve, eltStrForLog, errSel))
															someElementsFailed = true
															continue // Skip this element
														}
														constStrVal, constFound = resolveConstStringValue(ctx, constNameToResolve, selPkgs[0], nil)
													} else {
														slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Error: Unhandled selector expression in string() argument: X is %T, not *ast.Ident for element %s", optMeta.Name, evalResult.IdentifierName, selExpr.X, eltStrForLog))
														someElementsFailed = true
														continue // Skip this element
													}
												} else {
													slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Error: Unhandled argument to string() conversion: %T for element %s", optMeta.Name, evalResult.IdentifierName, arg, eltStrForLog))
													someElementsFailed = true
													continue // Skip this element
												}

												if constFound {
													tempValues = append(tempValues, constStrVal)
													slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Successfully resolved element %s to value '%s'", optMeta.Name, evalResult.IdentifierName, eltStrForLog, constStrVal))
												} else {
													slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Error: Could not resolve constant value for element %s in initializer of %s", optMeta.Name, evalResult.IdentifierName, eltStrForLog, evalResult.IdentifierName))
													someElementsFailed = true
													// continue: already at end of this path for element
												}
											} else { // Not a string(IDENT) or string(pkg.IDENT) call
												slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Warning: Element %s is a CallExpr but not the expected string(IDENT) pattern.", optMeta.Name, evalResult.IdentifierName, eltStrForLog))
												someElementsFailed = true
												// continue: already at end of this path for element
											}
										} else { // Not a CallExpr, try to resolve it using the new function
											eltStr := astutils.ExprToTypeName(elt)
											slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Processing variable initializer element: %s", optMeta.Name, evalResult.IdentifierName, eltStr))
											elementEvalResult := astutils.EvaluateArg(elt)
											strVal, success := resolveEvalResultToEnumString(ctx, elementEvalResult, loader, pkg.ImportPath, loadedFileAst)
											if success {
												tempValues = append(tempValues, strVal)
												slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Successfully resolved element %s to value '%s' via resolveEvalResultToEnumString", optMeta.Name, evalResult.IdentifierName, eltStr, strVal))
											} else {
												slog.DebugContext(ctx, fmt.Sprintf("[extractEnumValuesFromEvalResult] Field %s, Var %s: Warning: Failed to resolve enum value from variable initializer element '%s'. Element EvalResult: %+v", optMeta.Name, evalResult.IdentifierName, eltStr, elementEvalResult))
												someElementsFailed = true
												// continue: already at end of this path for element
											}
										}
									} // End of for loop: for _, elt := range compLit.Elts

									foundValues = tempValues // Assign collected values (can be empty or partial)
									foundDecl = true         // We found and processed the var declaration

									if someElementsFailed {
										slog.DebugContext(ctx, fmt.Sprintf("Warning: Some elements of composite literal for '%s' in package '%s' could not be resolved.", evalResult.IdentifierName, pkg.ImportPath))
									}
									if len(foundValues) > 0 {
										slog.DebugContext(ctx, fmt.Sprintf("Successfully resolved enum identifier '%s' in package '%s' by custom composite literal parsing to values: %v", evalResult.IdentifierName, pkg.ImportPath, foundValues))
									} else if !someElementsFailed { // No values and no failures means it was an empty literal
										slog.DebugContext(ctx, fmt.Sprintf("Enum identifier '%s' in package '%s' resolved to an empty list (all elements processed successfully but yielded no strings, or literal was empty).", evalResult.IdentifierName, pkg.ImportPath))
									}
									// If someElementsFailed and len(foundValues)==0, the warning above covers it.
								} else {
									// Fallback to original logic if initializer is not a CompositeLit (e.g. alias to another var)
									resolvedSlice := astutils.EvaluateSliceArg(initializerExpr)
									if resolvedSlice.Value != nil {
										if s, ok := resolvedSlice.Value.([]any); ok {
											foundValues = s
											slog.DebugContext(ctx, fmt.Sprintf("Successfully resolved enum identifier '%s' in package '%s' to values (via fallback EvaluateSliceArg): %v", evalResult.IdentifierName, pkg.ImportPath, foundValues))
											foundDecl = true
										} else {
											slog.DebugContext(ctx, fmt.Sprintf("Enum variable '%s' initializer in package '%s' resolved via fallback, but not to []any: %T", evalResult.IdentifierName, pkg.ImportPath, resolvedSlice.Value))
										}
									} else if resolvedSlice.IdentifierName != "" {
										slog.DebugContext(ctx, fmt.Sprintf("Enum variable '%s' in package '%s' is an alias to another identifier '%s' (pkg '%s') (via fallback). Transitive resolution not yet supported.", evalResult.IdentifierName, pkg.ImportPath, resolvedSlice.IdentifierName, resolvedSlice.PkgName))
									} else {
										slog.DebugContext(ctx, fmt.Sprintf("Enum variable '%s' in package '%s' does not have a resolvable slice literal or identifier (via fallback): %T", evalResult.IdentifierName, pkg.ImportPath, initializerExpr))
									}
								}
							} else {
								slog.DebugContext(ctx, fmt.Sprintf("Enum variable '%s' in package '%s' has no initializer value at index %d.", evalResult.IdentifierName, pkg.ImportPath, i))
							}
							// Whether resolved or not, we found the declaration, so stop searching for this name.
							// If it wasn't the right type (e.g., not a slice), foundDecl remains false,
							// and the outer logic will report failure to resolve values.
							// To prevent re-processing the same var if it appears multiple times (which shouldn't happen for VARs at package level):
							if !foundDecl { // If not successfully resolved to values
								slog.DebugContext(ctx, fmt.Sprintf("Declaration for '%s' found but values not extracted. Stopping further search for this name.", evalResult.IdentifierName))
								// We should stop inspecting further for *this specific name* if it's found but not resolvable.
								// The current ast.Inspect logic will stop if foundDecl is true.
								// If it's found but not usable, we mark it as "found" to stop further search for this specific identifier.
								// This is tricky because `foundDecl` is used to signal successful value extraction.
								// Let's refine: if name matches, we consider it "handled" for this Inspect run.
								// The `return false` below for the GenDecl should be sufficient for `nameIdent.Name == evalResult.IdentifierName`.
							}
							return false // Stop inspecting within this GenDecl once the matching name is processed.
						}
					}
				}
				return true // Continue inspecting other GenDecls or nodes
			})

			if foundDecl { // If found in this file, break from iterating pkg.Files
				break
			}
		}

		if foundDecl {
			optMeta.EnumValues = foundValues
		} else {
			slog.DebugContext(ctx, fmt.Sprintf("Could not find VAR declaration or resolve values for enum identifier '%s' in package '%s' (path searched: '%s', field %s, marker %s)", evalResult.IdentifierName, pkg.ImportPath, targetPkgPath, optMeta.Name, markerType)) // Use pkg.ImportPath
		}
		return
	}

	// Neither Value nor IdentifierName is set
	slog.DebugContext(ctx, fmt.Sprintf("Enum argument for field %s (marker %s, type %T) could not be evaluated to a literal slice or a resolvable identifier. EvalResult: %+v", optMeta.Name, markerType, evalResult, evalResult))
}
