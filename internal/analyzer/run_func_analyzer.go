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
func AnalyzeRunFunc(fileAst *ast.File, funcName string) (*metadata.RunFuncInfo, string, error) {
	var runFuncDecl *ast.FuncDecl
	var docComment string

	ast.Inspect(fileAst, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == funcName {
			runFuncDecl = fn
			if fn.Doc != nil {
				docComment = fn.Doc.Text()
			}
			return false // Stop searching
		}
		return true
	})

	if runFuncDecl == nil {
		return nil, "", fmt.Errorf("function '%s' not found", funcName)
	}

	info := &metadata.RunFuncInfo{
		Name:        runFuncDecl.Name.Name,
		PackageName: fileAst.Name.Name, // Assuming it's in the same package
	}

	// Analyze parameters: expecting `run(options MyOptions) error` or `run(ctx context.Context, options MyOptions) error`
	params := runFuncDecl.Type.Params.List
	if len(params) == 1 { // run(options MyOptions) error
		param := params[0]
		if len(param.Names) > 0 {
			info.OptionsArgName = param.Names[0].Name
		}
		info.OptionsArgType = astutils.ExprToTypeName(param.Type)
	} else if len(params) == 2 { // run(ctx context.Context, options MyOptions) error
		// TODO: Handle context parameter if necessary, for now assume 2nd is options
		ctxParam := params[0]
		optionsParam := params[1]

		if len(ctxParam.Names) > 0 {
			info.ContextArgName = ctxParam.Names[0].Name
		}
		info.ContextArgType = astutils.ExprToTypeName(ctxParam.Type)

		if len(optionsParam.Names) > 0 {
			info.OptionsArgName = optionsParam.Names[0].Name
		}
		info.OptionsArgType = astutils.ExprToTypeName(optionsParam.Type)
	} else {
		return nil, docComment, fmt.Errorf("function '%s' has unexpected signature: expected 1 or 2 parameters, got %d", funcName, len(params))
	}

	// TODO: Analyze return type (expecting `error`)

	return info, strings.TrimSpace(docComment), nil
}
