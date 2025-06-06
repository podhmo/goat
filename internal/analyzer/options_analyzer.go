package analyzer

import (
	"fmt"
	"go/ast"
	"go/token" // New import
	"reflect"
	"strings"

	"github.com/podhmo/goat/internal/loader" // New import
	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/astutils"
	"github.com/podhmo/goat/internal/utils/stringutils"
)

// AnalyzeOptions finds the Options struct definition (given its type name)
// and extracts metadata for each of its fields.
func AnalyzeOptions(fset *token.FileSet, files []*ast.File, optionsTypeName string, currentPackageName string) ([]*metadata.OptionMetadata, string, error) {
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
						return false // Stop searching this file
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
		if len(field.Names) == 0 { // Embedded struct
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
			IsPointer:  astutils.IsPointerType(field.Type),
			IsRequired: false, // Default to false, will be set by tag
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
			if goatTag, ok := tag.Lookup("goat"); ok {
				if strings.Contains(goatTag, "required") {
					opt.IsRequired = true
				}
				// TODO: Potentially parse other goat-specific options here if any, e.g., "file", "enum"
			}
			// TODO: Add support for other non-goat tags if needed, or consolidate all tag parsing.
		}

		extractedOptions = append(extractedOptions, opt)
	}

	return extractedOptions, actualStructName, nil
}
