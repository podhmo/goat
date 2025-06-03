package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"os"
	// "flag" // No longer used directly in main, but in subcommands

	"github.com/podhmo/goat/internal/analyzer"
	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/config"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/interpreter"
	"github.com/podhmo/goat/internal/loader"
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
		fmt.Println(helpMsg) // Print to stdout

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

	log.Println("Goat: Processing finished.")
	return nil
}

func scanMain(fset *token.FileSet, cfg *config.Config) (*metadata.CommandMetadata, *ast.File, error) {
	log.Printf("Goat: Analyzing %s with runFunc=%s, optionsInitializer=%s", cfg.TargetFile, cfg.RunFuncName, cfg.OptionsInitializerName)

	fileAST, err := loader.LoadFile(fset, cfg.TargetFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load target file %s: %w", cfg.TargetFile, err)
	}

	cmdMetadata, optionsStructName, err := analyzer.Analyze(fileAST, cfg.RunFuncName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to analyze AST: %w", err)
	}
	log.Printf("Goat: Command metadata extracted for command: %s", cmdMetadata.Name)

	if cfg.OptionsInitializerName != "" && optionsStructName != "" {
		err = interpreter.InterpretInitializer(fileAST, optionsStructName, cfg.OptionsInitializerName, cmdMetadata.Options, "github.com/podhmo/goat/goat")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to interpret options initializer %s: %w", cfg.OptionsInitializerName, err)
		}
		log.Printf("Goat: Options initializer interpreted successfully.")
	} else {
		log.Printf("Goat: Skipping options initializer interpretation (initializer name or options struct not found/specified).")
	}
	return cmdMetadata, fileAST, nil
}
