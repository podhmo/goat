package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/podhmo/goat/internal/loader/lazyload"
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
	cfg := lazyload.Config{
		Locator: myCustomLocator,
		Context: lazyload.BuildContext{
			GOOS:      "custom_os", // Just an example context property
			BuildTags: []string{"custom_tag"},
		},
	}
	loader := lazyload.NewLoader(cfg)

	// Use a pattern that our custom locator understands
	pkgs, err := loader.Load("custom/pkg/one")
	if err != nil {
		log.Fatalf("Failed to load packages with custom locator: %v", err)
	}

	if len(pkgs) == 0 {
		log.Fatalf("Custom locator returned no packages for 'custom/pkg/one'")
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
		log.Fatalf("Failed to get CustomStruct from %s: %v", pkgOne.ImportPath, err)
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
		resolvedImport, err := pkgOne.ResolveImport(importToResolve)
		if err != nil {
			log.Fatalf("Failed to resolve import %s: %v", importToResolve, err)
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
func myCustomLocator(pattern string, locatorBaseDir string, buildCtx lazyload.BuildContext) ([]lazyload.PackageMetaInfo, error) {
	// locatorBaseDir is the directory context provided by the loader.
	// This mock locator creates its own isolated temp environment for simplicity,
	// so it doesn't strictly use locatorBaseDir but logs it.
	fmt.Printf("CustomLocator called with pattern: %q, locatorBaseDir: %q, BuildContext: %+v\n", pattern, locatorBaseDir, buildCtx)

	// This locator only "knows" about a fake package "custom/pkg/one".
	// It creates its own temporary base directory for this example.
	baseDirForLocator, err := os.MkdirTemp("", "custom-locator-pkgs-")
	if err != nil {
		return nil, fmt.Errorf("custom_locator: failed to create temp dir: %w", err)
	}
	// The main function will manage cleanup of tempDirsCreated based on Package.Dir.
	// This temp dir (baseDirForLocator) needs to be added to tempDirsCreated in main IF a package from it is returned and used.
	// However, the logic in main currently derives the cleanup path from pkgOne.Dir.
	// Ensure that pkgOne.Dir is set correctly relative to what main expects for cleanup.

	// Let's create dummy files for "custom/pkg/one" inside baseDirForLocator.
	pkgOneDir := filepath.Join(baseDirForLocator, "custom", "pkg", "one")
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
		return []lazyload.PackageMetaInfo{
			{
				ImportPath:    "custom/pkg/one",
				Name:          "one",                   // Package name
				Dir:           pkgOneDir,               // Absolute path to the package directory
				GoFiles:       []string{"one.go"},      // File names relative to Dir
				DirectImports: []string{"another/pkg"}, // Example direct import
				// ModulePath, ModuleDir can be set if applicable, relative to baseDirForLocator
				ModuleDir: baseDirForLocator, // Example of setting module dir
			},
		}, nil
	}
	if pattern == "another/pkg" { // So ResolveImport can find it
		anotherPkgDir := filepath.Join(baseDirForLocator, "another", "pkg")
		if err := os.MkdirAll(anotherPkgDir, 0755); err != nil {
			return nil, err
		}
		anotherGoFileContent := `package pkg; type AnotherType struct{ Val int }`
		if err := os.WriteFile(filepath.Join(anotherPkgDir, "another.go"), []byte(anotherGoFileContent), 0644); err != nil {
			return nil, err
		}
		return []lazyload.PackageMetaInfo{
			{
				ImportPath: "another/pkg",
				Name:       "pkg",
				Dir:        anotherPkgDir,
				GoFiles:    []string{"another.go"},
			},
		}, nil
	}

	// For other patterns, return "not found" or an empty list.
	return nil, nil // Or return a specific error like lazyload.PackageNotFoundError
}
