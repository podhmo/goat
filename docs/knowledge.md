# Testing Multi-Package Scenarios with a Custom Locator

To effectively test package loading capabilities, especially for scenarios involving multiple interdependent packages (like structs embedding types from other local packages), a custom package locator such as `testdataLocator` is invaluable. This approach allows for isolated and controlled testing without reliance on `go list` or complex Go module setups external to the test suite.

## Setting Up the Test Environment

1.  **Organize Test Data**:
    *   Place your test Go files in dedicated subdirectories under a common test data root (e.g., `internal/loader/testdata/`).
    *   Each subdirectory represents a distinct Go package for your test. For example, to test `userpkg` importing `basepkg`:
        *   `testdata/basepkg/base.go` (defines types in `basepkg`)
        *   `testdata/userpkg/user.go` (defines types in `userpkg`, imports `basepkg`)

2.  **Use Locator-Specific Import Paths**:
    *   In your test Go files (e.g., `user.go`), use import paths that your custom locator is designed to recognize and map to the test data directories.
    *   For `testdataLocator` (which uses `example.com/` as a prefix), an import like `import "example.com/basepkg"` in `testdata/userpkg/user.go` will be resolved to the `testdata/basepkg` directory.

3.  **Implement Test Logic**:
    *   Initialize the loader with your custom locator (e.g., `NewLoader(Config{Locator: testdataLocator})`).
    *   Load the primary test package using its locator-specific import path (e.g., `loader.Load("example.com/userpkg")`).
    *   The loader, via the custom locator, will find and process the necessary Go files from your `testdata` subdirectories.
    *   Your test can then retrieve struct information, resolve imports between your testdata packages (`Package.ResolveImport()`), and verify that types, embedded fields, and other relevant details are correctly processed across these test packages.

## Advantages

This method provides a self-contained way to test:
-   Resolution of types and structs across multiple local packages.
-   Correct parsing and representation of imported types, including embedded structs.
-   The loader's ability to handle package dependencies as defined within the test data, isolated from the broader project or system Go environment.

This setup is crucial for detailed unit testing of package analysis and loading logic.

# Package Loading Strategy

The project employs a custom mechanism for loading Go package information, primarily centered around the `internal/loader` package (originally `internal/loader/lazyload`). This strategic choice is based on several factors, moving away from direct reliance on `golang.org/x/tools/go/packages` for all loading tasks due to the following considerations:

-   **Avoiding Eager Imports**:
    A significant reason to avoid direct use of `go/packages` for all tasks is that it tends to make imports eager. This can lead to issues with resolving types and packages, especially in complex scenarios or when full type information across all dependencies is not strictly needed. Eager loading can increase processing time and resource consumption unnecessarily. The lazy loading system (formerly `lazyload`), in contrast, offers more control, preferring `go/parser` for AST-level analysis and `golang.org/x/tools/go/types` for type checking when more control over the loading process is required. The `loader` package itself (now at `internal/loader`) is an example of such a targeted loading mechanism.

-   **Fine-grained Control and Lazy Evaluation**:
    The lazy loading system (formerly `lazyload`) allows for a two-step loading process. Initially, it can use `go list -json` (via `loader.GoListLocator`) to gather minimal package metadata (like import paths, directory, file names, and direct imports). The full parsing of Go source files into ASTs and the resolution of further dependencies are deferred until a specific package's details are actually requested (e.g., by `Package.Files()` or `Package.ResolveImport()`). This lazy evaluation provides granular control over performance and resource usage, avoiding the upfront cost of processing an entire package graph if only partial information is needed.

-   **AST-Centric Analysis**:
    The primary consumers of package information, such as the analyzers in `internal/analyzer/` (e.g., `options_analyzer.go`), predominantly operate on Abstract Syntax Trees (ASTs). Once the relevant Go source files for a package are located, direct parsing using `go/parser` is often sufficient for their needs. Currently, these analyzers do not heavily rely on comprehensive type information from `go/types` (which `go/packages` excels at providing). The `AnalyzeOptions` function, for example, is designed to work with pre-parsed ASTs and defers full type checking.

-   **Decoupling of Concerns**:
    The analyzers are generally designed to consume pre-parsed ASTs or specific metadata structures. This decouples the analysis logic from the concrete mechanism of how packages are discovered, loaded, and parsed. The `loader.Loader` type and its `loader.PackageLocator` function type allow for different strategies to find and initially describe packages, while the analyzers focus on the already-loaded code representation.

