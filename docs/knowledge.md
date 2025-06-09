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
