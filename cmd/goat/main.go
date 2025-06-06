package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/podhmo/goat/internal/analyzer"
	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/interpreter"
	"github.com/podhmo/goat/internal/loader"
	"github.com/podhmo/goat/internal/metadata"
)

// Options holds the configuration for the goat tool itself,
// typically derived from its command-line arguments.
type Options struct {
	RunFuncName            string // Name of the target 'run' function (e.g., "run")
	OptionsInitializerName string // Name of the options initializer function (e.g., "NewOptions")
	TargetFile             string // Path to the target Go file to be processed
}

func main() {
	if len(os.Args) < 2 {
		// Print general usage if no subcommand is provided
		fmt.Fprintln(os.Stderr, "Usage: goat <subcommand> [options]")
		fmt.Fprintln(os.Stderr, "Available subcommands: init, emit, help-message, scan")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		// Handle init
		fmt.Println("TODO: init subcommand")
	case "emit":
		emitCmd := flag.NewFlagSet("emit", flag.ExitOnError)
		var (
			runFuncName            string
			optionsInitializerName string
		)
		emitCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		emitCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")
		// Add usage for emitCmd
		emitCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: goat emit [options] <target_gofile.go>\n\nOptions:\n")
			emitCmd.PrintDefaults()
		}
		emitCmd.Parse(os.Args[2:]) // Parse flags for emit

		if emitCmd.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Error: Target Go file must be specified for emit.")
			emitCmd.Usage()
			os.Exit(1)
		}
		targetFilename := emitCmd.Arg(0)

		opts := &Options{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             targetFilename,
		}
		if err := runGoat(opts); err != nil {
			slog.Error("Error running goat (emit)", "error", err)
			os.Exit(1)
		}

	case "help-message":
		helpMessageCmd := flag.NewFlagSet("help-message", flag.ExitOnError)
		var (
			runFuncName            string
			optionsInitializerName string
		)
		helpMessageCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		helpMessageCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")

		helpMessageCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: goat help-message [options] <target_gofile.go>\n\nOptions:\n")
			helpMessageCmd.PrintDefaults()
		}
		helpMessageCmd.Parse(os.Args[2:])

		if helpMessageCmd.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Error: Target Go file must be specified for help-message.")
			helpMessageCmd.Usage()
			os.Exit(1)
		}
		targetFilename := helpMessageCmd.Arg(0)

		opts := &Options{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             targetFilename,
		}

		fset := token.NewFileSet()
		cmdMetadata, _, err := scanMain(fset, opts) // fileAST is not needed here
		if err != nil {
			slog.Error("Error scanning main for help-message", "error", err)
			os.Exit(1)
		}

		helpMsg := help.GenerateHelp(cmdMetadata)
		fmt.Print(helpMsg) // Print to stdout, helpMsg likely has its own trailing newline

	case "scan":
		scanCmd := flag.NewFlagSet("scan", flag.ExitOnError)
		var (
			runFuncName            string
			optionsInitializerName string
		)
		scanCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		scanCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")

		scanCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: goat scan [options] <target_gofile.go>\n\nOptions:\n")
			scanCmd.PrintDefaults()
		}
		scanCmd.Parse(os.Args[2:])

		if scanCmd.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Error: Target Go file must be specified for scan.")
			scanCmd.Usage()
			os.Exit(1)
		}
		targetFilename := scanCmd.Arg(0)

		opts := &Options{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             targetFilename,
		}

		fset := token.NewFileSet()
		cmdMetadata, _, err := scanMain(fset, opts) // fileAST is not needed here
		if err != nil {
			slog.Error("Error scanning main for scan", "error", err)
			os.Exit(1)
		}

		jsonData, err := json.MarshalIndent(cmdMetadata, "", "  ")
		if err != nil {
			slog.Error("Error marshalling metadata to JSON for scan", "error", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData)) // Print JSON to stdout
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown subcommand '%s'\n", os.Args[1])
		// Print general usage
		fmt.Fprintln(os.Stderr, "Available subcommands: init, emit, help-message, scan")
		os.Exit(1)
	}
}

