package loader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os" // Required for os.ReadFile
	"os/exec"
	"path/filepath" // New import
	"strings"

	"errors"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// getGoModCacheDir attempts to find the Go module cache directory.
// It checks GOMODCACHE env var, then defaults to $GOPATH/pkg/mod or $HOME/go/pkg/mod.
func getGoModCacheDir() (string, error) {
	if cacheDir := os.Getenv("GOMODCACHE"); cacheDir != "" {
		absCacheDir, err := filepath.Abs(cacheDir)
		if err != nil {
			return "", fmt.Errorf("GOMODCACHE path %s could not be made absolute: %w", cacheDir, err)
		}
		return absCacheDir, nil
	}

	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		gopaths := filepath.SplitList(gopath)
		if len(gopaths) > 0 && gopaths[0] != "" {
			absGoPath, err := filepath.Abs(gopaths[0])
			if err != nil {
				return "", fmt.Errorf("GOPATH element %s could not be made absolute: %w", gopaths[0], err)
			}
			return filepath.Join(absGoPath, "pkg", "mod"), nil
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("could not determine GOMODCACHE: UserHomeDir error and GOPATH not suitable")
	}
	return filepath.Join(homeDir, "go", "pkg", "mod"), nil
}

// PackageMetaInfo holds basic information about a Go package,
// sufficient for initiating a lazy load.
type PackageMetaInfo struct {
	ImportPath    string   // Canonical import path
	Name          string   // Package name (can be empty if not determined by locator)
	Dir           string   // Directory containing package sources
	GoFiles       []string // Go source files (non-test, relative to Dir)
	TestGoFiles   []string // _test.go files in package (relative to Dir)
	XTestGoFiles  []string // _test.go files for external tests (relative to Dir)
	DirectImports []string // List of canonical import paths directly imported by this package
	ModulePath    string   // Module path if part of a module
	ModuleDir     string   // Module root directory if part of a module
	Error         string   // Error message if package loading failed (from go list)
}

// PackageLocator is a function type that locates packages based on a pattern
// and returns their metadata.
// The build context provides parameters like GOOS, GOARCH, and build tags.
type PackageLocator func(pattern string, buildCtx BuildContext) ([]PackageMetaInfo, error)

// GoModLocator is a PackageLocator that resolves import paths
// without relying on the `go list` command.
type GoModLocator struct {
	workingDir string // The working directory, typically the root of the main module.
}

