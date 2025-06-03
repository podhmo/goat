package main

import (
	"flag"
	"flag"
	"fmt"
	"go/ast"
	"go/build" // Added import
	"go/token"
	"log"
	"os"
	"path/filepath" // Added import
	"strings"       // Added import

	"github.com/podhmo/goat/internal/analyzer" // Ensure full path
	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/config"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/interpreter"
	"github.com/podhmo/goat/internal/loader" // Ensure full path
	"github.com/podhmo/goat/internal/metadata"
)

func main() {
	var (
		runFuncName            string
		optionsInitializerName string
		targetFilename         string
		// mainFuncName string // TODO: for specifying target main func name
	)

	flag.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint (e.g., run(Options) error)")
	flag.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct (e.g., newOptions() *Options)")
	// TODO: add more flags for goat's configuration if needed

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <target_gofile.go>\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: Target Go file must be specified.")
		flag.Usage()
		os.Exit(1)
	}
	targetFilename = flag.Arg(0)

	cfg := &config.Config{
		RunFuncName:            runFuncName,
		OptionsInitializerName: optionsInitializerName,
		TargetFile:             targetFilename,
	}

	if err := runGoat(cfg); err != nil {
		log.Fatalf("Error: %+v", err)
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

	log.Println("Goat: Processing finished.")
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

	filesForAnalysis := packageFiles
	// If LoadPackageFiles returned empty (e.g. directory has no .go files other than _test.go)
	// or if it errored but we decided to proceed, make sure we at least have the target file.
	if len(filesForAnalysis) == 0 {
		if targetFileAst != nil {
			log.Printf("Warning: loader.LoadPackageFiles returned no files for import path '%s'. Proceeding with only the directly loaded target file: %s", importPath, cfg.TargetFile)
			filesForAnalysis = []*ast.File{targetFileAst}
		} else {
			// This case should ideally be caught by the initial LoadFile failure, but as a safeguard:
			return nil, nil, fmt.Errorf("no Go files found for package (import path %s) and target file %s also failed to load or was nil", importPath, cfg.TargetFile)
		}
	}

	// The optionsStructName returned by Analyze needs to be captured.
	cmdMetadata, returnedOptionsStructName, err := analyzer.Analyze(fset, filesForAnalysis, cfg.RunFuncName, currentPackageName)
	if err != nil {
		return nil, targetFileAst, fmt.Errorf("failed to analyze AST: %w", err)
	}
	log.Printf("Goat: Command metadata extracted for command: %s (options struct: %s)", cmdMetadata.Name, returnedOptionsStructName)

	if cfg.OptionsInitializerName != "" && returnedOptionsStructName != "" {
		// InterpretInitializer might need to look into multiple files if the initializer is not in the main target file.
		// For now, it's passed targetFileAst, which might need adjustment if initializers can be in other package files.
		// The currentPackageName is now more accurately determined.
		err = interpreter.InterpretInitializer(targetFileAst, returnedOptionsStructName, cfg.OptionsInitializerName, cmdMetadata.Options, currentPackageName)
		if err != nil {
			return nil, targetFileAst, fmt.Errorf("failed to interpret options initializer %s: %w", cfg.OptionsInitializerName, err)
		}
		log.Printf("Goat: Options initializer interpreted successfully.")
	} else {
		log.Printf("Goat: Skipping options initializer interpretation (initializer name: '%s', options struct name: '%s').", cfg.OptionsInitializerName, returnedOptionsStructName)
	}
	return cmdMetadata, targetFileAst, nil
}
