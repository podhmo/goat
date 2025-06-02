package interpreter

import (
	"fmt"
	"go/ast"
	"log"

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
	markerPkgImportPath string, // e.g., "github.com/podhmo/goat/goat"
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

	// Map option field names to their metadata for quick lookup
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

	// Look for return statement like 'return &Options{...}'
	// or assignments like 'opt.Field = ...'
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
						extractMarkerInfo(stmtNode.Rhs[0], optMeta, fileAst, markerPkgImportPath)
					}
				}
			}

		case *ast.ReturnStmt: // e.g. return &Options{ Field: goat.Default(...) }
			if len(stmtNode.Results) == 1 {
				if compLit, ok := stmtNode.Results[0].(*ast.CompositeLit); ok {
					// Check if this composite literal is for our Options struct
					// This requires resolving compLit.Type to optionsStructName, which can be complex.
					// For a simpler start, assume if it's a struct literal in NewOptions, it's the one.
					log.Printf("Found return composite literal in %s", initializerFuncName)
					for _, elt := range compLit.Elts {
						if kvExpr, ok := elt.(*ast.KeyValueExpr); ok {
							if keyIdent, ok := kvExpr.Key.(*ast.Ident); ok {
								fieldName := keyIdent.Name
								if optMeta, exists := optionsMap[fieldName]; exists {
									log.Printf("Found key-value for options field in return: %s", fieldName)
									extractMarkerInfo(kvExpr.Value, optMeta, fileAst, markerPkgImportPath)
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
func extractMarkerInfo(valueExpr ast.Expr, optMeta *metadata.OptionMetadata, fileAst *ast.File, markerPkgImportPath string) {
	callExpr, ok := valueExpr.(*ast.CallExpr)
	if !ok {
		// Value is not a function call, could be a direct literal (TODO: handle direct literals as defaults)
		return
	}

	markerFuncName, markerPkgAlias := astutils.GetFullFunctionName(callExpr.Fun)
	actualMarkerPkgPath := astutils.GetImportPath(fileAst, markerPkgAlias)

	if actualMarkerPkgPath != markerPkgImportPath {
		// log.Printf("Skipping call to %s.%s, not the target marker package (%s vs %s)", markerPkgAlias, markerFuncName, actualMarkerPkgPath, markerPkgImportPath)
		return
	}

	switch markerFuncName {
	case "Default":
		log.Printf("Interpreting goat.Default for field %s", optMeta.Name)
		if len(callExpr.Args) > 0 {
			// First arg is the default value
			optMeta.DefaultValue = astutils.EvaluateArg(callExpr.Args[0])
			log.Printf("  Default value: %v", optMeta.DefaultValue)

			// Subsequent args could be an Enum call for enumConstraint
			if len(callExpr.Args) > 1 {
				// Assume second arg is the enumConstraint, which might be a goat.Enum() call
				// or a slice literal.
				enumArg := callExpr.Args[1]
				if enumCallExpr, ok := enumArg.(*ast.CallExpr); ok {
					enumFuncName, enumPkgAlias := astutils.GetFullFunctionName(enumCallExpr.Fun)
					actualEnumPkgPath := astutils.GetImportPath(fileAst, enumPkgAlias)
					if actualEnumPkgPath == markerPkgImportPath && enumFuncName == "Enum" {
						if len(enumCallExpr.Args) == 1 {
							optMeta.EnumValues = astutils.EvaluateSliceArg(enumCallExpr.Args[0])
							log.Printf("  Enum values from goat.Enum: %v", optMeta.EnumValues)
						}
					}
				} else if _, ok := enumArg.(*ast.CompositeLit); ok { // Direct slice literal
					optMeta.EnumValues = astutils.EvaluateSliceArg(enumArg)
					log.Printf("  Enum values from slice literal: %v", optMeta.EnumValues)
				}
			}
		}
	case "Enum":
		log.Printf("Interpreting goat.Enum for field %s", optMeta.Name)
		if len(callExpr.Args) == 1 {
			optMeta.EnumValues = astutils.EvaluateSliceArg(callExpr.Args[0])
			log.Printf("  Enum values: %v", optMeta.EnumValues)
		}
	default:
		// Not a recognized marker function from the specified package
		// log.Printf("  Not a goat marker function: %s.%s", markerPkgAlias, markerFuncName)
	}
}
