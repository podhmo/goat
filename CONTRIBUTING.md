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
