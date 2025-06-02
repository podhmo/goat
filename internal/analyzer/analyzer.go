package analyzer

import (
	"fmt"
	"go/ast"

	"github.com/podhmo/goat/internal/metadata"
)

// Analyze inspects the AST of a Go file to extract command metadata,
// focusing on the run function and its associated options struct.
// It returns the main CommandMetadata, the name of the Options struct, and any error encountered.
func Analyze(fileAst *ast.File, runFuncName string) (*metadata.CommandMetadata, string /* optionsStructName */, error) {
	cmdMeta := &metadata.CommandMetadata{
		Options: []*metadata.OptionMetadata{},
	}
	var optionsStructName string

	// 1. Find the run function and extract its info (doc comment, params)
	runFuncInfo, runFuncDoc, err := AnalyzeRunFunc(fileAst, runFuncName)
	if err != nil {
		return nil, "", fmt.Errorf("analyzing run function '%s': %w", runFuncName, err)
	}
	cmdMeta.Name = fileAst.Name.Name // Use package name as initial command name, can be refined
	cmdMeta.Description = runFuncDoc
	cmdMeta.RunFunc = runFuncInfo

	// 2. If run function is found, analyze its Options struct
	if runFuncInfo != nil && runFuncInfo.OptionsArgName != "" && runFuncInfo.OptionsArgType != "" {
		options, foundOptionsStructName, err := AnalyzeOptions(fileAst, runFuncInfo.OptionsArgType, runFuncInfo.PackageName)
		if err != nil {
			return nil, "", fmt.Errorf("analyzing options struct for run function '%s': %w", runFuncName, err)
		}
		cmdMeta.Options = options
		optionsStructName = foundOptionsStructName
	} else {
		return nil, "", fmt.Errorf("run function '%s' or its options parameter not found or not in expected format", runFuncName)
	}

	// 3. TODO: Find the main function to get its position for future code replacement
	// mainFuncPos, err := FindMainFuncPosition(fileAst)
	// if err != nil {
	//    // Optional: main func might not exist if user is building a library part
	// }
	// cmdMeta.MainFuncPosition = mainFuncPos

	return cmdMeta, optionsStructName, nil
}
