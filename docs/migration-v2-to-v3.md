# Migration Guide: v2 to v3

## 1. Introduction

This document outlines the migration process from v2 to v3 of our library. The v3 release introduces significant architectural changes aimed at improving performance, reducing memory consumption, and providing a more robust foundation for future development. While these changes bring substantial benefits, they also necessitate some adjustments in how the library is used. This guide will walk you through the key differences and provide steps to update your projects.

## 2. Key Changes in v3

The most significant architectural shift in v3 is the move away from `go/packages` as the primary mechanism for loading and analyzing Go source code.

### 2.1. Departure from `go/packages`

In v2, `go/packages` was used to load entire package graphs, including all direct and indirect dependencies. While comprehensive, this approach often led to:

*   **Performance bottlenecks:** Loading and parsing all dependencies, even those not directly relevant to the task at hand, could be time-consuming for large projects.
*   **High memory usage:** Holding the ASTs and type information for an entire project's dependency graph in memory was resource-intensive.

To address these issues, v3 introduces a more granular and on-demand approach to package loading.

### 2.2. New Package Loading and Analysis

V3 implements a custom package loading mechanism tailored to our specific needs. This allows for:

*   **Selective loading:** Only packages directly requested or essential for the current analysis are fully parsed and type-checked.
*   **Optimized caching:** More intelligent caching strategies are employed to reduce redundant work.

### 2.3. Lazy Loading of Dependencies

Dependencies are now loaded lazily. This means that information about a package is only loaded when it is explicitly requested or required for a specific analysis step. This significantly speeds up initial processing and reduces the memory footprint for many common use cases.

### 2.4. Limited Type Information for Indirect Dependencies

A key consequence of lazy loading and the move away from `go/packages`' full graph traversal is that v3 will, by default, have **limited type information for indirectly imported packages**.

*   **Direct Imports:** Packages directly imported by the code being analyzed will generally have full type information available.
*   **Indirect Imports:** For packages that are dependencies of dependencies (and so on), v3 may only load their exported API signatures without fully type-checking their internals or resolving all their own dependencies.

This trade-off is crucial for achieving the performance and memory goals of v3. While it might seem like a limitation, for many static analysis tasks, full type information for distant dependencies is not strictly necessary.

## 3. Impact on Users

These changes will primarily affect how you interact with packages and their type information, especially for dependencies.

*   **API Changes:** The API for querying package information and type details may have changed to reflect the new loading model.
*   **Adapting Code:** You may need to update your code to handle cases where type information for certain packages (especially indirect ones) is less complete than in v2. This might involve:
    *   Checking for the availability of type information before using it.
    *   Relying more on syntactic analysis for certain checks where full type resolution was previously assumed.
*   **Potential Challenges:**
    *   Analyses that heavily relied on deep inspection of transitive dependency types might require new strategies.
    *   Understanding when and why type information might be limited will be important.

## 4. Migration Steps

Migrating your project from v2 to v3 will involve the following general steps:

1.  **Update Library Version:** Modify your project's dependencies to use the v3 release of the library.
2.  **Consult API Documentation:** Familiarize yourself with any changes to the public API, particularly functions related to package loading, type querying, and AST traversal.
3.  **Initial Compilation/Testing:** Attempt to build and run your project. Address any immediate compilation errors, which will likely point to direct API incompatibilities.
4.  **Review Analysis Logic:** Carefully review parts of your code that perform static analysis or rely on type information.
    *   Identify areas that might be affected by the limited type information for indirect dependencies.
    *   Consider if your analysis can be made more resilient to incomplete type data or if it can be refocused on directly relevant packages.
5.  **Adapt to New Patterns:**
    *   If your tools need to force deeper loading for specific indirect dependencies, look for new API mechanisms in v3 that might allow this (though this might come at a performance cost).
    *   Implement checks for the presence of type information before accessing it to avoid nil-pointer dereferences or unexpected behavior.
6.  **Testing:** Thoroughly test your project, paying close attention to edge cases and scenarios involving complex dependency graphs.

## 5. Core Development Task List for v3

The development of v3 involved the following key tasks, which provide context for the changes you're observing:

