package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/podhmo/goat/internal/analyzer"
	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/help"
	"github.com/podhmo/goat/internal/interpreter"
	"github.com/podhmo/goat/internal/loader"
	"github.com/podhmo/goat/internal/metadata"
)

// Options holds the configuration for the goat tool itself.
type Options struct {
	RunFuncName            string
	OptionsInitializerName string
	TargetFile             string
	LocatorName            string
}

func main() {
	if _, ok := os.LookupEnv("DEBUG"); ok {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: goat <subcommand> [options]")
		fmt.Fprintln(os.Stderr, "Available subcommands: init, emit, help-message, scan")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		initCmd := flag.NewFlagSet("init", flag.ExitOnError)
		initCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: goat init\n\nGenerates a basic main.go file in the current directory.\n")
		}
		initCmd.Parse(os.Args[2:])
		if err := initMain(); err != nil {
			slog.Error("Error running goat (init)", "error", err)
			os.Exit(1)
		}
		slog.Info("Goat: init command finished successfully.") // Updated
	case "emit":
		emitCmd := flag.NewFlagSet("emit", flag.ExitOnError)
		var runFuncName, optionsInitializerName, locatorName string
		emitCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		emitCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")
		emitCmd.StringVar(&locatorName, "locator", "golist", "Locator to use for package discovery (golist or gomod)")
		emitCmd.Usage = func() {
			fmt.Fprintf(os.Stderr, "Usage: goat emit [options] <target_gofile.go>\n\nOptions:\n")
			emitCmd.PrintDefaults()
		}
		emitCmd.Parse(os.Args[2:])
		if emitCmd.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Error: Target Go file must be specified for emit.")
			emitCmd.Usage()
			os.Exit(1)
		}
		opts := &Options{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             emitCmd.Arg(0),
			LocatorName:            locatorName,
		}
		if err := runGoat(opts); err != nil {
			slog.Error("Error running goat (emit)", "error", err)
			os.Exit(1)
		}
	case "help-message":
		helpMessageCmd := flag.NewFlagSet("help-message", flag.ExitOnError)
		var runFuncName, optionsInitializerName, locatorName string
		helpMessageCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		helpMessageCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")
		helpMessageCmd.StringVar(&locatorName, "locator", "golist", "Locator to use for package discovery (golist or gomod)")
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
		opts := &Options{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             helpMessageCmd.Arg(0),
			LocatorName:            locatorName,
		}
		fset := token.NewFileSet()
		cmdMetadata, _, err := scanMain(fset, opts)
		if err != nil {
			slog.Error("Error scanning main for help-message", "error", err)
			os.Exit(1)
		}
		helpMsg := help.GenerateHelp(cmdMetadata)
		fmt.Print(helpMsg)
	case "scan":
		scanCmd := flag.NewFlagSet("scan", flag.ExitOnError)
		var runFuncName, optionsInitializerName, locatorName string
		scanCmd.StringVar(&runFuncName, "run", "run", "Name of the function to be treated as the entrypoint")
		scanCmd.StringVar(&optionsInitializerName, "initializer", "", "Name of the function that initializes the options struct")
		scanCmd.StringVar(&locatorName, "locator", "golist", "Locator to use for package discovery (golist or gomod)")
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
		opts := &Options{
			RunFuncName:            runFuncName,
			OptionsInitializerName: optionsInitializerName,
			TargetFile:             scanCmd.Arg(0),
			LocatorName:            locatorName,
		}
		fset := token.NewFileSet()
		cmdMetadata, _, err := scanMain(fset, opts)
		if err != nil {
			slog.Error("Error scanning main for scan", "error", err)
			os.Exit(1)
		}
		jsonData, err := json.MarshalIndent(cmdMetadata, "", "  ")
		if err != nil {
			slog.Error("Error marshalling metadata to JSON for scan", "error", err)
			os.Exit(1)
		}
		fmt.Println(string(jsonData))
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown subcommand '%s'\n", os.Args[1])
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
	newMainContent, err := codegen.GenerateMain(cmdMetadata, helpMsg, false)
	if err != nil {
		return fmt.Errorf("failed to generate new main.go content: %w", err)
	}
	err = codegen.WriteMain(opts.TargetFile, fset, fileAST, newMainContent, cmdMetadata.MainFuncPosition)
	if err != nil {
		return fmt.Errorf("failed to write modified main.go: %w", err)
	}
	fmt.Fprintln(os.Stdout, "Goat: Processing finished.")
	return nil
}

