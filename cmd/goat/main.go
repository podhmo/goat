package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"os"

	"github.com/podhmo/goat/internal/analyzer"
	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/config"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/interpreter"
	"github.com/podhmo/goat/internal/loader"
	"github.com/podhmo/goat/internal/metadata"
)

func main() {
	// Define command-line flags for goat tool itself
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
		log.Fatalf("Error: %+v", err) // Use %+v for more detailed error from pkg/errors
	}
}

func runGoat(cfg *config.Config) error {
	fset := token.NewFileSet()
	cmdMetadata, fileAST, err := scanMain(fset, cfg)
	if err != nil {
		return fmt.Errorf("failed to scan main: %w", err)
	}

	// 4. Generate help message
	helpMsg := help.GenerateHelp(cmdMetadata)

	// 5. TODO: Generate new main.go content (Future Step)
	newMainContent, err := codegen.GenerateMain(cmdMetadata, helpMsg)
	if err != nil {
		return fmt.Errorf("failed to generate new main.go content: %w", err)
	}

	// 6. TODO: Write the new content (Future Step)
	// For now, just print the target path where it would be written or how it would be modified
	err = codegen.WriteMain(cfg.TargetFile, fset, fileAST, newMainContent, cmdMetadata.MainFuncPosition)
	if err != nil {
		return fmt.Errorf("failed to write modified main.go: %w", err)
	}

	log.Println("Goat: Processing finished.")
	return nil
}

func scanMain(fset *token.FileSet, cfg *config.Config) (*metadata.CommandMetadata, *ast.File, error) {
	log.Printf("Goat: Analyzing %s with runFunc=%s, optionsInitializer=%s", cfg.TargetFile, cfg.RunFuncName, cfg.OptionsInitializerName)

	// 1. Load and parse the target Go file
	fileAST, err := loader.LoadFile(fset, cfg.TargetFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load target file %s: %w", cfg.TargetFile, err)
	}

	// 2. Analyze the AST to extract metadata
	// This step identifies the main command, run function, options struct, and its fields.
	cmdMetadata, optionsStructName, err := analyzer.Analyze(fileAST, cfg.RunFuncName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to analyze AST: %w", err)
	}
	log.Printf("Goat: Command metadata extracted for command: %s", cmdMetadata.Name)

	// 3. Interpret the options initializer function (e.g., newOptions)
	// This step evaluates goat.Default() and goat.Enum() calls to populate default values and enum choices.
	if cfg.OptionsInitializerName != "" && optionsStructName != "" {
		err = interpreter.InterpretInitializer(fileAST, optionsStructName, cfg.OptionsInitializerName, cmdMetadata.Options, "github.com/podhmo/goat/goat") // Pass marker package path
		if err != nil {
			return nil, nil, fmt.Errorf("failed to interpret options initializer %s: %w", cfg.OptionsInitializerName, err)
		}
		log.Printf("Goat: Options initializer interpreted successfully.")
	} else {
		log.Printf("Goat: Skipping options initializer interpretation (initializer name or options struct not found/specified).")
	}
	return cmdMetadata, fileAST, nil
}
