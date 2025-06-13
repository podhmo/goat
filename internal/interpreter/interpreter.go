package interpreter

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"

	"github.com/podhmo/goat/internal/loader"
	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/astutils"
)

// InterpretInitializer analyzes the AST of an options initializer function (e.g., NewOptions)
// to extract default values and enum choices by "interpreting" calls to goat.Default() and goat.Enum().
// It modifies the passed cmdMetadata.Options directly.
func InterpretInitializer(
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

	log.Printf("Interpreting initializer: %s", initializerFuncName)

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
						log.Printf("Found assignment to options field: %s", fieldName)
						extractMarkerInfo(stmtNode.Rhs[0], optMeta, fileAst, markerPkgImportPath, loader, currentPkgPath)
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
					log.Printf("Found return composite literal in %s", initializerFuncName)
					for _, elt := range compLit.Elts {
						if kvExpr, ok := elt.(*ast.KeyValueExpr); ok {
							if keyIdent, ok := kvExpr.Key.(*ast.Ident); ok {
								fieldName := keyIdent.Name
								if optMeta, exists := optionsMap[fieldName]; exists {
									extractMarkerInfo(kvExpr.Value, optMeta, fileAst, markerPkgImportPath, loader, currentPkgPath)
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
			log.Printf("  Field %s is assigned an identifier '%s' (pkg '%s') directly. If this is an enum, use goat.Enum(%s) or ensure type information is available for resolution.", optMeta.Name, evalRes.IdentifierName, evalRes.PkgName, evalRes.IdentifierName)
		} else if evalRes.Value != nil {
			// It's a literal value assigned directly, e.g. `FieldName: "defaultValue"`
			// This could be a default value.
			// We need to be careful not to overwrite defaults set by goat.Default() if that's preferred.
			// For now, let's assume goat.X markers are the primary source of metadata.
			log.Printf("  Field %s is assigned a literal value '%v' directly. This might be a default, but typically use goat.Default() for clarity.", optMeta.Name, evalRes.Value)
		}
		return
	}

	markerFuncName, markerPkgAlias := astutils.GetFullFunctionName(callExpr.Fun)
	actualMarkerPkgPath := astutils.GetImportPath(fileAst, markerPkgAlias)

	// Allow original goat path or the one used in cmd/goat tests via testcmdmodule
	isKnownMarkerPackage := (actualMarkerPkgPath == markerPkgImportPath || // e.g. "github.com/podhmo/goat"
		actualMarkerPkgPath == "testcmdmodule/internal/goat") // For cmd/goat tests

	if !isKnownMarkerPackage {
		log.Printf("  Call is to package '%s' (alias '%s'), not the recognized marker package(s) ('%s' or 'testcmdmodule/internal/goat')", actualMarkerPkgPath, markerPkgAlias, markerPkgImportPath)
		return
	}

	switch markerFuncName {
	case "Default":
		log.Printf("Interpreting goat.Default for field %s (current Pkg: %s)", optMeta.Name, currentPkgPath)
		if len(callExpr.Args) > 0 {
			// Default value is the first argument
			defaultEvalResult := astutils.EvaluateArg(callExpr.Args[0])
			if defaultEvalResult.Value != nil {
				optMeta.DefaultValue = defaultEvalResult.Value
				log.Printf("  Default value: %v", optMeta.DefaultValue)
			} else if defaultEvalResult.IdentifierName != "" {
				// TODO: Default value could also be a constant that needs resolving.
				// For now, we only handle literal defaults.
				log.Printf("  Default value for field %s is an identifier '%s' (pkg '%s'). Resolution of identifiers for default values is not yet implemented.", optMeta.Name, defaultEvalResult.IdentifierName, defaultEvalResult.PkgName)
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
							extractEnumValuesFromEvalResult(evalResult, optMeta, fileAst, loader, currentPkgPath, "Default (via goat.Enum)")
						}
					} else {
						log.Printf("  Second argument to goat.Default for field %s is a call to %s.%s, not goat.Enum. Ignoring for enum constraints.", optMeta.Name, innerPkgAlias, innerFuncName)
					}
				} else { // goat.Default("val", MyEnumVarOrSliceLiteral)
					enumEvalResult := astutils.EvaluateSliceArg(enumArg)
					if enumEvalResult.Value != nil {
						if s, ok := enumEvalResult.Value.([]any); ok {
							optMeta.EnumValues = s
							log.Printf("  Enum values for Default from direct evaluation: %v", optMeta.EnumValues)
						} else {
							log.Printf("  Enum values for Default for field %s from direct evaluation was not []any, but %T", optMeta.Name, enumEvalResult.Value)
						}
					} else if enumEvalResult.IdentifierName != "" {
						log.Printf("  Enum constraint for Default for field %s is an identifier '%s' (pkg '%s'). Loader resolution for this case is not yet fully implemented in Default.", optMeta.Name, enumEvalResult.IdentifierName, enumEvalResult.PkgName)
						// Per subtask, log that loader resolution for Default's direct identifier enum is not yet fully implemented.
						// If we wanted to implement it, we would call:
						// extractEnumValuesFromEvalResult(enumEvalResult, optMeta, fileAst, loader, currentPkgPath, "Default (direct ident)")
					} else {
						// This case handles where enumEvalResult.Value is nil AND enumEvalResult.IdentifierName is empty.
						log.Printf("  Enum argument for Default for field %s (type %T) could not be evaluated to a literal slice or a resolvable identifier. EvalResult: %+v", optMeta.Name, enumArg, enumEvalResult)
					}
				}
			}
		}
	case "Enum": // Handles o.MyField = goat.Enum(MyEnumValues) or o.MyField = goat.Enum(pkg.MyEnumValues)
		log.Printf("Interpreting goat.Enum for field %s (current Pkg: %s)", optMeta.Name, currentPkgPath)
		if len(callExpr.Args) == 1 {
			evalResult := astutils.EvaluateSliceArg(callExpr.Args[0])
			extractEnumValuesFromEvalResult(evalResult, optMeta, fileAst, loader, currentPkgPath, "Enum (direct)")
		}
	case "File":
		log.Printf("Interpreting goat.File for field %s", optMeta.Name)
		if len(callExpr.Args) > 0 {
			optMeta.DefaultValue = astutils.EvaluateArg(callExpr.Args[0])
			log.Printf("  Default path: %v", optMeta.DefaultValue)
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
								log.Printf("  FileOption: MustExist")
							case "GlobPattern":
								optMeta.FileGlobPattern = true
								log.Printf("  FileOption: GlobPattern")
							default:
								log.Printf("  Unknown FileOption: %s", optionFuncName)
							}
						}
					}
				}
			}
		}
	default:
		// Not a recognized marker function from the specified package
		log.Printf("  Not a goat marker function: %s.%s", markerPkgAlias, markerFuncName)
	}
}

