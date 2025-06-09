# v3 Migration: Technical Architecture and Development Guide

## 1. v3 Architecture Overview

The v3 release introduces significant architectural changes primarily aimed at improving performance and reducing memory consumption compared to v2. The core design goals are to enable faster analysis of large Go projects and to minimize the memory footprint, especially during complex analysis tasks.

To achieve these objectives, v3 incorporates several key technical decisions:

*   **Departure from `go/packages`:** Unlike v2, which relied on `go/packages` to load entire package graphs (including all direct and indirect dependencies), v3 implements a custom package loading mechanism. The `go/packages` approach, while comprehensive, often led to performance bottlenecks by loading and parsing all dependencies, and high memory usage due to holding ASTs and type information for the entire dependency graph. V3's custom loader provides more granular control.

*   **Lazy Loading of Dependencies:** Dependencies are loaded lazily. Information about a package is only fully processed when it is explicitly requested or essential for a specific analysis step. This significantly speeds up initial processing and reduces the memory footprint for many common use cases by avoiding the upfront cost of loading and parsing potentially unused parts of the dependency tree.

*   **Limited Type Information for Indirect Dependencies (AST-centric analysis):** A key consequence of lazy loading and the custom loader is that v3, by default, operates with limited type information for indirectly imported packages.
    *   **Direct Imports:** Packages directly imported by the code being analyzed will generally have full type information available.
    *   **Indirect Imports:** For packages that are dependencies of dependencies, v3 may only load their exported API signatures or perform AST-level analysis without fully type-checking their internals or resolving all their own dependencies.
    This trade-off is crucial for achieving the performance and memory goals. For many static analysis tasks, full type information for distant dependencies is not strictly necessary, and AST-based analysis can often provide sufficient insights.

These foundational changes necessitate a different approach to how package information is accessed and utilized within the analysis tools built on this library.

## 2. Package Loader Design (v3)

The custom package loader in v3 is a cornerstone of its performance improvements and reduced memory footprint, replacing the broader approach of `go/packages` with a more targeted and efficient system. This section details its design, drawing from what was previously "Appendix A."

*   **Core Components and Flow:**
    *   The package loading mechanism is primarily implemented within `internal/loader/loader.go` and `internal/loader/lazyload/`.
    *   `internal/loader/loader.go` likely serves as the primary entry point and coordinator for package loading requests. It handles the initial resolution of package paths and orchestrates the loading process.
    *   `internal/loader/lazyload/` contains the specific mechanisms for deferred loading. This includes creating stubs or placeholders for packages that are not yet fully loaded and the logic to trigger full loading when an analysis path requires deeper information.
    *   The interaction involves the main loader identifying a dependency and, instead of immediately parsing its entire AST and type information, registering a lazy loader task. This task is only executed if the specific package's details (beyond its exported API, for example) are requested by the analysis engine.

*   **AST-based Package Information Gathering:**
    *   As an alternative to `go list` (which often shells out and can be slow), v3 directly parses Go source files using `go/parser` to gather necessary metadata. This parsing is selective. For indirectly referenced packages, parsing might be limited to only exported declarations or even just import statements.
    *   **Import Path Resolution:** The loader must be able to locate packages based on import paths. This involves searching `GOROOT`, `GOPATH` (or module resolution paths), and vendor directories, similar to the Go compiler's own resolution logic, but implemented within the library.
    *   **Metadata Extraction:** From the ASTs, the loader extracts crucial information: package name, import paths of its dependencies, exported type names, function signatures, and variable/constant declarations. This metadata is used to build a partial, on-demand view of the package graph.

*   **Lazy Loading Specifics:**
    *   When a package `P1` imports `P2`, and `P2` imports `P3`:
        *   If `P1` is the target of analysis, its source is fully parsed.
        *   `P2` might be fully parsed if its types are directly used in `P1`'s API or if the analysis requires traversing into `P2`.
        *   `P3` might initially only have its existence and exported API (potentially just names or import spec) noted. If `P2`'s logic (which might be analyzed if `P1` depends on it deeply) references `P3`'s internals, then `P3` would be more fully parsed at that point. The depth and breadth of parsing for `P3` depend on the specific analysis query.

*   **Caching Strategy:**
    *   Parsed ASTs (partially or fully) and extracted metadata are cached in memory.
    *   The cache is typically keyed by package path and potentially by a hash of the source file content or modification timestamp to handle changes.
    *   Cache eviction policies might be necessary for very large analyses to manage memory, though the primary goal of lazy loading is to reduce the initial peak memory requirement.
    *   The granularity of caching is important: caching individual file ASTs, package-level metadata, or even specific resolved types can optimize subsequent accesses.