// Locate implements the PackageLocator interface for GoModLocator.
// It resolves package paths without using `go list`.
func (gml *GoModLocator) Locate(pattern string, buildCtx BuildContext) ([]PackageMetaInfo, error) {
	// Handle vendor paths (placeholder)
	if strings.Contains(pattern, "/vendor/") || strings.HasPrefix(pattern, "vendor/") {
		return nil, errors.New("GoModLocator.Locate: vendor directory handling is not yet implemented")
	}

	if strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, "../") {
		// Handle relative path
		pkgDir := filepath.Clean(filepath.Join(gml.workingDir, pattern))
		absPkgDir, err := filepath.Abs(pkgDir) // Ensure pkgDir is absolute
		if err != nil {
			return nil, fmt.Errorf("could not get absolute path for relative dir %s: %w", pkgDir, err)
		}
		pkgDir = absPkgDir

		// Get package name (e.g., directory name)
		pkgName := filepath.Base(pkgDir)

		pkgName := filepath.Base(pkgDir)
		goFiles, testGoFiles, xTestGoFiles, errList := gml.listGoFiles(pkgDir)
		if errList != nil {
			return nil, fmt.Errorf("error listing go files in %s (from pattern %s): %w", pkgDir, pattern, errList)
		}
		if len(goFiles) == 0 && len(testGoFiles) == 0 && len(xTestGoFiles) == 0 {
			return nil, fmt.Errorf("no Go files found in relative path %s (resolved to %s)", pattern, pkgDir)
		}
		importPath := pattern
		moduleDir, _ := gml.findModuleRoot(pkgDir)  // findModuleRoot returns absolute path or ""
		var modulePath string
		if moduleDir != "" {
			modFile, errMod := gml.parseGoMod(filepath.Join(moduleDir, "go.mod"))
			if errMod == nil && modFile != nil && modFile.Module != nil {
				modulePath = modFile.Module.Mod.Path
				relativePathFromModuleRoot, errRel := filepath.Rel(moduleDir, pkgDir)
				if errRel == nil {
					if relativePathFromModuleRoot == "." {
						importPath = modulePath
					} else if !strings.HasPrefix(relativePathFromModuleRoot, "..") { // Ensure it's not outside
						importPath = filepath.ToSlash(filepath.Join(modulePath, relativePathFromModuleRoot))
					}
				}
			}
		}
		meta := PackageMetaInfo{
			ImportPath:    importPath, Name: pkgName, Dir: pkgDir,
			GoFiles:      goFiles,
			GoFiles: goFiles, TestGoFiles: testGoFiles, XTestGoFiles: xTestGoFiles,
			ModulePath: modulePath, ModuleDir: moduleDir,
			DirectImports: []string{},
		}
		// Ensure slices are non-nil (already done by listGoFiles for xTestGoFiles, and by var init for others if empty)
		// but DirectImports and others if they were nil conceptually.
		if meta.GoFiles == nil { meta.GoFiles = []string{} }
		if meta.TestGoFiles == nil { meta.TestGoFiles = []string{} }
		if meta.XTestGoFiles == nil { meta.XTestGoFiles = []string{} }
		if meta.DirectImports == nil { meta.DirectImports = []string{} }
		return []PackageMetaInfo{meta}, nil
	}

	currentModuleRoot, err := gml.findModuleRoot(gml.workingDir) // findModuleRoot returns absolute path
	var currentModFile *modfile.File
	var currentModulePath string

	if err == nil && currentModuleRoot != "" {
		currentGoModPath := filepath.Join(currentModuleRoot, "go.mod")
		currentModFile, err = gml.parseGoMod(currentGoModPath) // Use existing err variable
		if err == nil && currentModFile != nil && currentModFile.Module != nil {
			currentModulePath = currentModFile.Module.Mod.Path

			if strings.HasPrefix(pattern, currentModulePath) {
				relativePathInModule := strings.TrimPrefix(pattern, currentModulePath)
				relativePathInModule = strings.TrimPrefix(relativePathInModule, "/")
				pkgDir := filepath.Join(currentModuleRoot, relativePathInModule)
				// pkgDir is absolute because currentModuleRoot is absolute

				if stat, statErr := os.Stat(pkgDir); statErr == nil && stat.IsDir() {
					pkgName := filepath.Base(pkgDir)
					goFiles, testGoFiles, xTestGoFiles, listErr := gml.listGoFiles(pkgDir)

					if goFiles == nil { goFiles = []string{} }
					if testGoFiles == nil { testGoFiles = []string{} }
					if xTestGoFiles == nil { xTestGoFiles = []string{} }

					if listErr == nil && (len(goFiles) > 0 || len(testGoFiles) > 0 || len(xTestGoFiles) > 0) {
						meta := PackageMetaInfo{
							ImportPath: pattern, Name: pkgName, Dir: pkgDir,
							GoFiles: goFiles, TestGoFiles: testGoFiles, XTestGoFiles: xTestGoFiles,
							ModulePath: currentModulePath, ModuleDir: currentModuleRoot,
							DirectImports: []string{},
						}
						return []PackageMetaInfo{meta}, nil
					}
				}
			}
		}
	}

	// --- Start: External dependency handling ---
	if currentModFile != nil { // currentModFile might be nil if parsing failed or not in a module
		var depModPath, depModVersion, depModDirPrefix string

		// Find the longest matching prefix among requirements
		for _, req := range currentModFile.Require {
			if req.Mod.Path == "" { continue } // Should not happen with valid go.mod
			if strings.HasPrefix(pattern, req.Mod.Path) {
				// Check if this is a longer (more specific) match
				if len(req.Mod.Path) > len(depModPath) {
					depModPath = req.Mod.Path
					depModVersion = req.Mod.Version
				}
			}
		}

		if depModPath != "" {
			goModCache, cacheErr := getGoModCacheDir() // getGoModCacheDir returns absolute path
			if cacheErr != nil {
				return nil, fmt.Errorf("GoModLocator: failed to get module cache directory: %w", cacheErr)
			}

			// module.EscapePath deals with replacing uppercase letters with !lower case
			escapedModPath, escErr := module.EscapePath(depModPath)
			if escErr != nil {
				return nil, fmt.Errorf("GoModLocator: failed to escape module path %s: %w", depModPath, escErr)
			}
			// Path in cache: $GOMODCACHE/escapedModPath@version/subPath
			depModDirPrefix = filepath.Join(goModCache, escapedModPath + "@" + depModVersion) // depModDirPrefix is absolute

			subPath := strings.TrimPrefix(pattern, depModPath)
			subPath = strings.TrimPrefix(subPath, "/")
			pkgDir := filepath.Join(depModDirPrefix, subPath) // pkgDir is absolute

			if stat, statErr := os.Stat(pkgDir); statErr == nil && stat.IsDir() {
				pkgName := filepath.Base(pkgDir)
				goFiles, testGoFiles, xTestGoFiles, listErr := gml.listGoFiles(pkgDir)

				if goFiles == nil { goFiles = []string{} }
				if testGoFiles == nil { testGoFiles = []string{} }
				if xTestGoFiles == nil { xTestGoFiles = []string{} }

				if listErr == nil && (len(goFiles) > 0 || len(testGoFiles) > 0 || len(xTestGoFiles) > 0) {
					meta := PackageMetaInfo{
						ImportPath:    pattern, Name: pkgName, Dir: pkgDir,
						GoFiles:       goFiles, TestGoFiles:   testGoFiles, XTestGoFiles:  xTestGoFiles,
						ModulePath:    depModPath,     // The module path of the dependency
						ModuleDir:     depModDirPrefix, // The root directory of the dependency in the cache
						DirectImports: []string{},
					}
					return []PackageMetaInfo{meta}, nil
				}
			}
		}
	}
	// --- End: External dependency handling ---

	return nil, fmt.Errorf("GoModLocator: package %q not found by any method (relative, in-module, or go.mod dependency)", pattern)
}

