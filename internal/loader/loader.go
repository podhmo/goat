package loader

import (
	"context"
	"fmt"
	"go/ast" // Added import
	"go/token"
	"os" // New import for os.Getwd()
	"sync"
)

// BuildContext defines the build parameters for locating and loading packages.
type BuildContext struct {
	GOOS        string
	GOARCH      string
	BuildTags   []string
	ToolDir     string // Optional: directory for build tools like go command
	UseGoModule bool   // Whether to operate in Go modules mode
}

// Config defines the configuration for a Loader.
type Config struct {
	// Context specifies the build context.
	Context BuildContext

	// Fset is the file set used for parsing files.
	Fset *token.FileSet

	// Locator is a function that finds packages based on a pattern.
	// If nil, a default locator (e.g., using `go list`) will be used.
	Locator PackageLocator
	// TODO: Add other configurations like ParseFile func if needed
}

// SymbolInfo stores information about a resolved symbol, providing easy access
// to its definition and location.
type SymbolInfo struct {
	// PackagePath is the import path of the package where the symbol is defined.
	PackagePath string
	// SymbolName is the name of the symbol (e.g., function name, type name).
	SymbolName string
	// FilePath is the canonical path to the .go file where the symbol is defined.
	FilePath string
	// Node is the AST node representing the symbol's declaration
	// (e.g., *ast.FuncDecl, *ast.TypeSpec, *ast.ValueSpec).
	Node ast.Node
}

// Loader is responsible for loading packages.
type Loader struct {
	fset  *token.FileSet
	cfg   Config
	mu    sync.Mutex
	cache map[string]*Package // cache of loaded packages by import path

	// fileASTCache caches ASTs by their canonical file path.
	// This helps avoid re-parsing files that might be part of multiple packages
	// or accessed multiple times.
	fileASTCache map[string]*ast.File

	// symbolCache caches information about resolved symbols.
	// The key is a full symbol name, e.g., "<package_path>:<symbol_name>".
	symbolCache map[string]SymbolInfo
}

// New creates a new Loader with the given configuration.
func New(cfg Config) *Loader {
	if cfg.Locator == nil {
		// Default to GoModLocator
		wd, err := os.Getwd()
		if err != nil {
			// This is a fallback or error handling strategy.
			// For now, let panic, as a valid working dir is crucial.
			// Alternatively, could return an error from New.
			panic(fmt.Sprintf("failed to get working directory for GoModLocator: %v", err))
		}
		gml := &GoModLocator{WorkingDir: wd} // Corrected to use exported field
		cfg.Locator = gml.Locate             // Use the method from the instance
	}
	fset := cfg.Fset
	if fset == nil {
		fset = token.NewFileSet()
	}
	return &Loader{
		fset:         fset,
		cfg:          cfg,
		cache:        make(map[string]*Package),
		fileASTCache: make(map[string]*ast.File),  // Initialize new cache
		symbolCache:  make(map[string]SymbolInfo), // Initialize new cache
	}
}

// Load loads the packages matching the given patterns.
// It only loads the metadata for the top-level packages.
// Dependent packages are loaded lazily when accessed.
func (l *Loader) Load(ctx context.Context, patterns ...string) ([]*Package, error) {
	var pkgs []*Package
	var errs []error

	for _, pattern := range patterns {
		metaInfos, err := l.cfg.Locator(ctx, pattern, l.cfg.Context)
		if err != nil {
			errs = append(errs, fmt.Errorf("error locating package for pattern %q: %w", pattern, err))
			continue
		}

	rawMetaLoop:
		for _, meta := range metaInfos {
			// Check cache first for the exact import path
			l.mu.Lock()
			if cachedPkg, ok := l.cache[meta.ImportPath]; ok {
				// Ensure this cached package matches the pattern's origin if relevant
				// For now, assume if import path matches, it's the same.
				pkgs = append(pkgs, cachedPkg)
				l.mu.Unlock()
				continue rawMetaLoop
			}
			l.mu.Unlock()

			pkg := NewPackage(meta, l)
			pkgs = append(pkgs, pkg)

			l.mu.Lock()
			l.cache[pkg.ImportPath] = pkg
			l.mu.Unlock()
		}
	}

	if len(errs) > 0 {
		// Combine errors, or handle them as per desired policy
		return pkgs, fmt.Errorf("encountered errors during load: %v", errs)
	}
	return pkgs, nil
}

// resolveImport is called by a Package to resolve one of its imports.
// It ensures that the imported package is loaded and returns it.
func (l *Loader) resolveImport(ctx context.Context, importerPath string, importPath string) (*Package, error) {
	l.mu.Lock()
	if pkg, ok := l.cache[importPath]; ok {
		l.mu.Unlock()
		return pkg, nil
	}
	l.mu.Unlock()

	// If not in cache, try to locate and load it.
	// The locator should be able to handle an absolute import path.
	metaInfos, err := l.cfg.Locator(ctx, importPath, l.cfg.Context)
	if err != nil {
		return nil, fmt.Errorf("loader: failed to locate imported package %q (imported by %q): %w", importPath, importerPath, err)
	}

	if len(metaInfos) == 0 {
		return nil, fmt.Errorf("loader: package %q not found (imported by %q)", importPath, importerPath)
	}
	if len(metaInfos) > 1 {
		// This should ideally not happen if importPath is canonical.
		// Or, the locator needs to be more precise for direct import paths.
		return nil, fmt.Errorf("loader: ambiguous import path %q resolved to multiple packages (imported by %q)", importPath, importerPath)
	}

	meta := metaInfos[0]
	// Ensure the located package has the expected import path
	if meta.ImportPath != importPath {
		return nil, fmt.Errorf("loader: located package import path %q does not match requested %q (imported by %q)", meta.ImportPath, importPath, importerPath)
	}

	pkg := NewPackage(meta, l)

	l.mu.Lock()
	// Double check cache in case of concurrent loads
	if existingPkg, ok := l.cache[importPath]; ok {
		l.mu.Unlock()
		return existingPkg, nil
	}
	l.cache[importPath] = pkg
	l.mu.Unlock()

	return pkg, nil
}

// GetAST retrieves a parsed AST from the cache by its canonical file path.
// It returns the AST and true if found, otherwise nil and false.
func (l *Loader) GetAST(filePath string) (*ast.File, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	astFile, found := l.fileASTCache[filePath]
	return astFile, found
}

// LookupSymbol retrieves symbol information from the cache by its full name
// (e.g., "<package_path>:<symbol_name>").
// It returns the SymbolInfo and true if found, otherwise an empty SymbolInfo and false.
func (l *Loader) LookupSymbol(fullSymbolName string) (SymbolInfo, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	symbolInfo, found := l.symbolCache[fullSymbolName]
	return symbolInfo, found
}