*   **Comparison with v2 Loader (`go/packages` based):**
    *   **v2:** Loaded the entire dependency graph upfront, leading to high initial latency and memory usage for large projects. Provided comprehensive type information for all packages.
    *   **v3:** Loads information on demand. Significantly lower initial latency and memory usage. Provides full type information primarily for directly relevant packages, with more limited (often AST-based) information for indirect dependencies.
    *   **Advantages of v3 Loader:** Performance (speed and memory), flexibility in defining scope of analysis.
    *   **Challenges of v3 Loader:** Requires the analysis engine to be tolerant of incomplete type information for some parts of the program. Heuristics may be needed where full type resolution was previously relied upon.

## 3. Analyzer Design (v3)

Given v3's package loader design, which emphasizes lazy loading and potentially limited type information for indirect dependencies, the analysis engine (`AnalyzeOptionsV3` or similar configuration structures guiding it) must adopt specific strategies. This section outlines these strategies, incorporating and expanding on "Appendix B."

*   **Role of `AnalyzeOptionsV3` (Conceptual):**
    *   This configuration structure guides the analysis engine on how to behave with potentially incomplete type information.
    *   It might include flags or settings to control the depth of analysis, how aggressively to load indirect dependencies (if at all), and which heuristics to apply.

*   **Resolving External Package Types and Embedded Structs via Lazy AST Loading:**
    *   **Approach:** When the analyzer encounters a type from an external package (e.g., `extpkg.TypeName`) or an embedded struct from another package, it first checks if this information is already loaded and sufficiently detailed.
    *   If not, it requests the package loader to (lazily) load the necessary parts of `extpkg`. This might involve parsing only the specific file defining `TypeName` or the minimal set of files to understand the struct's definition, including its fields.
    *   **Limitations:**
        *   Cyclic dependencies or very complex type relationships across package boundaries can make precise lazy loading difficult or computationally intensive if deep loading is triggered too often.
        *   The decision of "how much" to load for an external type can be heuristic. Loading too little might miss important details (e.g., embedded fields in an external struct that themselves come from yet another package). Loading too much negates some benefits of lazy loading.
        *   Full type checking (a la `go/types`) for these lazily loaded ASTs might not always be performed to save time and resources; instead, the system might rely on the syntactic structure and directly extracted information from the AST.

*   **Heuristic AST-based Methods for Interface Detection (e.g., `TextUnmarshaler`/`TextMarshaler`):**
    *   Since full type information (and thus, method sets from `go/types`) might not be available for all types, especially those from indirect dependencies or types defined in packages loaded with limited scope, detecting interface satisfaction (e.g., for `encoding.TextUnmarshaler` or `encoding.TextMarshaler`) often requires AST-level heuristics.
    *   **Detection Strategy:**
        1.  Identify the type in question (e.g., a struct field's type).
        2.  If the type is from a package that is not fully loaded or for which type information is partial, trigger a lazy load of the AST for the file defining that type (if not already parsed).
        3.  Scan the AST for method declarations associated with that type.
        4.  Check if methods with the required signatures (e.g., `UnmarshalText(text []byte) error` and `MarshalText() (text []byte, err error)`) exist syntactically.
    *   **Limitations:**
        *   This approach won't understand interfaces satisfied by embedded types if the embedding itself is complex or spans multiple unloaded/partially-loaded packages (e.g., if an embedded anonymous field provides the method).
        *   It won't resolve method sets as robustly as `go/types` (e.g., promotions of methods from embedded fields might be missed if the embedded field's type itself is not fully resolved).
        *   Alias types pointing to types with these methods might also be missed if not carefully handled by tracking type definitions through `ast.TypeSpec`.

