# Go Lazy Package Loader

`loader` is a Go library designed to inspect Go source code, similar to `go/packages`, but with a focus on **lazy loading** of package information and ASTs. This approach can be beneficial for tools that only need to inspect a small subset of a large codebase or want to minimize initial loading time.

**Core Principles:**

*   **Lazy AST Parsing**: ASTs for `.go` files are parsed only when explicitly requested for a package (e.g., when analyzing its structs or symbols).
*   **On-Demand Dependency Resolution**: When analyzing a package (e.g., a struct field referencing a type from another package `foo.Bar`), the package `foo` is resolved and loaded only at that moment, not upfront.
*   **Efficient Caching**: The loader implements multiple levels of caching to improve performance:
    *   **Package Cache**: Caches loaded `loader.Package` objects by import path.
    *   **File AST Cache**: Caches parsed `ast.File` objects by their absolute file paths, reducing redundant parsing.
    *   **Symbol Cache**: Caches `loader.SymbolInfo` (including the declaration node and file path) for quick lookups of symbols across loaded packages.
*   **AST-Focused (No Full Type Checking by Default)**: This library primarily provides AST-level information (syntax, struct tags, import declarations, symbol declarations) and does not perform full Go type checking by default. This simplifies initial loading and reduces overhead when complete type information is not strictly necessary.
*   **Flexible Package Location**:
    *   **Default `GoModLocator`**: By default, the loader uses `GoModLocator` which resolves packages by inspecting `go.mod` files and the module cache, without needing the `go list` command. It can locate packages within the current module and its dependencies.
    *   **Customizable**: The loader can be configured with a custom `PackageLocator` function (or use the provided `GoListLocator` which wraps `go list`) to support different build systems or package discovery mechanisms.

## Features

*   **Flexible Package Loading**: Load Go packages based on patterns (e.g., `./...`, `example.com/mymodule/pkg`) using configurable locators (`GoModLocator` by default).
*   **Rich Package Metadata**: Access comprehensive package metadata (`PackageMetaInfo`) including import path, directory, lists of Go files (`GoFiles`, `TestGoFiles`, `XTestGoFiles`), direct imports, and module information (path and directory).
*   **Lazy AST Parsing**: Parses Go source files into ASTs (`ast.File`) only when their content is needed (e.g., for struct inspection or symbol resolution), minimizing upfront work.
*   **On-Demand Dependency Resolution**: Resolve imported packages lazily when accessed via `Package.ResolveImport()`.
*   **Efficient Caching**: Utilizes caches for packages, file ASTs, and symbols to speed up repeated access and analysis.
*   **AST-Level Inspection**:
    *   Inspect struct definitions (`StructInfo`), including field names, Go struct tags, and AST-level type expressions (`FieldInfo`).
    *   Look up symbol declarations (`SymbolInfo`) across all loaded packages, providing access to the symbol's name, defining package, file path, and declaration AST node (`ast.Node`).
    *   Access methods for types and perform basic interface implementation checks (AST-based).
*   **(Future)** Extensible for more advanced AST queries and code analysis tasks.

## Basic Usage

This example demonstrates how to use the loader to load packages, inspect structs, resolve types from imported packages, and look up symbols.

**Note**: For this example to be conceptually runnable, assume you have a Go module (e.g., initialized with `go mod init example.com/mymodule`) with the following structure:

```
/mymodule/
  go.mod
  main.go         // Where you'd run this example code
  pkgA/
    a.go          // Defines package pkgA
  pkgB/
    b.go          // Defines package pkgB, imports pkgA
```

Content of `pkgA/a.go`:
```go
package pkgA

type Info struct {
	Message string
}
```

Content of `pkgB/b.go`:
```go
package pkgB

import (
	"example.com/mymodule/pkgA" // Assumes module path is example.com/mymodule
)

type Data struct {
	Name     string      `json:"name"`
	Details  pkgA.Info   `json:"details"`
	Age      int         `json:"age,omitempty"`
}

func (d *Data) GetName() string {
	return d.Name
}
```

Now, the `main.go` example:

