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
						return false // Stop searching this file
					}
				}
			}
			return true
		})
		if optionsStruct != nil {
			break // Found in one of the files
		}
	}

	if optionsStruct == nil {
		return nil, "", fmt.Errorf("options struct type '%s' not found in package '%s'", typeNameOnly, currentPackageName)
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
				parts := strings.SplitN(embeddedTypeName, ".", 2)
				importPathFromType := parts[0] // e.g., "myexternalpkg" or "*myexternalpkg"
				typeNameInExternalPkg := parts[1]

				// Clean pointer prefix from import path, e.g. "*pkg.Type" -> "pkg"
				if strings.HasPrefix(importPathFromType, "*") {
					importPathFromType = importPathFromType[1:]
				}
				// Clean pointer prefix from type name if it's something like "pkg.*Type" (less common)
				if strings.HasPrefix(typeNameInExternalPkg, "*") {
					typeNameInExternalPkg = typeNameInExternalPkg[1:]
				}

				var filesForExternalPkg []*ast.File
				foundInProvidedFiles := false

				// Check if ASTs for this import path were already provided
				for _, inputFileAst := range files { // 'files' is the input to AnalyzeOptions
					if inputFileAst.Name != nil && inputFileAst.Name.Name == importPathFromType {
						filesForExternalPkg = append(filesForExternalPkg, inputFileAst)
						foundInProvidedFiles = true
					}
				}

				if foundInProvidedFiles {
					// Use the ASTs found in the input 'files' slice
					embeddedOptions, _, err = AnalyzeOptions(fset, filesForExternalPkg, typeNameInExternalPkg, importPathFromType)
				} else {
					// ASTs not provided, so attempt to load them
					// TODO: Implement caching for loaded packages
					newlyLoadedFiles, loadErr := loader.LoadPackageFiles(fset, importPathFromType, typeNameInExternalPkg)
					if loadErr != nil {
						return nil, actualStructName, fmt.Errorf("error loading external package %s for embedded struct %s: %w", importPathFromType, embeddedTypeName, loadErr)
					}
					embeddedOptions, _, err = AnalyzeOptions(fset, newlyLoadedFiles, typeNameInExternalPkg, importPathFromType)
				}
				// 'err' from the recursive call is handled by the shared 'if err != nil' block below
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
			IsRequired: !astutils.IsPointerType(field.Type), // Basic assumption: non-pointer is required
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
			// TODO: Add support for other tags like 'file', 'default', 'enum' if defined directly in tags
		}

		extractedOptions = append(extractedOptions, opt)
	}

	return extractedOptions, actualStructName, nil
}