*   **Extraction of Enums, Basic Fields, and Descriptions from AST:**
    *   **Basic Fields (int, string, bool, etc.):**
        *   Extraction relies on iterating through `ast.Field` nodes in `ast.StructType` and identifying field types via `ast.Ident` (e.g., `string`, `int`).
        *   **v3 Constraints Impact:** This AST-based method functions effectively under v3's constraints for currently loaded packages, as it doesn't require deep type resolution beyond recognizing standard identifiers. Thus, for direct fields within a loaded struct's AST, this remains robust.
        *   Comments associated with fields (`ast.Field.Doc`) are extracted for descriptions.
    *   **Enums (Const Blocks):**
        *   Enums are typically represented as a block of constants (`ast.GenDecl` with `tok.CONST`). The analyzer looks for `ast.ValueSpec` nodes.
        *   Values can be literal or computed via `iota`. Comments are extracted from `Doc` fields.
        *   **v3 Constraints Impact:** The primary challenge is reliably associating a set of constants with a specific "enum" type, especially without full type information for the constant's defined type (e.g., `const C MyEnumType = 1`).
            *   If `MyEnumType` is defined in an external, unloaded package, or is a type alias (e.g., `type MyEnumType somepkg.ActualEnum`), resolving this link purely from the constant's AST without deeper type analysis is difficult.
            *   Heuristics like naming conventions or sequential declaration in the AST are used, but their reliability is limited, particularly for enums from external packages where the type definition might not be (or only partially) loaded.
            *   Associating constants to an enum type defined via a type alias to a primitive (e.g., `type MyEnum int; const Val1 MyEnum = 1`) is harder if the definition of `MyEnum` itself is not fully resolved.
            *   Further investigation into more robust solutions, such as improved heuristics or potentially specific comment markers (though less ideal), may be needed if current methods prove insufficient (see related tasks in Section 4).
    *   **Description Retrieval:** `Doc` fields on `ast.Field`, `ast.TypeSpec` (for type definitions), `ast.ValueSpec` (for constants/enums), `ast.GenDecl` nodes are the primary source.
        *   **v3 Constraints Impact:** This is largely unaffected by v3 constraints as it directly uses information present in the AST of loaded files.

*   **Handling `no-xxx` flags for booleans:**
    *   If a boolean field `EnableFeature` is found via AST parsing, a corresponding `no-enable-feature` flag can be inferred. This is based on naming conventions applied after field extraction.
    *   **v3 Constraints Impact:** As this is primarily a naming convention and AST-based pattern matching task, the impact of limited type information under v3 is minimal.

*   **Functional Requirements (formerly Appendix D topics) - v3 Realization and Challenges:**

    *   **External package imports (e.g., `extpkg.MyType` field extraction):**
        *   _Realization:_ Fields of `extpkg.MyType` are extracted by the loader lazily parsing parts of `extpkg`'s AST, typically the file defining `MyType`.
        *   _Challenges & Limitations:_ The current shallow loading strategy (focused on the defining file) means that if `extpkg.MyType` embeds types from other, not-yet-loaded packages (e.g., `MyType` embeds `anotherpkg.AnotherType`), the fields from `anotherpkg.AnotherType` will not be resolved unless a deeper, transitive load of `anotherpkg` is explicitly triggered. The policy for how deep to load external ASTs is crucial and currently implicit. Direct fields of `extpkg.MyType` can often be extracted with high accuracy, but fields from deeply nested or transitively embedded types in unloaded packages will likely be missing. This is an area for future improvement (see "外部パッケージASTの読み込み深度制御" in Section 4).

    *   **Enums from external packages:**
        *   _Realization:_ Similar to local enums, relies on lazy loading the source AST of the external package and applying heuristics to identify relevant `const` blocks.
        *   _Challenges & Limitations:_ Subject to the same reliability issues as local enums when full type information is absent. Associating constants to a specific enum type becomes significantly harder if the enum type itself is an alias or defined in yet another package not covered by the current load depth. The AST heuristics may not be sufficient for complex cases.

    *   **`flags.TextVar` support (types implementing `encoding.TextMarshaler`/`Unmarshaler`):**
        *   _Realization:_ Relies on the heuristic AST-based method detection (matching method names and signatures) described earlier in the "Heuristic AST-based Methods for Interface Detection" subsection.
        *   _Challenges & Limitations:_ This heuristic is **highly limited** in v3 due to the lack of complete type information. It often fails to detect interfaces satisfied via:
            *   **Embedded types:** A very common pattern in Go (e.g., `type Foo struct { Bar embedpkg.Bar }` where `embedpkg.Bar` implements the interface). The heuristic may not inspect `embedpkg.Bar` if it's not fully loaded and its methods resolved.
            *   **Type aliases:** If a type alias points to a type that implements the interface, this link may not be followed without deeper type resolution.
        *   **v3 Realizability:** The practical utility of this specific heuristic for `TextVar` is currently low for achieving broad compatibility. More robust solutions are necessary. This is a key item for improvement, as detailed in Section 4's task: "`TextUnmarshaler`/`TextMarshaler` 判定の改善 (Improving `TextUnmarshaler`/`TextMarshaler` Detection)".

