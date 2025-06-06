package analyzer

import (
	"fmt"
	"go/ast"
	// "go/build" // Unused
	"go/token"
	"go/types"
	// "log/slog" // Unused
	"os" // Re-add for ReadDir
	"path/filepath" // Re-add for Join
	"reflect"
	"strings"

	// No longer need "bytes" or "go/format" for overlay population from ASTs
	"golang.org/x/tools/go/packages"

	// "github.com/podhmo/goat/internal/loader" // Unused in V2, recursive calls use AnalyzeOptionsV2
	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/astutils"
	"github.com/podhmo/goat/internal/utils/stringutils"
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
		types.NewTuple(types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type())), // results
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
// - fset: Token fileset for parsing.
// - astFilesForLookup: ASTs of files in the target package, primarily used to locate the options struct AST.
//                      These ASTs must have been parsed from files whose paths are on disk.
// - optionsTypeName: Name of the options struct type (e.g., "MainConfig").
// - targetPackageID: The import path of the package containing optionsTypeName (e.g., "testmodule/example.com/mainpkg").
// - moduleRootPath: Absolute path to the root of the temporary module (where go.mod is).
func AnalyzeOptionsV2(fset *token.FileSet, astFilesForLookup []*ast.File, optionsTypeName string, targetPackageID string, moduleRootPath string) ([]*metadata.OptionMetadata, string, error) {
	cfg := &packages.Config{
		Fset:    fset,
		Mode:    packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedModule,
		Dir:     moduleRootPath, // Root of the temporary module
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
	simpleOptionsTypeName := optionsTypeName
	if strings.Contains(optionsTypeName, ".") {
		parts := strings.Split(optionsTypeName, ".")
		simpleOptionsTypeName = parts[len(parts)-1]
	}
	if strings.HasPrefix(simpleOptionsTypeName, "*") {
		simpleOptionsTypeName = simpleOptionsTypeName[1:]
	}


	var optionsStruct *ast.TypeSpec
	var actualStructName string       // This will be simpleOptionsTypeName if found
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
					if types.Implements(tv, textUnmarshalerType) { opt.IsTextUnmarshaler = true }
					if !opt.IsTextUnmarshaler && types.Implements(types.NewPointer(tv), textUnmarshalerType) { opt.IsTextUnmarshaler = true }
					if types.Implements(tv, textMarshalerType) { opt.IsTextMarshaler = true }
					if !opt.IsTextMarshaler && types.Implements(types.NewPointer(tv), textMarshalerType) { opt.IsTextMarshaler = true }
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


// Original AnalyzeOptions - keep for now if other parts of the codebase use it,
// or remove if AnalyzeOptionsV2 is a direct replacement.
// For this refactoring, we assume it's being replaced.
/*
func AnalyzeOptions(fset *token.FileSet, files []*ast.File, optionsTypeName string, currentPackageName string) ([]*metadata.OptionMetadata, string, error) {
	// ... original content ...
}
*/
