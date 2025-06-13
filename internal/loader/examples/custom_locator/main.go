package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/podhmo/goat/internal/loader"
)

var tempDirsCreated []string // To clean up at the end

func main() {
	defer func() {
		for _, dir := range tempDirsCreated {
			os.RemoveAll(dir)
		}
	}()

	// Create a dummy base directory that myCustomLocator will use.
	// Note: myCustomLocator itself creates temp dirs. This logic is a bit tangled for a simple example.
	// A better custom locator might take a base path from Config or BuildContext.
	// For this example, we'll let myCustomLocator handle its temp dir creation and just call it.
	// The key is that `Dir` in `PackageMetaInfo` must be valid when `Package.ensureParsed` is called.

	fmt.Println("Running custom locator example...")
	ctx := context.Background() // Create a top-level context
	cfg := loader.Config{
		Locator: myCustomLocator,
		Context: loader.BuildContext{
			GOOS:      "custom_os", // Just an example context property
			BuildTags: []string{"custom_tag"},
		},
	}
	loaderInst := loader.New(cfg) // Renamed variable to avoid conflict if 'loader' is package name

	// Use a pattern that our custom locator understands
	pkgs, err := loaderInst.Load(ctx, "custom/pkg/one")
	if err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("Failed to load packages with custom locator: %v", err))
		os.Exit(1)
	}

	if len(pkgs) == 0 {
		slog.ErrorContext(ctx, fmt.Sprintf("Custom locator returned no packages for 'custom/pkg/one'"), "error", errors.New(fmt.Sprintf("Custom locator returned no packages for 'custom/pkg/one'")))
		os.Exit(1)
	}

	pkgOne := pkgs[0]
	// Store the temp dir used by the locator for cleanup
	// This is a bit of a hack for the example; a real locator would manage paths better.
	if pkgOne.Dir != "" {
		// The actual temp dir is one level up from "custom/pkg/one" in the mock locator
		tempDirsCreated = append(tempDirsCreated, filepath.Dir(filepath.Dir(filepath.Dir(pkgOne.Dir))))
	}

	fmt.Printf("Package loaded via custom locator: %s (ImportPath: %s, Dir: %s)\n", pkgOne.Name, pkgOne.ImportPath, pkgOne.Dir)

	// Attempt to get a struct. This will trigger parsing of the dummy file.
	structInfo, err := pkgOne.GetStruct("CustomStruct")
	if err != nil {
		slog.ErrorContext(context.Background(), fmt.Sprintf("Failed to get CustomStruct from %s: %v", pkgOne.ImportPath, err))
		os.Exit(1)
	}

	fmt.Printf("Found struct: %s\n", structInfo.Name)
	for _, field := range structInfo.Fields {
		fmt.Printf("  Field: %s, Tag: `%s` (tag key 'tag': %q)\n",
			field.Name,
			field.Tag,
			field.GetTag("tag"),
		)
	}

	// Example of resolving an import if CustomStruct had a field from "another/pkg"
	// For this, "custom/pkg/one/one.go" would need an import like:
	// import _ "another/pkg"
	// And field like: Foreign another.AnotherType
	// For now, let's try to resolve it directly if declared in PackageMetaInfo.DirectImports
	if len(pkgOne.RawMeta.DirectImports) > 0 {
		importToResolve := pkgOne.RawMeta.DirectImports[0]
		fmt.Printf("Attempting to resolve direct import: %s\n", importToResolve)
		resolvedImport, err := pkgOne.ResolveImport(ctx, importToResolve)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("Failed to resolve import %s: %v", importToResolve, err))
			os.Exit(1)
		}
		fmt.Printf("Successfully resolved imported package: %s (Dir: %s)\n", resolvedImport.Name, resolvedImport.Dir)
		// Store its temp dir too if it's different
		if resolvedImport.Dir != "" {
			importTempBase := filepath.Dir(filepath.Dir(resolvedImport.Dir))
			isNewTempDir := true
			for _, knownDir := range tempDirsCreated {
				if knownDir == importTempBase {
					isNewTempDir = false
					break
				}
			}
			if isNewTempDir {
				tempDirsCreated = append(tempDirsCreated, importTempBase)
			}
		}
	}

	fmt.Println("\nCustom locator example finished successfully.")
}

// myCustomLocator is a mock locator for demonstration.
// In a real scenario, this would interact with a custom build system,
// read a proprietary project manifest, or scan a non-standard directory structure.
func myCustomLocator(ctx context.Context, pattern string, buildCtx loader.BuildContext) ([]loader.PackageMetaInfo, error) {
	fmt.Printf("CustomLocator called with pattern: %q, BuildContext: %+v, ContextParam: %v\n", pattern, buildCtx, ctx)

	// This locator only "knows" about a fake package "custom/pkg/one".
	// It assumes a flat file structure in a predefined base directory.
	// For simplicity, we'll use a temporary directory for this example.

	baseDir, err := os.MkdirTemp("", "custom-locator-pkgs-")
	if err != nil {
		return nil, fmt.Errorf("custom_locator: failed to create temp dir: %w", err)
	}
	// We'll clean this up later, but in a real test, manage temp dirs carefully.
	// defer os.RemoveAll(baseDir) // This defer won't work as baseDir is local to this call.
	// For this example, the main function will clean up. This is just for demonstration.
	// This temporary directory needs to exist when ParseFile is called by the Package object.
	// So, the actual file creation should happen here, or the path should be predictable.
	// Let's create dummy files for "custom/pkg/one".
	pkgOneDir := filepath.Join(baseDir, "custom", "pkg", "one")
	if err := os.MkdirAll(pkgOneDir, 0755); err != nil {
		return nil, err
	}
	dummyGoFileContent := `package one
type CustomStruct struct { Message string ` + "`tag:\"message_tag\"`" + ` }
// Import "another/pkg" here if you want to test import resolution
// import _ "another/pkg"
`
	if err := os.WriteFile(filepath.Join(pkgOneDir, "one.go"), []byte(dummyGoFileContent), 0644); err != nil {
		return nil, err
	}

	if pattern == "custom/pkg/one" || pattern == "./..." && buildCtx.GOOS == "custom_os" { // Example specific condition
		return []loader.PackageMetaInfo{
			{
				ImportPath:    "custom/pkg/one",
				Name:          "one",                   // Package name
				Dir:           pkgOneDir,               // Absolute path to the package directory
				GoFiles:       []string{"one.go"},      // File names relative to Dir
				DirectImports: []string{"another/pkg"}, // Example direct import
				// ModulePath, ModuleDir can be set if applicable
			},
		}, nil
	}
	if pattern == "another/pkg" { // So ResolveImport can find it
		anotherPkgDir := filepath.Join(baseDir, "another", "pkg")
		if err := os.MkdirAll(anotherPkgDir, 0755); err != nil {
			return nil, err
		}
		anotherGoFileContent := `package pkg; type AnotherType struct{ Val int }`
		if err := os.WriteFile(filepath.Join(anotherPkgDir, "another.go"), []byte(anotherGoFileContent), 0644); err != nil {
			return nil, err
		}
		return []loader.PackageMetaInfo{
			{
				ImportPath: "another/pkg",
				Name:       "pkg",
				Dir:        anotherPkgDir,
				GoFiles:    []string{"another.go"},
			},
		}, nil
	}

	// For other patterns, return "not found" or an empty list.
	return nil, nil // Or return a specific error like loader.PackageNotFoundError
}