The analyzer in v3 must be designed to be resilient and adaptable, capable of working with varying levels of detail about the codebase it examines. It prioritizes speed and reduced memory by default, relying on deeper analysis only when explicitly configured or deemed essential by its internal logic.

## 4. v3開発における現状の課題と不足事項 (TODO) (Current Issues and TODOs in v3 Development)

This section outlines known issues, areas for ongoing improvement, and pending tasks related to the v3 architecture and its implementation. While the initial v3 development addressed many core items, the following areas require further attention:

**ローダー (Package Loader - `PackageLoaderV3` and `internal/loader/lazyload/`) に関する課題:**

*   **複雑なビルドタグやCGOへの対応 (Handling Complex Build Tags and CGO):**
    *   **考察:**
        *   The current `lazyload.BuildContext` includes `BuildTags`, but its effectiveness in complex CGO projects is untested.
        *   Need to verify how `go list` behaves with CGO, specifically if information like `CgoCFLAGS` needs to be incorporated into `PackageMetadata`.
        *   Cache strategies must account for different file sets resulting from varying build tags.
    *   **アクションアイテム:**
        *   Test loader with complex CGO projects.
        *   Investigate `go list` output for CGO-related information and determine if `PackageMetadata` needs extension.
        *   Design and implement cache key modifications or separate caches for build-tag-dependent file sets.

*   **モジュール外のローカルパッケージの解決 (Resolving Local Packages Outside Modules):**
    *   **考察:**
        *   Investigate `go list`'s capability to resolve relative paths like `../mylib` and how `lazyload.Locator` should handle these.
        *   Clarify the handling of `PackageMetadata.Module` when references point outside the main module root.
    *   **アクションアイテム:**
        *   Test `go list` with various relative path scenarios.
        *   Adapt `lazyload.Locator` and associated path resolution logic to correctly locate and load such packages.
        *   Define behavior for `PackageMetadata.Module` in these cases.

*   **ファイル変更検知とキャッシュ無効化戦略 (File Change Detection and Cache Invalidation):**
    *   **考察:**
        *   The current `loader.cache`, keyed by import path, doesn't detect file changes.
        *   Options include monitoring file system modification times (`mtime`) or comparing file hashes.
        *   The scope (in-process, disk cache) and duration of caches need definition.
    *   **アクションアイテム:**
        *   Evaluate trade-offs between `mtime` checking and hash comparison for change detection.
        *   Implement a chosen strategy for cache invalidation.
        *   Define policies for cache scope and eviction. Consider persistent caching options.

*   **Refine Load Depth Control:** Enhance mechanisms to more precisely control the depth and breadth of package loading, especially for transitive dependencies. This includes clearer APIs or configuration options to manage how much information is loaded for indirect dependencies based on analysis needs.

*   **Lazy Loading State Management:** Improve the robustness of state management (unloaded, loading, loaded, error) within `internal/loader/lazyload/`, particularly for concurrent access and complex dependency graphs (e.g., cycles, diamond dependencies).

*   **Selective Parsing Granularity:** Further optimize selective AST parsing (e.g., parsing only specific function bodies or type definitions) to minimize processing time for common analysis patterns.

**アナライザー (Analyzer - `AnalyzeOptionsV3`) に関する課題:**

*   **特定型パターンの解決精度 (Accuracy of Resolving Specific Type Patterns):**
    *   **ジェネリクス (Generics):**
        *   **考察:** How accurately can v3's AST-based analysis handle generic types? Challenges include resolving type parameters and identifying instantiated types. A decision is needed on whether to provide limited support or explicitly not resolve them.
        *   **アクションアイテム:** Research and experiment with AST-based analysis of generics. Define the scope of generic type support in v3 and document limitations.
    *   **複雑な型エイリアス (Complex Type Aliases):**
        *   **考察:** Assess the traceability of multi-level type aliases or aliases to types in external packages.
        *   **アクションアイテム:** Test current alias resolution capabilities. Improve tracking if necessary, possibly by deeper loading or more sophisticated AST traversal.
    *   **`interface{}` への型アサーション (Type Assertions to `interface{}`):**
        *   **考察:** Detecting type assertion patterns from AST is possible, but identifying the actual runtime type is challenging. What heuristics are feasible, or should this be unsupported?
        *   **アクションアイテム:** Explore potential AST-based heuristics for common assertion patterns. Define and document the limitations.

