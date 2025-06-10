package analyzer

import (
	"fmt"
	"go/ast"

	// "go/build" // Unused
	"go/token"
	"go/types"

	// "log/slog" // Unused
	"os"            // Re-add for ReadDir
	"path/filepath" // Re-add for Join
	"reflect"
	"strings"

	// No longer need "bytes" or "go/format" for overlay population from ASTs
	"golang.org/x/tools/go/packages"

	"github.com/podhmo/goat/internal/loader/lazyload"
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

// AnalyzeOptionsV2 finds the Options struct definition using an on-disk temporary module structure.
//   - fset: Token fileset for parsing.
//   - astFilesForLookup: ASTs of files in the target package, primarily used to locate the options struct AST.
//     These ASTs must have been parsed from files whose paths are on disk.
//   - optionsTypeName: Name of the options struct type (e.g., "MainConfig").
//   - targetPackageID: The import path of the package containing optionsTypeName (e.g., "testmodule/example.com/mainpkg").
//   - moduleRootPath: Absolute path to the root of the temporary module (where go.mod is).
func AnalyzeOptionsV2(fset *token.FileSet, astFilesForLookup []*ast.File, optionsTypeName string, targetPackageID string, moduleRootPath string) ([]*metadata.OptionMetadata, string, error) {
	cfg := &packages.Config{
		Fset:    fset,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedModule,
		Dir:     moduleRootPath,          // Root of the temporary module
		Overlay: make(map[string][]byte), // Overlay is not strictly needed if all files are on disk and up-to-date
		// Env: os.Environ(), // Inherit environment
	}
	if moduleRootPath == "" {
		return nil, "", fmt.Errorf("moduleRootPath cannot be empty")
	}
	if targetPackageID == "" {
		return nil, "", fmt.Errorf("targetPackageID cannot be empty")
	}

	var loadPatterns []string
	if targetPackageID == "." {
		// Special handling for non-module, directory-based package loading.
		// List .go files in cfg.Dir and use "file=" patterns.
		goFiles, err := os.ReadDir(cfg.Dir)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read files in cfg.Dir '%s' for '.' target: %w", cfg.Dir, err)
		}
		for _, file := range goFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".go") && !strings.HasSuffix(file.Name(), "_test.go") {
				absPath := filepath.Join(cfg.Dir, file.Name())
				loadPatterns = append(loadPatterns, "file="+absPath)
			}
		}
		if len(loadPatterns) == 0 {
			return nil, "", fmt.Errorf("no .go files found in cfg.Dir '%s' for '.' target", cfg.Dir)
		}
	} else {
		loadPatterns = []string{targetPackageID}
	}

	pkgs, err := packages.Load(cfg, loadPatterns...)
	if err != nil {
		return nil, "", fmt.Errorf("error loading package information (cfg.Dir='%s', patterns=%q): %w", cfg.Dir, loadPatterns, err)
	}
	if len(pkgs) == 0 {
		return nil, "", fmt.Errorf("no packages loaded for type analysis (cfg.Dir='%s', patterns=%q)", cfg.Dir, loadPatterns)
	}

	// Find the specific package that matches targetPackageID
	var currentPkg *packages.Package
	for _, pkg := range pkgs {
		if pkg.ID == targetPackageID {
			currentPkg = pkg
			// Check for critical errors in this specific package
			if len(pkg.Errors) > 0 {
				var errs []string
				for _, e := range pkg.Errors {
					errs = append(errs, e.Error())
				}
				return nil, "", fmt.Errorf("errors in loaded target package %s: %s", pkg.ID, strings.Join(errs, "; "))
			}
			break
		}
	}

	if currentPkg == nil {
		var foundPkgIDs []string
		var errsForPkgs []string
		for _, p := range pkgs {
			foundPkgIDs = append(foundPkgIDs, p.ID)
			if len(p.Errors) > 0 {
				for _, e := range p.Errors {
					errsForPkgs = append(errsForPkgs, fmt.Sprintf("pkg %s error: %s", p.ID, e.Error()))
				}
			}
		}
		if len(errsForPkgs) > 0 {
			return nil, "", fmt.Errorf("target package '%s' not found among loaded packages (%v), and other errors encountered: %s. (cfg.Dir='%s', patterns=%q)", targetPackageID, foundPkgIDs, strings.Join(errsForPkgs, "; "), cfg.Dir, loadPatterns)
		}
		return nil, "", fmt.Errorf("target package '%s' not found among loaded packages: %v. (cfg.Dir='%s', patterns=%q)", targetPackageID, foundPkgIDs, cfg.Dir, loadPatterns)
	}

	// Remove potential module prefix from optionsTypeName if it's fully qualified
	// e.g. "testmodule/example.com/mainpkg.MainConfig" -> "MainConfig"
	// The optionsTypeName should be the simple name for lookup within the package's ASTs.
	var _ *types.Info = nil // TODO: Implement type checking to populate types.Info (currently assigned to _ to avoid unused variable error)
	simpleOptionsTypeName := optionsTypeName
	if strings.Contains(optionsTypeName, ".") {
		parts := strings.Split(optionsTypeName, ".")
		simpleOptionsTypeName = parts[len(parts)-1]
	}
	if strings.HasPrefix(simpleOptionsTypeName, "*") {
		simpleOptionsTypeName = simpleOptionsTypeName[1:]
	}

	var optionsStruct *ast.TypeSpec
	var actualStructName string               // This will be simpleOptionsTypeName if found
	var fileContainingOptionsStruct *ast.File // The AST of the file where the struct is defined

	// Iterate through the ASTs that belong to the currentPkg to find the struct.
	// currentPkg.Syntax contains ASTs for files in this package.
	for _, fileAst := range currentPkg.Syntax { // Use ASTs from the loaded package
		ast.Inspect(fileAst, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				if ts.Name.Name == simpleOptionsTypeName {
					if _, isStruct := ts.Type.(*ast.StructType); isStruct {
						optionsStruct = ts
						actualStructName = ts.Name.Name
						fileContainingOptionsStruct = fileAst
						return false // Stop searching
					}
				}
			}
			return true
		})
		if optionsStruct != nil {
			break
		}
	}

	if optionsStruct == nil {
		return nil, "", fmt.Errorf("options struct type '%s' (simple name '%s') not found in package '%s'", optionsTypeName, simpleOptionsTypeName, currentPkg.ID)
	}
	if fileContainingOptionsStruct == nil { // Should be set if optionsStruct is not nil
		return nil, "", fmt.Errorf("internal error: options struct '%s' found but its containing AST was not identified within package %s", actualStructName, currentPkg.ID)
	}

	structType, ok := optionsStruct.Type.(*ast.StructType)
	if !ok {
		return nil, actualStructName, fmt.Errorf("type '%s' is not a struct type", actualStructName)
	}

	var extractedOptions []*metadata.OptionMetadata
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 { // Embedded struct
			embeddedTypeName := astutils.ExprToTypeName(field.Type) // e.g., "MyEmbedded", "pkg.ExternalType", "*pkg.ExternalType"
			var embeddedOptions []*metadata.OptionMetadata
			var err error

			selParts := strings.SplitN(strings.TrimPrefix(embeddedTypeName, "*"), ".", 2)
			if len(selParts) == 2 { // External package selector found, e.g. "myexternalpkg.ExternalEmbedded"
				pkgSelectorInAST := selParts[0] // e.g., "myexternalpkg"
				typeNameInExternalPkg := selParts[1]

				// Resolve pkgSelectorInAST to its full import path.
				// currentPkg.Imports is map[string]*Package where key is import path, value is Package.
				// pkgSelectorInAST is the package name used in the selector expression (e.g., "myexternalpkg").
				var externalPkgImportPath string
				var resolvedImportedPkg *packages.Package

				// Iterate through currentPkg.Imports to find the one whose Name matches pkgSelectorInAST.
				// This is necessary because the selector uses the package's actual name (or alias),
				// not necessarily its full import path directly in the AST.
				foundImportMatchingSelector := false
				for _, imp := range currentPkg.Imports {
					if imp.Name == pkgSelectorInAST {
						externalPkgImportPath = imp.ID
						resolvedImportedPkg = imp
						foundImportMatchingSelector = true
						break
					}
				}

				if !foundImportMatchingSelector {
					return nil, actualStructName, fmt.Errorf("could not resolve external package selector '%s' (looking for package named '%s') via imports of package '%s'. Available imports (path -> name): %v", pkgSelectorInAST, pkgSelectorInAST, currentPkg.ID, currentPkg.Imports)
				}

				// The astFilesForLookup passed to the recursive call should ideally be the ASTs
				// specific to the externalPkgImportPath. However, packages.Load would have loaded them,
				// and the recursive call will select the correct ASTs from currentPkg.Syntax of *that* package.
				// So, passing the original astFilesForLookup (which corresponds to the initial targetPackageID's files)
				// is not correct here. The recursive call needs to operate on the ASTs of *its* target package.
				// The `pkgs` slice from the initial Load should contain all necessary packages.
				// The `astFilesForLookup` argument is primarily for finding the top-level struct.
				// For recursion, we primarily need fset, typeName, targetPackageID, and moduleRootPath.
				// The recursive call will then use its own targetPackageID to find its ASTs from its loaded pkg.Syntax.
				// Thus, we can pass nil or an empty slice for astFilesForLookup in recursive calls,
				// as the relevant ASTs are already loaded by packages.Load and are in pkg.Syntax.
				// The ASTs for the external package are in resolvedImportedPkg.Syntax
				if resolvedImportedPkg == nil { // Should not happen if foundImportMatchingSelector is true
					return nil, actualStructName, fmt.Errorf("internal error: resolvedImportedPkg is nil for selector '%s'", pkgSelectorInAST)
				}
				relevantASTsForExternal := resolvedImportedPkg.Syntax
				if len(relevantASTsForExternal) == 0 && resolvedImportedPkg.Name != "" { // Check PkgPath for stdlib
					// This might indicate an issue if a package (especially stdlib) has no ASTs,
					// but type info should still be available. AnalyzeOptionsV2 expects ASTs for struct lookup.
					// If the embedded type is from stdlib and has no ASTs in Syntax, this will fail to find the struct AST.
					// This logic assumes all analyzed structs (even from stdlib) will have their ASTs available.
					// For now, proceed; if typeNameInExternalPkg is not found, it will error appropriately.
				}

				embeddedOptions, _, err = AnalyzeOptionsV2(fset, relevantASTsForExternal, typeNameInExternalPkg, externalPkgImportPath, moduleRootPath)
			} else { // Embedded struct from the same package
				cleanEmbeddedTypeName := strings.TrimPrefix(embeddedTypeName, "*")
				// For same-package embedded structs, use currentPkg.Syntax.
				embeddedOptions, _, err = AnalyzeOptionsV2(fset, currentPkg.Syntax, cleanEmbeddedTypeName, targetPackageID, moduleRootPath)
			}

			if err != nil {
				return nil, actualStructName, fmt.Errorf("error analyzing embedded struct '%s' (from type %s): %w", embeddedTypeName, currentPkg.ID, err)
			}
			extractedOptions = append(extractedOptions, embeddedOptions...)
			continue
		}

		fieldName := field.Names[0].Name
		if !ast.IsExported(fieldName) {
			continue
		}

		opt := &metadata.OptionMetadata{
			Name:              fieldName,
			CliName:           stringutils.ToKebabCase(fieldName),
			TypeName:          astutils.ExprToTypeName(field.Type),
			IsPointer:         astutils.IsPointerType(field.Type),
			IsRequired:        !astutils.IsPointerType(field.Type),
			IsTextUnmarshaler: false,
			IsTextMarshaler:   false,
		}

		if currentPkg.TypesInfo != nil && field.Names[0] != nil {
			obj := currentPkg.TypesInfo.Defs[field.Names[0]]
			if obj != nil {
				tv := obj.Type()
				if tv != nil {
					if types.Implements(tv, textUnmarshalerType) {
						opt.IsTextUnmarshaler = true
					}
					if !opt.IsTextUnmarshaler && types.Implements(types.NewPointer(tv), textUnmarshalerType) {
						opt.IsTextUnmarshaler = true
					}
					if types.Implements(tv, textMarshalerType) {
						opt.IsTextMarshaler = true
					}
					if !opt.IsTextMarshaler && types.Implements(types.NewPointer(tv), textMarshalerType) {
						opt.IsTextMarshaler = true
					}
				}
			} else {
				// Fallback for fields that might not be in Defs (e.g. embedded fields from external unaliased packages)
				// Try TypeOf if Defs fails.
				tv := currentPkg.TypesInfo.TypeOf(field.Type)
				if tv != nil {
					if types.Implements(tv, textUnmarshalerType) {
						opt.IsTextUnmarshaler = true
					}
					if !opt.IsTextUnmarshaler && types.Implements(types.NewPointer(tv), textUnmarshalerType) {
						opt.IsTextUnmarshaler = true
					}
					if types.Implements(tv, textMarshalerType) {
						opt.IsTextMarshaler = true
					}
					if !opt.IsTextMarshaler && types.Implements(types.NewPointer(tv), textMarshalerType) {
						opt.IsTextMarshaler = true
					}
				}
			}
		}

		if field.Doc != nil {
			opt.HelpText = strings.TrimSpace(field.Doc.Text())
		}
		if field.Comment != nil {
			if opt.HelpText != "" {
				opt.HelpText += "\n"
			}
			opt.HelpText += strings.TrimSpace(field.Comment.Text())
			opt.HelpText = strings.TrimSpace(opt.HelpText)
		}

		if field.Tag != nil {
			tagStr := strings.Trim(field.Tag.Value, "`")
			tag := reflect.StructTag(tagStr)
			if envVar, ok := tag.Lookup("env"); ok {
				opt.EnvVar = envVar
			}
		}
		extractedOptions = append(extractedOptions, opt)
	}
	return extractedOptions, actualStructName, nil
}

