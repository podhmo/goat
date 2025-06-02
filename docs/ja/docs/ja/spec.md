# Goat - 仕様概要 (v0.1 - 初期構想)

## 1. コアコンセプト

- `go generate` と連携し、Goのソースコード (`main.go` 等) を静的解析してコマンドラインインターフェース (CLI) を自動生成するツール。
- ユーザーは規約に沿った `run` 関数と `Options` 構造体を定義する。
- `goat` はASTを直接解析・操作し、リフレクションや `go run` を使用しない。

## 2. 基本的な動作フロー

1.  ユーザーが `//go:generate goat [options] <target_file.go>` のようなコメントをソースファイルに記述。
2.  `go generate` 実行時に `goat` コマンドが起動。
3.  `goat` は指定されたターゲットファイルをロードし、ASTを構築 (`go/parser`)。
4.  ASTを解析 (`internal/analyzer`):
    -   `run` 関数 (デフォルト名: `run`、`-run` オプションで変更可能) を特定。
        -   シグネチャ: `run(options MyOptions) error` または `run(ctx context.Context, options MyOptions) error`。
        -   ドキュメントコメントを抽出し、コマンド全体のヘルプメッセージの元とする。
    -   `run` 関数の引数である `Options` 構造体を特定。
        -   フィールド名、型、コメント、構造体タグ (`env` 等) を抽出。
5.  `Options` 構造体の初期化関数 (デフォルト名: `NewOptions`、`-initializer` オプションで変更可能) を解釈 (`internal/interpreter`):
    -   `goat.Default()` や `goat.Enum()` マーカー関数の呼び出しを検出し、引数を評価。
    -   デフォルト値やEnumの選択肢をメタデータに反映。
6.  抽出・解釈されたメタデータ (`internal/metadata`) を元に処理を実行:
    -   **初期目標:** ヘルプメッセージを生成し表示 (`internal/help`)。
    -   **将来:** 新しい `main()` 関数のGoコードを生成 (`internal/codegen`)。
        -   フラグ定義 (`flag` パッケージ利用)。
        -   環境変数読み込み。
        -   デフォルト値設定。
        -   Enumバリデーション。
        -   `run` 関数の呼び出しとエラーハンドリング。
    -   **将来:** 生成したコードで既存の `main()` 関数を上書き (`internal/codegen`)。

## 3. `Options` 構造体の規約

-   ユーザー定義の構造体であること。
-   エクスポートされたフィールドのみがCLIオプションとして扱われる。
-   **フィールドコメント:** 対応するCLIオプションのヘルプメッセージとなる。
-   **フィールド型:**
    -   ポインタ型でないフィールド: 必須オプションとして扱われる (`IsRequired = true`)。
    -   ポインタ型フィールド: オプショナルなオプションとして扱われる (`IsRequired = false`)。
    -   サポートされる型 (初期): `string`, `int`, `bool`, `float64` およびこれらのポインタ型、`[]string` 等。
-   **構造体タグ:**
    -   `env:"ENV_VAR_NAME"`: 対応する環境変数名を指定。

## 4. マーカー関数 (`github.com/podhmo/goat/goat` パッケージ)

-   `goat.Default[T](defaultValue T, enumConstraint ...[]T) T`:
    -   フィールドのデフォルト値を指定。
    -   `goat` インタプリタが `defaultValue` を抽出。
    -   オプションで `goat.Enum()` の結果を `enumConstraint`として渡し、ヘルプに反映。
-   `goat.Enum[T](values []T) []T`:
    -   フィールドが取りうる値のリスト (Enum) を指定。
    -   `goat` インタプリタが `values` を抽出。
    -   ヘルプメッセージに選択肢として表示。将来的にはバリデーションにも使用。

## 5. 「サブセット言語」の解釈 (＠ `Options` 初期化関数内)

-   目的: `goat.Default()` や `goat.Enum()` マーカー関数の呼び出しを安全かつ正確に検出・評価するため。
-   `goat` インタプリタは、`Options` 初期化関数内のASTを限定的に「実行」する。
-   **サポートする構文 (解釈対象):**
    -   リテラル値 (文字列、数値、真偽値、スライスリテラル等)。
    -   `goat` パッケージのマーカー関数呼び出し。
    -   構造体リテラル (`&MyOptions{...}`) でのフィールドへの代入。
    -   変数への代入 (`opts.Field = ...`)。
-   **サポートしない構文 (エラーまたは無視):**
    -   条件分岐 (`if`, `switch`)。
    -   ループ (`for`)。
    -   `goat` マーカー関数以外の複雑な関数呼び出しや変数解決。
    -   チャネル操作、goroutine起動など。

## 6. 生成されるCLIの機能 (目標)

-   標準的なフラグパース (`--flag value`, `-f value`)。
-   ヘルプメッセージ表示 (`-h`, `--help`)。
    -   コマンド説明 (from `run` 関数のコメント)。
    -   各オプションの説明、型、必須/任意、デフォルト値、Enum選択肢、環境変数名。
-   必須フラグのチェック。
-   環境変数からの値の読み込み (フラグより優先度は低い)。
-   Enum値のバリデーション (指定された値以外が渡されたらエラー)。
-   型に基づいた基本的なバリデーション (例: `int` 型フラグに文字列が渡されたらエラー)。

## 7. `goat` コマンド自体のオプション (例)

-   `goat -run <run_func_name> -initializer <init_func_name> <target_file.go>`
-   (将来的に) `-main <main_func_name>`: 上書き対象の `main` 関数名を指定。
-   (将来的に) `-output <output_file.go>`: 生成結果を別ファイルに出力。

## 8. 技術的制約・方針 (再掲)

-   AST (Abstract Syntax Tree) を直接操作。
-   `go run` コマンドやリフレクション (`reflect` パッケージ) は生成されるCLIのランタイムでは使用しない (goatツール自体はAST操作のために使う)。
-   `go/packages` はパフォーマンス懸念から使用を避ける (現状の方針)。
-   生成されるコードや参照するコードは、Goの型チェックが通る有効なものであること。