*   **`TextUnmarshaler`/`TextMarshaler` 判定の改善 (Improving `TextUnmarshaler`/`TextMarshaler` Detection):**
    *   **考察:**
        *   The current heuristic (exact method name and signature match) is limited, especially with methods provided via embedded types.
        *   Alternatives: Using marker comments like `//go:generate` (though this is for generation, not analysis), or explicit configuration files.
        *   Consider the priority of this feature in v3 and alternative solutions (e.g., handling in the generator side) if robust AST-based detection is unfeasible.
    *   **アクションアイテム:**
        *   Investigate techniques to identify methods on embedded types via AST.
        *   Evaluate the feasibility and utility of marker comments or configuration files for this specific interface detection.
        *   Make a decision on the future of this heuristic and document it.

*   **パフォーマンス測定と最適化 (PerformanceMeasurement and Optimization):**
    *   **考察:**
        *   Need to install profiling points in key processing steps (package loading, AST parsing, info extraction).
        *   Measure cache hit rates to evaluate and tune caching strategies.
        *   Potential for memory reduction and speed improvement by pruning unnecessary parts of ASTs.
    *   **アクションアイテム:**
        *   Integrate profiling tools (e.g., `pprof`).
        *   Implement metrics for cache performance.
        *   Experiment with AST pruning techniques.

*   **外部パッケージASTの読み込み深度制御 (Controlling Read Depth of External Package ASTs):**
    *   **考察:**
        *   Currently, the depth of reading external ASTs is implicit. Clearer criteria are needed (e.g., only direct type definitions, include related constants/variables).
        *   Consider adding parameters to `AnalyzeOptionsV3` to control this.
    *   **アクションアイテム:**
        *   Define default and configurable levels for external AST read depth.
        *   Implement controls in `AnalyzeOptionsV3` if deemed necessary.

*   **User Hints for Deeper Analysis:** If the "escape hatch" for users to hint at deeper loading for specific indirect dependencies is maintained, refine its API and clearly document its performance implications. The goal is to minimize its necessity.

**その他 (Others):**

*   **`internal/interpreter` および `internal/codegen` への影響 (Impact on `internal/interpreter` and `internal/codegen`):**
    *   **考察:**
        *   `interpreter`: How does limited type information in v3 affect existing evaluation logic, especially for constants and variables from external packages?
        *   `codegen`: If the structure or content of metadata from v3 (`metadata.CommandMetadata`) changes, code generation logic may need updates.
    *   **アクションアイテム:**
        *   Conduct a detailed review of `internal/interpreter` and `internal/codegen` to identify dependencies on full type information.
        *   Plan and implement necessary adaptations or refactoring.

*   **v2由来ロジックの見直し (Review of v2-derived Logic):**
    *   **考察:**
        *   Review the entire codebase for logic based on v2 assumptions (e.g., availability of complete type information) that conflict with v3 design principles.
        *   Areas expecting full type info need modification or functional re-evaluation under v3 constraints.
    *   **アクションアイテム:**
        *   Perform a systematic code audit to identify and document v2-era assumptions.
        *   Prioritize and refactor critical components to align with v3 architecture.

**Testing and Benchmarking (General):**

*   **Expand Test Coverage:**
    *   Develop more comprehensive unit tests for the package loader, focusing on edge cases in path resolution, parsing of malformed or unusual Go code, and metadata extraction.
    *   Increase integration test coverage for analysis of diverse project structures, including those with complex module dependencies, vendoring, and build tags (tying into CGO/build tag testing).
    *   Add specific tests for AST-based heuristics to measure their accuracy against type-checked results.
*   **Performance and Memory Benchmarking (General):**
    *   Establish a continuous benchmarking process.
    *   Profile and optimize performance and memory usage hotspots.
    *   Test against a wider variety of large, real-world open-source projects.

**Documentation and Tooling (General):**

*   **API Documentation:** Ensure `PackageLoaderV3` API and related structures are exhaustively documented, including behavior with limited type information and new control parameters.
*   **Developer/User Guides:**
    *   Update examples to showcase best practices.
    *   Provide detailed guidance on common patterns for dealing with limited type information.
*   **Debugging and Diagnostics:** Develop internal tooling or logging mechanisms.

**General Architectural Considerations (General):**

*   **Thread Safety:** Continuously verify and improve thread-safety.
*   **Error Reporting:** Enhance error reporting for clarity.

This list is not exhaustive and will evolve as v3 is used more extensively and new requirements emerge.

## Appendix

(Currently empty. This section can be used for supplementary information that doesn't fit neatly into the main sections, such as specific performance benchmarks or detailed data on heuristic accuracy if gathered later.)