-   **Simplicity and Targeted Information**:
    For some straightforward tasks, direct use of `os` functions, `go/parser`, or a minimal invocation of `go list` (as used by `loader.GoListLocator`) is simpler and provides exactly the information needed without overkill. The `AnalyzeOptions` function in `options_analyzer.go` leverages the `loader` package. While `loader` can use `go list` for package discovery, the primary analysis within `AnalyzeOptions` focuses on ASTs. Full type information via `go/types` is used by `loader` when resolving types or checking interfaces as needed, but this is done on-demand rather than globally. This targeted approach aligns with the goal of minimizing unnecessary processing.


# Interpreter Design for Enum Resolution

The `internal/interpreter` package is responsible for understanding default values and enum constraints provided in user code, typically within initializer functions (e.g., `NewOptions`). A key challenge was to resolve enum values when they are defined not as literal slices directly within `goat.Enum()` calls, but as variables (e.g., `var MyEnum = []string{"a", "b"}`) potentially in different packages.

## Core Requirements and Constraints:

1.  **Resolve Enum Variables**: The interpreter needed to find the definition of an identifier passed to `goat.Enum()` and extract its values.
2.  **Support Cross-Package Enums**: Enums might be defined in a separate package and imported.
3.  **Avoid `go/types`**: A specific project constraint was to avoid using the `go/types` package for this part of the type resolution, favoring AST-level analysis and the existing `internal/loader`.

## Implementation Strategy:

To meet these requirements, the following approach was adopted:

1.  **`astutils` Enhancement**:
    *   The functions `astutils.EvaluateArg` and `astutils.EvaluateSliceArg` were modified to return a new struct, `astutils.EvalResult`.
    *   `EvalResult` includes fields `Value` (for literal evaluations) and `IdentifierName` / `PkgName` (if the AST node was an identifier, possibly qualified with a package alias). This allows the interpreter to distinguish between a literal slice and a variable name that needs further lookup.

2.  **Interpreter and Loader Integration**:
    *   The main `internal/interpreter.InterpretInitializer` function was updated to accept an instance of `loader.Loader` and the `currentPkgPath` (import path of the package being interpreted).
    *   When `extractMarkerInfo` (specifically the part handling `goat.Enum()`, refactored into `extractEnumValuesFromEvalResult`) encounters an `EvalResult` indicating an identifier:
        *   It determines the target package path for the identifier (using `currentPkgPath` for unqualified idents or resolving the package alias via `astutils.GetImportPath` for qualified idents).
        *   It uses the passed `loader.Loader` instance to load the AST of the target package (`loader.Load(targetPkgPath)`).
        *   It then inspects the AST of the loaded package to find the `var` declaration matching the identifier's name.
        *   If the `var` is found and its initializer is a slice literal (e.g., `[]string{"a", "b"}` or `[]string{string(ConstA), string(ConstB)}`), this initializer is processed to extract the final string values.

3.  **Handling `string(Constant)` in Enums**:
    *   The `extractEnumValuesFromEvalResult` function was enhanced to specifically handle `var` initializers that are composite literals (slices) where elements might be of the form `string(CONST_IDENT)` or `string(pkg.CONST_IDENT)`.
    *   A helper function, `resolveConstStringValue(constName string, pkg *loader.Package, identFile *ast.File) (string, bool)`, was added. This helper inspects the AST of the given package (`pkg`) to find a `const` declaration matching `constName`. If the constant is a basic string literal, its value is returned.
    *   When processing a slice element like `string(CONST_IDENT)`:
        *   `CONST_IDENT` is resolved using `resolveConstStringValue` within its defining package (which might be the current package or an imported one, resolved using the loader).
        *   The string value of the constant is then used.
    *   This allows the interpreter to correctly extract string values from enums defined using the `string(CONST)` pattern, by looking up the constant's definition in the relevant package's AST. This mechanism works for constants defined as string literals.

## Benefits:

*   Allows users to define enums as variables, which is more idiomatic in Go than always requiring literal slices in `goat.Enum()`.
*   Enables discovery of enums across package boundaries by leveraging the existing `loader`'s capabilities.
*   Adheres to the constraint of not using `go/types` directly in the interpreter logic for this specific resolution path, relying instead on AST inspection of loaded packages.

This approach provides a balance between functionality and adherence to project-specific constraints.
