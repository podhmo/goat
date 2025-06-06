package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types" // New import
	"reflect"
	"strings"

	"bytes" // New import
	"go/format" // New import
	"golang.org/x/tools/go/packages" // New import

	"github.com/podhmo/goat/internal/loader"
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

// AnalyzeOptions finds the Options struct definition (given its type name)
// and extracts metadata for each of its fields.
func AnalyzeOptions(fset *token.FileSet, files []*ast.File, optionsTypeName string, currentPackageName string) ([]*metadata.OptionMetadata, string, error) {
	// Load package information for type analysis
	cfg := &packages.Config{
		Fset: fset,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Overlay: make(map[string][]byte),
	}

	filePaths := make([]string, len(files))
	for i, fileAst := range files {
		filePath := fset.File(fileAst.Pos()).Name()
		filePaths[i] = filePath
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, fileAst); err != nil {
			return nil, "", fmt.Errorf("error formatting AST for overlay: %w", err)
		}
		cfg.Overlay[filePath] = buf.Bytes()
	}

	// Using "pattern=file=..." for specific files. This assumes files are in the current directory or paths are absolute.
	// If files are from different packages not rooted in current dir, this might need adjustment or multiple Load calls.
	// For now, we assume 'files' belong to a single primary package context for this call.
	// The pattern here is tricky. If filePaths are absolute, they can be used directly.
	// If they are relative, packages.Load might need more context.
	// A common pattern is to load based on directory: `pkgs, err := packages.Load(cfg, "./...")`
	// But we want to be specific to the files provided.
	// Let's try with the file paths directly. This works if they are discoverable by the build system.
	// If `files` can come from anywhere, `loader.LoadPackageFiles` for external packages (as done for embedded structs)
	// is a more robust approach for *those* specific external files.
	// For the *current* set of files, if they represent a single package, their file paths should work.
	// The challenge is when `files` is a mix from different packages not related by local import paths.
	// The original code's recursive calls to AnalyzeOptions with `loader.LoadPackageFiles` for external *embedded*
	// types is a good model. Here, for the *initial* call, we assume `files` represent the target options struct's package.
	var loadPatterns []string
	for _, fp := range filePaths {
		loadPatterns = append(loadPatterns, "file="+fp)
	}
	if len(loadPatterns) == 0 && len(files) > 0 { // fallback if filePaths somehow empty but files exist
		// This case implies an issue with fset.File(fileAst.Pos()).Name() or how files were passed.
		// Defaulting to current directory, but this is a guess.
		loadPatterns = []string{"."}
	} else if len(loadPatterns) == 0 { // No files provided
		return nil, "", fmt.Errorf("no files provided to AnalyzeOptions")
	}


	pkgs, err := packages.Load(cfg, loadPatterns...)
	if err != nil {
		return nil, "", fmt.Errorf("error loading package information for type analysis: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, "", fmt.Errorf("no packages loaded for type analysis (patterns: %v)", loadPatterns)
	}
	// We expect one primary package for the options struct itself.
	// If multiple files from same package are given, they'll be part of the same packages.Package.
	// If files are from different packages, pkgs[0] might not be the one we want.
	// For now, assume pkgs[0] is the relevant one containing the optionsTypeName.
	// A more robust way would be to find which pkg in pkgs defines optionsTypeName.
	currentPkg := pkgs[0] // Simplified assumption
	if len(currentPkg.Errors) > 0 {
		var errs []string
		for _, e := range currentPkg.Errors {
			errs = append(errs, e.Error())
		}
		return nil, "", fmt.Errorf("errors in loaded package %s: %s", currentPkg.ID, strings.Join(errs, "; "))
	}


	var optionsStruct *ast.TypeSpec
	var actualStructName string
	var fileContainingOptionsStruct *ast.File

	// Remove package prefix if present (e.g. "main.Options" -> "Options")
	// And remove pointer prefix if present (e.g. "*Options" -> "Options")
	parts := strings.Split(optionsTypeName, ".")
	typeNameOnly := parts[len(parts)-1]
	if strings.HasPrefix(typeNameOnly, "*") {
		typeNameOnly = typeNameOnly[1:]
	}

	for _, fileAst := range files {
		ast.Inspect(fileAst, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				if ts.Name.Name == typeNameOnly {
					if _, isStruct := ts.Type.(*ast.StructType); isStruct {
						optionsStruct = ts
						actualStructName = ts.Name.Name
						fileContainingOptionsStruct = fileAst // Store the file
						return false                          // Stop searching this file
					}
				}
			}
			return true
		})
		if optionsStruct != nil {
			if fileContainingOptionsStruct == nil && len(files) == 1 { // Safety check / common case
				fileContainingOptionsStruct = files[0]
			}
			break // Found in one of the files
		}
	}

	if optionsStruct == nil {
		return nil, "", fmt.Errorf("options struct type '%s' not found in package '%s'", typeNameOnly, currentPackageName)
	}
	if fileContainingOptionsStruct == nil {
		// This should ideally not be reached if optionsStruct was found
		return nil, "", fmt.Errorf("internal error: options struct '%s' found but its containing file was not identified", actualStructName)
	}

	structType, ok := optionsStruct.Type.(*ast.StructType)
	if !ok {
		// This should not happen if the previous check passed
		return nil, actualStructName, fmt.Errorf("type '%s' is not a struct type", actualStructName)
	}

	var extractedOptions []*metadata.OptionMetadata
	for _, field := range structType.Fields.List {
		// Ensure field.Names is not empty before proceeding to avoid panic
		if len(field.Names) == 0 { // Embedded struct
			// For embedded structs, the type analysis for TextMarshaler/Unmarshaler
			// would need to happen within the recursive call to AnalyzeOptions,
			// ensuring the correct package context is loaded there.
			// The current pkgs[0] is for the *containing* struct's package.
			embeddedTypeName := astutils.ExprToTypeName(field.Type)
			var embeddedOptions []*metadata.OptionMetadata
			var err error

			if strings.Contains(embeddedTypeName, ".") { // External package
				parts := strings.SplitN(embeddedTypeName, ".", 2) // E.g., "myexternalpkg.ExternalEmbedded" or "*myexternalpkg.ExternalEmbedded"
				packageSelector := parts[0]
				typeNameInExternalPkg := parts[1]

				// Clean pointer prefix from selector, e.g. "*pkg.Type" -> "pkg"
				if strings.HasPrefix(packageSelector, "*") {
					packageSelector = packageSelector[1:]
				}
				if strings.HasPrefix(typeNameInExternalPkg, "*") {
					typeNameInExternalPkg = typeNameInExternalPkg[1:]
				}

				var preLoadedExternalFiles []*ast.File
				for _, f := range files { // Check if ASTs for this package selector were already provided
					if f.Name != nil && f.Name.Name == packageSelector {
						preLoadedExternalFiles = append(preLoadedExternalFiles, f)
					}
				}

				if len(preLoadedExternalFiles) > 0 {
					// Case 1: ASTs for the external package (matched by packageSelector name) were provided directly.
					// This handles TestAnalyzeOptions_WithMixedPackageAsts.
					// The 'packageSelector' is used as the 'currentPackageName' for the recursive call.
					embeddedOptions, _, err = AnalyzeOptions(fset, preLoadedExternalFiles, typeNameInExternalPkg, packageSelector)
				} else {
					// Case 2: ASTs not provided directly, resolve selector to full import path and load.
					actualImportPath := astutils.GetImportPath(fileContainingOptionsStruct, packageSelector)
					if actualImportPath == "" {
						// Fallback: if selector can't be resolved via imports, try using selector as path.
						// This might be brittle. A warning could be logged here.
						actualImportPath = packageSelector
					}

					newlyLoadedFiles, loadErr := loader.LoadPackageFiles(fset, actualImportPath, typeNameInExternalPkg)
					if loadErr != nil {
						return nil, actualStructName, fmt.Errorf("error loading external package '%s' (selector '%s') for embedded struct %s: %w", actualImportPath, packageSelector, embeddedTypeName, loadErr)
					}

					externalPackageActualName := ""
					if len(newlyLoadedFiles) > 0 && newlyLoadedFiles[0].Name != nil {
						externalPackageActualName = newlyLoadedFiles[0].Name.Name
					} else if len(newlyLoadedFiles) == 0 {
						return nil, actualStructName, fmt.Errorf("no files loaded for external package '%s' (selector '%s')", actualImportPath, packageSelector)
					} else {
						// If package name is missing from loaded files, use selector as best guess.
						externalPackageActualName = packageSelector
					}
					embeddedOptions, _, err = AnalyzeOptions(fset, newlyLoadedFiles, typeNameInExternalPkg, externalPackageActualName)
				}
			} else { // Same package
				cleanEmbeddedTypeName := embeddedTypeName
				if strings.HasPrefix(cleanEmbeddedTypeName, "*") {
					cleanEmbeddedTypeName = cleanEmbeddedTypeName[1:]
				}
				// Pass the original 'files' slice for same-package recursion
				embeddedOptions, _, err = AnalyzeOptions(fset, files, cleanEmbeddedTypeName, currentPackageName)
			}

			if err != nil {
				// More generic error message; specific context (like package name) should be in the wrapped 'err'.
				return nil, actualStructName, fmt.Errorf("error analyzing embedded struct %s: %w", embeddedTypeName, err)
			}
			extractedOptions = append(extractedOptions, embeddedOptions...)
			continue
		}
		fieldName := field.Names[0].Name
		if !ast.IsExported(fieldName) {
			// Skip unexported fields
			continue
		}

		opt := &metadata.OptionMetadata{
			Name:       fieldName,
			CliName:    stringutils.ToKebabCase(fieldName),
			TypeName:   astutils.ExprToTypeName(field.Type),
			IsPointer:         astutils.IsPointerType(field.Type),
			IsRequired:        !astutils.IsPointerType(field.Type), // Basic assumption: non-pointer is required
			IsTextUnmarshaler: false,                               // Initialize
			IsTextMarshaler:   false,                               // Initialize
		}

		// Type analysis for TextMarshaler/Unmarshaler
		if currentPkg != nil && currentPkg.TypesInfo != nil && len(field.Names) > 0 && field.Names[0] != nil {
			// field.Type is an ast.Expr. We need its types.Type.
			tv := currentPkg.TypesInfo.TypeOf(field.Type)
			if tv != nil {
				// Check if T implements the interface
				if types.Implements(tv, textUnmarshalerType) {
					opt.IsTextUnmarshaler = true
				}
				// Check if *T implements the interface (common for unmarshaler methods)
				// types.Implements handles this correctly if tv is T and methods are on *T,
				// but an explicit check for pointer receivers on non-pointer types might be needed
				// if types.Implements(T, I) fails.
				// However, types.Implements should be sufficient if the method set of T includes methods of *T.
				// Let's add the explicit pointer check for robustness with pointer receivers.
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


		if field.Doc != nil {
			opt.HelpText = strings.TrimSpace(field.Doc.Text())
		}
		if field.Comment != nil {
			// Line comments might also be relevant, concatenate if necessary
			opt.HelpText = strings.TrimSpace(opt.HelpText + "\n" + field.Comment.Text())
			opt.HelpText = strings.TrimSpace(opt.HelpText)
		}

		if field.Tag != nil {
			tagStr := strings.Trim(field.Tag.Value, "`")
			tag := reflect.StructTag(tagStr)
			if envVar, ok := tag.Lookup("env"); ok {
				opt.EnvVar = envVar
			}
			// TODO: Add support for other non-goat tags if needed, or consolidate all tag parsing.
		}

		extractedOptions = append(extractedOptions, opt)
	}

	return extractedOptions, actualStructName, nil
}
