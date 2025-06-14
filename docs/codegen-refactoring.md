# Refactoring Plan for internal/codegen

This document outlines a refactoring plan for the `internal/codegen` package and related components within the `goat` tool. The primary goal is to improve modularity, testability, and maintainability by decoupling metadata extraction from code generation.

## 1. Background and Motivation

The `goat` tool currently has an `emit` command that generates a `main.go` file based on metadata parsed from a target Go project. A `scan` command is also envisioned (and partially implemented) to inspect the target project and display the metadata that `goat` understands. This is useful for users to verify `goat`'s interpretation of their code, especially for complex cases like enums and comprehensive option sets (e.g., `examples/enum` and `examples/fullset`).

The current structure appears to mix metadata extraction logic (likely within `cmd/goat/main.go`'s command handlers like `scanMain`) with code generation logic (`internal/codegen/main_generator.go`). This refactoring aims to separate these concerns.

## 2. Assumptions about the `scan` Command

For this refactoring proposal, the `scan` command is assumed to:

*   **Primary Purpose:** Inspect target Go source files and output the structured metadata that `goat` extracts (i.e., `metadata.CommandMetadata`). This is the same metadata that the `emit` command would use.
*   **Output Content:** Include details of the run function, command-line options (name, type, default, environment variable, help text, required status, enum values), overall help message, and initializer function.
*   **Output Format:** JSON is the current output format, which is suitable for structured data.
*   **No Code Generation:** `scan` is a read-only analysis tool and does not modify source files.

## 3. Analysis of `scan` Output (Conceptual)

Executing `scan` on examples like `examples/enum/main.go` and `examples/fullset/main.go` should produce a JSON representation of `metadata.CommandMetadata`. Key aspects to verify in this output include:

*   Correct identification of all option types (string, int, bool, pointers, slices, custom types).
*   Accurate capture of default values.
*   Correct determination of `IsRequired` status for flags.
*   Population of `EnvVar` for environment variables.
*   Extraction of `HelpText`.
*   Correct identification and listing of `EnumValues` for enum types.
*   Flags for `IsTextUnmarshaler` and `IsTextMarshaler`.
*   Correct generation of `CliName` (kebab-case).
*   Identification of the `InitializerFunc`.

While direct output from the `scan` execution was not captured in this specific interactive session, existing tests (`TestScanSubcommand`) indicate that `scan` outputs `CommandMetadata` in JSON format. The refactoring will make it easier to write targeted tests to ensure the accuracy of this metadata.

## 4. Proposed Refactoring Plan

The core idea is to centralize metadata extraction into a new package, making it a distinct step that both `scan` and `emit` (and potentially other future commands) can consume.

### 4.1. New Package: `internal/analysis`

*   **Purpose:** This package will be solely responsible for parsing Go source files and extracting all relevant information into a `metadata.CommandMetadata` structure.
*   **Key Components:**
    *   `analysis.Config`: A struct to pass necessary configuration (target file, run function name, initializer name, locator strategy) to the extraction logic.
    *   `analysis.ExtractCommandMetadata(ctx context.Context, config Config) (*metadata.CommandMetadata, error)`: The main function that performs the static analysis of the user's code and returns the populated metadata object.
*   **Implementation:** The existing logic currently in `cmd/goat/main.go` (e.g., within a function like `scanMain` and its helpers) that handles file parsing, AST traversal, type checking, and populating `metadata.CommandMetadata` will be moved into this package.

### 4.2. Refactor `internal/codegen`

*   **`main_generator.go`:**
    *   The `GenerateMain` function will no longer be responsible for initiating any parsing.
    *   It will receive a fully populated `metadata.CommandMetadata` object as its primary input.
    *   Its signature will remain `func GenerateMain(cmdMeta *metadata.CommandMetadata, helpText string, generateFullFile bool) (string, error)`.
    *   The internal `generateMainContent` function will operate as it currently does, using the provided `cmdMeta`.
*   **`writer.go`:**
    *   No significant changes are anticipated. Its responsibility for writing generated content to a file and formatting imports remains valid.

### 4.3. Refactor CLI Command Handlers in `cmd/goat/main.go`

*   **`emit` command:**
    1.  Parse CLI arguments to create an `analysis.Config`.
    2.  Call `analysis.ExtractCommandMetadata()` to obtain the `cmdMeta`.
    3.  Generate help text (e.g., using `internal/helpgen.GenerateHelp(cmdMeta)`).
    4.  Call `codegen.GenerateMain(cmdMeta, helpText, true)` to get the generated main function code string.
    5.  Call `codegen.WriteMain()` to write this string to the target file.
*   **`scan` command:**
    1.  Parse CLI arguments to create an `analysis.Config`.
    2.  Call `analysis.ExtractCommandMetadata()` to obtain the `cmdMeta`.
    3.  Marshal the `cmdMeta` object to JSON and print it to standard output.
*   **`help-message` command:**
    1.  Parse CLI arguments to create an `analysis.Config`.
    2.  Call `analysis.ExtractCommandMetadata()` to obtain the `cmdMeta`.
    3.  Call `internal/helpgen.GenerateHelp(cmdMeta)` to generate the help message.
    4.  Print the help message to standard output.

## 5. Benefits of Refactoring

*   **Clear Separation of Concerns:**
    *   `internal/analysis`: Code parsing and metadata extraction.
    *   `internal/codegen`: Go code generation from metadata.
    *   `internal/helpgen`: Help text generation from metadata.
    *   `cmd/goat`: CLI interaction and orchestration.
*   **Improved Testability:**
    *   Metadata extraction (`internal/analysis`) can be unit-tested independently by feeding it various code snippets (including those from `examples/enum` and `examples/fullset`) and asserting the correctness of the output `CommandMetadata` struct.
    *   Code generation (`internal/codegen`) can be unit-tested by providing mock `CommandMetadata` objects.
*   **Enhanced Maintainability:** Bug fixes and feature enhancements will be more localized. For example, issues with metadata interpretation would be addressed in `internal/analysis`, while code generation bugs would be in `internal/codegen`.
*   **Extensibility:** Future commands requiring Go code analysis can readily reuse the `internal/analysis` package.
*   **Reliable `scan` Output:** The `scan` command will directly output the result from `internal/analysis`, ensuring it reflects the exact metadata used by `emit`. This directly addresses the user's need to understand `goat`'s interpretation and check for discrepancies.

## 6. Conclusion

This refactoring will lead to a more robust, understandable, and maintainable architecture for the `goat` tool. It establishes a clear pipeline: CLI input -> Code Analysis & Metadata Extraction -> Code Generation / Information Display. This structure will be beneficial for current functionality and future development.