*   **Design new API for package loading:** This API was designed from the ground up to support lazy loading and manage scenarios with limited type information for indirect dependencies.
*   **Implement the new package loading mechanism:** This involved creating a custom loader that is more efficient and selective than the `go/packages` approach for our use cases.
*   **Update core analysis logic to work with the new package representation:** All internal analysis components were updated to understand and operate with the potentially less detailed information available for some packages.
*   **Develop strategies for handling limited type information:** This included:
    *   Implementing conservative analysis techniques where exact types are unknown.
    *   Providing ways for users to potentially hint or force deeper loading when absolutely necessary (though this is generally discouraged for performance reasons).
*   **Write comprehensive documentation for the new API and migration process:** This document is part of that effort.
*   **Test thoroughly with various project structures and dependencies:** Extensive testing was performed to ensure stability and identify common issues across different project types.

## 6. Conclusion

Migrating to v3 offers significant advantages in terms of performance and resource usage. While the changes, particularly regarding package loading and type information, may require some adjustments to your existing code, the long-term benefits of a faster and more scalable analysis library are substantial. We encourage you to carefully review this guide and the API documentation. Support will be available to assist users during this transition period.

## Appendix: Technical Details

### Appendix A: Package Loader Design without `go/packages`

The custom package loader in v3 is a cornerstone of its performance improvements and reduced memory footprint. It replaces the broader approach of `go/packages` with a more targeted and efficient system.

*   **Integration of `internal/loader/loader.go` and `internal/loader/lazyload/`:**
    *   `internal/loader/loader.go` likely serves as the primary entry point and coordinator for package loading requests. It handles the initial resolution of package paths and orchestrates the loading process.
    *   `internal/loader/lazyload/` would contain the specific mechanisms for deferred loading. This includes stubs or placeholders for packages that are not yet fully loaded, and the logic to trigger full loading when an analysis path requires deeper information.
    *   The interaction involves the main loader identifying a dependency and, instead of immediately parsing it, registering a lazy loader task. This task is only executed if the specific package's details (beyond its exported API, for example) are requested.

*   **AST-based Package Information Gathering:**
    *   As an alternative to `go list` (which often shells out and can be slow), v3 directly parses Go source files to gather necessary metadata.
    *   **Import Path Resolution:** The loader must be able to locate packages based on import paths. This involves searching `GOROOT`, `GOPATH`, and vendor directories, similar to the Go compiler's own resolution logic, but implemented within the library.
    *   **Parsing:** `go/parser` is used to parse source files into ASTs. However, this is done selectively. For indirectly referenced packages, parsing might be limited to only exported declarations.
    *   **Metadata Extraction:** From the ASTs, the loader extracts crucial information: package name, import paths of its dependencies, exported type names, function signatures, and variable/constant declarations. This metadata is used to build a partial view of the package graph.

*   **Specifics of Lazy Loading and Caching Integration:**
    *   **Lazy Loading:** When a package `P1` imports `P2`, and `P2` imports `P3`:
        *   If `P1` is the target of analysis, its source is fully parsed.
        *   `P2` might be fully parsed if its types are directly used in `P1`'s API.
        *   `P3` might initially only have its existence and exported API (potentially just names) noted. If `P2`'s logic (which might be analyzed if `P1` depends on it deeply) references `P3`'s internals, then `P3` would be more fully parsed at that point.
    *   **Caching:**
        *   Parsed ASTs and extracted metadata are cached in memory.
        *   The cache is keyed by package path and potentially by a hash of the source file content to handle changes.
        *   Cache eviction policies might be necessary for very large analyses to manage memory.
        *   The granularity of caching is important: caching individual file ASTs, package-level metadata, or even specific resolved types can optimize subsequent accesses.

### Appendix B: Analysis Strategies in `AnalyzeOptionsV3` with Limited Type Information

`AnalyzeOptionsV3` (or a similar configuration structure) will need to guide the analysis engine on how to behave given the potential for incomplete type information.

*   **Resolving External Package Types and Embedded Structs via Lazy AST Loading:**
    *   **Approach:** When encountering a type from an external package (e.g., `extpkg.TypeName`) or an embedded struct from another package, the analyzer will first check if this information is already loaded.
    *   If not, it will trigger the lazy loader to parse the necessary parts of `extpkg`. This might involve parsing only the specific file defining `TypeName` or the minimal set of files to understand the struct's definition.
    *   **Limitations:**
        *   Cyclic dependencies or very complex type relationships across package boundaries can make precise lazy loading difficult.
        *   The decision of "how much" to load for an external type can be heuristic. Loading too little might miss important details (e.g., embedded fields in an external struct that themselves come from yet another package). Loading too much negates the benefits of lazy loading.
        *   Full type checking (a la `go/types`) for these lazily loaded ASTs might not be performed to save time; instead, the system might rely on the syntactic structure.

