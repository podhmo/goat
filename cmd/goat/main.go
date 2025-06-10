package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	// "go/build" // No longer used
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/podhmo/goat/internal/analyzer"
	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/interpreter"
	"github.com/podhmo/goat/internal/loader/lazyload" // Added
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
	// debug mode: if DEBUG environment variable is set, enable debug logging
	if _, ok := os.LookupEnv("DEBUG"); ok {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

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

	// --- Start of refactoring for lazyload.Loader ---
	// Old loader calls related to loader.LoadFile, loader.LoadPackageFiles,
	// loader.FindModuleRoot, loader.GetModuleName, and the construction of
	// finalFilesForAnalysis, targetPackageID, moduleRootPath, currentPackageName (partially)
	// are removed here. They will be replaced by logic using lazyload.Loader.

	// Placeholder for currentPackageName, will be derived from lazyload.Package
	var currentPackageName string
	// Placeholder for finalFilesForAnalysis, will be derived from lazyload.Package
	var finalFilesForAnalysis []*ast.File
	// Placeholder for targetFileAst, will be found among finalFilesForAnalysis
	var targetFileAst *ast.File
	// Placeholder for targetPackageID, will be derived from lazyload.Package
	var targetPackageID string
	// Placeholder for moduleRootPath, will be derived from lazyload.Package
	var moduleRootPath string

	llCfg := lazyload.Config{Fset: fset}
	l := lazyload.NewLoader(llCfg)

	targetDir := filepath.Dir(opts.TargetFile) // opts.TargetFile is already an absolute path
	slog.Debug("Goat: Loading package", "directory", targetDir)
	loadedPkgs, err := l.Load(targetDir) // Using targetDir as the load pattern
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load package for directory %s: %w", targetDir, err)
	}
	if len(loadedPkgs) == 0 {
		return nil, nil, fmt.Errorf("no packages found for directory %s", targetDir)
	}
	currentPkg := loadedPkgs[0] // Assume the first package is the relevant one

	pkgFilesMap, err := currentPkg.Files()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get AST files for package %s: %w", currentPkg.ImportPath(), err)
	}

	finalFilesForAnalysis = make([]*ast.File, 0, len(pkgFilesMap))
	for _, fileAst := range pkgFilesMap {
		finalFilesForAnalysis = append(finalFilesForAnalysis, fileAst)
	}

	// Find the specific targetFileAst. opts.TargetFile is already an absolute path.
	// Note: targetFileAst is one of the elements in finalFilesForAnalysis.
	for _, fileAstCandidate := range finalFilesForAnalysis {
		if fset.File(fileAstCandidate.Pos()).Name() == opts.TargetFile {
			targetFileAst = fileAstCandidate
			break
		}
	}
	if targetFileAst == nil {
		// This might happen if opts.TargetFile is not part of the loaded package's files,
		// or if the path matching is incorrect.
		slog.Error("Target file AST not found in loaded package files", "targetFile", opts.TargetFile, "package", currentPkg.ImportPath())
		// Log available files for debugging
		for _, f := range finalFilesForAnalysis {
			slog.Debug("Available file in package", "path", fset.File(f.Pos()).Name())
		}
		return nil, nil, fmt.Errorf("target file AST %s not found in loaded package %s", opts.TargetFile, currentPkg.ImportPath())
	}

	targetPackageID = currentPkg.ImportPath()
	if mi := currentPkg.Module(); mi != nil {
		moduleRootPath = mi.Dir
	} else {
		// If not part of a module (e.g. GOPATH mode or single file),
		// use the package's directory as a fallback for moduleRootPath.
		// This might not be strictly a "module root" but is a sensible root for package context.
		moduleRootPath = currentPkg.Dir()
		slog.Debug("Goat: No module information found for package, using package directory as effective root.", "packageDir", moduleRootPath)
	}
	currentPackageName = currentPkg.Name()

	// Ensure moduleRootPath is non-empty; default to current directory if all else fails.
	if moduleRootPath == "" {
		slog.Warn("Module root path is empty, defaulting to current working directory '.'")
		moduleRootPath = "." // Or handle as an error if a module context is strictly required
	}

	cmdMetadata, returnedOptionsStructName, err := analyzer.Analyze(fset, finalFilesForAnalysis, opts.RunFuncName, targetPackageID, moduleRootPath, l)
	if err != nil {
		return nil, targetFileAst, fmt.Errorf("failed to analyze AST (targetPkgID: %s, modRoot: %s): %w", targetPackageID, moduleRootPath, err)
	}
	slog.Info("Goat: Command metadata extracted", "commandName", cmdMetadata.Name, "optionsStruct", returnedOptionsStructName)

	const goatMarkersImportPath = "github.com/podhmo/goat" // Define the correct import path

	if opts.OptionsInitializerName != "" && returnedOptionsStructName != "" {
		// This also needs targetFileAst to be correctly identified.
		// For now, this will likely not run or fail if targetFileAst is nil.
		if targetFileAst == nil {
			slog.Warn("Skipping options initializer interpretation as targetFileAst is not yet identified in refactoring.")
		} else {
			err = interpreter.InterpretInitializer(targetFileAst, returnedOptionsStructName, opts.OptionsInitializerName, cmdMetadata.Options, goatMarkersImportPath)
			if err != nil {
				return nil, targetFileAst, fmt.Errorf("failed to interpret options initializer %s: %w", opts.OptionsInitializerName, err)
			}
			slog.Info("Goat: Options initializer interpreted successfully.")
		}
	} else {
		slog.Info("Goat: Skipping options initializer interpretation", "initializerName", opts.OptionsInitializerName, "optionsStructName", returnedOptionsStructName)
	}
	return cmdMetadata, targetFileAst, nil
}
