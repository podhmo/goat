package analyzer

import (
	"fmt"
	"go/ast"
	"go/token" // Added import
	"log/slog"
	"strings" // Added import

	"github.com/podhmo/goat/internal/metadata"
)

// Analyze inspects the AST of Go files to extract command metadata.
// - fset: Token FileSet.
// - files: ASTs of the files to analyze (typically from the target package).
// - runFuncName: Name of the main run function.
// - targetPackageID: Import path of the package containing the runFuncName (e.g., "testmodule/example.com/mainpkg").
// - moduleRootPath: Absolute path to the root of the module this package belongs to.
func Analyze(fset *token.FileSet, files []*ast.File, runFuncName string, targetPackageID string, moduleRootPath string) (*metadata.CommandMetadata, string, error) {
	cmdMeta := &metadata.CommandMetadata{
		Options: []*metadata.OptionMetadata{},
	}
	var optionsStructName string // Will be returned

	// AnalyzeRunFunc finds the run function within the provided files.
	// It does not need module context, only ASTs.
	runFuncInfo, runFuncDoc, err := AnalyzeRunFunc(files, runFuncName)
	if err != nil {
		return nil, "", fmt.Errorf("analyzing run function '%s' in package '%s': %w", runFuncName, targetPackageID, err)
	}
	if runFuncInfo == nil { // Should be caught by AnalyzeRunFunc's error, but as a safeguard.
		return nil, "", fmt.Errorf("run function '%s' not found in package '%s'", runFuncName, targetPackageID)
	}

	// runFuncInfo.PackageName should be the actual Go package name (e.g. "mainpkg"),
	// not necessarily the full targetPackageID. This might need refinement if targetPackageID is different.
	// For now, let's assume the last part of targetPackageID is the Go package name.
	pkgNameParts := strings.Split(targetPackageID, "/")
	runFuncInfo.PackageName = pkgNameParts[len(pkgNameParts)-1]


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

	cmdMeta.Name = targetPackageID // Use targetPackageID as the command name identifier
	cmdMeta.Description = runFuncDoc
	cmdMeta.RunFunc = runFuncInfo

	if runFuncInfo.OptionsArgName != "" && runFuncInfo.OptionsArgType != "" {
		// optionsTypeName is the simple name of the type, e.g. "Options" or "*Options"
		// targetPackageID is the import path of the package where this type is defined.
		// moduleRootPath is the filesystem root of the module containing this package.
		options, foundOptionsStructName, err := AnalyzeOptionsV2(fset, files, runFuncInfo.OptionsArgType, targetPackageID, moduleRootPath)
		if err != nil {
			return nil, "", fmt.Errorf("analyzing options struct for run function '%s' in package '%s': %w", runFuncName, targetPackageID, err)
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