*   **Heuristic AST-based Methods for `TextUnmarshaler`/`TextMarshaler` Interface Detection:**
    *   Since full type information (and thus, method sets from `go/types`) might not be available for all types, especially those from indirect dependencies, detecting interface satisfaction like `encoding.TextUnmarshaler` or `encoding.TextMarshaler` requires AST-level heuristics.
    *   **Detection Strategy:**
        1.  Identify the type in question (e.g., a struct field's type).
        2.  If the type is from a package that is not fully loaded, trigger a lazy load of the AST for the file defining that type.
        3.  Scan the AST for method declarations associated with that type.
        4.  Check if methods with the signatures `UnmarshalText(text []byte) error` and `MarshalText() (text []byte, err error)` exist.
    *   **Limitations:** This approach won't understand interfaces satisfied by embedded types if the embedding itself is complex or spans multiple unloaded packages. It also won't resolve method sets as robustly as `go/types`. Alias types pointing to types with these methods might also be missed if not carefully handled.

*   **Reiteration of How Enums and Basic Fields are Extracted from AST:**
    *   **Basic Fields (int, string, bool, etc.):**
        *   For struct types, iterate through `ast.Field` nodes in `ast.StructType`.
        *   The field type is an `ast.Expr`. For basic types, this will typically be an `ast.Ident` (e.g., `string`, `int`).
        *   The analyzer maps these identifiers to known basic types. Comments associated with fields (`ast.Field.Doc`) are extracted for descriptions.
    *   **Enums (Const Blocks):**
        *   Enums are typically represented as a block of constants (`ast.GenDecl` with `tok.CONST`).
        *   The type of the enum is often inferred from the first constant declared with an explicit type (e.g., `const MyEnum MyType = iota`), or it might be an untyped constant that gets its type upon usage.
        *   The analyzer looks for `ast.ValueSpec` nodes. The `Doc` field of `ast.ValueSpec` or `ast.GenDecl` provides comments.
        *   Values can be literal or computed via `iota`.
        *   The challenge is reliably associating a set of constants with a specific "enum" type without full type information. Often, naming conventions or sequential declaration in the AST are used as heuristics.

### Appendix C: Detailed Technical Task List for v3 Migration (Internal)

This breaks down the high-level tasks from Section 5 into more granular steps for the library's internal development.

1.  **Design New API for Package Loading (`PackageLoaderV3`)**
    *   Sub-task: Define core interfaces for requesting package info (e.g., `Load(path string) (*PackageInfo, error)`).
    *   Sub-task: Specify `PackageInfo` structure: ASTs (partial or full), list of dependencies, exported types/functions, errors.
    *   Sub-task: Design configuration options for the loader (e.g., load depth, custom search paths).
    *   *Technical Note:* Focus on minimalism and extensibility. Ensure thread-safety if concurrent loading is planned.

2.  **Implement the New Package Loading Mechanism**
    *   Sub-task: Develop import path resolution logic (GOROOT, GOPATH, vendor).
        *   *Point of Attention:* Mimic Go's resolution order accurately. Handle module mode vs. GOPATH mode if necessary.
    *   Sub-task: Implement selective AST parsing (`go/parser`).
        *   *Technical Note:* Create functions to parse only exported declarations vs. full file.
    *   Sub-task: Implement metadata extraction from ASTs (package name, imports, exports).
    *   Sub-task: Build the lazy loading infrastructure (`internal/loader/lazyload/`).
        *   *Point of Attention:* Manage states (unloaded, loading, loaded, error).
    *   Sub-task: Implement caching for parsed files and package metadata.
        *   *Technical Note:* Consider cache invalidation strategies (e.g., file modification times).

3.  **Update Core Analysis Logic**
    *   Sub-task: Identify all points where `go/packages.Package` was used.
    *   Sub-task: Adapt these points to use `PackageInfoV3` (or equivalent).
    *   Sub-task: Refactor type resolution logic to query `PackageInfoV3` and trigger lazy loading if necessary.
        *   *Point of Attention:* Clearly define how much information is "enough" for a given analysis step.

4.  **Develop Strategies for Handling Limited Type Information**
    *   Sub-task: Implement AST-based heuristics for common interface checks (e.g., `TextMarshaler`).
        *   *Technical Note:* Document limitations of these heuristics.
    *   Sub-task: For external type resolution, define rules for how deep to load (e.g., just the type definition, or related types too).
    *   Sub-task: Design mechanisms for users to provide "hints" if an analysis requires deeper type info for a specific indirect dependency (use with caution).
        *   *Point of Attention:* This should be an escape hatch, not standard practice.

5.  **Write Comprehensive Documentation**
    *   Sub-task: Document the new `PackageLoaderV3` API.
    *   Sub-task: Update all examples to use the new API.
    *   Sub-task: Create this migration guide (`migration-v2-to-v3.md`).
    *   Sub-task: Document common patterns for dealing with limited type information.

6.  **Test Thoroughly**
    *   Sub-task: Create unit tests for the new loader (path resolution, parsing, metadata).
    *   Sub-task: Create integration tests for analysis of projects with direct and indirect dependencies.
        *   *Technical Note:* Include tests with vendored dependencies and modules.
    *   Sub-task: Test performance and memory usage against v2 benchmarks.
    *   Sub-task: Test with diverse real-world open-source projects.

### Appendix D: Functional Requirements under v3 Architecture

This section discusses how core functional requirements are met in v3, given its architecture.

*   **int/bool/string fields:**
    *   **Realization:** Extracted directly from struct field definitions in the AST (`ast.Ident` for type). Comments on fields are used for descriptions.
    *   **Constraints:** No major constraints. This is syntactically straightforward.

*   **`no-xxx` flags for bools:**
    *   **Realization:** If a boolean field `EnableFeature` is found, a `no-enable-feature` flag can be inferred. This is a naming convention applied after field extraction.
    *   **Constraints:** Relies on consistent naming conventions.

*   **External package imports (e.g., `extpkg.MyType`):**
    *   **Realization:**
        1.  The import path `extpkg` is identified from the `import` declarations in the source AST.
        2.  The type `MyType` is referenced. The loader attempts to lazily load `extpkg`.
        3.  Initially, this might only load the AST of the file in `extpkg` that defines `MyType`.
        4.  Fields of `extpkg.MyType` are then extracted from this lazily-loaded AST.
    *   **Constraints:**
        *   If `MyType` itself embeds types from other packages not yet loaded, resolving these deeply can be complex and might be limited by a predefined depth to avoid loading too much.
        *   Full type compatibility (e.g., assignability) across complex external types might not be checkable without more extensive loading.

*   **Enums (especially from external packages):**
    *   **Realization:**
        1.  Similar to external types, if an enum type is from `extpkg.MyEnum`, `extpkg` is lazily loaded.
        2.  The AST for `extpkg` is scanned for `const` blocks.
        3.  Heuristics (e.g., `const ( MyEnumVal1 MyEnum = iota ... )`) are used to identify constants belonging to `MyEnum`.
    *   **Constraints:**
        *   Reliably grouping constants under a specific enum type without full `go/types` information can be challenging if conventions aren't clear (e.g., untyped constants used as enum values).
        *   Comments for enum values are extracted from the `ast.ValueSpec` or `ast.GenDecl`.

*   **Description retrieval (from comments):**
    *   **Realization:** `Doc` fields on `ast.Field`, `ast.TypeSpec`, `ast.ValueSpec`, `ast.GenDecl` nodes are the primary source.
    *   **Constraints:** No major constraints beyond the quality of comments in the source code itself.

*   **`flags.TextVar` support (types implementing `encoding.TextMarshaler`/`Unmarshaler`):**
    *   **Realization:**
        1.  When a field's type is encountered, the system checks if it's a known basic type.
        2.  If not, and it's a candidate for `TextVar` (e.g., a named type from the current package or an external one), the AST-based heuristic (see Appendix B) is used.
        3.  This involves lazily loading the AST for the type's definition (if not already available) and looking for `MarshalText` and `UnmarshalText` method signatures.
    *   **Constraints:**
        *   This is a heuristic. It won't understand complex interface satisfaction scenarios (e.g., via embedding an unexported type that satisfies the interface, or complex type aliases).
        *   It relies on exact signature matches. Minor variations might be missed.
        *   Performance: Repeatedly parsing external packages for this check could be slow if not cached aggressively. The goal is that once a type is checked, its interface satisfaction status is cached.

This detailed appendix aims to provide deeper insight into the technical considerations and design choices behind the v3 migration.
