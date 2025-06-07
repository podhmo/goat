package interpreter

import (
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"strings"

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
	baseIndent int,
) error {
	slog.Debug(strings.Repeat("\t", baseIndent)+"InterpretInitializer: start", "optionsStructName", optionsStructName, "initializerName", initializerFuncName)
	var initializerFunc *ast.FuncDecl
	ast.Inspect(fileAst, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == initializerFuncName {
			initializerFunc = fn
			slog.Debug(strings.Repeat("\t", baseIndent+1)+"Found initializer function", "name", initializerFuncName)
			return false
		}
		return true
	})

	if initializerFunc == nil {
		slog.Debug(strings.Repeat("\t", baseIndent) + "InterpretInitializer: end")
		return fmt.Errorf("initializer function '%s' not found", initializerFuncName)
	}

	if initializerFunc.Body == nil {
		slog.Debug(strings.Repeat("\t", baseIndent) + "InterpretInitializer: end")
		return fmt.Errorf("initializer function '%s' has no body", initializerFuncName)
	}

	slog.Debug(strings.Repeat("\t", baseIndent+1)+"Mapping options", "numOptions", len(options))
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

	slog.Debug(strings.Repeat("\t", baseIndent+1)+"Interpreting initializer body", "name", initializerFuncName)
	ast.Inspect(initializerFunc.Body, func(n ast.Node) bool {
		slog.Debug(strings.Repeat("\t", baseIndent+2)+"Inspecting node", "type", fmt.Sprintf("%T", n))
		switch stmtNode := n.(type) {
		case *ast.AssignStmt: // e.g. options.Field = goat.Default(...) or var x = goat.Default(...)
			slog.Debug(strings.Repeat("\t", baseIndent+3) + "Assignment statement found")
			// We need to trace assignments to see if they end up in an OptionMetadata field
			// This example focuses on direct assignments to struct fields.
			// E.g., `opt.MyField = goat.Default("value")`
			if len(stmtNode.Lhs) == 1 && len(stmtNode.Rhs) == 1 {
				if selExpr, ok := stmtNode.Lhs[0].(*ast.SelectorExpr); ok {
					// Assuming selExpr.X is the options struct variable, selExpr.Sel is the field name
					fieldName := selExpr.Sel.Name
					if optMeta, exists := optionsMap[fieldName]; exists {
						slog.Debug(strings.Repeat("\t", baseIndent+4)+"Found assignment to options field", "fieldName", fieldName)
						extractMarkerInfo(stmtNode.Rhs[0], optMeta, fileAst, markerPkgImportPath, baseIndent+5)
					}
				}
			}

		case *ast.ReturnStmt: // e.g. return &Options{ Field: goat.Default(...) }
			slog.Debug(strings.Repeat("\t", baseIndent+3) + "Return statement found")
			if len(stmtNode.Results) == 1 {
				actualExpr := stmtNode.Results[0]
				if unaryExpr, ok := actualExpr.(*ast.UnaryExpr); ok && unaryExpr.Op == token.AND {
					actualExpr = unaryExpr.X
				}

				if compLit, ok := actualExpr.(*ast.CompositeLit); ok {
					slog.Debug(strings.Repeat("\t", baseIndent+4) + "Return composite literal found")
					// Check if this composite literal is for our Options struct
					// This requires resolving compLit.Type to optionsStructName, which can be complex.
					// For a simpler start, assume if it's a struct literal in NewOptions, it's the one.
					for _, elt := range compLit.Elts {
						if kvExpr, ok := elt.(*ast.KeyValueExpr); ok {
							if keyIdent, ok := kvExpr.Key.(*ast.Ident); ok {
								fieldName := keyIdent.Name
								if optMeta, exists := optionsMap[fieldName]; exists {
									slog.Debug(strings.Repeat("\t", baseIndent+5)+"Found key-value for options field in composite literal", "fieldName", fieldName)
									extractMarkerInfo(kvExpr.Value, optMeta, fileAst, markerPkgImportPath, baseIndent+6)
								}
							}
						}
					}
				}
			}
		}
		return true
	})
	slog.Debug(strings.Repeat("\t", baseIndent) + "InterpretInitializer: end")
	return nil
}

