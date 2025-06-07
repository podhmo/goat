# Contributing to goat

Thank you for considering contributing to `goat`!

## Development Workflow

1.  **Make your changes:** Implement your bug fix or feature.
2.  **Update Examples (if applicable):**
    If your changes affect the CLI generation (e.g., modify how `Options` structs are parsed, add new marker functions, or change generated code), you should update the code examples.
    - The primary example for showcasing features is `examples/fullset/main.go`.
    - The example in the main `README.md` should be kept concise.
3.  **Regenerate Example Code:**
    After updating example sources (like `examples/fullset/main.go`), you need to regenerate the `main()` function within those examples using:
    ```bash
    make examples-emit
    ```
    Alternatively, you can run `go generate` directly on the specific example file if you've configured its `//go:generate` directive appropriately. For instance:
    ```bash
    go generate ./examples/fullset/main.go
    ```
4.  **Test your changes:** Ensure all tests pass.
    ```bash
    go test ./...
    ```
5.  **Commit and Push:** Commit your changes with a clear message and push to your fork.
6.  **Open a Pull Request:** Submit a PR against the main `goat` repository.

## Known Issues

### `make examples-emit` and `examples/fullset/main.go`

Currently, the `goat emit` command has an internal issue when processing Go files that directly import the `github.com/podhmo/goat` package itself (as `examples/fullset/main.go` does). This results in an error like:

`ERROR ... could not import github.com/podhmo/goat (invalid package name: "")`

This prevents `make examples-emit` (or direct `go generate` on `examples/fullset/main.go`) from successfully regenerating the `main()` function for this specific example.

While the source code of `examples/fullset/main.go` should be kept up-to-date to reflect all features, the generated `main()` function within it might be stale until this underlying issue in the `goat` tool is resolved. Other examples that do not import `github.com/podhmo/goat` (like `examples/hello/main.go`) should process correctly.

Please be aware of this limitation when working with the `fullset` example.
