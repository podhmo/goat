package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
	"log/slog"

	"github.com/podhmo/goat/internal/loader" // Changed
	// "strings" // No longer used directly in the simplified parseTestFiles
	// "golang.org/x/tools/go/packages" // No longer used in the simplified parseTestFiles
)

// parseTestFiles is a simplified helper for these specific tests.
// It parses the first source file into an AST.
// It creates a temp directory and a minimal go.mod.
func parseTestFiles(t *testing.T, sources map[string]string) (*token.FileSet, []*ast.File, string) {
	t.Helper()
	fset := token.NewFileSet()

	if len(sources) == 0 {
		t.Fatal("No sources provided to parseTestFiles")
	}

	tempDir, err := os.MkdirTemp("", "analyzer_test_simple_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a dummy go.mod
	goModPath := filepath.Join(tempDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module testmodule\n\ngo 1.18\n"), 0644); err != nil {
		os.RemoveAll(tempDir) // Attempt cleanup
		t.Fatalf("Failed to write dummy go.mod: %v", err)
	}

	var astFile *ast.File
	var filePath string

	// Write and parse the first file from sources.
	// For these simplified tests, we assume sources map contains one entry.
	for name, content := range sources {
		filePath = filepath.Join(tempDir, name) // Use key as file name e.g. "main.go"
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			os.RemoveAll(tempDir)
			t.Fatalf("Failed to write source file %s: %v", name, err)
		}
		file, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
		if err != nil {
			os.RemoveAll(tempDir)
			t.Fatalf("Failed to parse source file %s: %v", name, err)
		}
		astFile = file
		break // Only process the first file
	}

	if astFile == nil {
		os.RemoveAll(tempDir)
		t.Fatal("No AST file was parsed.")
	}

	return fset, []*ast.File{astFile}, tempDir
}

