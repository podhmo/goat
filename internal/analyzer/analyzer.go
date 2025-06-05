package analyzer

import (
	"fmt"
	"go/ast"
	"go/token" // Added import
	"log/slog"
	"strings" // Added import

	"github.com/podhmo/goat/internal/metadata"
)

// Analyze inspects the AST of Go files to extract command metadata,
// focusing on the run function and its associated options struct.
// It returns the main CommandMetadata, the name of the Options struct, and any error encountered.
func Analyze(fset *token.FileSet, files []*ast.File, runFuncName string, mainPackageName string) (*metadata.CommandMetadata, string, error) {
	cmdMeta := &metadata.CommandMetadata{
		Options: []*metadata.OptionMetadata{},
	}
	var optionsStructName string // Will be returned

	runFuncInfo, runFuncDoc, err := AnalyzeRunFunc(files, runFuncName)
	if err != nil {
		return nil, "", fmt.Errorf("analyzing run function '%s': %w", runFuncName, err)
	}
	if runFuncInfo != nil {
		runFuncInfo.PackageName = mainPackageName // Set the package name here

		// Populate OptionsArgTypeNameStripped and OptionsArgIsPointer
		if runFuncInfo.OptionsArgType != "" {
			if strings.HasPrefix(runFuncInfo.OptionsArgType, "*") {
				runFuncInfo.OptionsArgIsPointer = true
				runFuncInfo.OptionsArgTypeNameStripped = strings.TrimPrefix(runFuncInfo.OptionsArgType, "*")
			} else {
				runFuncInfo.OptionsArgIsPointer = false
				runFuncInfo.OptionsArgTypeNameStripped = runFuncInfo.OptionsArgType
			}
		}
	}

	cmdMeta.Name = mainPackageName // Use provided main package name
	cmdMeta.Description = runFuncDoc
	cmdMeta.RunFunc = runFuncInfo

	if runFuncInfo != nil && runFuncInfo.OptionsArgName != "" && runFuncInfo.OptionsArgType != "" {
		options, foundOptionsStructName, err := AnalyzeOptions(fset, files, runFuncInfo.OptionsArgType, mainPackageName)
		if err != nil {
			return nil, "", fmt.Errorf("analyzing options struct for run function '%s': %w", runFuncName, err)
		}
		cmdMeta.Options = options
		optionsStructName = foundOptionsStructName // Assign to the variable that will be returned
	} else {
		// If there's no options arg, it's not necessarily an error, command might not have options.
		// The original code had an error here, but it might be too strict.
		// For now, let's keep it consistent with the original strictness.
		// If this needs to be changed, the error below can be removed or softened.
		return nil, "", fmt.Errorf("run function '%s' must have an options parameter, or it's not in the expected format", runFuncName)
	}

	// Find the main function to get its position for future code replacement
	emitTargetFuncName := "main" // TODO: Make this configurable if needed
	for _, targetFileAst := range files {
		funcOb := targetFileAst.Scope.Lookup(emitTargetFuncName)
		if funcOb != nil && funcOb.Kind == ast.Fun && funcOb.Decl != nil {
			// We found the main function, capture its position
			if funcDecl, ok := funcOb.Decl.(*ast.FuncDecl); ok {
				pos := fset.Position(funcDecl.Pos())
				cmdMeta.MainFuncPosition = &pos
				slog.Info("Goat: Found main function", "name", emitTargetFuncName, "position", cmdMeta.MainFuncPosition)
			} else {
				slog.Warn("Goat: Found main function but it is not a FuncDecl", "name", emitTargetFuncName, "type", fmt.Sprintf("%T", funcOb.Decl))
			}
			break
		}
	}

	return cmdMeta, optionsStructName, nil
}
