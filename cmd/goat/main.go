package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build" // Added import
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/podhmo/goat/internal/analyzer" // Ensure full path
	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/config"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/interpreter"
	"github.com/podhmo/goat/internal/loader" // Ensure full path
	"github.com/podhmo/goat/internal/metadata"
)

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

		cfg := &config.Config{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             targetFilename,
		}
		if err := runGoat(cfg); err != nil {
			log.Fatalf("Error running goat (emit): %+v", err)
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

		cfg := &config.Config{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             targetFilename,
		}

		fset := token.NewFileSet()
		cmdMetadata, _, err := scanMain(fset, cfg) // fileAST is not needed here
		if err != nil {
			log.Fatalf("Error scanning main for help-message: %+v", err)
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

		cfg := &config.Config{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             targetFilename,
		}

		fset := token.NewFileSet()
		cmdMetadata, _, err := scanMain(fset, cfg) // fileAST is not needed here
		if err != nil {
			log.Fatalf("Error scanning main for scan: %+v", err)
		}

		jsonData, err := json.MarshalIndent(cmdMetadata, "", "  ")
		if err != nil {
			log.Fatalf("Error marshalling metadata to JSON for scan: %+v", err)
		}
		fmt.Println(string(jsonData)) // Print JSON to stdout
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown subcommand '%s'\n", os.Args[1])
		// Print general usage
		fmt.Fprintln(os.Stderr, "Available subcommands: init, emit, help-message, scan")
		os.Exit(1)
	}
}