// parseGoMod reads and parses a go.mod file.
func (gml *GoModLocator) parseGoMod(modFilePath string) (*modfile.File, error) {
	data, err := os.ReadFile(modFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod file %s: %w", modFilePath, err)
	}
	modFile, err := modfile.Parse(modFilePath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod file %s: %w", modFilePath, err)
	}
	return modFile, nil
}

// findModuleRoot searches for the go.mod file starting from startDir and going upwards.
// It returns the directory containing the go.mod file, or an error if not found.
func (gml *GoModLocator) findModuleRoot(startDir string) (string, error) {
	dir := startDir
	if !filepath.IsAbs(dir) {
		absDir, err := filepath.Abs(filepath.Join(gml.workingDir, dir))
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path for %s: %w", dir, err)
		}
		dir = absDir
	}

	for {
		modFilePath := filepath.Join(dir, "go.mod")
		info, err := os.Stat(modFilePath)
		if err == nil && !info.IsDir() {
			return dir, nil // Found go.mod
		}
		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			// Reached the root directory without finding go.mod
			return "", errors.New("go.mod not found in or above " + startDir)
		}
		dir = parentDir
	}
}

// listGoFiles scans a directory and categorizes Go files.
// It returns regular Go files, in-package test files, and external test files.
// File paths are relative to the provided dirPath.
func (gml *GoModLocator) listGoFiles(dirPath string) (goFiles, testGoFiles, xTestGoFiles []string, err error) {
	absDirPath := dirPath
	if !filepath.IsAbs(dirPath) {
		absDirPath = filepath.Join(gml.workingDir, dirPath)
	}

	entries, err := os.ReadDir(absDirPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read directory %s: %w", absDirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".go") {
			if strings.HasSuffix(name, "_test.go") {
				// Further check if it's an external test file by trying to parse package declaration
				// This is a simplified check. A more robust check would parse the file.
				// For now, we assume all _test.go files are TestGoFiles unless they have a specific build tag `ignore`.
				// Proper XTestGoFiles detection requires parsing and checking package name.
				// We'll defer full XTestGoFiles discrimination for now or simplify.
				// A common convention for XTestGoFiles is that they have `_test` package name.
				// Reading the package name is too complex for this helper at this stage.
				// Let's assume _test.go files in the main package are TestGoFiles,
				// and those with a different package (e.g. foo_test) are XTestGoFiles.
				// This heuristic isn't perfect. `go list` does more sophisticated checks.

				// For now, let's put all _test.go files into TestGoFiles.
				// Proper XTestGoFile identification can be added later if critical.
				testGoFiles = append(testGoFiles, name)
			} else {
				goFiles = append(goFiles, name)
			}
		}
	}
	// XTestGoFiles are typically in the same directory but have a package name ending in `_test`.
	// This simple listGoFiles won't distinguish them accurately without parsing.
	// For now, XTestGoFiles will be empty. This is a simplification.
	return goFiles, testGoFiles, []string{}, nil
}

