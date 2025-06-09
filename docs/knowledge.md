# Testing Embedded Structs from Different Packages with `testdataLocator`

When testing the lazy loading capabilities of packages, especially how they handle embedded structs from different (foreign) packages, a controlled test environment is crucial. The `testdataLocator` mechanism within the `internal/loader/lazyload` package provides this control, allowing for isolated testing without needing `go list` or complex module setups.

## Test Data Structure

Test data should be organized within the `internal/loader/lazyload/testdata/` directory. Each distinct test package or scenario typically resides in its own subdirectory. For instance, to test a struct `UserStruct` in `userpkg` that embeds `BaseStruct` from `basepkg`, you would create:

-   `internal/loader/lazyload/testdata/basepkg/base.go` (defining `BaseStruct`)
-   `internal/loader/lazyload/testdata/userpkg/user.go` (defining `UserStruct` which imports and embeds `basepkg.BaseStruct`)

## Import Paths and `testdataLocator`

In your test Go files (e.g., `user.go`), you use pseudo-module import paths. The `testdataLocator` is designed to resolve these paths to the corresponding directories within `testdata/`. For example, if `testdataLocator` uses a prefix like `example.com/`, an import statement:

```go
import "example.com/basepkg"
```
in `user.go` will be mapped by `testdataLocator` to the `internal/loader/lazyload/testdata/basepkg` directory. This allows the loader to find and parse the source files of `basepkg` as if it were a real, resolvable Go package.

## Test Logic Steps

A typical test (`TestGetStructWithEmbeddedForeignStruct` serves as an example) involves the following steps:

1.  **Load Primary Package**: Initialize the loader with `testdataLocator` and load the package containing the struct with the embedded field (e.g., `example.com/userpkg`).
2.  **Get Struct with Embedded Field**: Retrieve the `StructInfo` for the main struct (e.g., `UserStruct`).
3.  **Identify Embedded Type**: Iterate through the fields of `UserStruct`. The embedded field (e.g., `BaseStruct`) will have its `Embedded` flag set to `true`. Its `TypeExpr` will typically be an `*ast.SelectorExpr` (e.g., `basepkg.BaseStruct`).
4.  **Extract Package Alias**: From the `ast.SelectorExpr` (e.g., `selExpr`), `selExpr.X` will be an `*ast.Ident` representing the package alias used in the source code (e.g., `basepkg`). `selExpr.Sel.Name` will be the type name (e.g., `BaseStruct`).
5.  **Find Full Import Path**:
    *   Access the parsed AST of the source file(s) for the primary package (e.g., `userPkg.Files()`).
    *   Iterate through `astFile.Imports` for each `ast.File`.
    *   For each `ast.ImportSpec`, compare the package alias extracted in the previous step with `ImportSpec.Name.Name` (if an explicit alias is used) or the derived package name from `ImportSpec.Path.Value` (if no explicit alias is used).
    *   Once matched, the unquoted `ImportSpec.Path.Value` (e.g., `"example.com/basepkg"`) is the full import path.
6.  **Resolve Import**: Use `primaryPackage.ResolveImport(fullImportPath)` with the path found. This call triggers the loader (via `testdataLocator`) to load and parse the foreign package (e.g., `basepkg`) if it hasn't been already. It returns a `Package` object for the imported package.
7.  **Get Struct from Resolved Package**: Call `resolvedImportedPackage.GetStruct("BaseStruct")` to retrieve the `StructInfo` for the embedded struct from the now-loaded foreign package.
8.  **Verify Fields**: Perform assertions on the fields of both the primary struct and the embedded struct to ensure types, tags, and other properties are correctly parsed and represented.

## Benefits

This approach allows for:
-   Testing complex scenarios like cross-package struct embedding.
-   Ensuring the loader correctly resolves and parses dependencies.
-   Verifying that type information, including from foreign packages, is accurately captured.
-   All of this is achieved without needing to set up actual Go modules on the filesystem or relying on external tools like `go list` during the test execution, leading to faster and more reliable unit tests for the loader's capabilities.

This documentation should help in understanding and creating new tests for similar scenarios involving the `lazyload` package.