const mainGoTemplate = `package main

import (
	"flag"
	"fmt"
	"os"
)

// Options defines the command line options.
type Options struct {
	Message string // Message to print
}

// run is the actual command logic.
func run(opts Options) error {
	fmt.Println(opts.Message)
	return nil
}

//go:generate goat emit -run run -initializer "" main.go
func main() {
	options := Options{}
	flag.StringVar(&options.Message, "message", "Hello, world!", "Message to print")
	flag.Parse()

	if err := run(options); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

// initMain initializes a basic main.go file in the current directory.
func initMain() error {
	slog.Info("Goat: Initializing main.go in current directory.")

	// Create main.go in current directory
	mainGoPath := "main.go"
	f, err := os.Create(mainGoPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", mainGoPath, err)
	}
	defer f.Close()

	// Data for template execution
	templateData := map[string]string{
		"Name": "main", // Package name for main.go in current directory
	}

	// Parse and execute template
	tmpl, err := template.New("main.go").Parse(mainGoTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse main.go template: %w", err)
	}
	if err := tmpl.Execute(f, templateData); err != nil {
		return fmt.Errorf("failed to execute main.go template: %w", err)
	}

	slog.Info("Goat: Created main.go in current directory", "path", mainGoPath)
	fmt.Fprintln(os.Stdout, "Goat: main.go initialized successfully in current directory.")
	return nil
}

func scanMain(fset *token.FileSet, opts *Options) (*metadata.CommandMetadata, *ast.File, error) {
	absTargetFile, err := filepath.Abs(opts.TargetFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for target file %s: %w", opts.TargetFile, err)
	}
	opts.TargetFile = absTargetFile

	slog.Info("Goat: Analyzing file", "targetFile", opts.TargetFile, "runFunc", opts.RunFuncName, "optionsInitializer", opts.OptionsInitializerName)

	targetDir := filepath.Dir(opts.TargetFile)

	// Parse only the target .go file
	targetFileAst, err := parser.ParseFile(fset, opts.TargetFile, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse target file %s: %w", opts.TargetFile, err)
	}
	if targetFileAst == nil { // Should be caught by err, but good practice
		return nil, nil, fmt.Errorf("parser.ParseFile returned nil AST for %s without an error", opts.TargetFile)
	}

	// Dynamically choose the locator
	var selectedLocator loader.PackageLocator
	switch opts.LocatorName {
	case "gomod":
		gmLocator := &loader.GoModLocator{}
		selectedLocator = gmLocator.Locate
		slog.Debug("Goat: Using GoModLocator for package discovery", "workingDir", targetDir)
	default: // "golist" or any other unspecified value
		selectedLocator = loader.GoListLocator
		slog.Debug("Goat: Using GoListLocator for package discovery")
	}

	// Create an instance of our custom locator
	customLocator := &execDirectoryLocator{
		BaseDir:        targetDir,
		WrappedLocator: selectedLocator,
	}

	llCfg := loader.Config{
		Fset:    fset,
		Locator: customLocator.Locate,
	}
	l := loader.New(llCfg) // Loader is still needed for internal/analyzer

	// We still need package information (like import path) for the target file's package.
	// Load package info for the directory containing the target file.
	// This is less ideal as it might still load more than just metadata,
	// but it's a common way to get the canonical import path.
	// The alternative would be complex AST walking for package decl and trying to map to dir.
	slog.Debug("Goat: Loading package info for target directory", "directory", targetDir, "pattern", ".")
	loadedPkgs, err := l.Load(".") // This uses the customLocator, so it runs 'go list .' in targetDir
	if err != nil {
		return nil, targetFileAst, fmt.Errorf("failed to load package info for directory %s (pattern .): %w", targetDir, err)
	}
	if len(loadedPkgs) == 0 {
		return nil, targetFileAst, fmt.Errorf("no packages found for directory %s (pattern .)", targetDir)
	}
	currentPkg := loadedPkgs[0] // Assuming the first package is the one we care about
	targetPackageID := currentPkg.ImportPath
	slog.Debug("Goat: Determined target package ID", "targetPackageID", targetPackageID, "packageDir", currentPkg.Dir)

	moduleRootPath, err := findModuleRoot(currentPkg.Dir) // currentPkg.Dir comes from loader
	if err != nil {
		slog.Warn("Error trying to find module root", "packageDir", currentPkg.Dir, "error", err)
		moduleRootPath = currentPkg.Dir
	} else if moduleRootPath == "" {
		slog.Warn("go.mod not found upwards from package directory. Using package directory as module root.", "packageDir", currentPkg.Dir)
		moduleRootPath = currentPkg.Dir
	}
	slog.Debug("Determined module root", "moduleRootPath", moduleRootPath)

	// The files for analysis is now just the single parsed target file.
	// However, analyzer.Analyze expects a slice.
	filesForAnalysis := []*ast.File{targetFileAst}

	cmdMetadata, returnedOptionsStructName, err := analyzer.Analyze(fset, filesForAnalysis, opts.RunFuncName, opts.OptionsInitializerName, targetPackageID, moduleRootPath, l)
	if err != nil {
		return nil, targetFileAst, fmt.Errorf("failed to analyze AST (targetPkgID: %s, modRoot: %s): %w", targetPackageID, moduleRootPath, err)
	}
	slog.Info("Goat: Command metadata extracted", "commandName", cmdMetadata.Name, "optionsStruct", returnedOptionsStructName)

	const goatMarkersImportPath = "github.com/podhmo/goat"
	if opts.OptionsInitializerName != "" && returnedOptionsStructName != "" {
		// targetFileAst is already available and is the correct AST to interpret the initializer from.
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

// findModuleRoot searches for a go.mod file starting from dir and going upwards.
func findModuleRoot(dir string) (string, error) {
	current := dir
	for {
		goModPath := filepath.Join(current, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return current, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("error checking for go.mod at %s: %w", goModPath, err)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", nil
		}
		current = parent
	}
}

// execDirectoryLocator is a helper struct to manage locating packages from a specific base directory.
type execDirectoryLocator struct {
	BaseDir        string
	WrappedLocator loader.PackageLocator
	// Fset is not used by Locate but can be stored if needed for other purposes.
	// Fset           *token.FileSet
}

// Locate matches the loader.PackageLocator function signature.
// It executes the wrapped locator from BaseDir.
func (l *execDirectoryLocator) Locate(pattern string, buildCtx loader.BuildContext) ([]loader.PackageMetaInfo, error) {
	if l.WrappedLocator == nil {
		return nil, fmt.Errorf("ExecDirectoryLocator: WrappedLocator is nil")
	}

	originalWD, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("ExecDirectoryLocator: failed to get current working directory: %w", err)
	}

	if l.BaseDir != "" && l.BaseDir != "." && l.BaseDir != originalWD {
		if err := os.Chdir(l.BaseDir); err != nil {
			return nil, fmt.Errorf("ExecDirectoryLocator: failed to change directory to %s: %w", l.BaseDir, err)
		}
		slog.Debug("ExecDirectoryLocator: Changed working directory", "to", l.BaseDir)
		defer func() {
			if err := os.Chdir(originalWD); err != nil {
				slog.Error("ExecDirectoryLocator: failed to restore original working directory", "originalWD", originalWD, "error", err)
			} else {
				slog.Debug("ExecDirectoryLocator: Restored working directory", "to", originalWD)
			}
		}()
	}

	// Pass the original buildCtx to the wrapped locator.
	// The buildCtx in this struct is primarily for the wrapped locator if it needs it.
	return l.WrappedLocator(pattern, buildCtx)
}