// AnalyzeOptionsV3 finds the Options struct definition using the lazyload package.
// It performs type analysis for interface checking and resolving embedded structs.
//
//   - fset: Token fileset for parsing.
//   - optionsTypeName: Name of the options struct type (e.g., "MainConfig").
//   - targetPackagePath: The import path of the package containing optionsTypeName.
//   - baseDir: The base directory from which to resolve targetPackagePath (often module root).
//   - llConfig: Configuration for the lazyload.Loader. If nil, a default will be used.
func AnalyzeOptionsV3(
	fset *token.FileSet, // Still needed for some astutils. Ideally, use fset from ldr.
	optionsTypeName string,
	targetPackagePath string, // The expected import path of the package.
	baseDir string, // The file system directory of the package.
	ldr *lazyload.Loader,
) ([]*metadata.OptionMetadata, string, error) {
	if ldr == nil {
		return nil, "", fmt.Errorf("AnalyzeOptionsV3: loader cannot be nil")
	}

	// The primary way to load the package is by its directory `baseDir`.
	// The `targetPackagePath` (import path) is used to verify we loaded the correct package.
	loadedPkgs, err := ldr.Load(baseDir)
	if err != nil {
		return nil, "", fmt.Errorf("error loading package at dir '%s' with loader: %w", baseDir, err)
	}
	if len(loadedPkgs) == 0 {
		return nil, "", fmt.Errorf("no package found at dir '%s' by loader", baseDir)
	}

	var currentPkg *lazyload.Package
	for _, pkg := range loadedPkgs {
		// Verify that the loaded package's directory or import path matches expectations.
		// The GoListLocator, when given a directory, should return a package whose Dir matches that directory
		// and whose ImportPath matches what `go list` resolves for that directory.
		if pkg.Dir == baseDir {
			if pkg.ImportPath == targetPackagePath {
				currentPkg = pkg
				break
			} else {
				// If Dir matches but ImportPath doesn't, it might be a test setup where
				// go.mod's module name doesn't align with the expected targetPackagePath structure.
				// For tests, often targetPackagePath is just the module name (e.g., "testcmdmodule").
				// If pkg.Name (Go package name) also matches the last part of targetPackagePath, consider it a match.
				// This is a heuristic. A more robust check would involve comparing module paths.
				_, pkgNameFromFile := filepath.Split(targetPackagePath)
				if pkg.Name == pkgNameFromFile {
					currentPkg = pkg
					break
				}
			}
		}
	}

	if currentPkg == nil {
		var availablePkgInfo []string
		for _, pkg := range loadedPkgs {
			availablePkgInfo = append(availablePkgInfo, fmt.Sprintf("{ImportPath: %s, Dir: %s, Name: %s}", pkg.ImportPath, pkg.Dir, pkg.Name))
		}
		return nil, "", fmt.Errorf("target package '%s' (expected at dir '%s') not found or matched in loaded packages: %s", targetPackagePath, baseDir, strings.Join(availablePkgInfo, ", "))
	}

	// Ensure the found package's import path is consistent with targetPackagePath, if possible.
	// This helps catch cases where baseDir loaded something unexpected.
	if currentPkg.ImportPath != targetPackagePath {
		// This might be too strict for some test cases where module name vs import path can be tricky.
		// For example, if targetPackagePath is "testmodule" and ImportPath is "testmodule/testcmdmodule"
		// due to go list behavior with subdirectories.
		// For now, we proceed if a package was found by directory.
		// slog.Debug("AnalyzeOptionsV3: Loaded package import path mismatch", "expected", targetPackagePath, "actual", currentPkg.ImportPath, "dir", baseDir)
	}

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
						tempResolvedPkg, errTmpResolve := currentPkg.ResolveImport(path)
						if errTmpResolve == nil && tempResolvedPkg != nil && tempResolvedPkg.Name == externalPkgSelector {
							resolvedExternalImportPath = path
							break
						}
					}
				}
				if resolvedExternalImportPath == "" {
					return nil, actualStructName, fmt.Errorf("unable to resolve import path for selector '%s' in embedded type '%s'", externalPkgSelector, embeddedTypeName)
				}
				resolvedExternalPkg, errResolve := currentPkg.ResolveImport(resolvedExternalImportPath)
				if errResolve != nil {
					return nil, actualStructName, fmt.Errorf("could not resolve imported package for path '%s': %w", resolvedExternalImportPath, errResolve)
				}
				if resolvedExternalPkg == nil {
					return nil, actualStructName, fmt.Errorf("resolved imported package is nil for path '%s'", resolvedExternalImportPath)
				}
				embeddedOptions, _, embErr = AnalyzeOptionsV3(fset, typeNameInExternalPkg, resolvedExternalPkg.ImportPath, resolvedExternalPkg.Dir, ldr)
			} else { // Embedded struct from the same package
				cleanEmbeddedTypeName := strings.TrimPrefix(embeddedTypeName, "*")
				embeddedOptions, _, embErr = AnalyzeOptionsV3(fset, cleanEmbeddedTypeName, targetPackagePath, baseDir, ldr)
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

		isUnmarshaler, errUnmarshaler := fieldInfo.ImplementsInterface("encoding", "TextUnmarshaler")
		if errUnmarshaler != nil {
			fmt.Println(fmt.Sprintf("analyzer: warning: error checking TextUnmarshaler for field %s type %s: %v", fieldInfo.Name, opt.TypeName, errUnmarshaler))
			opt.IsTextUnmarshaler = false
		} else {
			opt.IsTextUnmarshaler = isUnmarshaler
		}

		isMarshaler, errMarshaler := fieldInfo.ImplementsInterface("encoding", "TextMarshaler")
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
