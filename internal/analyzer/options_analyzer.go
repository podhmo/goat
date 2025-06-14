package analyzer

import (
	"context"
	"fmt"
	"go/ast"

	// "go/build" // Unused
	"go/token"
	"go/types"

	// "log/slog" // Unused
	"os"            // Re-add for ReadDir
	"path/filepath" // Re-add for Join
	"strings"

	// No longer need "bytes" or "go/format" for overlay population from ASTs
	// "golang.org/x/tools/go/packages" // Removed unused import

	"github.com/podhmo/goat/internal/loader" // Changed
	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/astutils"
	"github.com/podhmo/goat/internal/utils/stringutils"
	// Added for V3
	// "golang.org/x/tools/go/importer" // May need for V3 type checking without go/packages
)

var (
	textUnmarshalerType *types.Interface
	textMarshalerType   *types.Interface
)

func init() {
	// func UnmarshalText(text []byte) error
	textUnmarshalerMeth := types.NewFunc(token.NoPos, nil, "UnmarshalText", types.NewSignatureType(
		nil, // recv
		nil, // recv type params
		nil, // type params
		types.NewTuple(types.NewParam(token.NoPos, nil, "", types.NewSlice(types.Universe.Lookup("byte").Type()))), // params
		types.NewTuple(types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type())),                // results
		false, // variadic
	))
	textUnmarshalerType = types.NewInterfaceType([]*types.Func{textUnmarshalerMeth}, nil).Complete()

	// func MarshalText() (text []byte, err error)
	textMarshalerMeth := types.NewFunc(token.NoPos, nil, "MarshalText", types.NewSignatureType(
		nil, // recv
		nil, // recv type params
		nil, // type params
		nil, // params
		types.NewTuple( // results
			types.NewParam(token.NoPos, nil, "", types.NewSlice(types.Universe.Lookup("byte").Type())),
			types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type()),
		),
		false, // variadic
	))
	textMarshalerType = types.NewInterfaceType([]*types.Func{textMarshalerMeth}, nil).Complete()
}