```go
package main

import (
	"context"
	"fmt"
	"go/ast"
	"log"
	"path/filepath" // Used for printing file paths cleanly

	// Replace with the actual import path to *your* loader package
	"example.com/mymodule/internal/loader"
)

func main() {
	ctx := context.Background()

	// Configure the loader.
	// By default, New() uses GoModLocator if Config.Locator is nil.
	// GoModLocator typically requires UseGoModule: true in BuildContext
	// and that the current working directory is inside a Go module.
	cfg := loader.Config{
		Context: loader.BuildContext{
			UseGoModule: true,
			// GOOS: "linux", GOARCH: "amd64", BuildTags: []string{"mytag"}, // Optional settings
		},
	}
	loaderInst := loader.New(cfg)

	// Load pkgB. Since pkgB imports pkgA, pkgA might also be processed
	// or fully loaded depending on subsequent operations.
	// Adjust "example.com/mymodule/pkgB" to your actual module path + /pkgB.
	pkgs, err := loaderInst.Load(ctx, "example.com/mymodule/pkgB")
	if err != nil {
		log.Fatalf("Failed to load packages: %v", err)
	}

	for _, pkg := range pkgs {
		fmt.Printf("Loaded Package: %s (%s)\n", pkg.Name, pkg.ImportPath)
		fmt.Printf("  Directory: %s\n", pkg.Dir)
		if len(pkg.GoFiles) > 0 {
			fmt.Printf("  Primary GoFile: %s\n", pkg.GoFiles[0])
		}

		// Get a struct from pkgB
		dataStruct, err := pkg.GetStruct("Data")
		if err != nil {
			log.Printf("  Error getting struct Data: %v\n", err)
			continue
		}
		fmt.Printf("  Found struct: %s\n", dataStruct.Name)

		for _, field := range dataStruct.Fields {
			fmt.Printf("    Field: %s, AST Type: %T, Tag: '%s'\n", field.Name, field.TypeExpr, field.Tag)

			// If field type is a selector expression (e.g., pkgA.Info)
			if selExpr, ok := field.TypeExpr.(*ast.SelectorExpr); ok {
				pkgAliasIdent, _ := selExpr.X.(*ast.Ident) // This is the package alias, e.g., "pkgA"
				typeName := selExpr.Sel.Name              // This is the type name, e.g., "Info"
				fmt.Printf("      Field '%s' is a selector: %s.%s\n", field.Name, pkgAliasIdent.Name, typeName)

				// To resolve where pkgAliasIdent.Name (e.g., "pkgA") comes from,
				// we need the AST of the file containing the 'Data' struct.
				// Calling pkg.GetStruct("Data") already parsed pkgB's files.
				// We need to find which file AST contains the definition of 'Data'.
				// For simplicity, let's assume it's the first file.
				// A robust solution would iterate pkg.Files() and inspect ASTs.
				var definingFileAST *ast.File
				if len(pkg.GoFiles) > 0 {
					allFiles, _ := pkg.Files() // ensure all files in pkg are parsed
					if fAST, ok := allFiles[pkg.GoFiles[0]]; ok { // get AST for the first file
						definingFileAST = fAST
					}
				}

				if definingFileAST == nil {
					log.Printf("        Could not get AST for defining file of struct %s\n", dataStruct.Name);
					continue
				}

				// Use GetImportPathBySelector to find the full import path for the alias "pkgA"
				importedPkgPath, resolvedImportedPkg, err := pkg.GetImportPathBySelector(ctx, pkgAliasIdent.Name, definingFileAST)
				if err != nil {
					log.Printf("        Could not resolve import for alias %s: %v\n", pkgAliasIdent.Name, err)
				} else {
					fmt.Printf("        Alias '%s' in struct '%s' resolves to import path: %s\n", pkgAliasIdent.Name, dataStruct.Name, importedPkgPath)

					// Now 'resolvedImportedPkg' is the loader.Package for "example.com/mymodule/pkgA".
					// We can inspect it, for example, to get the 'Info' struct.
					infoStruct, err := resolvedImportedPkg.GetStruct(typeName)
					if err != nil {
						log.Printf("          Could not get struct '%s' from package '%s': %v\n", typeName, resolvedImportedPkg.ImportPath, err)
					} else {
						fmt.Printf("          Successfully resolved and found struct '%s' in package '%s'\n", infoStruct.Name, resolvedImportedPkg.ImportPath)
						for _, infoField := range infoStruct.Fields {
							fmt.Printf("            Field in %s: %s of type %T\n", infoStruct.Name, infoField.Name, infoField.TypeExpr)
						}
					}
				}
			}
		}
		fmt.Println("  ---")

		// Example: Look up a symbol (e.g., the 'Data' type or its method 'GetName')
		// Symbol cache is populated by pkg.ensureParsed(), which is called by pkg.GetStruct() or pkg.Files().

		// Looking up the 'Data' type symbol
		fullSymbolNameData := pkg.ImportPath + ":" + "Data"
		symInfoData, foundData := loaderInst.LookupSymbol(fullSymbolNameData)
		if foundData {
			fmt.Printf("  Found symbol '%s' in package '%s' (File: %s)\n", symInfoData.SymbolName, symInfoData.PackagePath, filepath.Base(symInfoData.FilePath))
		} else {
			fmt.Printf("  Symbol '%s' not found.\n", fullSymbolNameData)
		}

		// Looking up the 'GetName' method symbol associated with 'Data'
		fullSymbolNameGetName := pkg.ImportPath + ":" + "GetName"
		symInfoGetName, foundGetName := loaderInst.LookupSymbol(fullSymbolNameGetName)
		if foundGetName {
			// Note: For methods, symInfo.Node would be an *ast.FuncDecl.
			fmt.Printf("  Found symbol '%s' in package '%s' (File: %s)\n", symInfoGetName.SymbolName, symInfoGetName.PackagePath, filepath.Base(symInfoGetName.FilePath))
		} else {
			fmt.Printf("  Symbol '%s' not found.\n", fullSymbolNameGetName)
		}
	}
}

```
The example above assumes your project's module path is `example.com/mymodule`. **You will need to replace `example.com/mymodule` with your actual module path in the import statements within the Go code blocks for it to be relevant.** The `loader` package itself is also assumed to be at `example.com/mymodule/internal/loader`.

