package analyzer

import (
	"fmt"
	"go/ast"
	"reflect"
	"strings"

	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/astutils"
	"github.com/podhmo/goat/internal/utils/stringutils"
)

// AnalyzeOptions finds the Options struct definition (given its type name)
// and extracts metadata for each of its fields.
func AnalyzeOptions(fileAst *ast.File, optionsTypeName string, packageName string) ([]*metadata.OptionMetadata, string, error) {
	var optionsStruct *ast.TypeSpec
	var actualStructName string

	// Remove package prefix if present (e.g. "main.Options" -> "Options")
	parts := strings.Split(optionsTypeName, ".")
	typeNameOnly := parts[len(parts)-1]

	ast.Inspect(fileAst, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if ts.Name.Name == typeNameOnly {
				if _, isStruct := ts.Type.(*ast.StructType); isStruct {
					optionsStruct = ts
					actualStructName = ts.Name.Name
					return false // Stop searching
				}
			}
		}
		return true
	})

	if optionsStruct == nil {
		return nil, "", fmt.Errorf("options struct type '%s' not found in package '%s'", typeNameOnly, packageName)
	}

	structType, ok := optionsStruct.Type.(*ast.StructType)
	if !ok {
		// This should not happen if the previous check passed
		return nil, actualStructName, fmt.Errorf("type '%s' is not a struct type", actualStructName)
	}

	var extractedOptions []*metadata.OptionMetadata
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			// Embedded struct, skip for now or handle later if needed
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