// GoListLocator is a PackageLocator that uses `go list` command.
func GoListLocator(pattern string, buildCtx BuildContext) ([]PackageMetaInfo, error) {
	args := []string{"list", "-json"}
	if len(buildCtx.BuildTags) > 0 {
		args = append(args, "-tags", strings.Join(buildCtx.BuildTags, ","))
	}
	args = append(args, pattern)

	cmd := exec.Command("go", args...)
	if buildCtx.GOOS != "" {
		cmd.Env = append(cmd.Environ(), "GOOS="+buildCtx.GOOS)
	}
	if buildCtx.GOARCH != "" {
		cmd.Env = append(cmd.Environ(), "GOARCH="+buildCtx.GOARCH)
	}
	// TODO: Consider GOPATH, GOMODCACHE, etc. if not running in a module-aware dir.
	// For module mode, `go list` typically works well from within the module or by specifying full import paths.

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("`go list %s` failed: %w (stderr: %s)", pattern, err, stderr.String())
	}

	var results []PackageMetaInfo
	// `go list -json` outputs a stream of JSON objects for multiple packages, or a single one.
	// We need to handle this by decoding object by object.
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var meta struct { // Structure matching `go list -json` output
			ImportPath   string
			Name         string
			Dir          string
			GoFiles      []string
			TestGoFiles  []string
			XTestGoFiles []string
			Imports      []string // Direct imports
			Module       *struct {
				Path string
				Dir  string
			}
			Error *struct { // go list uses a nested struct for errors
				Err string
			}
		}
		if err := decoder.Decode(&meta); err != nil {
			return nil, fmt.Errorf("failed to decode `go list -json` output: %w", err)
		}

		pmMeta := PackageMetaInfo{
			ImportPath:    meta.ImportPath,
			Name:          meta.Name,
			Dir:           meta.Dir,
			GoFiles:       meta.GoFiles,
			TestGoFiles:   meta.TestGoFiles,
			XTestGoFiles:  meta.XTestGoFiles,
			DirectImports: meta.Imports,
		}
		if meta.Module != nil {
			pmMeta.ModulePath = meta.Module.Path
			pmMeta.ModuleDir = meta.Module.Dir
		}
		if meta.Error != nil {
			pmMeta.Error = meta.Error.Err
		}
		results = append(results, pmMeta)
	}

	return results, nil
}