## Package Location

The loader offers flexible ways to locate packages:

### Default: `GoModLocator`
By default, if no `Locator` is specified in `loader.Config`, the `Loader` uses `GoModLocator`.
*   **Strategy**: `GoModLocator` resolves package import paths without invoking the `go list` command. It works by:
    1.  Analyzing the `go.mod` file of the current module (if `BuildContext.UseGoModule` is true and the loader is operating within a module).
    2.  Resolving relative paths (e.g., `./mypkg`) based on the current working directory.
    3.  Locating packages belonging to the current module by combining the module path with the relative path within the module.
    4.  Finding dependencies listed in the `go.mod` file by searching the Go module cache (typically `$GOMODCACHE` or `$GOPATH/pkg/mod`).
*   **Configuration**: For `GoModLocator` to work effectively, especially for resolving non-relative import paths, ensure `BuildContext.UseGoModule` is set to `true` in `loader.Config.Context`, and the loader is initialized from a working directory within your Go module.

### Alternative: `GoListLocator`
The loader also provides `GoListLocator`, which uses the `go list -json` command to find packages. You can configure the loader to use it like so:
```go
// import "example.com/path/to/loader" // Your loader import path

// In your setup:
cfg.Locator = loader.GoListLocator // Use the provided GoListLocator
loaderInst := loader.New(cfg)
// Now loaderInst.Load() will use 'go list'
```
This can be useful if you prefer `go list`'s resolution behavior or are working outside of a module context where `GoModLocator` might be less effective.

### Custom Package Locators
For more advanced scenarios, such as integrating with different build systems or proprietary package management, you can provide a custom `PackageLocator` function in the `loader.Config`.

A `PackageLocator` function has the following signature:
```go
type PackageLocator func(ctx context.Context, pattern string, buildCtx loader.BuildContext) ([]loader.PackageMetaInfo, error)
```
It takes a pattern (like an import path or a relative path) and a `BuildContext`, and should return a slice of `PackageMetaInfo` structs or an error.

```go
// Example of a skeleton for a custom locator
func myCustomLocator(ctx context.Context, pattern string, buildCtx loader.BuildContext) ([]loader.PackageMetaInfo, error) {
    // Your logic to find packages based on the pattern and buildCtx.
    // This might involve:
    // - Reading custom build files.
    // - Querying a version control system.
    // - Interacting with a proprietary artifact repository.

    // Example: if pattern is "my.special.pkg/foo"
    if pattern == "my.special.pkg/foo" {
        // You'd determine the directory, Go files, package name, and direct imports.
        return []loader.PackageMetaInfo{
            {
                ImportPath:    "my.special.pkg/foo",
                Name:          "foo", // Determined by parsing a file or from metadata
                Dir:           "/path/to/custom/pkg/foo", // Absolute path
                GoFiles:       []string{"foo.go", "bar.go"}, // Relative to Dir
                // TestGoFiles, XTestGoFiles can also be populated
                DirectImports: []string{"standardlib/fmt", "my.special.pkg/bar"}, // Canonical import paths
                // ModulePath and ModuleDir if applicable
            },
        }, nil
    }
    return nil, fmt.Errorf("package %q not found by custom locator", pattern)
}

// ... in main or setup code:
// cfg.Locator = myCustomLocator
// loaderInst := loader.New(cfg)
```
When implementing a custom locator, ensure that `PackageMetaInfo.Dir` is an absolute path and `GoFiles` (and other file lists) are relative to `Dir`. The `ImportPath` should be the canonical import path for the package.

## How Lazy Resolution Works

