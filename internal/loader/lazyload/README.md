# Go Lazy Package Loader

`lazyload` is a Go library designed to inspect Go source code, similar to `go/packages`, but with a focus on **lazy loading** of package information and ASTs. This approach can be beneficial for tools that only need to inspect a small subset of a large codebase or want to minimize initial loading time.

**Core Principles:**

*   **Lazy AST Parsing**: ASTs for `.go` files are parsed only when explicitly requested for a package (e.g., when analyzing its structs).
*   **On-Demand Dependency Resolution**: When analyzing a package (e.g., a struct field referencing a type from another package `foo.Bar`), the package `foo` is resolved and loaded only at that moment, not upfront.
*   **No Type Checking (by default)**: This library focuses on AST-level information (syntax, struct tags, import declarations) and does not perform full type checking like `go/types`. This simplifies the process and reduces overhead when type information is not strictly necessary.
*   **Pluggable Package Location**: While `go list` is the default mechanism for finding packages, the loader can be configured with a custom `PackageLocator` function to support different build systems or environments.

## Features

*   Load Go packages based on patterns (e.g., `./...`, `example.com/mymodule/pkg`).
*   Access package metadata (import path, directory, file list).
*   Lazily parse Go source files into ASTs (`ast.File`).
*   Resolve imported packages on demand when their information is needed.
*   Inspect struct definitions, including field names, Go struct tags, and AST-level type expressions.
*   (Future) Extensible for more advanced AST queries.

## Basic Usage

```go
package main

import (
	"fmt"
	"log"

	"example.com/path/to/lazyload" // Replace with your actual import path
)

func main() {
	cfg := lazyload.Config{
		Context: lazyload.BuildContext{ /* ... configure GOOS, GOARCH, BuildTags if needed ... */ },
	}
	loader := lazyload.NewLoader(cfg)

	// Load initial packages (metadata only at this stage)
	pkgs, err := loader.Load("./...") // Load all packages in the current module
	if err != nil {
		log.Fatalf("Failed to load packages: %v", err)
	}

	for _, pkg := range pkgs {
		fmt.Printf("Package: %s (%s)\n", pkg.Name, pkg.ImportPath)

		// Example: Get a specific struct
		// This will trigger parsing of files in 'pkg' if not already done.
		structInfo, err := pkg.GetStruct("MyStruct")
		if err == nil {
			fmt.Printf("  Found struct: %s\n", structInfo.Name)
			for _, field := range structInfo.Fields {
				fmt.Printf("    Field: %s, Type (AST): %T, Tag: `%s` (json: %q)\n",
					field.Name,
					field.TypeExpr, // This is an ast.Expr
					field.Tag,
					field.GetTag("json"),
				)

				// If field.TypeExpr is an *ast.SelectorExpr (e.g., otherpkg.OtherType),
				// you could then use pkg.ResolveImport("path/to/otherpkg")
				// to get the 'otherpkg' Package object and inspect 'OtherType'.
				if selExpr, ok := field.TypeExpr.(*ast.SelectorExpr); ok {
					if pkgIdent, ok := selExpr.X.(*ast.Ident); ok {
						// This is a simplified lookup. A real one would scan pkg.fileImports
						// to map pkgIdent.Name to its full import path.
						fmt.Printf("      Field type might be from package alias: %s, Selector: %s\n", pkgIdent.Name, selExpr.Sel.Name)
						// For a robust solution:
						// 1. Parse the file containing this struct.
						// 2. Find the import statement corresponding to pkgIdent.Name.
						// 3. Get the full import path.
						// 4. Call importedPkg, err := pkg.ResolveImport(fullImportPath)
						// 5. Call otherStructInfo, err := importedPkg.GetStruct(selExpr.Sel.Name)
					}
				}
			}
		} else {
			// Might be normal if struct doesn't exist or not an error to be logged always
			// log.Printf("  Could not get struct 'MyStruct' from %s: %v", pkg.ImportPath, err)
		}
		fmt.Println("---")
	}
}

// Placeholder for MyStruct if you want to run the example
// package main
// type MyStruct struct {
//     Name string `json:"name"`
//     Age int `json:"age,omitempty"`
//	   Other otherpkg.OtherType `json:"other_type"`
// }
//
// package otherpkg
// type OtherType struct {
//    Value string
// }

```

## Package Location

By default, the loader uses `go list -json` to find packages. You can provide a custom `PackageLocator` function in the `Loader`'s `Config` to integrate with other build systems or environments where `go list` might not be suitable or available.

```go
// Example of a custom locator (simplified)
func myCustomLocator(pattern string, buildCtx lazyload.BuildContext) ([]lazyload.PackageMetaInfo, error) {
    // ... your logic to find packages and their files ...
    // This might involve reading custom build files, querying a proprietary system, etc.
    return []lazyload.PackageMetaInfo{
        {
            ImportPath: "custom/pkg/foo",
            Name: "foo",
            Dir: "/path/to/custom/pkg/foo",
            GoFiles: []string{"foo.go", "bar.go"},
            DirectImports: []string{"standardlib/fmt", "custom/pkg/bar"},
        },
    }, nil
}

// ... in main
// cfg.Locator = myCustomLocator
// loader := lazyload.NewLoader(cfg)
```

## How Lazy Resolution Works

1.  **`loader.Load(pattern)`**:
    *   The `PackageLocator` (e.g., `GoListLocator`) is called to find packages matching the `pattern`.
    *   For each found package, it gathers metadata like import path, directory, and list of `.go` files (`PackageMetaInfo`).
    *   A `lazyload.Package` object is created for each, but no `.go` files are parsed yet.

2.  **`pkg.GetStruct("MyType")` (or similar AST access)**:
    *   The `pkg.ensureParsed()` method is called.
    *   This parses all `.go` files within `pkg` using `go/parser` and stores the `ast.File` objects. It also records the import declarations from each file.
    *   The ASTs are then traversed to find the `MyType` struct definition.

3.  **Analyzing a Field `F OtherPkg.OtherType`**:
    *   The `FieldInfo` for `F` will have `TypeExpr` as an `*ast.SelectorExpr` (representing `OtherPkg.OtherType`).
    *   To find out what `OtherPkg` is:
        1.  Inspect the `ast.File` where `MyType` is defined to find the `import` statement for the alias `OtherPkg`. This gives you the full import path (e.g., `"example.com/project/otherpkg"`).
        2.  Call `resolvedOtherPkg, err := pkg.ResolveImport("example.com/project/otherpkg")`.
    *   **`pkg.ResolveImport(importPath)`**:
        1.  The `pkg.loader` (the central `Loader` instance) is asked to resolve this `importPath`.
        2.  The `loader` checks its internal cache for this `importPath`.
        3.  If not cached, it calls the `PackageLocator` again, this time with the specific `importPath`.
        4.  A new `lazyload.Package` object is created for `OtherPkg`, cached, and returned.
    *   Now you have `resolvedOtherPkg`, and you can call `resolvedOtherPkg.GetStruct("OtherType")` on it.

This on-demand mechanism ensures that only necessary packages and files are processed.
```