func runGoat(opts *Options) error {
	fset := token.NewFileSet()
	cmdMetadata, fileAST, err := scanMain(fset, opts)
	if err != nil {
		return fmt.Errorf("failed to scan main: %w", err)
	}

	helpMsg := help.GenerateHelp(cmdMetadata)

	newMainContent, err := codegen.GenerateMain(cmdMetadata, helpMsg, false /* generateFullFile */)
	if err != nil {
		return fmt.Errorf("failed to generate new main.go content: %w", err)
	}

	err = codegen.WriteMain(opts.TargetFile, fset, fileAST, newMainContent, cmdMetadata.MainFuncPosition)
	if err != nil {
		return fmt.Errorf("failed to write modified main.go: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Goat: Processing finished.") // Print to stdout for test capture
	return nil
}

func scanMain(fset *token.FileSet, opts *Options) (*metadata.CommandMetadata, *ast.File, error) {
	absTargetFile, err := filepath.Abs(opts.TargetFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for target file %s: %w", opts.TargetFile, err)
	}
	opts.TargetFile = absTargetFile // Update to absolute path

	slog.Info("Goat: Analyzing file", "targetFile", opts.TargetFile, "runFunc", opts.RunFuncName, "optionsInitializer", opts.OptionsInitializerName)

	targetFileAst, err := loader.LoadFile(fset, opts.TargetFile)
	if err != nil {
		// Still return nil for *ast.File if targetFileAst itself failed to load
		return nil, nil, fmt.Errorf("failed to load target file %s: %w", opts.TargetFile, err)
	}
	currentPackageName := targetFileAst.Name.Name

	targetDir := filepath.Dir(opts.TargetFile)
	// Determine currentPackageName from the targetFileAst first.
	if targetFileAst.Name != nil {
		currentPackageName = targetFileAst.Name.Name
	}

	// Use targetDir directly to load other files from the same package.
	// This avoids relying on build.ImportDir for temporary/non-standard paths,
	// which might default to "." (current working directory) if it can't resolve targetDir.
	slog.Info("Loading other files from package directory", "dir", targetDir)
	packageFiles, err := loader.LoadPackageFiles(fset, targetDir, "") // Pass targetDir directly
	if err != nil {
		slog.Warn("Failed to load other package files, proceeding with only the target file.", "dir", targetDir, "error", err)
		// Proceeding with just targetFileAst in filesForAnalysis is handled by subsequent logic.
	}

	// Ensure currentPackageName has a default if still empty (e.g. if targetFileAst.Name was nil)
	if strings.TrimSpace(currentPackageName) == "" {
		// Try to get package name from build context of targetDir as a fallback
		buildPkg, buildErr := build.ImportDir(targetDir, 0)
		if buildErr == nil && buildPkg.Name != "" {
			currentPackageName = buildPkg.Name
		} else {
			slog.Warn("Could not determine package name from AST or build context, defaulting to 'main'", "targetFile", opts.TargetFile, "targetDir", targetDir)
			currentPackageName = "main"
		}
	}

	// New logic: Ensure targetFileAst is always included and handle potential duplicates.
	finalFilesForAnalysis := []*ast.File{targetFileAst}
	seenFilePaths := make(map[string]bool)
	targetFileRegisteredPath := fset.File(targetFileAst.Pos()).Name() // Absolute path from fset
	seenFilePaths[targetFileRegisteredPath] = true

	for _, f := range packageFiles {
		tokenFile := fset.File(f.Pos())
		if tokenFile != nil {
			filePath := tokenFile.Name() // Absolute path from fset
			if !seenFilePaths[filePath] {
				finalFilesForAnalysis = append(finalFilesForAnalysis, f)
				seenFilePaths[filePath] = true
			}
		}
	}
	if len(finalFilesForAnalysis) == 0 { // targetFileAst must have been nil if this is true
		return nil, nil, fmt.Errorf("no Go files could be prepared for analysis (target file %s was not loaded)", opts.TargetFile)
	}

	// Determine moduleRootPath and targetPackageID
	// For tests, opts.TargetFile is like /tmp/TestXYZ.../moduleName/example.com/pkg/file.go
	// or /tmp/TestXYZ.../file.go (if moduleName is used directly for TempDir)
	// The go.mod would be at /tmp/TestXYZ.../go.mod or /tmp/TestXYZ.../moduleName/go.mod

	moduleRootPath, err := loader.FindModuleRoot(targetFileRegisteredPath)
	if err != nil {
		slog.Warn("Failed to find module root, using directory of target file as root.", "target", targetFileRegisteredPath, "error", err)
		moduleRootPath = filepath.Dir(targetFileRegisteredPath)
	}

	moduleName, err := loader.GetModuleName(moduleRootPath)
	if err != nil {
		slog.Warn("Failed to get module name from go.mod, using fallback.", "modRoot", moduleRootPath, "error", err)
		// Fallback: if go.mod is unparsable or module name is weird, construct a pseudo-ID.
		// This part is tricky; robustly finding a package ID outside a module is hard.
		// For now, many tests create a module name. If not, this will be an issue.
		// If module name is essential, this should be a hard error.
		// Let's assume tests provide a clear module structure.
		// If not, use "." for packageID and moduleRootPath as Dir for packages.Load.
		// The currentPackageName (Go package name) is already derived.
	}

	// targetPackageID should be like "moduleName/path/to/pkg" or just "path/to/pkg" if moduleName is empty/unknown
	relPathFromModRoot, err := filepath.Rel(moduleRootPath, filepath.Dir(targetFileRegisteredPath))
	if err != nil {
		return nil, targetFileAst, fmt.Errorf("failed to make target path relative to module root: %w", err)
	}
	targetPackageID := filepath.ToSlash(relPathFromModRoot)
	if moduleName != "" {
		if targetPackageID == "." { // File is in module root
			targetPackageID = moduleName
		} else {
			targetPackageID = moduleName + "/" + targetPackageID
		}
	}
	// If moduleName is empty and targetPackageID is ".", it implies a simple dir-based package.
	// currentPackageName (the Go `package foo` name) is used by AnalyzeOptionsV2 for struct lookup within the package.

	cmdMetadata, returnedOptionsStructName, err := analyzer.Analyze(fset, finalFilesForAnalysis, opts.RunFuncName, targetPackageID, moduleRootPath)
	if err != nil {
		return nil, targetFileAst, fmt.Errorf("failed to analyze AST (targetPkgID: %s, modRoot: %s): %w", targetPackageID, moduleRootPath, err)
	}
	slog.Info("Goat: Command metadata extracted", "commandName", cmdMetadata.Name, "optionsStruct", returnedOptionsStructName)

	const goatMarkersImportPath = "github.com/podhmo/goat" // Define the correct import path

	if opts.OptionsInitializerName != "" && returnedOptionsStructName != "" {
		// InterpretInitializer might need to look into multiple files if the initializer is not in the main target file.
		// For now, it's passed targetFileAst, which might need adjustment if initializers can be in other package files.
		// The currentPackageName is now more accurately determined.
		err = interpreter.InterpretInitializer(targetFileAst, returnedOptionsStructName, opts.OptionsInitializerName, cmdMetadata.Options, goatMarkersImportPath)
		if err != nil {
			return nil, targetFileAst, fmt.Errorf("failed to interpret options initializer %s: %w", opts.OptionsInitializerName, err)
		}
		slog.Info("Goat: Options initializer interpreted successfully.")
	} else {
		slog.Info("Goat: Skipping options initializer interpretation", "initializerName", opts.OptionsInitializerName, "optionsStructName", returnedOptionsStructName)
	}
	return cmdMetadata, targetFileAst, nil
}