// extractMarkerInfo extracts default value and enum choices from a marker function call.
func extractMarkerInfo(valueExpr ast.Expr, optMeta *metadata.OptionMetadata, fileAst *ast.File, markerPkgImportPath string, baseIndent int) {
	slog.Debug(strings.Repeat("\t", baseIndent)+"extractMarkerInfo: start", "optMetaName", optMeta.Name)
	callExpr, ok := valueExpr.(*ast.CallExpr)
	if !ok {
		slog.Debug(strings.Repeat("\t", baseIndent+1)+"Value is not a call expression", "type", fmt.Sprintf("%T", valueExpr))
		slog.Debug(strings.Repeat("\t", baseIndent) + "extractMarkerInfo: end")
		return
	}

	markerFuncName, markerPkgAlias := astutils.GetFullFunctionName(callExpr.Fun)
	actualMarkerPkgPath := astutils.GetImportPath(fileAst, markerPkgAlias)
	slog.Debug(strings.Repeat("\t", baseIndent+1)+"Extracted call info", "markerFuncName", markerFuncName, "markerPkgAlias", markerPkgAlias, "actualMarkerPkgPath", actualMarkerPkgPath)

	// Allow original goat path or the one used in cmd/goat tests via testcmdmodule
	isKnownMarkerPackage := (actualMarkerPkgPath == markerPkgImportPath || // e.g. "github.com/podhmo/goat"
		actualMarkerPkgPath == "testcmdmodule/internal/goat") // For cmd/goat tests

	if !isKnownMarkerPackage {
		slog.Debug(strings.Repeat("\t", baseIndent+1)+"Call is to a non-marker package", "actualMarkerPkgPath", actualMarkerPkgPath, "expectedMarkerPkgPath", markerPkgImportPath)
		slog.Debug(strings.Repeat("\t", baseIndent) + "extractMarkerInfo: end")
		return
	}

	slog.Debug(strings.Repeat("\t", baseIndent+1)+"Processing marker function", "markerFuncName", markerFuncName)
	switch markerFuncName {
	case "Default":
		slog.Debug(strings.Repeat("\t", baseIndent+2)+"Interpreting goat.Default for field", "field", optMeta.Name)
		if len(callExpr.Args) > 0 {
			optMeta.DefaultValue = astutils.EvaluateArg(callExpr.Args[0])
			slog.Debug(strings.Repeat("\t", baseIndent+3)+"Default value extracted", "defaultValue", optMeta.DefaultValue)

			// Subsequent args could be an Enum call for enumConstraint
			if len(callExpr.Args) > 1 {
				slog.Debug(strings.Repeat("\t", baseIndent+3) + "Additional args found, checking for Enum constraint")
				// Assume second arg is the enumConstraint, which might be a goat.Enum() call
				// or a slice literal.
				enumArg := callExpr.Args[1]
				if enumCallExpr, ok := enumArg.(*ast.CallExpr); ok {
					enumFuncName, enumPkgAlias := astutils.GetFullFunctionName(enumCallExpr.Fun)
					actualEnumPkgPath := astutils.GetImportPath(fileAst, enumPkgAlias)
					slog.Debug(strings.Repeat("\t", baseIndent+4)+"Enum argument is a call expression", "enumFuncName", enumFuncName, "actualEnumPkgPath", actualEnumPkgPath)
					if actualEnumPkgPath == markerPkgImportPath && enumFuncName == "Enum" {
						if len(enumCallExpr.Args) == 1 {
							optMeta.EnumValues = astutils.EvaluateSliceArg(enumCallExpr.Args[0])
							slog.Debug(strings.Repeat("\t", baseIndent+5)+"Enum values from goat.Enum extracted", "enumValues", optMeta.EnumValues)
						}
					}
				} else if _, ok := enumArg.(*ast.CompositeLit); ok { // Direct slice literal
					optMeta.EnumValues = astutils.EvaluateSliceArg(enumArg)
					slog.Debug(strings.Repeat("\t", baseIndent+4)+"Enum values from slice literal extracted", "enumValues", optMeta.EnumValues)
				}
			}
		}
	case "Enum":
		slog.Debug(strings.Repeat("\t", baseIndent+2)+"Interpreting goat.Enum for field", "field", optMeta.Name)
		if len(callExpr.Args) == 1 {
			optMeta.EnumValues = astutils.EvaluateSliceArg(callExpr.Args[0])
			slog.Debug(strings.Repeat("\t", baseIndent+3)+"Enum values extracted", "enumValues", optMeta.EnumValues)
		}
	case "File":
		slog.Debug(strings.Repeat("\t", baseIndent+2)+"Interpreting goat.File for field", "field", optMeta.Name)
		if len(callExpr.Args) > 0 {
			optMeta.DefaultValue = astutils.EvaluateArg(callExpr.Args[0])
			slog.Debug(strings.Repeat("\t", baseIndent+3)+"Default path extracted", "defaultPath", optMeta.DefaultValue)
			optMeta.TypeName = "string" // File paths are strings

			// Subsequent args are FileOption calls (e.g., goat.MustExist(), goat.GlobPattern())
			if len(callExpr.Args) > 1 {
				slog.Debug(strings.Repeat("\t", baseIndent+3) + "Additional args found, checking for FileOption calls")
				for i, arg := range callExpr.Args[1:] {
					slog.Debug(strings.Repeat("\t", baseIndent+4)+"Processing FileOption argument", "index", i)
					if optionCallExpr, ok := arg.(*ast.CallExpr); ok {
						optionFuncName, optionFuncPkgAlias := astutils.GetFullFunctionName(optionCallExpr.Fun)
						actualOptionFuncPkgPath := astutils.GetImportPath(fileAst, optionFuncPkgAlias)
						slog.Debug(strings.Repeat("\t", baseIndent+5)+"FileOption argument is a call expression", "optionFuncName", optionFuncName, "actualOptionFuncPkgPath", actualOptionFuncPkgPath)

						if actualOptionFuncPkgPath == markerPkgImportPath { // Ensure it's a goat.Xxx call
							switch optionFuncName {
							case "MustExist":
								optMeta.FileMustExist = true
								slog.Debug(strings.Repeat("\t", baseIndent+6) + "FileOption: MustExist set")
							case "GlobPattern":
								optMeta.FileGlobPattern = true
								slog.Debug(strings.Repeat("\t", baseIndent+6) + "FileOption: GlobPattern set")
							default:
								slog.Warn(strings.Repeat("\t", baseIndent+6)+"Unknown FileOption encountered", "optionFuncName", optionFuncName)
							}
						}
					}
				}
			}
		}
	default:
		slog.Warn(strings.Repeat("\t", baseIndent+2)+"Not a recognized goat marker function", "markerFuncName", markerFuncName, "markerPkgAlias", markerPkgAlias)
	}
	slog.Debug(strings.Repeat("\t", baseIndent) + "extractMarkerInfo: end")
}
