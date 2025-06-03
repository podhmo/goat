package analyzer

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/astutils"
)

// AnalyzeRunFunc finds the specified 'run' function in the AST and extracts its metadata.
// It returns the RunFuncInfo, the function's doc comment, and any error.
func AnalyzeRunFunc(files []*ast.File, funcName string) (*metadata.RunFuncInfo, string, error) {
	var runFuncDecl *ast.FuncDecl
	var docComment string
	// var foundInFile *ast.File // To store the AST of the file where the function is found - Not strictly needed by current logic

	for _, currentFileAst := range files {
		ast.Inspect(currentFileAst, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
				runFuncDecl = fn
				if fn.Doc != nil {
					docComment = fn.Doc.Text()
				}
				// foundInFile = currentFileAst // Save the file where it was found
				return false // Stop searching this file
			}
			return true
		})
		if runFuncDecl != nil {
			break // Found in one of the files
		}
	}

	if runFuncDecl == nil {
		return nil, "", fmt.Errorf("function '%s' not found in provided files", funcName)
	}

	info := &metadata.RunFuncInfo{
		Name: runFuncDecl.Name.Name,
		// PackageName will be set by the caller (analyzer.Analyze)
	}

	// Analyze parameters: expecting `run(options MyOptions) error` or `run(ctx context.Context, options MyOptions) error`
	params := runFuncDecl.Type.Params.List
	if len(params) == 1 {
		if len(params[0].Names) > 0 { info.OptionsArgName = params[0].Names[0].Name }
		info.OptionsArgType = astutils.ExprToTypeName(params[0].Type)
	} else if len(params) == 2 {
		if len(params[0].Names) > 0 { info.ContextArgName = params[0].Names[0].Name }
		info.ContextArgType = astutils.ExprToTypeName(params[0].Type)
		if len(params[1].Names) > 0 { info.OptionsArgName = params[1].Names[0].Name }
		info.OptionsArgType = astutils.ExprToTypeName(params[1].Type)
	} else {
		return nil, strings.TrimSpace(docComment), fmt.Errorf("function '%s' has unexpected signature: expected 1 or 2 parameters, got %d", funcName, len(params))
	}

	// TODO: Analyze return type (expecting `error`)

	return info, strings.TrimSpace(docComment), nil
}
