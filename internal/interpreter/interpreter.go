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
	slog.DebugContext(ctx, "[InterpretInitializer] --- ENTER ---", "optionsStructName", optionsStructName, "initializerFuncName", initializerFuncName, "currentPkgPath", currentPkgPath)
	defer slog.DebugContext(ctx, "[InterpretInitializer] --- EXIT ---", "optionsStructName", optionsStructName, "initializerFuncName", initializerFuncName)

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
						slog.DebugContext(ctx, "[InterpretInitializer] Found assignment to options field", "field", fieldName, "optionsStruct", optionsStructName)
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
					slog.DebugContext(ctx, "[InterpretInitializer] Found return composite literal", "optionsStruct", optionsStructName, "initializer", initializerFuncName)
					for _, elt := range compLit.Elts {
						if kvExpr, ok := elt.(*ast.KeyValueExpr); ok {
							if keyIdent, ok := kvExpr.Key.(*ast.Ident); ok {
								fieldName := keyIdent.Name
								if optMeta, exists := optionsMap[fieldName]; exists {
									slog.DebugContext(ctx, "[InterpretInitializer] Processing composite literal field", "field", fieldName)
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
	slog.DebugContext(ctx, "[extractMarkerInfo] --- ENTER ---", "field", optMeta.Name, "valueExpr", astutils.ExprToTypeName(valueExpr))
	defer slog.DebugContext(ctx, "[extractMarkerInfo] --- EXIT ---", "field", optMeta.Name, "DefaultValue", optMeta.DefaultValue, "EnumValues", optMeta.EnumValues)

	callExpr, ok := valueExpr.(*ast.CallExpr)
	if !ok {
		slog.DebugContext(ctx, "[extractMarkerInfo] Value expression is not a CallExpr", "field", optMeta.Name, "valueExprType", fmt.Sprintf("%T", valueExpr))
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
		slog.DebugContext(ctx, "[extractMarkerInfo] Call is not to a recognized marker package", "field", optMeta.Name, "markerPackageAlias", markerPkgAlias, "resolvedPath", actualMarkerPkgPath, "expectedPath", markerPkgImportPath)
		return
	}
	slog.DebugContext(ctx, "[extractMarkerInfo] Detected marker function", "field", optMeta.Name, "markerFuncName", markerFuncName, "markerPackage", actualMarkerPkgPath)

	switch markerFuncName {
	case "Default":
		slog.DebugContext(ctx, "[extractMarkerInfo] Processing goat.Default", "field", optMeta.Name, "argCount", len(callExpr.Args))
		if len(callExpr.Args) > 0 {
			// Default value is the first argument
			slog.DebugContext(ctx, "[extractMarkerInfo] DefaultValue: Before eval", "field", optMeta.Name, "defaultValue", optMeta.DefaultValue, "arg0", astutils.ExprToTypeName(callExpr.Args[0]))
			defaultEvalResult := astutils.EvaluateArg(ctx, callExpr.Args[0])
			if defaultEvalResult.IdentifierName == "" { // If it's a literal or directly evaluatable value
				optMeta.DefaultValue = defaultEvalResult.Value
				slog.DebugContext(ctx, "[extractMarkerInfo] DefaultValue: Literal", "field", optMeta.Name, "value", optMeta.DefaultValue)
			} else { // Default value is an identifier
				slog.DebugContext(ctx, "[extractMarkerInfo] DefaultValue: Identifier, attempting resolution", "field", optMeta.Name, "identifier", defaultEvalResult.IdentifierName, "pkg", defaultEvalResult.PkgName)
				resolvedStrVal, success := resolveEvalResultToEnumString(ctx, defaultEvalResult, loader, currentPkgPath, fileAst)
				if success {
					optMeta.DefaultValue = resolvedStrVal
					slog.DebugContext(ctx, "[extractMarkerInfo] DefaultValue: Resolved identifier", "field", optMeta.Name, "value", optMeta.DefaultValue)
				} else {
					slog.DebugContext(ctx, "[extractMarkerInfo] DefaultValue: Failed to resolve identifier", "field", optMeta.Name, "identifier", defaultEvalResult.IdentifierName)
					optMeta.DefaultValue = nil
				}
			}
			slog.DebugContext(ctx, "[extractMarkerInfo] DefaultValue: After eval", "field", optMeta.Name, "defaultValue", optMeta.DefaultValue)


			// Subsequent args could be an Enum call for enumConstraint
			if len(callExpr.Args) > 1 {
				slog.DebugContext(ctx, "[extractMarkerInfo] Default has second argument for EnumValues", "field", optMeta.Name, "arg1", astutils.ExprToTypeName(callExpr.Args[1]))
				enumArg := callExpr.Args[1]
				slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues: Before extraction", "field", optMeta.Name, "enumValues", optMeta.EnumValues)
				if enumInnerCallExpr, ok := enumArg.(*ast.CallExpr); ok { // goat.Default("val", goat.Enum(MyEnumVar))
					innerFuncName, innerPkgAlias := astutils.GetFullFunctionName(enumInnerCallExpr.Fun)
					resolvedInnerPkgPath := astutils.GetImportPath(fileAst, innerPkgAlias)
					isGoatEnumCall := (resolvedInnerPkgPath == markerPkgImportPath || resolvedInnerPkgPath == "testcmdmodule/internal/goat") && innerFuncName == "Enum"
					slog.DebugContext(ctx, "[extractMarkerInfo] Default's EnumValues arg is a CallExpr", "field", optMeta.Name, "isGoatEnumCall", isGoatEnumCall, "innerFunc", innerFuncName, "innerPkg", resolvedInnerPkgPath)

					if isGoatEnumCall {
						if len(enumInnerCallExpr.Args) > 0 {
							evalResult := astutils.EvaluateSliceArg(ctx, enumInnerCallExpr.Args[0])
							extractEnumValuesFromEvalResult(ctx, evalResult, optMeta, fileAst, loader, currentPkgPath, "Default (via goat.Enum)")
						}
					} else {
						slog.DebugContext(ctx, "[extractMarkerInfo] Second argument to goat.Default is a non-goat.Enum CallExpr, ignoring for enum constraints.", "field", optMeta.Name, "calledFunc", innerFuncName)
					}
				} else { // goat.Default("val", MyEnumVarOrSliceLiteral)
					slog.DebugContext(ctx, "[extractMarkerInfo] Default's EnumValues arg is not a CallExpr, evaluating as direct slice/identifier", "field", optMeta.Name)
					enumEvalResult := astutils.EvaluateSliceArg(ctx, enumArg)
					if enumEvalResult.Value != nil {
						if s, ok := enumEvalResult.Value.([]any); ok {
							optMeta.EnumValues = s
							slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues from direct literal slice in Default", "field", optMeta.Name, "values", optMeta.EnumValues)
						} else {
							slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues from direct evaluation in Default was not []any", "field", optMeta.Name, "type", fmt.Sprintf("%T", enumEvalResult.Value))
						}
					} else if enumEvalResult.IdentifierName != "" {
						slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues in Default is an identifier, attempting resolution", "field", optMeta.Name, "identifier", enumEvalResult.IdentifierName, "pkg", enumEvalResult.PkgName)
						extractEnumValuesFromEvalResult(ctx, enumEvalResult, optMeta, fileAst, loader, currentPkgPath, "Default (direct ident)")
					} else {
						slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues argument for Default could not be evaluated", "field", optMeta.Name, "argType", fmt.Sprintf("%T", enumArg), "evalResult", fmt.Sprintf("%+v", enumEvalResult))
					}
				}
				slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues: After extraction attempt in Default", "field", optMeta.Name, "enumValues", optMeta.EnumValues)
			}
		}
	case "Enum":
		slog.DebugContext(ctx, "[extractMarkerInfo] Processing goat.Enum", "field", optMeta.Name, "argCount", len(callExpr.Args))
		var valuesArg ast.Expr
		if len(callExpr.Args) == 1 { // goat.Enum(MyEnumValuesVarOrLiteral)
			valuesArg = callExpr.Args[0]
			slog.DebugContext(ctx, "[extractMarkerInfo] goat.Enum with 1 arg", "field", optMeta.Name, "arg0", astutils.ExprToTypeName(valuesArg))
		} else if len(callExpr.Args) == 2 { // goat.Enum((*MyType)(nil), MyEnumValuesVarOrLiteral)
			valuesArg = callExpr.Args[1]
			slog.DebugContext(ctx, "[extractMarkerInfo] goat.Enum with 2 args, using arg1", "field", optMeta.Name, "arg1", astutils.ExprToTypeName(valuesArg))
		} else {
			slog.WarnContext(ctx, "[extractMarkerInfo] goat.Enum called with unexpected number of arguments", "field", optMeta.Name, "argCount", len(callExpr.Args))
			return
		}

		slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues: Before extraction", "field", optMeta.Name, "enumValues", optMeta.EnumValues)
		if valuesArg != nil {
			evalResult := astutils.EvaluateSliceArg(ctx, valuesArg)
			slog.DebugContext(ctx, "[extractMarkerInfo] goat.Enum EvaluateSliceArg result", "field", optMeta.Name, "evalResult.Value", evalResult.Value, "evalResult.IdentifierName", evalResult.IdentifierName)

			if evalResult.Value == nil && evalResult.IdentifierName == "" { // Potential composite literal with identifiers
				if compLit, ok := valuesArg.(*ast.CompositeLit); ok {
					slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues is a composite literal, resolving elements", "field", optMeta.Name)
					var resolvedEnumStrings []any
					for i, elt := range compLit.Elts {
						slog.DebugContext(ctx, "[extractMarkerInfo] Resolving composite literal element", "field", optMeta.Name, "elementIndex", i, "elementExpr", astutils.ExprToTypeName(elt))
						elementEvalResult := astutils.EvaluateArg(ctx, elt)
						strVal, success := resolveEvalResultToEnumString(ctx, elementEvalResult, loader, currentPkgPath, fileAst)
						if success {
							resolvedEnumStrings = append(resolvedEnumStrings, strVal)
						} else {
							slog.WarnContext(ctx, "[extractMarkerInfo] Could not resolve enum element in composite literal", "field", optMeta.Name, "elementExpr", astutils.ExprToTypeName(elt))
						}
					}
					if len(resolvedEnumStrings) > 0 {
						optMeta.EnumValues = resolvedEnumStrings
						slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues: Resolved from composite literal", "field", optMeta.Name, "values", optMeta.EnumValues)
					} else {
						slog.WarnContext(ctx, "[extractMarkerInfo] Composite literal for enum yielded no resolvable string values", "field", optMeta.Name)
					}
				} else {
					slog.WarnContext(ctx, "[extractMarkerInfo] Enum argument could not be processed as slice or composite literal", "field", optMeta.Name, "argType", fmt.Sprintf("%T", valuesArg), "evalResult", fmt.Sprintf("%+v", evalResult))
				}
			} else { // Literal slice or identifier to a slice
				slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues is a literal slice or identifier, calling extractEnumValuesFromEvalResult", "field", optMeta.Name)
				extractEnumValuesFromEvalResult(ctx, evalResult, optMeta, fileAst, loader, currentPkgPath, "Enum (direct)")
			}
		}
		slog.DebugContext(ctx, "[extractMarkerInfo] EnumValues: After extraction attempt in Enum", "field", optMeta.Name, "enumValues", optMeta.EnumValues)
	case "File":
		slog.DebugContext(ctx, "[extractMarkerInfo] Processing goat.File", "field", optMeta.Name, "argCount", len(callExpr.Args))
		if len(callExpr.Args) > 0 {
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
	slog.DebugContext(ctx, "[resolveEvalResultToEnumString] --- ENTER ---", "elementEvalResult", fmt.Sprintf("%+v", elementEvalResult), "currentPkgPath", currentPkgPath, "fileAstName", fileAstForContext.Name.Name)
	defer slog.DebugContext(ctx, "[resolveEvalResultToEnumString] --- EXIT ---", "elementEvalResult", fmt.Sprintf("%+v", elementEvalResult))

	if elementEvalResult.Value != nil {
		if strVal, ok := elementEvalResult.Value.(string); ok {
			slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Value is direct string", "value", strVal)
			return strVal, true
		}
		slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Value is not a string", "valueType", fmt.Sprintf("%T", elementEvalResult.Value), "value", elementEvalResult.Value)
		return "", false
	}

	if elementEvalResult.IdentifierName == "" {
		slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Value is nil and IdentifierName is empty")
		return "", false
	}

	identName := elementEvalResult.IdentifierName
	pkgAlias := elementEvalResult.PkgName
	targetPkgPath := ""
	slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Identifier identified", "identifierName", identName, "pkgAlias", pkgAlias)


	if pkgAlias != "" { // Qualified identifier like mypkg.MyConst
		if fileAstForContext == nil {
			slog.ErrorContext(ctx, "[resolveEvalResultToEnumString] fileAstForContext is nil for qualified identifier", "identifier", identName, "pkgAlias", pkgAlias)
			return "", false
		}
		resolvedImportPath := astutils.GetImportPath(fileAstForContext, pkgAlias)
		if resolvedImportPath == "" {
			slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Could not resolve import path for package alias", "pkgAlias", pkgAlias, "identifier", identName, "contextFile", fileAstForContext.Name.Name)
			return "", false
		}
		targetPkgPath = resolvedImportPath
		slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Resolved qualified identifier", "pkgAlias", pkgAlias, "targetPkgPath", targetPkgPath, "identifier", identName)
	} else { // Unqualified identifier, assume current package context
		targetPkgPath = currentPkgPath
		if targetPkgPath == "" {
			slog.ErrorContext(ctx, "[resolveEvalResultToEnumString] Current package path is empty for unqualified identifier", "identifier", identName)
			return "", false
		}
		slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Unqualified identifier, using current package path", "identifier", identName, "targetPkgPath", targetPkgPath)
	}

	fullSymbolName := targetPkgPath + ":" + identName
	slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Looking up symbol", "fullSymbolName", fullSymbolName)

	symInfo, found := loader.LookupSymbol(fullSymbolName)
	if !found {
		slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Symbol not found in loader cache", "fullSymbolName", fullSymbolName)
		return "", false
	}

	slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Found symbol in loader", "fullSymbolName", fullSymbolName, "filePath", symInfo.FilePath, "nodeType", fmt.Sprintf("%T", symInfo.Node))

	valSpec, ok := symInfo.Node.(*ast.ValueSpec)
	if !ok {
		slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Symbol is not an ast.ValueSpec", "fullSymbolName", fullSymbolName, "actualNodeType", fmt.Sprintf("%T", symInfo.Node))
		return "", false
	}

	var specNames []string
	for _, name := range valSpec.Names { specNames = append(specNames, name.Name) }
	var specValuesStr []string
	if valSpec.Values != nil {
		for _, val := range valSpec.Values { specValuesStr = append(specValuesStr, astutils.ExprToTypeName(val)) }
	}
	slog.DebugContext(ctx, "[resolveEvalResultToEnumString] ValueSpec details", "fullSymbolName", fullSymbolName, "identToFind", identName, "specNames", specNames, "specValues", specValuesStr)

	for i, nameIdent := range valSpec.Names {
		if nameIdent.Name == identName {
			slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Matched identName in ValueSpec", "identName", identName, "index", i)
			if len(valSpec.Values) > i {
				valueNode := valSpec.Values[i]
				if basicLit, ok := valueNode.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
					unquotedVal, errUnquote := strconv.Unquote(basicLit.Value)
					if errUnquote != nil {
						slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Error unquoting string for const/var", "rawValue", basicLit.Value, "error", errUnquote)
						return "", false
					}
					slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Resolved to string literal", "value", unquotedVal)
					return unquotedVal, true
				} else if callExpr, ok := valueNode.(*ast.CallExpr); ok {
					if identFun, ok := callExpr.Fun.(*ast.Ident); ok && identFun.Name == "string" && len(callExpr.Args) == 1 {
						slog.DebugContext(ctx, "[resolveEvalResultToEnumString] Value is a string() cast, resolving argument", "argExpr", astutils.ExprToTypeName(callExpr.Args[0]))
						argFileAst, argFileAstFound := loader.GetAST(symInfo.FilePath)
						if !argFileAstFound {
							slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Could not get AST for file where const/var is defined, cannot resolve string cast argument", "filePath", symInfo.FilePath)
							return "", false
						}
						argEvalResult := astutils.EvaluateArg(ctx, callExpr.Args[0])
						return resolveEvalResultToEnumString(ctx, argEvalResult, loader, symInfo.PackagePath, argFileAst)
					}
					slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Value is CallExpr but not string() cast", "call", astutils.ExprToTypeName(callExpr))
					return "", false
				}
				slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Value in ValueSpec is not a string literal or string() cast", "valueNodeType", fmt.Sprintf("%T", valueNode))
				return "", false
			}
			slog.WarnContext(ctx, "[resolveEvalResultToEnumString] ValueSpec has no corresponding value for identifier", "identName", identName, "index", i)
			return "", false
		}
	}

	slog.WarnContext(ctx, "[resolveEvalResultToEnumString] Identifier name not found in ValueSpec names", "identName", identName, "specNames", specNames)
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
	slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] --- ENTER ---", "field", optMeta.Name, "markerType", markerType, "evalResult", fmt.Sprintf("%+v", evalResult))
	originalEnumValues := optMeta.EnumValues // For logging change
	originalIsEnum := optMeta.IsEnum         // For logging change

	defer func() {
		slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] --- EXIT ---",
			"field", optMeta.Name,
			"markerType", markerType,
			"originalEnumValues", originalEnumValues, "finalEnumValues", optMeta.EnumValues,
			"originalIsEnum", originalIsEnum, "finalIsEnum", optMeta.IsEnum)
	}()

	if evalResult.Value != nil {
		if s, ok := evalResult.Value.([]any); ok {
			optMeta.EnumValues = s
			slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] EnumValues from literal slice", "field", optMeta.Name, "values", optMeta.EnumValues)
		} else {
			slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Enum argument evaluated to a non-slice value", "field", optMeta.Name, "valueType", fmt.Sprintf("%T", evalResult.Value), "value", evalResult.Value)
		}
		// Set IsEnum after this block, before returning from function
	} else if evalResult.IdentifierName != "" {
		slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Enum argument is an identifier, attempting resolution", "field", optMeta.Name, "identifier", evalResult.IdentifierName, "pkg", evalResult.PkgName)
		targetPkgPath := ""
		if evalResult.PkgName != "" { // Qualified identifier
			resolvedImportPath := astutils.GetImportPath(fileAst, evalResult.PkgName)
			if resolvedImportPath == "" {
				slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Could not resolve import path for package alias", "pkgAlias", evalResult.PkgName, "identifier", evalResult.IdentifierName, "field", optMeta.Name)
				// Set IsEnum before returning
				optMeta.IsEnum = len(optMeta.EnumValues) > 0
				return
			}
			targetPkgPath = resolvedImportPath
		} else { // Unqualified identifier
			targetPkgPath = currentPkgPath
			if targetPkgPath == "" {
				slog.ErrorContext(ctx, "[extractEnumValuesFromEvalResult] Current package path is empty for unqualified identifier", "identifier", evalResult.IdentifierName, "field", optMeta.Name)
				optMeta.IsEnum = len(optMeta.EnumValues) > 0
				return
			}
		}

		fullSymbolName := targetPkgPath + ":" + evalResult.IdentifierName
		slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Looking up symbol for enum variable", "fullSymbolName", fullSymbolName)

		symInfo, found := loader.LookupSymbol(fullSymbolName)
		if !found {
			slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Enum variable symbol not found in cache", "fullSymbolName", fullSymbolName, "field", optMeta.Name)
			optMeta.IsEnum = len(optMeta.EnumValues) > 0
			return
		}
		slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Found symbol for enum variable", "fullSymbolName", fullSymbolName, "filePath", symInfo.FilePath, "nodeType", fmt.Sprintf("%T", symInfo.Node))

		valSpec, ok := symInfo.Node.(*ast.ValueSpec)
		if !ok {
			slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Symbol for enum variable is not a ValueSpec", "fullSymbolName", fullSymbolName, "nodeType", fmt.Sprintf("%T", symInfo.Node), "field", optMeta.Name)
			optMeta.IsEnum = len(optMeta.EnumValues) > 0
			return
		}

		nameIdx := -1
		for i, nameIdent := range valSpec.Names {
			if nameIdent.Name == evalResult.IdentifierName { nameIdx = i; break }
		}
		if nameIdx == -1 {
			slog.ErrorContext(ctx, "[extractEnumValuesFromEvalResult] Identifier name not found in ValueSpec (should not happen)", "fullSymbolName", fullSymbolName, "identifierName", evalResult.IdentifierName, "field", optMeta.Name)
			optMeta.IsEnum = len(optMeta.EnumValues) > 0
			return
		}
		if len(valSpec.Values) <= nameIdx {
			slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Enum variable ValueSpec has no corresponding initializer", "fullSymbolName", fullSymbolName, "nameIndex", nameIdx, "field", optMeta.Name)
			optMeta.IsEnum = len(optMeta.EnumValues) > 0
			return
		}
		initializerExpr := valSpec.Values[nameIdx]

		if compLit, ok := initializerExpr.(*ast.CompositeLit); ok {
			slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Enum variable is a composite literal, processing elements", "fullSymbolName", fullSymbolName)
			var tempValues []any
			var someElementsFailed bool
			definingFileAST, astFound := loader.GetAST(symInfo.FilePath)
			if !astFound {
				slog.ErrorContext(ctx, "[extractEnumValuesFromEvalResult] AST for defining file of enum var not found", "filePath", symInfo.FilePath, "field", optMeta.Name)
				optMeta.IsEnum = len(optMeta.EnumValues) > 0
				return
			}
			for iel, elt := range compLit.Elts {
				eltStrForLog := astutils.ExprToTypeName(elt)
				slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Processing composite literal element", "elementIndex", iel, "elementExpr", eltStrForLog)
				var strVal string
				var success bool
				if callExpr, ok := elt.(*ast.CallExpr); ok && callExpr.Fun.(*ast.Ident).Name == "string" && len(callExpr.Args) == 1 { // string(IDENTIFIER)
					slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Element is string() cast, resolving argument", "argExpr", astutils.ExprToTypeName(callExpr.Args[0]))
					argEvalResult := astutils.EvaluateArg(ctx, callExpr.Args[0])
					strVal, success = resolveEvalResultToEnumString(ctx, argEvalResult, loader, symInfo.PackagePath, definingFileAST)
				} else { // Direct identifier or other literal
					elementEvalResult := astutils.EvaluateArg(ctx, elt)
					strVal, success = resolveEvalResultToEnumString(ctx, elementEvalResult, loader, symInfo.PackagePath, definingFileAST)
				}
				if success {
					tempValues = append(tempValues, strVal)
					slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Resolved composite literal element to string", "elementExpr", eltStrForLog, "value", strVal)
				} else {
					slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Failed to resolve composite literal element", "elementExpr", eltStrForLog)
					someElementsFailed = true
				}
			}
			if len(tempValues) > 0 {
				optMeta.EnumValues = tempValues
				slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Resolved EnumValues from composite literal", "values", optMeta.EnumValues)
			} else if !someElementsFailed { // No values, but no failures means empty list
				optMeta.EnumValues = []any{}
				slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Composite literal resolved to empty list")
			}
			if someElementsFailed && len(tempValues) == 0 {
				slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] All elements of composite literal failed to resolve", "fullSymbolName", fullSymbolName)
			}
		} else { // Fallback for non-composite literal initializers
			slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Enum variable initializer is not CompositeLit, using fallback EvaluateSliceArg", "fullSymbolName", fullSymbolName, "initializerType", fmt.Sprintf("%T", initializerExpr))
			resolvedSlice := astutils.EvaluateSliceArg(ctx, initializerExpr)
			if resolvedSlice.Value != nil {
				if s, ok := resolvedSlice.Value.([]any); ok {
					optMeta.EnumValues = s
					slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Resolved EnumValues via fallback", "values", optMeta.EnumValues)
				} else {
					slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Fallback resolved to non-slice", "type", fmt.Sprintf("%T", resolvedSlice.Value))
				}
			} else if resolvedSlice.IdentifierName != "" {
				slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Fallback resolved to another identifier, transitive resolution not fully supported here", "aliasTo", resolvedSlice.IdentifierName)
			} else {
				slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Fallback could not resolve to slice or identifier")
			}
		}
	} else {
		slog.WarnContext(ctx, "[extractEnumValuesFromEvalResult] Enum argument could not be evaluated to a literal slice or a resolvable identifier", "field", optMeta.Name, "markerType", markerType, "evalResult", fmt.Sprintf("%+v", evalResult))
	}

	// Final IsEnum determination
	if len(optMeta.EnumValues) > 0 {
		optMeta.IsEnum = true
	} else {
		optMeta.IsEnum = false // Ensure it's false if EnumValues is empty or nil
	}
	slog.DebugContext(ctx, "[extractEnumValuesFromEvalResult] Final IsEnum status", "field", optMeta.Name, "isEnum", optMeta.IsEnum, "enumValues", optMeta.EnumValues)
}
