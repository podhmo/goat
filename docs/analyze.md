# Missing TODOs and Unimplemented Features

## From Code Comments (TODO/FIXME)

### `internal/analyzer/analyzer.go`

- Line 61: `TODO: Make this configurable if needed`

### `internal/analyzer/options_analyzer.go`

- Line 169: `TODO: Add support for other non-goat tags if needed, or consolidate all tag parsing.`

### `internal/analyzer/run_func_analyzer.go`

- Line 59: `TODO: Analyze return type (expecting \`error\`)`

### `internal/codegen/main_generator_test.go`

- Line 127: `TODO: anothercmd.RunWithOptions(options)`
- Line 181: `TODO: dataproc.ProcessData(options)`
- Line 228: `TODO: task.DoSomething(*options)`
- Line 268: `TODO: control.SetMode(options)`
- Line 336: `TODO: setup.Configure(options)`
- Line 373: `TODO: featureproc.ProcessWithFeature(options)`
- Line 761: `TODO: mytool.RunMyTool(options)`
- Line 790: `TODO: othertool.AnotherTool()`

### `internal/interpreter/interpreter.go`

- Line 45: `TODO: This is a very simplified interpreter.`
- Line 108: `TODO: handle direct literals as defaults)`

### `internal/metadata/types.go`

- Line 12: `TODO: For knowing where to replace main func content`

### `internal/utils/astutils/astutils.go`

- Line 27: `TODO: Differentiate array and slice if necessary`
- Line 31: `TODO: Add other types as needed (FuncType, ChanType, InterfaceType, etc.)`
- Line 136: `TODO: Could try to resolve other idents if we had a symbol table`
- Line 154: `TODO: Add *ast.CompositeLit for simple slice/map literals if needed directly as default`

### `internal/utils/astutils/astutils_test.go`

- Line 192: `TODO: Add char, negative numbers, etc.`

No FIXME comments found that match the pattern `//.*FIXME`. If there were any not matching this specific pattern but present in the code, they wouldn't be listed here.

## From Documentation (README.md & docs/ja/spec.md)

This section lists features mentioned in the documentation that may not be fully implemented yet or could be considered outstanding tasks.

### README.md Analysis

- **`init` subcommand:** Currently a placeholder. The README states: "Currently, this command is a placeholder and will print a 'TODO: init subcommand' message. Future functionality might involve scaffolding a new `goat`-compatible `main.go` file or project structure."
- **Development Section:** The README has a placeholder: "(Details on building `goat` itself, running tests, etc. will go here.)"
- **Contributing Section:** The README has a placeholder: "(Contribution guidelines will go here.)"
- **License Section:** The README has a placeholder: "(License information will go here, e.g., MIT License.)"

### docs/ja/spec.md Analysis

- **ヘルプメッセージの生成と表示 (初期目標):** The spec mentions "初期目標: ヘルプメッセージを生成し表示 (`internal/help`)". While help messages are generated, ensuring full alignment with all spec details (e.g., context.Context variant in run func) could be an ongoing task.
- **新しい `main()` 関数のGoコード生成 (将来):** The spec states "将来: 新しい `main()` 関数のGoコードを生成 (`internal/codegen`)." This is the core functionality of `emit`.
  - フラグ定義 (`flag` パッケージ利用)
  - 環境変数読み込み
  - デフォルト値設定
  - Enumバリデーション
  - `run` 関数の呼び出しとエラーハンドリング
- **生成したコードで既存の `main()` 関数を上書き (将来):** Spec: "将来: 生成したコードで既存の `main()` 関数を上書き (`internal/codegen`)." This is also part of the `emit` command.
- **Enumバリデーション (将来):** Spec: "ヘルプメッセージに選択肢として表示。将来的にはバリデーションにも使用。" and under CLI features: "Enum値のバリデーション (指定された値以外が渡されたらエラー)."
- **`goat` コマンド自体のオプション (将来的に):**
  - `-main <main_func_name>`: 上書き対象の `main` 関数名を指定。
  - `-output <output_file.go>`: 生成結果を別ファイルに出力。
- **`run` 関数の `context.Context` サポート:** The spec mentions `run(ctx context.Context, options MyOptions) error` as a possible signature. Verification of full support in codegen and help messages.
- **サポートされる型 (初期):** Spec lists `string`, `int`, `bool`, `float64`, `[]string`. Verification of full support for all these types and their pointer variants in all subcommands (`emit`, `scan`, `help-message`).
