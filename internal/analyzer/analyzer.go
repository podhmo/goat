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

	// After runFuncInfo is populated, try to find an initializer function for the options struct
	if runFuncInfo.OptionsArgTypeNameStripped != "" {
		initializerFuncName := "New" + runFuncInfo.OptionsArgTypeNameStripped
		slog.Debug("Goat: Looking for conventional options initializer function", "expectedName", initializerFuncName)
		initializerFuncFoundInAst := false // Flag to track if we found any function with the name

		for _, file := range files {
			if runFuncInfo.InitializerFunc != "" { // Already found and validated
				break
			}
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == initializerFuncName {
					initializerFuncFoundInAst = true // Found a function with the conventional name
					// Check signature: must have no parameters.
					// A more robust future check might inspect return types: e.g. *OptionsType or (*OptionsType, error).
					if fn.Type.Params == nil || len(fn.Type.Params.List) == 0 {
						runFuncInfo.InitializerFunc = initializerFuncName
						slog.Info("Goat: Found and using conventional initializer function", "name", initializerFuncName, "package", runFuncInfo.PackageName)
						// No need to 'break' inner loop here, outer loop will break due to InitializerFunc being set.
					} else {
						slog.Warn("Goat: Conventional initializer function found but has unexpected parameters; it will be ignored.",
							"functionName", initializerFuncName,
							"paramCount", len(fn.Type.Params.List),
							"package", runFuncInfo.PackageName)
						// Do not set runFuncInfo.InitializerFunc, let it remain empty.
					}
					break // Found the function by name, processed it (either used or warned), stop checking other decls in this file.
				}
			}
		}

		if runFuncInfo.InitializerFunc == "" && !initializerFuncFoundInAst {
			slog.Info("Goat: No conventional initializer function found with the expected name", "expectedName", initializerFuncName, "package", runFuncInfo.PackageName)
		} else if runFuncInfo.InitializerFunc == "" && initializerFuncFoundInAst {
			// This case means a function was found by name, but it had the wrong signature (and a warning was logged).
			// No additional general message needed here, the specific warning is sufficient.
			slog.Debug("Goat: A function matching conventional initializer name was found but ignored due to signature.", "expectedName", initializerFuncName)
		}
	}

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