// AnalyzeOptions finds the Options struct definition using the lazyload package.
// It performs type analysis for interface checking and resolving embedded structs.
//
//   - fset: Token fileset for parsing.
//   - optionsTypeName: Name of the options struct type (e.g., "MainConfig").
//   - targetPackagePath: The import path of the package containing optionsTypeName.
//
// It performs type analysis for interface checking and resolving embedded structs.
//
//   - fset: Token fileset for parsing.
//   - optionsTypeName: Name of the options struct type (e.g., "MainConfig").
//   - targetPackagePath: The import path of the package containing optionsTypeName.
//   - baseDir: The base directory from which to resolve targetPackagePath (often module root).
//   - loader: Instance of loader.Loader.
func AnalyzeOptions( // Renamed from AnalyzeOptionsV3
	ctx context.Context,
	fset *token.FileSet, // Still needed for some astutils
	optionsTypeName string,
	targetPackagePath string,
	baseDir string,
	loader *loader.Loader, // Changed from llConfig *loader.Config
) ([]*metadata.OptionMetadata, string, error) {
	// Heuristic adjustment for loadPattern based on typical test setups.
	// If targetPackagePath is simple (no slashes, e.g., a module name) and baseDir is set,
	// it's likely a test scenario where baseDir is the module root. In this case, "." is the correct
	// pattern for `go list` to identify the package at the root of the module.
	loadPattern := targetPackagePath
	if baseDir != "" && !strings.Contains(targetPackagePath, "/") {
		// Check if go.mod exists to strengthen the heuristic, assuming module mode.
		goModPath := filepath.Join(baseDir, "go.mod")
		if _, statErr := os.Stat(goModPath); statErr == nil {
			loadPattern = "." // Load package in current directory (baseDir)
		}
	}

	// loader is now passed in directly

	loadedPkgs, err := loader.Load(ctx, loadPattern) // Use the passed-in loader. baseDir is handled by the locator if necessary.
	if err != nil {
		// Updated error message to reflect that baseDir is not directly used by Load() here.
		return nil, "", fmt.Errorf("error loading package '%s' (derived load pattern '%s') with loader: %w", targetPackagePath, loadPattern, err)
	}
	if len(loadedPkgs) == 0 {
		// Updated error message
		return nil, "", fmt.Errorf("no package found for '%s' (derived load pattern '%s') by loader", targetPackagePath, loadPattern)
	}
	currentPkg := loadedPkgs[0]

	simpleOptionsTypeName := optionsTypeName
	if strings.Contains(optionsTypeName, ".") {
		parts := strings.Split(optionsTypeName, ".")
		simpleOptionsTypeName = parts[len(parts)-1]
	}
	if strings.HasPrefix(simpleOptionsTypeName, "*") {
		simpleOptionsTypeName = simpleOptionsTypeName[1:]
	}

	actualStructName := simpleOptionsTypeName // Default if not found, GetStruct will confirm.
	optionsStructInfo, err := currentPkg.GetStruct(simpleOptionsTypeName)
	if err != nil {
		return nil, "", fmt.Errorf("options struct type '%s' (simple name '%s') not found in package '%s': %w", optionsTypeName, simpleOptionsTypeName, currentPkg.ImportPath, err)
	}
	actualStructName = optionsStructInfo.Name

	var fileContainingOptionsStruct *ast.File
	filesMap, errFiles := currentPkg.Files()
	if errFiles != nil {
		return nil, actualStructName, fmt.Errorf("could not get AST files for package '%s': %w", currentPkg.ImportPath, errFiles)
	}
	for _, astFile := range filesMap {
		ast.Inspect(astFile, func(n ast.Node) bool {
			if n == optionsStructInfo.Node {
				fileContainingOptionsStruct = astFile
				return false
			}
			return true
		})
		if fileContainingOptionsStruct != nil {
			break
		}
	}
	if fileContainingOptionsStruct == nil {
		return nil, actualStructName, fmt.Errorf("could not find AST file for options struct '%s' in package '%s'", actualStructName, currentPkg.ImportPath)
	}

	var extractedOptions []*metadata.OptionMetadata
	for _, fieldInfo := range optionsStructInfo.Fields {
		if fieldInfo.Embedded {
			embeddedTypeName := astutils.ExprToTypeName(fieldInfo.TypeExpr)
			var embeddedOptions []*metadata.OptionMetadata
			var embErr error

			var externalPkgSelector string
			var typeNameInExternalPkg string
			isExternal := false

			typeExpr := fieldInfo.TypeExpr
			if starExpr, ok := typeExpr.(*ast.StarExpr); ok {
				typeExpr = starExpr.X
			}
			if selExpr, ok := typeExpr.(*ast.SelectorExpr); ok {
				if ident, ok := selExpr.X.(*ast.Ident); ok {
					externalPkgSelector = ident.Name
					typeNameInExternalPkg = selExpr.Sel.Name
					isExternal = true
				}
			}

			if isExternal {
				resolvedExternalImportPath := ""
				for _, importSpec := range fileContainingOptionsStruct.Imports {
					path := strings.Trim(importSpec.Path.Value, "\"")
					if importSpec.Name != nil {
						if importSpec.Name.Name == externalPkgSelector {
							resolvedExternalImportPath = path
							break
						}
					} else {
						tempResolvedPkg, errTmpResolve := currentPkg.ResolveImport(ctx, path)
						if errTmpResolve == nil && tempResolvedPkg != nil && tempResolvedPkg.Name == externalPkgSelector {
							resolvedExternalImportPath = path
							break
						}
					}
				}
				if resolvedExternalImportPath == "" {
					return nil, actualStructName, fmt.Errorf("unable to resolve import path for selector '%s' in embedded type '%s'", externalPkgSelector, embeddedTypeName)
				}
				resolvedExternalPkg, errResolve := currentPkg.ResolveImport(ctx, resolvedExternalImportPath)
				if errResolve != nil {
					return nil, actualStructName, fmt.Errorf("could not resolve imported package for path '%s': %w", resolvedExternalImportPath, errResolve)
				}
				if resolvedExternalPkg == nil {
					return nil, actualStructName, fmt.Errorf("resolved imported package is nil for path '%s'", resolvedExternalImportPath)
				}
				// Pass loader directly
				embeddedOptions, _, embErr = AnalyzeOptions(ctx, fset, typeNameInExternalPkg, resolvedExternalPkg.ImportPath, resolvedExternalPkg.Dir, loader)
			} else { // Embedded struct from the same package
				cleanEmbeddedTypeName := strings.TrimPrefix(embeddedTypeName, "*")
				// Pass loader directly
				embeddedOptions, _, embErr = AnalyzeOptions(ctx, fset, cleanEmbeddedTypeName, targetPackagePath, baseDir, loader)
			}

			if embErr != nil {
				return nil, actualStructName, fmt.Errorf("error analyzing embedded struct '%s': %w", embeddedTypeName, embErr)
			}
			extractedOptions = append(extractedOptions, embeddedOptions...)
			continue
		}

		fieldName := fieldInfo.Name
		if !ast.IsExported(fieldName) {
			continue
		}

		opt := &metadata.OptionMetadata{
			Name:       fieldName,
			CliName:    stringutils.ToKebabCase(fieldName),
			TypeName:   astutils.ExprToTypeName(fieldInfo.TypeExpr),
			IsPointer:  astutils.IsPointerType(fieldInfo.TypeExpr),
			IsRequired: !astutils.IsPointerType(fieldInfo.TypeExpr),
		}
		opt.UnderlyingKind = "" // Initialize

		var typeExprForKindCheck ast.Expr = fieldInfo.TypeExpr
		// Use opt.IsPointer as it's already determined by astutils.IsPointerType
		if opt.IsPointer {
			if starExpr, ok := fieldInfo.TypeExpr.(*ast.StarExpr); ok {
				typeExprForKindCheck = starExpr.X
			}
		}

		var resolvedTypeSpec *ast.TypeSpec
		var resolveErr error

		switch te := typeExprForKindCheck.(type) {
		case *ast.Ident: // Type is in the current package
			typeSpec, _, err := currentPkg.FindTypeSpec(te.Name)
			if err == nil {
				resolvedTypeSpec = typeSpec
			} else {
				resolveErr = fmt.Errorf("error finding typespec for %s in current pkg: %w", te.Name, err)
			}
		case *ast.SelectorExpr: // Type is in an imported package
			pkgSelectorIdent, okX := te.X.(*ast.Ident)
			if !okX {
				resolveErr = fmt.Errorf("unsupported selector expression X: %T for field %s", te.X, fieldInfo.Name)
				break // Break from switch
			}
			// fileContainingOptionsStruct is available from the outer scope in AnalyzeOptions
			// currentPkg is also available.
			_, definingPkg, err := currentPkg.GetImportPathBySelector(ctx, pkgSelectorIdent.Name, fileContainingOptionsStruct)
			if err == nil && definingPkg != nil {
				typeSpec, _, err := definingPkg.FindTypeSpec(te.Sel.Name)
				if err == nil {
					resolvedTypeSpec = typeSpec
				} else {
					resolveErr = fmt.Errorf("error finding typespec for %s in pkg %s: %w", te.Sel.Name, definingPkg.ImportPath, err)
				}
			} else if err != nil {
				resolveErr = fmt.Errorf("error resolving import for selector %s for field %s: %w", pkgSelectorIdent.Name, fieldInfo.Name, err)
			} else {
				resolveErr = fmt.Errorf("definingPkg is nil for selector %s for field %s", pkgSelectorIdent.Name, fieldInfo.Name)
			}
		default:
			// Not a type name we can easily look up (e.g., could be built-in, or complex like []string, map, func)
			// opt.UnderlyingKind remains ""
		}

		if resolveErr != nil {
			// Optional: log this error for debugging if needed, but don't fail the whole analysis.
			// For example:
			// fmt.Printf("analyzer: warning: could not fully resolve type for UnderlyingKind check for field %s: %v\n", fieldInfo.Name, resolveErr)
		}

		if resolvedTypeSpec != nil && resolvedTypeSpec.Type != nil {
			if ident, ok := resolvedTypeSpec.Type.(*ast.Ident); ok {
				// ident.Name will be the string representation of the underlying type
				// e.g., "string", "int", "bool" for basic types.
				switch ident.Name {
				case "string", "int", "bool", "float64", "float32", "int64", "int32", "int16", "int8", "uint64", "uint32", "uint16", "uint8", "uintptr", "byte", "rune":
					opt.UnderlyingKind = ident.Name
				default:
					// Not a basic type we're explicitly handling for UnderlyingKind.
					// opt.UnderlyingKind remains ""
				}
			}
		}
		// End of UnderlyingKind determination

		isUnmarshaler, errUnmarshaler := fieldInfo.ImplementsInterface(ctx, "encoding", "TextUnmarshaler")
		if errUnmarshaler != nil {
			fmt.Println(fmt.Sprintf("analyzer: warning: error checking TextUnmarshaler for field %s type %s: %v", fieldInfo.Name, opt.TypeName, errUnmarshaler))
			opt.IsTextUnmarshaler = false
		} else {
			opt.IsTextUnmarshaler = isUnmarshaler
		}

		isMarshaler, errMarshaler := fieldInfo.ImplementsInterface(ctx, "encoding", "TextMarshaler")
		if errMarshaler != nil {
			fmt.Println(fmt.Sprintf("analyzer: warning: error checking TextMarshaler for field %s type %s: %v", fieldInfo.Name, opt.TypeName, errMarshaler))
			opt.IsTextMarshaler = false
		} else {
			opt.IsTextMarshaler = isMarshaler
		}

		var fieldASTNode *ast.Field
		structTypeSpec := optionsStructInfo.Node
		if structType, okType := structTypeSpec.Type.(*ast.StructType); okType {
			for _, astField := range structType.Fields.List {
				for _, nameIdent := range astField.Names {
					if nameIdent.Name == fieldName {
						fieldASTNode = astField
						break
					}
				}
				if fieldASTNode != nil {
					break
				}
			}
		}

		if fieldASTNode != nil {
			if fieldASTNode.Doc != nil {
				opt.HelpText = strings.TrimSpace(fieldASTNode.Doc.Text())
			}
			if fieldASTNode.Comment != nil {
				if opt.HelpText != "" {
					opt.HelpText += "\n"
				}
				opt.HelpText += strings.TrimSpace(fieldASTNode.Comment.Text())
			}
			opt.HelpText = strings.TrimSpace(opt.HelpText) // From AST comments

			// Populate HelpText from struct tag "comment" or "description"
			// Tag value overrides AST comments if present.
			tagComment := fieldInfo.GetTag("comment")
			if tagComment != "" {
				opt.HelpText = tagComment
			} else {
				tagDescription := fieldInfo.GetTag("description")
				if tagDescription != "" {
					opt.HelpText = tagDescription
				}
			}
			// Ensure HelpText is trimmed one last time after potentially using tags
			opt.HelpText = strings.TrimSpace(opt.HelpText)
		}

		if tagVal := fieldInfo.GetTag("env"); tagVal != "" {
			opt.EnvVar = tagVal
		}
		extractedOptions = append(extractedOptions, opt)
	}
	return extractedOptions, actualStructName, nil
}

/*
// Original AnalyzeOptions - keep for now if other parts of the codebase use it,
// or remove if AnalyzeOptionsV2 is a direct replacement.
// For this refactoring, we assume it's being replaced.
func AnalyzeOptions_Original(fset *token.FileSet, files []*ast.File, optionsTypeName string, currentPackageName string) ([]*metadata.OptionMetadata, string, error) {
	// This is a placeholder for the original AnalyzeOptions function's content.
	// To make it a valid, non-interfering comment, ensure it's properly commented out.
	// For example, if it contained block comments, ensure they are nested correctly or removed.
	return nil, "", nil // Placeholder return
}
*/
// Ensure there's a newline at the very end of the file.