func runGoat(cfg *config.Config) error {
	fset := token.NewFileSet()
	cmdMetadata, fileAST, err := scanMain(fset, cfg)
	if err != nil {
		return fmt.Errorf("failed to scan main: %w", err)
	}

	helpMsg := help.GenerateHelp(cmdMetadata)

	// 5. TODO: Generate new main.go content (Future Step)
	newMainContent, err := codegen.GenerateMain(cmdMetadata, helpMsg)
	if err != nil {
		return fmt.Errorf("failed to generate new main.go content: %w", err)
	}

	// 6. TODO: Write the new content (Future Step)
	err = codegen.WriteMain(cfg.TargetFile, fset, fileAST, newMainContent, cmdMetadata.MainFuncPosition)
	if err != nil {
		return fmt.Errorf("failed to write modified main.go: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Goat: Processing finished.") // Print to stdout for test capture
	return nil
}

func scanMain(fset *token.FileSet, cfg *config.Config) (*metadata.CommandMetadata, *ast.File, error) {
	log.Printf("Goat: Analyzing %s with runFunc=%s, optionsInitializer=%s", cfg.TargetFile, cfg.RunFuncName, cfg.OptionsInitializerName)

	targetFileAst, err := loader.LoadFile(fset, cfg.TargetFile)
	if err != nil {
		// Still return nil for *ast.File if targetFileAst itself failed to load
		return nil, nil, fmt.Errorf("failed to load target file %s: %w", cfg.TargetFile, err)
	}
	currentPackageName := targetFileAst.Name.Name

	targetDir := filepath.Dir(cfg.TargetFile)
	var importPath string
	buildPkg, err := build.ImportDir(targetDir, 0)
	if err != nil {
		log.Printf("Warning: go/build.ImportDir failed for %s: %v. Will attempt to use '.' as import path.", targetDir, err)
		importPath = "."
	} else {
		importPath = buildPkg.ImportPath
		if buildPkg.Name != "" && currentPackageName == "" {
			currentPackageName = buildPkg.Name
		}
	}
	if importPath == "" {
		log.Printf("Warning: could not determine specific import path via go/build for %s. Using '.' .", targetDir)
		importPath = "."
	}
	// Ensure currentPackageName has a default if still empty
	if strings.TrimSpace(currentPackageName) == "" {
		log.Printf("Warning: could not determine package name for %s (AST: %s, Build: %s). Defaulting to 'main'.", cfg.TargetFile, targetFileAst.Name.Name, buildPkg.Name)
		currentPackageName = "main"
	}

	packageFiles, err := loader.LoadPackageFiles(fset, importPath, "")
	if err != nil {
		// If loading package files fails, we might still proceed with targetFileAst if analysis supports single file.
		// However, the new Analyze function expects a slice.
		log.Printf("Warning: failed to load package files for import path '%s' (derived from %s): %v. Proceeding with only the target file.", importPath, cfg.TargetFile, err)
		// Proceeding with just targetFileAst in filesForAnalysis
	}

	// filesForAnalysis := packageFiles // Unused variable
	// If LoadPackageFiles returned empty (e.g. directory has no .go files other than _test.go)
	// or if it errored but we decided to proceed, make sure we at least have the target file.
	// if len(filesForAnalysis) == 0 { // Old logic
	// 	if targetFileAst != nil {
	// 		log.Printf("Warning: loader.LoadPackageFiles returned no files for import path '%s'. Proceeding with only the directly loaded target file: %s", importPath, cfg.TargetFile)
	// 		filesForAnalysis = []*ast.File{targetFileAst}
	// 	} else {
	// 		// This case should ideally be caught by the initial LoadFile failure, but as a safeguard:
	// 		return nil, nil, fmt.Errorf("no Go files found for package (import path %s) and target file %s also failed to load or was nil", importPath, cfg.TargetFile)
	// 	}
	// }

	// New logic: Ensure targetFileAst is always included and handle potential duplicates.
	finalFilesForAnalysis := []*ast.File{targetFileAst}
	seenFilePaths := make(map[string]bool)
	if fset.File(targetFileAst.Pos()) != nil {
		seenFilePaths[fset.File(targetFileAst.Pos()).Name()] = true
	} else {
		// Fallback if position is not available, though unlikely for a loaded AST
		seenFilePaths[cfg.TargetFile] = true
	}

	for _, f := range packageFiles {
		tokenFile := fset.File(f.Pos())
		if tokenFile != nil {
			filePath := tokenFile.Name()
			if !seenFilePaths[filePath] {
				finalFilesForAnalysis = append(finalFilesForAnalysis, f)
				seenFilePaths[filePath] = true
			}
		}
	}
	if len(finalFilesForAnalysis) == 0 && targetFileAst == nil { // Should be caught by LoadFile earlier
		return nil, nil, fmt.Errorf("no Go files could be prepared for analysis, target file %s failed to load", cfg.TargetFile)
	}


	// The optionsStructName returned by Analyze needs to be captured.
	cmdMetadata, returnedOptionsStructName, err := analyzer.Analyze(fset, finalFilesForAnalysis, cfg.RunFuncName, currentPackageName)
	if err != nil {
		return nil, targetFileAst, fmt.Errorf("failed to analyze AST: %w", err)
	}
	log.Printf("Goat: Command metadata extracted for command: %s (options struct: %s)", cmdMetadata.Name, returnedOptionsStructName)

	const goatMarkersImportPath = "github.com/podhmo/goat/goat" // Define the correct import path

	if cfg.OptionsInitializerName != "" && returnedOptionsStructName != "" {
		// InterpretInitializer might need to look into multiple files if the initializer is not in the main target file.
		// For now, it's passed targetFileAst, which might need adjustment if initializers can be in other package files.
		// The currentPackageName is now more accurately determined.
		err = interpreter.InterpretInitializer(targetFileAst, returnedOptionsStructName, cfg.OptionsInitializerName, cmdMetadata.Options, goatMarkersImportPath)
		if err != nil {
			return nil, targetFileAst, fmt.Errorf("failed to interpret options initializer %s: %w", cfg.OptionsInitializerName, err)
		}
		log.Printf("Goat: Options initializer interpreted successfully.")
	} else {
		log.Printf("Goat: Skipping options initializer interpretation (initializer name: '%s', options struct name: '%s').", cfg.OptionsInitializerName, returnedOptionsStructName)
	}
	return cmdMetadata, targetFileAst, nil
}