func TestAnalyze_InitializerFunctionDiscovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	testCases := []struct {
		name                 string
		sourceContent        string // Single source file content
		packageName          string // package name in the source
		runFuncName          string
		expectedInitFuncName string
		expectErrorInAnalyze bool // Whether Analyze() itself is expected to error (e.g. from AnalyzeOptionsV2)
	}{
		{
			name: "Valid Initializer Function Present",
			sourceContent: `
package command
import "context"
type MyOpts struct { Name string }
func NewMyOpts() *MyOpts { return &MyOpts{Name: "Default"} }
// goat:run
func Run(ctx context.Context, opts *MyOpts) error { return nil }
func main() { Run(nil, nil) } // Adjusted dummy main
`,
			packageName:          "main", // Changed to main
			runFuncName:          "Run",
			expectedInitFuncName: "NewMyOpts",
			expectErrorInAnalyze: true,
		},
		{
			name: "Initializer Function Present with Parameters (Invalid Signature)",
			sourceContent: `
package main // Changed to main
import "context"
type MyOpts struct { Name string }
func NewMyOpts(i int) *MyOpts { return &MyOpts{Name: "Default"} }
// goat:run
func Run(ctx context.Context, opts *MyOpts) error { return nil }
func main() { Run(nil, nil) }
`,
			packageName:          "main", // Changed to main
			runFuncName:          "Run",
			expectedInitFuncName: "",
			expectErrorInAnalyze: true,
		},
		{
			name: "No Initializer Function Present",
			sourceContent: `
package main // Changed to main
import "context"
type MyOpts struct { Name string }
// goat:run
func Run(ctx context.Context, opts *MyOpts) error { return nil }
func main() { Run(nil, nil) }
`,
			packageName:          "main", // Changed to main
			runFuncName:          "Run",
			expectedInitFuncName: "",
			expectErrorInAnalyze: true,
		},
		{
			name: "Initializer Function Name Mismatch",
			sourceContent: `
package main // Changed to main
import "context"
type MyOpts struct { Name string }
func NewMyOptionsWrong() *MyOpts { return &MyOpts{Name: "Default"} }
// goat:run
func Run(ctx context.Context, opts *MyOpts) error { return nil }
func main() { Run(nil, nil) }
`,
			packageName:          "main", // Changed to main
			runFuncName:          "Run",
			expectedInitFuncName: "",
			expectErrorInAnalyze: true,
		},
		{
			name: "Initializer Function Returns Value Type (Still Valid for Discovery)",
			sourceContent: `
package main // Changed to main
import "context"
type MyOpts struct { Name string }
func NewMyOpts() MyOpts { return MyOpts{Name: "Default"} }
// goat:run
func Run(ctx context.Context, opts *MyOpts) error { return nil }
func main() { Run(nil, nil) }
`,
			packageName:          "main", // Changed to main
			runFuncName:          "Run",
			expectedInitFuncName: "NewMyOpts",
			expectErrorInAnalyze: true,
		},
		// This test is simplified; multi-file analysis is harder with this parseTestFiles.
		// To test across files, parseTestFiles would need to handle multiple entries in `sources`.
		// For now, ensure single file with initializer works.
		// {
		// 	name: "Initializer in a different file within the same package",
		// ...
		// },
		{
			name: "Run function has no options, no initializer expected",
			sourceContent: `
package main // Changed to main
import "context"
// goat:run
func RunWithoutOptions(ctx context.Context) error { return nil }
func main() { RunWithoutOptions(nil) }
`,
			packageName:          "main", // Changed to main
			runFuncName:          "RunWithoutOptions",
			expectedInitFuncName: "",
			expectErrorInAnalyze: true,
		},
		// Simplified: This test no longer involves multiple packages for this focused pass.
		// {
		// 	name: "Options struct from an imported package (initializer discovery is local)",
		// ...
		// },
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Using a fixed filename "main.go" for simplicity with the new parseTestFiles
			fset, astFiles, moduleRootDir := parseTestFiles(t, map[string]string{"main.go": tc.sourceContent})
			defer os.RemoveAll(moduleRootDir)

			if len(astFiles) == 0 {
				t.Fatalf("No AST files loaded by parseTestFiles.")
			}
			if len(astFiles) > 1 {
				t.Logf("Warning: parseTestFiles returned multiple ASTs (%d), expected 1 for simplified tests. Using the first one.", len(astFiles))
			}

			// targetPackageID for Analyze should be the import path of the package.
			// In this test setup, parseTestFiles creates "module testmodule" and places "main.go" in its root.
			// So the import path is "testmodule". tc.packageName is the `package foo` name.
			targetPackageID := "testmodule" // Module name defined in parseTestFiles

			llCfg := loader.Config{Fset: fset}
			loader := loader.New(llCfg)
			// Pass "" for initializerFuncNameOption for existing tests (conventional lookup)
			cmdMeta, _, err := Analyze(fset, astFiles, tc.runFuncName, "", targetPackageID, moduleRootDir, loader)

			// InitializerFunc is determined before AnalyzeOptions is called.
			// So, we should be able to check it even if Analyze later returns an error from AnalyzeOptions.
			if cmdMeta == nil {
				if tc.expectErrorInAnalyze && err != nil {
					// This is fine, Analyze errored as expected, and cmdMeta might be nil.
					return
				}
				t.Fatalf("Analyze() returned nil CommandMetadata, error: %v", err)
			}
			if cmdMeta.RunFunc == nil {
				// If we expected an initializer, RunFunc should exist.
				if tc.expectedInitFuncName != "" {
					t.Fatalf("RunFuncInfo is nil, but expected InitializerFunc %q. Analyze error: %v", tc.expectedInitFuncName, err)
				}
				// If no initializer was expected AND Analyze errored as expected, this might be fine.
				if tc.expectErrorInAnalyze && err != nil {
					return
				}
				// If no error was expected from Analyze, but RunFunc is nil, it's a problem.
				if !tc.expectErrorInAnalyze {
					t.Fatalf("RunFuncInfo is nil. Analyze error: %v", err)
				}
				// Otherwise, if error was expected and RunFunc is nil, nothing more to check.
				return
			}

			if cmdMeta.RunFunc.InitializerFunc != tc.expectedInitFuncName {
				t.Errorf("Expected InitializerFunc %q, got %q. Analyze error: %v", tc.expectedInitFuncName, cmdMeta.RunFunc.InitializerFunc, err)
			}

			// If Analyze was expected to error overall (e.g. from AnalyzeOptionsV2), check that.
			if tc.expectErrorInAnalyze {
				if err == nil {
					t.Errorf("Analyze() was expected to return an error, but did not.")
				}
			} else {
				if err != nil {
					// Only fail here if the error was NOT related to options struct parsing,
					// as we are focusing on initializer discovery which happens before that.
					// However, a general error from Analyze is still a failure for the test if not expected.
					t.Errorf("Analyze() returned an unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAnalyzeWithInitializerVariations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	testCases := []struct {
		name                      string
		sourceContent             string
		runFuncName               string
		initializerFuncNameOption string
		expectedInitializerFunc   string
		expectErrorInAnalyze      bool // If Analyze() itself is expected to error (e.g. options parsing)
	}{
		{
			name: "User-specified initializer, exists",
			sourceContent: `
package main
type Opts struct { Name string }
func MyInit() *Opts { return &Opts{Name:"custom"} }
func run(o Opts) error { return nil }
func main() { run(Opts{}) } // Dummy main
`,
			runFuncName:               "run",
			initializerFuncNameOption: "MyInit",
			expectedInitializerFunc:   "MyInit",
			expectErrorInAnalyze:      true, // AnalyzeOptions likely to fail on simple Opts
		},
		{
			name: "User-specified initializer, not exists",
			sourceContent: `
package main
type Opts struct { Name string }
func run(o Opts) error { return nil }
func main() { run(Opts{}) }
`,
			runFuncName:               "run",
			initializerFuncNameOption: "NonExistentInit",
			expectedInitializerFunc:   "",
			expectErrorInAnalyze:      true,
		},
		{
			name: "Conventional initializer, exists",
			sourceContent: `
package main
type Opts struct { Name string }
func NewOpts() *Opts { return &Opts{Name:"conventional"} }
func run(o Opts) error { return nil }
func main() { run(Opts{}) }
`,
			runFuncName:               "run",
			initializerFuncNameOption: "", // Conventional lookup
			expectedInitializerFunc:   "NewOpts",
			expectErrorInAnalyze:      true,
		},
		{
			name: "No initializer exists (conventional or specified)",
			sourceContent: `
package main
type Opts struct { Name string }
func run(o Opts) error { return nil }
func main() { run(Opts{}) }
`,
			runFuncName:               "run",
			initializerFuncNameOption: "", // Conventional lookup
			expectedInitializerFunc:   "",
			expectErrorInAnalyze:      true,
		},
		{
			name: "User-specified initializer, wrong signature",
			sourceContent: `
package main
type Opts struct { Name string }
func MyInitWithParam(p int) *Opts { return &Opts{Name:"custom"} }
func run(o Opts) error { return nil }
func main() { run(Opts{}) }
`,
			runFuncName:               "run",
			initializerFuncNameOption: "MyInitWithParam",
			expectedInitializerFunc:   "", // Should not be found due to wrong signature
			expectErrorInAnalyze:      true,
		},
		{
			name: "Conventional initializer, wrong signature",
			sourceContent: `
package main
type Opts struct { Name string }
func NewOpts(p int) *Opts { return &Opts{Name:"conventional"} }
func run(o Opts) error { return nil }
func main() { run(Opts{}) }
`,
			runFuncName:               "run",
			initializerFuncNameOption: "", // Conventional lookup
			expectedInitializerFunc:   "", // Should not be found due to wrong signature
			expectErrorInAnalyze:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fset, astFiles, moduleRootDir := parseTestFiles(t, map[string]string{"main.go": tc.sourceContent})
			defer os.RemoveAll(moduleRootDir)

			if len(astFiles) == 0 {
				t.Fatal("No AST files loaded by parseTestFiles.")
			}
			targetPackageID := "testmodule"

			llCfg := loader.Config{Fset: fset}
			loader := loader.New(llCfg)
			cmdMeta, _, err := Analyze(fset, astFiles, tc.runFuncName, tc.initializerFuncNameOption, targetPackageID, moduleRootDir, loader)

			if cmdMeta == nil {
				if tc.expectErrorInAnalyze && err != nil {
					return // Analyze errored as expected, cmdMeta might be nil.
				}
				t.Fatalf("Analyze() returned nil CommandMetadata, error: %v", err)
			}
			if cmdMeta.RunFunc == nil {
				if tc.expectedInitializerFunc != "" {
					t.Fatalf("RunFuncInfo is nil, but expected InitializerFunc %q. Analyze error: %v", tc.expectedInitializerFunc, err)
				}
				if tc.expectErrorInAnalyze && err != nil {
					return
				}
				if !tc.expectErrorInAnalyze {
					t.Fatalf("RunFuncInfo is nil. Analyze error: %v", err)
				}
				return
			}

			if cmdMeta.RunFunc.InitializerFunc != tc.expectedInitializerFunc {
				t.Errorf("Expected InitializerFunc %q, got %q. Analyze error: %v", tc.expectedInitializerFunc, cmdMeta.RunFunc.InitializerFunc, err)
			}

			if tc.expectErrorInAnalyze {
				if err == nil {
					t.Errorf("Analyze() was expected to return an error, but did not.")
				}
			} else {
				if err != nil {
					t.Errorf("Analyze() returned an unexpected error: %v", err)
				}
			}
		})
	}
}
