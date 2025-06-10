package lazyload

import (
	"fmt"
	"go/token"
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

// Loader is responsible for loading packages.
type Loader struct {
	fset  *token.FileSet
	cfg   Config
	mu    sync.Mutex
	cache map[string]*Package // cache of loaded packages by import path
}

// NewLoader creates a new Loader with the given configuration.
func NewLoader(cfg Config) *Loader {
	if cfg.Locator == nil {
		cfg.Locator = GoListLocator // Default locator
	}
	fset := cfg.Fset
	if fset == nil {
		fset = token.NewFileSet()
	}
	return &Loader{
		fset:  fset,
		cfg:   cfg,
		cache: make(map[string]*Package),
	}
}

// Load loads the packages matching the given patterns, using baseDir as the context for the locator.
// It only loads the metadata for the top-level packages.
// Dependent packages are loaded lazily when accessed.
func (l *Loader) Load(baseDir string, patterns ...string) ([]*Package, error) {
	var pkgs []*Package
	var errs []error

	for _, pattern := range patterns {
		metaInfos, err := l.cfg.Locator(pattern, baseDir, l.cfg.Context)
		if err != nil {
			errs = append(errs, fmt.Errorf("error locating package for pattern %q in dir %q: %w", pattern, baseDir, err))
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
// The importerDir is the directory of the package that is importing, used as context if needed.
func (l *Loader) resolveImport(importerDir string, importerPath string, importPath string) (*Package, error) {
	l.mu.Lock()
	if pkg, ok := l.cache[importPath]; ok {
		l.mu.Unlock()
		return pkg, nil
	}
	l.mu.Unlock()

	// If not in cache, try to locate and load it.
	// The locator should be able to handle an absolute import path.
	// Use importerDir as the baseDir for resolving the import.
	metaInfos, err := l.cfg.Locator(importPath, importerDir, l.cfg.Context)
	if err != nil {
		return nil, fmt.Errorf("loader: failed to locate imported package %q (imported by %q from dir %q): %w", importPath, importerPath, importerDir, err)
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
