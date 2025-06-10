package lazyload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os" // Added for os.Environ()
	"os/exec"
	"strings"
)

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
// The baseDir specifies the working directory for the locator.
type PackageLocator func(pattern string, baseDir string, buildCtx BuildContext) ([]PackageMetaInfo, error)

// GoListLocator is a PackageLocator that uses `go list` command.
func GoListLocator(pattern string, baseDir string, buildCtx BuildContext) ([]PackageMetaInfo, error) {
	args := []string{"list", "-json"}
	if len(buildCtx.BuildTags) > 0 {
		args = append(args, "-tags", strings.Join(buildCtx.BuildTags, ","))
	}
	args = append(args, pattern)

	cmd := exec.Command("go", args...)
	if baseDir != "" {
		cmd.Dir = baseDir
	}

	// Build the environment for the command
	env := os.Environ()
	cleanEnv := make([]string, 0, len(env)+2) // +2 for GOWORK and GO111MODULE
	for _, e := range env {
		if !strings.HasPrefix(e, "GOPATH=") {
			cleanEnv = append(cleanEnv, e)
		}
	}
	cleanEnv = append(cleanEnv, "GOWORK=off")
	cleanEnv = append(cleanEnv, "GO111MODULE=on")

	if buildCtx.GOOS != "" {
		cleanEnv = append(cleanEnv, "GOOS="+buildCtx.GOOS)
	}
	if buildCtx.GOARCH != "" {
		cleanEnv = append(cleanEnv, "GOARCH="+buildCtx.GOARCH)
	}
	cmd.Env = cleanEnv
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