// extractEnumValuesFromEvalResult is a helper to resolve enum values from EvalResult.
// It populates optMeta.EnumValues if resolution is successful.

// resolveConstStringValue searches for a constant `constName` in the given `pkg`
// and returns its string value if it's a basic literal string.
func resolveConstStringValue(constName string, pkg *loader.Package, identFile *ast.File) (string, bool) {
	pkgFiles, err := pkg.Files()
	if err != nil {
		log.Printf("    Error getting files for package '%s' to resolve const '%s': %v", pkg.ImportPath, constName, err)
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
									log.Printf("    Const '%s' in package '%s' is not a direct string literal, actual type %T", constName, pkg.ImportPath, constValEval.Value)
								} else {
									log.Printf("    Const '%s' in package '%s' has no value", constName, pkg.ImportPath)
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
	log.Printf("    Const '%s' not found in package '%s'", constName, pkg.ImportPath)
	return "", false
}

func extractEnumValuesFromEvalResult(
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
			log.Printf("  Enum values for %s (field %s) from literal slice: %v", markerType, optMeta.Name, optMeta.EnumValues)
		} else {
			log.Printf("  Error: Enum argument for %s (field %s) evaluated to a non-slice value: %T (%v)", markerType, optMeta.Name, evalResult.Value, evalResult.Value)
		}
		return
	}

	if evalResult.IdentifierName != "" {
		log.Printf("  Enum argument for %s (field %s) is an identifier '%s' (pkg '%s'), attempting loader resolution", markerType, optMeta.Name, evalResult.IdentifierName, evalResult.PkgName)
		targetPkgPath := ""
		if evalResult.PkgName != "" { // Qualified identifier like mypkg.MyEnumVar
			// fileAst is the AST of the file where the goat.Enum marker is called.
			resolvedImportPath := astutils.GetImportPath(fileAst, evalResult.PkgName)
			if resolvedImportPath == "" {
				log.Printf("  Error: Could not resolve import path for package alias '%s' in file %s (used for enum in %s for field %s)", evalResult.PkgName, fileAst.Name.Name, markerType, optMeta.Name)
				return
			}
			targetPkgPath = resolvedImportPath
		} else { // Unqualified identifier, assume current package
			targetPkgPath = currentPkgPath
			if targetPkgPath == "" {
				log.Printf("  Error: Current package path is empty, cannot resolve unqualified identifier '%s' (used for enum in %s for field %s)", evalResult.IdentifierName, markerType, optMeta.Name)
				return
			}
		}

		log.Printf("  Attempting to load package: '%s' for enum identifier '%s' (field %s, marker %s)", targetPkgPath, evalResult.IdentifierName, optMeta.Name, markerType)
		loadedPkgs, err := loader.Load(targetPkgPath)
		if err != nil {
			log.Printf("  Error loading package '%s' for enum identifier '%s': %v (field %s, marker %s)", targetPkgPath, evalResult.IdentifierName, err, optMeta.Name, markerType)
			return
		}
		if len(loadedPkgs) == 0 {
			log.Printf("  No package found at path '%s' when resolving enum identifier '%s' (field %s, marker %s)", targetPkgPath, evalResult.IdentifierName, optMeta.Name, markerType)
			return
		}

		// Assuming the first loaded package is the relevant one.
		// For specific import paths, loader.Load should ideally return one package.
		// If targetPkgPath was ".", it might return multiple in some loader implementations,
		// but for Go module structure, "." usually maps to one package.
		pkg := loadedPkgs[0]
		var foundValues []any
		var foundDecl bool // Flag to indicate if the variable declaration was found

		log.Printf("  Searching for VAR '%s' in package '%s' (loaded from '%s')", evalResult.IdentifierName, pkg.ImportPath, targetPkgPath) // Use pkg.ImportPath

		// Get the files from the package
		pkgFiles, err := pkg.Files()
		if err != nil {
			log.Printf("  Error getting files for package '%s': %v", pkg.ImportPath, err)
			return
		}

		for _, loadedFileAst := range pkgFiles { // Iterate through all files in the loaded package
			// Log the file being inspected if needed for detailed debugging:
			// log.Printf("    Inspecting file: %s (package %s)", loadedFileAst.Name.Name, pkg.ImportPath)

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
							log.Printf("    Found var declaration for '%s' in package '%s', file '%s'", evalResult.IdentifierName, pkg.ImportPath, loadedFileAst.Name.Name) // Use pkg.ImportPath
							if len(valSpec.Values) > i {
								initializerExpr := valSpec.Values[i]
								if compLit, ok := initializerExpr.(*ast.CompositeLit); ok {
									// Special handling for []string{string(CONST_A), string(CONST_B), ...}
									var tempValues []any
									elementsAreResolvable := true
									for _, elt := range compLit.Elts {
										if callExpr, okElt := elt.(*ast.CallExpr); okElt {
											if funIdent, okFun := callExpr.Fun.(*ast.Ident); okFun && funIdent.Name == "string" && len(callExpr.Args) == 1 {
												arg := callExpr.Args[0]
												var constStrVal string
												var constFound bool

												if constIdent, okConst := arg.(*ast.Ident); okConst {
													// string(ConstInSamePkg)
													// pkg here is the package where evalResult.IdentifierName (the enum slice variable) is defined.
													constStrVal, constFound = resolveConstStringValue(constIdent.Name, pkg, loadedFileAst)
												} else if selExpr, okSel := arg.(*ast.SelectorExpr); okSel {
													// string(otherpkg.Const)
													if pkgNameIdent, okPkgName := selExpr.X.(*ast.Ident); okPkgName {
														selPkgAlias := pkgNameIdent.Name
														constNameToResolve := selExpr.Sel.Name
														// We need to find the import path for selPkgAlias using the file where the var (e.g. MyLocalEnumValues) is declared (loadedFileAst)
														resolvedSelImportPath := astutils.GetImportPath(loadedFileAst, selPkgAlias)
														if resolvedSelImportPath == "" {
															log.Printf("    Could not resolve import path for package alias '%s' in file %s (used in string(%s.%s))", selPkgAlias, loadedFileAst.Name.Name, selPkgAlias, constNameToResolve)
															elementsAreResolvable = false
															break
														}
														// Load the selected package
														selPkgs, errSel := loader.Load(resolvedSelImportPath)
														if errSel != nil || len(selPkgs) == 0 {
															log.Printf("    Could not load package '%s' for resolving const '%s' in string(%s.%s): %v", resolvedSelImportPath, constNameToResolve, selPkgAlias, constNameToResolve, errSel)
															elementsAreResolvable = false
															break
														}
														constStrVal, constFound = resolveConstStringValue(constNameToResolve, selPkgs[0], nil) // Pass nil for identFile as const is in another package
													} else {
														log.Printf("    Unhandled selector expression in string() argument: X is %T, not *ast.Ident", selExpr.X)
														elementsAreResolvable = false
														break
													}
												} else {
													log.Printf("    Unhandled argument to string() conversion: %T", arg)
													elementsAreResolvable = false
													break
												}

												if constFound {
													tempValues = append(tempValues, constStrVal)
												} else {
													log.Printf("    Could not resolve constant value for element %s in initializer of %s", astutils.ExprToTypeName(elt), evalResult.IdentifierName)
													elementsAreResolvable = false
													break
												}
											} else { // Not a string(IDENT) or string(pkg.IDENT) call
												log.Printf("    Element is a CallExpr but not the expected string(IDENT) pattern: %s", astutils.ExprToTypeName(elt))
												elementsAreResolvable = false
												break
											}
										} else { // Not a CallExpr, try EvaluateArg directly
											elemEval := astutils.EvaluateArg(elt)
											if elemEval.Value != nil {
												tempValues = append(tempValues, elemEval.Value)
											} else {
												log.Printf("    Could not evaluate composite literal element %s directly for %s.", astutils.ExprToTypeName(elt), evalResult.IdentifierName)
												elementsAreResolvable = false
												break
											}
										}
									}
									if elementsAreResolvable {
										foundValues = tempValues
										log.Printf("    Successfully resolved enum identifier '%s' in package '%s' by custom composite literal parsing to values: %v", evalResult.IdentifierName, pkg.ImportPath, foundValues)
										foundDecl = true
									} else {
										log.Printf("    Failed to resolve all elements of composite literal for '%s' in package '%s'.", evalResult.IdentifierName, pkg.ImportPath)
									}
								} else {
									// Fallback to original logic if initializer is not a CompositeLit (e.g. alias to another var)
									resolvedSlice := astutils.EvaluateSliceArg(initializerExpr)
									if resolvedSlice.Value != nil {
										if s, ok := resolvedSlice.Value.([]any); ok {
											foundValues = s
											log.Printf("    Successfully resolved enum identifier '%s' in package '%s' to values (via fallback EvaluateSliceArg): %v", evalResult.IdentifierName, pkg.ImportPath, foundValues)
											foundDecl = true
										} else {
											log.Printf("    Enum variable '%s' initializer in package '%s' resolved via fallback, but not to []any: %T", evalResult.IdentifierName, pkg.ImportPath, resolvedSlice.Value)
										}
									} else if resolvedSlice.IdentifierName != "" {
										log.Printf("    Enum variable '%s' in package '%s' is an alias to another identifier '%s' (pkg '%s') (via fallback). Transitive resolution not yet supported.", evalResult.IdentifierName, pkg.ImportPath, resolvedSlice.IdentifierName, resolvedSlice.PkgName)
									} else {
										log.Printf("    Enum variable '%s' in package '%s' does not have a resolvable slice literal or identifier (via fallback): %T", evalResult.IdentifierName, pkg.ImportPath, initializerExpr)
									}
								}
							} else {
								log.Printf("    Enum variable '%s' in package '%s' has no initializer value at index %d.", evalResult.IdentifierName, pkg.ImportPath, i)
							}
							// Whether resolved or not, we found the declaration, so stop searching for this name.
							// If it wasn't the right type (e.g., not a slice), foundDecl remains false,
							// and the outer logic will report failure to resolve values.
							// To prevent re-processing the same var if it appears multiple times (which shouldn't happen for VARs at package level):
							if !foundDecl { // If not successfully resolved to values
								log.Printf("    Declaration for '%s' found but values not extracted. Stopping further search for this name.", evalResult.IdentifierName)
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
			log.Printf("  Could not find VAR declaration or resolve values for enum identifier '%s' in package '%s' (path searched: '%s', field %s, marker %s)", evalResult.IdentifierName, pkg.ImportPath, targetPkgPath, optMeta.Name, markerType) // Use pkg.ImportPath
		}
		return
	}

	// Neither Value nor IdentifierName is set
	log.Printf("  Enum argument for field %s (marker %s, type %T) could not be evaluated to a literal slice or a resolvable identifier. EvalResult: %+v", optMeta.Name, markerType, evalResult, evalResult)
}