The loader is designed to minimize upfront work and only process what's necessary. Here's a breakdown of the typical lazy resolution flow:

1.  **`loaderInst.Load(ctx, patterns...)`**:
    *   The configured `PackageLocator` (e.g., `GoModLocator` by default) is invoked for each pattern.
    *   The locator identifies matching packages and returns their metadata (`PackageMetaInfo` structs). This metadata typically includes the import path, directory, list of Go files (`GoFiles`, `TestGoFiles`, `XTestGoFiles`), direct imports, and module information.
    *   For each unique package identified, a `loader.Package` object is created and stored in the `Loader`'s **package cache** (keyed by import path). At this stage, no `.go` files are parsed. The `Package` object holds its `RawMeta` (the `PackageMetaInfo` from the locator) and references to its `GoFiles`.

2.  **Accessing Package Content (e.g., `pkg.GetStruct("MyType")`, `pkg.Files()`, or looking up a symbol from this package)**:
    *   This triggers the `pkg.ensureParsed()` method (called internally if not already done for the package).
    *   **`pkg.ensureParsed()`**:
        1.  Iterates through the `GoFiles` of the `pkg`.
        2.  For each Go file:
            *   It first checks the `Loader`'s global **file AST cache** (`loader.fileASTCache`, keyed by absolute file path). If the AST is found, it's reused.
            *   If not in the cache, the file is parsed using `go/parser` (with the `Loader`'s `token.FileSet`). The resulting `*ast.File` is stored in `pkg.parsedFiles` (keyed by relative file path) AND added to the `Loader`'s global **file AST cache**.
            *   Import declarations (`ast.ImportSpec`) from the file are collected into `pkg.fileImports`.
        3.  After parsing all files in the package:
            *   If the package name (`pkg.Name`) wasn't available from the locator, it's derived from the parsed ASTs.
            *   The `Loader`'s global **symbol cache** (`loader.symbolCache`) is populated. For each top-level declaration (functions, types, vars, consts) in each parsed file of the package, a `loader.SymbolInfo` object is created. This `SymbolInfo` (containing the symbol's name, its package import path, the absolute path to its defining file, and the `ast.Node` of its declaration) is stored in the `symbolCache` (keyed by `<package_import_path>:<symbol_name>`).
    *   Once parsed, `pkg.GetStruct("MyType")` would then traverse the relevant `*ast.File` in `pkg.parsedFiles` to find the `MyType` struct definition.

3.  **Resolving an Imported Package (e.g., a field `F OtherPkg.OtherType` in `MyType`)**:
    *   When inspecting `MyType`, you find a field `F` whose type is an `*ast.SelectorExpr` (representing `OtherPkg.OtherType`).
    *   To understand what `OtherPkg` refers to, you first need to find its full import path:
        *   This usually involves looking at the `import` declarations in the `*ast.File` where `MyType` (and thus field `F`) is defined. The `pkg.fileImports` map (populated during `ensureParsed`) or `pkg.GetImportPathBySelector()` utility can help map the alias `OtherPkg` to its canonical import path (e.g., `"example.com/project/otherpkg"`).
    *   Once you have the canonical `importPathToResolve`:
        *   Call `resolvedOtherPkg, err := pkg.ResolveImport(ctx, importPathToResolve)`.

4.  **`pkg.ResolveImport(ctx, importPathToResolve)`**:
    *   The method first checks if `pkg` itself has already resolved and cached this specific `importPathToResolve` in its `pkg.resolvedImports` map. If so, it's returned.
    *   If not, it asks the central `pkg.loader` instance to resolve it via `pkg.loader.resolveImport(ctx, pkg.ImportPath, importPathToResolve)`.
    *   **`loader.resolveImport(...)`**:
        1.  The `loader` checks its main **package cache** (`loader.cache`) for `importPathToResolve`. If found, the cached `*Package` is returned.
        2.  If not in the cache, the `loader` invokes its configured `PackageLocator` again, this time with the specific `importPathToResolve`.
        3.  A new `loader.Package` object is created for the located package, added to the `loader`'s **package cache**, and also stored in the originating `pkg`'s `resolvedImports` map.
        4.  This newly resolved `*Package` (for `OtherPkg`) is returned.
    *   Now you have `resolvedOtherPkg`, and you can call methods on it, like `resolvedOtherPkg.GetStruct("OtherType")`, which would in turn trigger `ensureParsed()` for `resolvedOtherPkg` if its files haven't been parsed yet.

This on-demand, cached mechanism ensures that only necessary packages and files are parsed, and that parsing/location work is not repeated, making it efficient for tools that may only need to inspect parts of a larger codebase.
```
