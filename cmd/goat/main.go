package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/podhmo/goat/internal/analyzer"
	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/interpreter"
	"github.com/podhmo/goat/internal/loader/lazyload"
	"github.com/podhmo/goat/internal/metadata"
)

// Options holds the configuration for the goat tool itself,
// typically derived from its command-line arguments.
type Options struct {
	RunFuncName            string // Name of the target 'run' function (e.g., "run")
	OptionsInitializerName string // Name of the options initializer function (e.g., "NewOptions")
	TargetFile             string // Path to the target Go file to be processed
	AnalyzerVersion        int    // Version of the analyzer to use (2 or 3)
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
			analyzerVersion        int
		)
		emitCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		emitCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")
		emitCmd.IntVar(&analyzerVersion, "analyzer-version", 2, "Version of the analyzer to use (2 or 3)")
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
			AnalyzerVersion:        analyzerVersion,
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
			analyzerVersion        int
		)
		helpMessageCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		helpMessageCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")
		helpMessageCmd.IntVar(&analyzerVersion, "analyzer-version", 2, "Version of the analyzer to use (2 or 3)")

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
			AnalyzerVersion:        analyzerVersion,
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
			analyzerVersion        int
		)
		scanCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		scanCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")
		scanCmd.IntVar(&analyzerVersion, "analyzer-version", 2, "Version of the analyzer to use (2 or 3)")

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
			AnalyzerVersion:        analyzerVersion,
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

	targetDir := filepath.Dir(opts.TargetFile) // opts.TargetFile is already absolute

	lazyLoadCfg := lazyload.Config{
		Fset: fset,
		// Using default GoListLocator by not setting Locator
		// Using default BuildContext by not setting Context
	}
	ldr := lazyload.NewLoader(lazyLoadCfg)

	loadedPkgs, err := ldr.Load(targetDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load package for directory %s: %w", targetDir, err)
	}
	if len(loadedPkgs) == 0 {
		return nil, nil, fmt.Errorf("no package found in directory %s", targetDir)
	}
	if len(loadedPkgs) > 1 {
		slog.Warn("Multiple packages found for directory, using the first one.", "dir", targetDir, "count", len(loadedPkgs))
	}
	loadedPkg := loadedPkgs[0]
	if loadedPkg.RawMeta.Error != "" {
		return nil, nil, fmt.Errorf("error loading package %s: %s", loadedPkg.ImportPath, loadedPkg.RawMeta.Error)
	}

	moduleRootPath := loadedPkg.RawMeta.ModuleDir
	if moduleRootPath == "" {
		slog.Warn("ModuleDir not found for package, using package directory as module root fallback.", "packageDir", loadedPkg.Dir)
		moduleRootPath = loadedPkg.Dir
	}
	targetPackageID := loadedPkg.ImportPath
	// currentPackageName will be derived by analyzer or interpreter as needed from actual ASTs.

	parsedFileMap, err := loadedPkg.Files()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse files for package %s: %w", loadedPkg.ImportPath, err)
	}

	var finalFilesForAnalysis []*ast.File
	for _, fileAst := range parsedFileMap {
		finalFilesForAnalysis = append(finalFilesForAnalysis, fileAst)
	}
	if len(finalFilesForAnalysis) == 0 {
		return nil, nil, fmt.Errorf("no Go files found in package %s", loadedPkg.ImportPath)
	}

	var targetFileAst *ast.File
	for _, fileAst := range finalFilesForAnalysis {
		pos := fset.Position(fileAst.Pos())
		if pos.Filename == opts.TargetFile {
			targetFileAst = fileAst
			break
		}
	}

	if targetFileAst == nil {
		return nil, nil, fmt.Errorf("target Go file %s not found among parsed files of package %s", opts.TargetFile, loadedPkg.ImportPath)
	}

	cmdMetadata, returnedOptionsStructName, err := analyzer.Analyze(fset, finalFilesForAnalysis, opts.RunFuncName, targetPackageID, moduleRootPath, opts.AnalyzerVersion, ldr)
	if err != nil {
		return nil, targetFileAst, fmt.Errorf("failed to analyze AST (targetPkgID: %s, modRoot: %s): %w", targetPackageID, moduleRootPath, err)
	}
	slog.Info("Goat: Command metadata extracted", "commandName", cmdMetadata.Name, "optionsStruct", returnedOptionsStructName)

	const goatMarkersImportPath = "github.com/podhmo/goat"

	if opts.OptionsInitializerName != "" && returnedOptionsStructName != "" {
		// InterpretInitializer uses targetFileAst to get context like package name.
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
