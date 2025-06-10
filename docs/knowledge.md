# Testing Multi-Package Scenarios with a Custom Locator

To effectively test package loading capabilities, especially for scenarios involving multiple interdependent packages (like structs embedding types from other local packages), a custom package locator such as `testdataLocator` is invaluable. This approach allows for isolated and controlled testing without reliance on `go list` or complex Go module setups external to the test suite.

## Setting Up the Test Environment

1.  **Organize Test Data**:
    *   Place your test Go files in dedicated subdirectories under a common test data root (e.g., `internal/loader/lazyload/testdata/`).
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

The project employs a custom mechanism for loading Go package information, primarily centered around the `internal/loader/lazyload` package. This strategic choice is based on several factors, moving away from direct reliance on `golang.org/x/tools/go/packages` for all loading tasks due to the following considerations:

-   **Avoiding Eager Imports**:
    A significant reason to avoid direct use of `go/packages` for all tasks is that it tends to make imports eager. This can lead to issues with resolving types and packages, especially in complex scenarios or when full type information across all dependencies is not strictly needed. Eager loading can increase processing time and resource consumption unnecessarily. The `lazyload` system, in contrast, offers more control, preferring `go/parser` for AST-level analysis and `golang.org/x/tools/go/types` for type checking when more control over the loading process is required. The `lazyload` package itself is an example of such a targeted loading mechanism.

-   **Fine-grained Control and Lazy Evaluation**:
    The `lazyload` system allows for a two-step loading process. Initially, it can use `go list -json` (via `lazyload.GoListLocator`) to gather minimal package metadata (like import paths, directory, file names, and direct imports). The full parsing of Go source files into ASTs and the resolution of further dependencies are deferred until a specific package's details are actually requested (e.g., by `Package.Files()` or `Package.ResolveImport()`). This lazy evaluation provides granular control over performance and resource usage, avoiding the upfront cost of processing an entire package graph if only partial information is needed.

-   **AST-Centric Analysis**:
    The primary consumers of package information, such as the analyzers in `internal/analyzer/` (e.g., `options_analyzer.go`), predominantly operate on Abstract Syntax Trees (ASTs). Once the relevant Go source files for a package are located, direct parsing using `go/parser` is often sufficient for their needs. Currently, these analyzers do not heavily rely on comprehensive type information from `go/types` (which `go/packages` excels at providing). The `AnalyzeOptions` function, for example, is designed to work with pre-parsed ASTs and defers full type checking.

-   **Decoupling of Concerns**:
    The analyzers are generally designed to consume pre-parsed ASTs or specific metadata structures. This decouples the analysis logic from the concrete mechanism of how packages are discovered, loaded, and parsed. The `loader.Loader` type (previously `lazyload.Loader` - note also that `lazyload.PackageLoader` was a typo in a previous version of this document, it was `lazyload.Loader`) and its `loader.PackageLocator` function type allow for different strategies to find and initially describe packages, while the analyzers focus on the already-loaded code representation.

-   **Simplicity and Targeted Information**:
    For some straightforward tasks, direct use of `os` functions, `go/parser`, or a minimal invocation of `go list` (as used by `loader.GoListLocator`) is simpler and provides exactly the information needed without overkill. The `AnalyzeOptions` function in `options_analyzer.go` leverages the `loader` package. While `loader` can use `go list` for package discovery, the primary analysis within `AnalyzeOptions` focuses on ASTs. Full type information via `go/types` is used by `loader` when resolving types or checking interfaces as needed, but this is done on-demand rather than globally. This targeted approach aligns with the goal of minimizing unnecessary processing.

### Refactoring of the Loader Package (Formerly `lazyload`)

To simplify the package structure and naming conventions within the project, the primary package responsible for lazy loading of Go source code has been refactored:

-   **Directory Move**: The package previously located at `internal/loader/lazyload` has been moved to `internal/loader/loader`.
-   **Package Name Change**: Correspondingly, the Go package name itself has been changed from `lazyload` to `loader`. All import paths and package references in the codebase have been updated to reflect this.
-   **Constructor Rename**: The constructor function for the loader, formerly `lazyload.NewLoader`, has been renamed to `loader.New`.

These changes aim to make the package's purpose and location more straightforward within the `internal/loader` directory, streamlining it to simply `loader`. This refactoring maintains the core lazy-loading strategy and functionalities previously established by the `lazyload` package.
[end of docs/knowledge.md]
