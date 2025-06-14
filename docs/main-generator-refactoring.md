# Refactoring Plan for internal/codegen/main_generator.go

This document outlines a specific refactoring plan for `internal/codegen/main_generator.go`. The primary goal is to reduce its internal complexity, particularly the number of states and conditional branches (e.g., `switch` statements), by introducing a more object-oriented and strategy-based approach to code generation for different option types. This plan is based on a detailed analysis of the existing code and the structure of `metadata.OptionMetadata`.

## 1. Background and Motivation

The `internal/codegen/main_generator.go` file, specifically the `generateMainContent` function, is responsible for generating the Go code for the `main()` function of a CLI application. It processes `metadata.CommandMetadata` (which includes a list of `metadata.OptionMetadata`) to generate logic for:
- Default value initialization
- Environment variable parsing
- Command-line flag registration and parsing
- Handling of required options
- Enum validation

The current implementation relies heavily on large `switch` statements based on `OptionMetadata.TypeName` and numerous `if/else` conditions to handle variations (e.g., pointer vs. value types, `TextUnmarshaler` interface). This leads to high cyclomatic complexity and makes the code hard to maintain and extend.

This refactoring aims to simplify `generateMainContent` by delegating type-specific code generation logic to specialized handlers.

## 2. Core Refactoring Strategy: Staged Code Generation with Type-Specific Handlers

The `generateMainContent` function will maintain its overall staged approach to code generation. However, within each stage that processes individual options, it will use a factory to obtain an `OptionHandler` specific to the option's characteristics (type, pointer, interfaces implemented). This handler will then be responsible for generating the necessary Go code snippets for that option at that particular stage.

### 2.1. `OptionHandler` Interface

An interface will define the contract for all type-specific handlers:

```go
package codegen // or codegen.handlers

import "github.com/podhmo/goat/internal/metadata"

// OptionCodeSnippets might hold different parts of generated code for an option.
// For instance, flag registration might need variable declarations before flag.Parse()
// and assignment logic after flag.Parse().
type OptionCodeSnippets struct {
    Declarations   string // e.g., temp variables for pointer flags before Parse
    Logic          string // e.g., the flag.StringVar call, env var parsing logic
    PostProcessing string // e.g., assigning temp var to actual option field after Parse
}

type OptionHandler interface {
    // Generates code for default value assignment when no InitializerFunc is present.
    GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets

    // Generates code for processing an environment variable.
    GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets

    // Generates code for flag registration.
    // globalTempVarPrefix helps ensure uniqueness of temp vars for flags.
    GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets

    // Generates code for assignments needed after flag.Parse() (primarily for pointer flags).
    GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets

    // Generates code for checking if a required option is missing.
    GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets

    // Generates code for validating enum values.
    GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets
}
```

### 2.2. Concrete Handler Implementations

Specific structs will implement `OptionHandler` for different kinds of options:
- `StringHandler`, `IntHandler`, `BoolHandler` (for value types)
- `StringPtrHandler`, `IntPtrHandler`, `BoolPtrHandler` (for pointer types, encapsulating temp variable logic for flags)
- `StringSliceHandler`
- `TextUnmarshalerHandler`, `TextUnmarshalerPtrHandler` (for types implementing `encoding.TextUnmarshaler`)
- `UnsupportedTypeHandler` (to generate a comment or error for types not yet handled)

Each handler will contain the detailed logic for generating Go code specific to its type. For example, `StringPtrHandler.GenerateFlagRegistrationCode` will produce the verbose `isNilInitially`, `tempVal`, conditional `flag.StringVar` calls, and `StringPtrHandler.GenerateFlagPostParseAssignmentCode` will produce the corresponding assignment from `tempVal`.

### 2.3. Handler Factory

A factory function, `GetOptionHandler(opt *metadata.OptionMetadata) OptionHandler`, will inspect the `OptionMetadata` (checking `TypeName`, `IsPointer`, `IsTextUnmarshaler`, `EnumValues`, etc.) and return the appropriate handler instance.

### 2.4. Refactored `generateMainContent` Flow

The `generateMainContent` function will be modified as follows:

1.  **Initialization:** Basic setup, declare `options` variable (either from `new()` or initializer).
2.  **Loop through Stages:** For each conceptual stage (Defaults, Env Vars, Flag Registration, Required Checks, Enum Validation):
    a.  Iterate through `cmdMeta.Options`.
    b.  For each `opt`, call `handler := GetOptionHandler(opt)`.
    c.  Call the relevant method on the `handler` (e.g., `handler.GenerateEnvVarProcessingCode(...)`).
    d.  Append the returned `OptionCodeSnippets` (or parts thereof) to the main `strings.Builder`.
3.  **Flag Handling Specifics:**
    *   `GenerateFlagRegistrationCode` will return snippets for declarations (to be placed before flag definitions) and the flag registration calls themselves.
    *   `GenerateFlagPostParseAssignmentCode` will return snippets to be placed *after* `flag.Parse()` and populating `isFlagExplicitlySetMap`.
4.  **Interspersed Logic:** Code like `flag.Parse()`, `flag.Visit()`, and the main run function call will remain in `generateMainContent` at their appropriate positions between stages.

## 3. Benefits of This Refactoring

*   **Reduced Complexity in `generateMainContent`:** The large `switch` statements and deeply nested conditionals will be replaced by polymorphic calls to handler methods. `generateMainContent` becomes an orchestrator.
*   **Encapsulation of Type-Specific Logic:** All the nuances of handling a particular type (e.g., how to parse a `*int` from an environment variable, how to register its flag) are contained within its specific handler.
*   **Improved Maintainability:**
    *   Fixing a bug for a specific type involves touching only its handler.
    *   Adding support for a new option type involves creating a new handler and updating the factory.
*   **Better Testability:** Each handler can be unit-tested in isolation.
*   **Clearer Code Structure:** The responsibilities become more clearly defined.

## 4. Impact on Specific Complexities

*   **Pointer Flag Logic:** The complex `isNilInitially`/`tempVal` pattern for pointer flags will be entirely managed within the respective pointer handlers (e.g., `StringPtrHandler`, `IntPtrHandler`).
*   **`TextUnmarshaler`:** Will have its own dedicated handlers, simplifying the main flow.
*   **Required Checks & Enum Validation:** These will also be delegated to handlers, allowing for type-aware validation logic.

This refactoring directly addresses the request to reduce state space and conditional branching in `main_generator.go` by abstracting type-specific code generation details into manageable, focused components.
