package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	// Required for module.EscapePath in setupMockGoModCache if that helper is used more broadly
	// and for direct use in TestGoModLocator_Locate_ExternalDependency
	"golang.org/x/mod/module"
)

// Helper to create a temporary directory structure for testing
func setupTestModule(t *testing.T, moduleName string, files map[string]string) string {
	tmpDir, err := os.MkdirTemp("", "locator_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create go.mod
	goModContent := "module " + moduleName + "\n\ngo 1.18\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	for relPath, content := range files {
		absPath := filepath.Join(tmpDir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			t.Fatalf("Failed to create parent dirs for %s: %v", relPath, err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", relPath, err)
		}
	}
	return tmpDir
}

func TestGoModLocator_Locate_RelativePaths(t *testing.T) {
	moduleName := "example.com/testmodule"
	files := map[string]string{
		"pkg/foo/foo.go":      "package foo\n\nfunc Foo() {}",
		"pkg/foo/foo_test.go": "package foo\n\nimport \"testing\"\n\nfunc TestFoo(t *testing.T) {}",
		"bar/bar.go":          "package bar",
	}
	testModuleDir := setupTestModule(t, moduleName, files)
	defer os.RemoveAll(testModuleDir)

	locator := &GoModLocator{workingDir: testModuleDir}

	// BuildContext can be minimal for these tests
	buildCtx := BuildContext{}

	testCases := []struct {
		name            string
		pattern         string
		expectedMeta    *PackageMetaInfo // Using pointer to allow nil for error cases
		expectErr       bool
		expectedErrText string // Substring to check in error message
	}{
		{
			name:    "valid relative path ./pkg/foo",
			pattern: "./pkg/foo",
			expectedMeta: &PackageMetaInfo{
				ImportPath:    "example.com/testmodule/pkg/foo",
				Name:          "foo",
				Dir:           filepath.Join(testModuleDir, "pkg/foo"),
				GoFiles:       []string{"foo.go"},
				TestGoFiles:   []string{"foo_test.go"},
				XTestGoFiles:  []string{}, // As per current listGoFiles simplification
				DirectImports: []string{}, // Explicitly initialize
				ModulePath:    moduleName,
				ModuleDir:     testModuleDir,
			},
			expectErr: false,
		},
		{
			name:    "valid relative path ./bar",
			pattern: "./bar",
			expectedMeta: &PackageMetaInfo{
				ImportPath:    "example.com/testmodule/bar",
				Name:          "bar",
				Dir:           filepath.Join(testModuleDir, "bar"),
				GoFiles:       []string{"bar.go"},
				TestGoFiles:   []string{}, // Ensure empty slice, not nil
				XTestGoFiles:  []string{}, // Ensure empty slice, not nil
				DirectImports: []string{}, // Explicitly initialize
				ModulePath:    moduleName,
				ModuleDir:     testModuleDir,
			},
			expectErr: false,
		},
		{
			name:            "relative path to non-existent directory",
			pattern:         "./nonexistent",
			expectErr:       true,
			expectedErrText: "failed to read directory", // Error from os.ReadDir via listGoFiles
		},
		{
			name:            "relative path to directory with no go files",
			pattern:         "./pkg", // pkg dir itself has no go files, only subdirs
			expectErr:       true,
			expectedErrText: "no Go files found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := locator.Locate(tc.pattern, buildCtx)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected an error for pattern '%s', but got nil", tc.pattern)
				} else if tc.expectedErrText != "" && !strings.Contains(err.Error(), tc.expectedErrText) {
					t.Errorf("Expected error for pattern '%s' to contain '%s', but got: %v", tc.pattern, tc.expectedErrText, err)
				}
				return // Stop further checks if error was expected
			}

			if err != nil {
				t.Fatalf("Expected no error for pattern '%s', but got: %v", tc.pattern, err)
			}

			if len(results) != 1 {
				t.Fatalf("Expected 1 package result for pattern '%s', got %d", tc.pattern, len(results))
			}
			meta := results[0]

			// Normalize paths for comparison
			tc.expectedMeta.Dir = filepath.Clean(tc.expectedMeta.Dir)
			meta.Dir = filepath.Clean(meta.Dir)
			tc.expectedMeta.ModuleDir = filepath.Clean(tc.expectedMeta.ModuleDir)
			meta.ModuleDir = filepath.Clean(meta.ModuleDir)

			// Sort file slices for consistent comparison
			// reflect.DeepEqual doesn't care about order for slices if they are otherwise identical in content,
			// but it's good practice if order isn't guaranteed by the function under test.
			// For GoFiles, TestGoFiles, XTestGoFiles, listGoFiles doesn't guarantee order.
			// However, for simplicity in this example, if the number of files is small and fixed,
			// we can rely on the current deterministic (though not explicitly sorted) output of listGoFiles.
			// For robust tests, sort these slices. For now, we'll assume order is stable for these test cases.

			if !reflect.DeepEqual(*tc.expectedMeta, meta) {
				t.Errorf("Mismatch for pattern '%s'. Overall structs not equal.\nExpected: %+v\nGot:      %+v",
					tc.pattern, *tc.expectedMeta, meta)
				// Field by field comparison
				if tc.expectedMeta.ImportPath != meta.ImportPath {
					t.Errorf("Field Mismatch: ImportPath. Expected: '%s', Got: '%s'", tc.expectedMeta.ImportPath, meta.ImportPath)
				}
				if tc.expectedMeta.Name != meta.Name {
					t.Errorf("Field Mismatch: Name. Expected: '%s', Got: '%s'", tc.expectedMeta.Name, meta.Name)
				}
				if tc.expectedMeta.Dir != meta.Dir {
					t.Errorf("Field Mismatch: Dir. Expected: '%s', Got: '%s'", tc.expectedMeta.Dir, meta.Dir)
				}
				if !reflect.DeepEqual(tc.expectedMeta.GoFiles, meta.GoFiles) {
					t.Errorf("Field Mismatch: GoFiles. Expected: %v, Got: %v", tc.expectedMeta.GoFiles, meta.GoFiles)
				}
				if !reflect.DeepEqual(tc.expectedMeta.TestGoFiles, meta.TestGoFiles) {
					t.Errorf("Field Mismatch: TestGoFiles. Expected: %v, Got: %v", tc.expectedMeta.TestGoFiles, meta.TestGoFiles)
				}
				if !reflect.DeepEqual(tc.expectedMeta.XTestGoFiles, meta.XTestGoFiles) {
					t.Errorf("Field Mismatch: XTestGoFiles. Expected: %v, Got: %v", tc.expectedMeta.XTestGoFiles, meta.XTestGoFiles)
				}
				if !reflect.DeepEqual(tc.expectedMeta.DirectImports, meta.DirectImports) {
					t.Errorf("Field Mismatch: DirectImports. Expected: %v, Got: %v", tc.expectedMeta.DirectImports, meta.DirectImports)
				}
				if tc.expectedMeta.ModulePath != meta.ModulePath {
					t.Errorf("Field Mismatch: ModulePath. Expected: '%s', Got: '%s'", tc.expectedMeta.ModulePath, meta.ModulePath)
				}
				if tc.expectedMeta.ModuleDir != meta.ModuleDir {
					t.Errorf("Field Mismatch: ModuleDir. Expected: '%s', Got: '%s'", tc.expectedMeta.ModuleDir, meta.ModuleDir)
				}
				if tc.expectedMeta.Error != meta.Error {
					t.Errorf("Field Mismatch: Error. Expected: '%s', Got: '%s'", tc.expectedMeta.Error, meta.Error)
				}
			}
		})
	}
}

// setupTestDirNoModule creates a temporary directory with specified Go files but no go.mod file.
func setupTestDirNoModule(t *testing.T, files map[string]string) string {
	tmpDir, err := os.MkdirTemp("", "locator_test_nomodule_")
	if err != nil {
		t.Fatalf("Failed to create temp dir for no-module test: %v", err)
	}

	for relPath, content := range files {
		absPath := filepath.Join(tmpDir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			t.Fatalf("Failed to create parent dirs for %s in no-module test: %v", relPath, err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s in no-module test: %v", relPath, err)
		}
	}
	return tmpDir
}

func TestGoModLocator_Locate_NoModuleContext(t *testing.T) {
	files := map[string]string{
		"localpkg/mylib.go": "package localpkg\n\nfunc MyFunc() {}",
		"another.go":        "package main", // A file in the root of the temp dir
	}
	testSetupDir := setupTestDirNoModule(t, files)
	defer os.RemoveAll(testSetupDir)

	locator := &GoModLocator{workingDir: testSetupDir}
	buildCtx := BuildContext{}

	testCases := []struct {
		name            string
		pattern         string
		expectedMeta    *PackageMetaInfo // Pointer to allow for nil in error cases
		expectErr       bool
		expectedErrText string
	}{
		{
			name:    "relative path ./localpkg in no-module context",
			pattern: "./localpkg",
			expectedMeta: &PackageMetaInfo{
				ImportPath:    "./localpkg", // Relative path is preserved as import path
				Name:          "localpkg",
				Dir:           filepath.Join(testSetupDir, "localpkg"),
				GoFiles:       []string{"mylib.go"},
				TestGoFiles:   []string{},
				XTestGoFiles:  []string{},
				DirectImports: []string{},
				ModulePath:    "", // No module context
				ModuleDir:     "", // No module context
			},
			expectErr: false,
		},
		{
			name:    "relative path ./ in no-module context",
			pattern: "./",
			expectedMeta: &PackageMetaInfo{
				ImportPath:    "./",
				Name:          "main", // From another.go
				Dir:           testSetupDir,
				GoFiles:       []string{"another.go"},
				TestGoFiles:   []string{},
				XTestGoFiles:  []string{},
				DirectImports: []string{},
				ModulePath:    "",
				ModuleDir:     "",
			},
			expectErr: false,
		},
		{
			name:            "absolute import path in no-module context",
			pattern:         "example.com/somelib/foo",
			expectErr:       true,
			expectedErrText: "package \"example.com/somelib/foo\" not found", // No go.mod to resolve from
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := locator.Locate(tc.pattern, buildCtx)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected an error for pattern '%s', got nil", tc.pattern)
					return
				}
				if tc.expectedErrText != "" && !strings.Contains(err.Error(), tc.expectedErrText) {
					t.Errorf("Expected error for pattern '%s' to contain '%s', got: %v", tc.pattern, tc.expectedErrText, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error for pattern '%s': %v", tc.pattern, err)
			}
			if len(results) != 1 {
				t.Fatalf("Expected 1 package result for pattern '%s', got %d", tc.pattern, len(results))
			}
			meta := results[0]

			tc.expectedMeta.Dir = filepath.Clean(tc.expectedMeta.Dir)
			meta.Dir = filepath.Clean(meta.Dir)
			// ModuleDir will be empty, clean won't affect it but good for consistency
			tc.expectedMeta.ModuleDir = filepath.Clean(tc.expectedMeta.ModuleDir)
			meta.ModuleDir = filepath.Clean(meta.ModuleDir)

			if tc.expectedMeta.GoFiles == nil {
				tc.expectedMeta.GoFiles = []string{}
			}
			if meta.GoFiles == nil {
				meta.GoFiles = []string{}
			}
			if tc.expectedMeta.TestGoFiles == nil {
				tc.expectedMeta.TestGoFiles = []string{}
			}
			if meta.TestGoFiles == nil {
				meta.TestGoFiles = []string{}
			}
			if tc.expectedMeta.XTestGoFiles == nil {
				tc.expectedMeta.XTestGoFiles = []string{}
			}
			if meta.XTestGoFiles == nil {
				meta.XTestGoFiles = []string{}
			}
			if tc.expectedMeta.DirectImports == nil {
				tc.expectedMeta.DirectImports = []string{}
			}
			if meta.DirectImports == nil {
				meta.DirectImports = []string{}
			}

			if !reflect.DeepEqual(*tc.expectedMeta, meta) {
				t.Errorf("Result mismatch for pattern '%s'.\nExpected: %+v\nGot:      %+v", tc.pattern, *tc.expectedMeta, meta)
			}
		})
	}
}
func TestGoModLocator_Locate_NotFound(t *testing.T) {
	moduleName := "example.com/testmodule"
	testModuleDir := setupTestModule(t, moduleName, map[string]string{
		"main.go": "package main",
	})
	defer os.RemoveAll(testModuleDir)

	locator := &GoModLocator{workingDir: testModuleDir}
	buildCtx := BuildContext{}

	testCases := []struct {
		name            string
		pattern         string
		expectedErrText string
	}{
		{
			name:            "package not found anywhere",
			pattern:         "nonexistent.com/pkg/foo",
			expectedErrText: "package \"nonexistent.com/pkg/foo\" not found",
		},
		{
			name:            "package not in current module or deps, but looks like current module",
			pattern:         moduleName + "/nonexistentpkg",
			expectedErrText: "package \"" + moduleName + "/nonexistentpkg\" not found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := locator.Locate(tc.pattern, buildCtx)
			if err == nil {
				t.Errorf("Expected an error for pattern '%s', but got nil", tc.pattern)
				return
			}
			if !strings.Contains(err.Error(), tc.expectedErrText) {
				t.Errorf("Expected error for pattern '%s' to contain '%s', but got: %v", tc.pattern, tc.expectedErrText, err)
			}
		})
	}
}

// setupMockGoModCache creates a directory structure mimicking a Go module cache
// for a given dependency.
func setupMockGoModCache(t *testing.T, depModulePath, depVersion string, files map[string]string) string {
	// Use a subdirectory within the main test temp dir for better cleanup, or a new global temp
	// For simplicity, let's create a new temp dir for the cache.
	mockCacheRoot, err := os.MkdirTemp("", "mock_gomodcache_")
	if err != nil {
		t.Fatalf("Failed to create mock GOMODCACHE root: %v", err)
	}

	// Mimic `go mod download` path escaping for module path
	// Example: github.com/user/repo -> github.com/user/repo
	// For uppercase in path: github.com/Azure/azure-sdk-for-go -> github.com/!azure/azure-sdk-for-go
	// The GoModLocator itself uses module.EscapePath, so the cache should reflect that.
	// For this test, we'll assume simple lowercase paths for the mock dependency for simplicity,
	// or handle escaping if the chosen dep path requires it.
	// Let's assume depModulePath is already in the form it would appear in the cache dir name (e.g. no '!').
	// A more robust solution would use module.EscapePath here if testing complex module paths.

	// escapedModulePath := strings.ReplaceAll(depModulePath, "/", string(filepath.Separator)) // This was unused
	// This is a simplified escaping. Real escaping is more complex (e.g. ! for uppercase).
	// For test `github.com/stretchr/testify`, it becomes `github.com/stretchr/testify`.
	// If a module path had uppercase, like `github.com/Azure/azure-sdk-for-go`, it would be `github.com/!azure/azure-sdk-for-go`.
	// The `module.EscapePath` function in `locator.go` does the correct escaping when forming the path to look up.
	// So, the directory in our mock cache should match that escaped form.

	// Let's use a dep path that doesn't require complex escaping for the test to keep it simple.
	// e.g., "my.test/somelib"
	// If we use "github.com/stretchr/testify", the actual cache uses "github.com/stretchr/testify@version".
	// The `module.EscapePath` in `locator.go` is applied to `depModPath` before joining with `@version`.

	actualEscapedPath, err := module.EscapePath(depModulePath)
	if err != nil {
		t.Fatalf("Error escaping module path for mock cache setup: %v", err)
	}

	depCacheDir := filepath.Join(mockCacheRoot, actualEscapedPath+"@"+depVersion)
	if err := os.MkdirAll(depCacheDir, 0755); err != nil {
		t.Fatalf("Failed to create directory for mock dependency %s in cache: %v", depModulePath, err)
	}

	// Create go.mod for the dependency
	depGoModContent := "module " + depModulePath + "\n\ngo 1.16\n" // Minimal go.mod
	if err := os.WriteFile(filepath.Join(depCacheDir, "go.mod"), []byte(depGoModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod for mock dependency %s: %v", depModulePath, err)
	}

	for relPath, content := range files {
		absPath := filepath.Join(depCacheDir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			t.Fatalf("Failed to create parent dirs for %s in mock dep: %v", relPath, err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s for mock dep: %v", relPath, err)
		}
	}
	return mockCacheRoot
}

func TestGoModLocator_Locate_ExternalDependency(t *testing.T) {
	// 1. Setup mock GOMODCACHE
	depModulePath := "my.test/somelib"
	depVersion := "v1.0.0"
	depFiles := map[string]string{
		"core/utils.go":      "package core\n\nfunc GetInfo() string { return \"dependency info\" }",
		"another/another.go": "package another",
	}
	mockCacheRoot := setupMockGoModCache(t, depModulePath, depVersion, depFiles)
	defer os.RemoveAll(mockCacheRoot)

	// Set GOMODCACHE environment variable for this test
	originalGoModCache, originalGoModCacheExists := os.LookupEnv("GOMODCACHE")
	if err := os.Setenv("GOMODCACHE", mockCacheRoot); err != nil {
		t.Fatalf("Failed to set GOMODCACHE: %v", err)
	}
	defer func() {
		if originalGoModCacheExists {
			if err := os.Setenv("GOMODCACHE", originalGoModCache); err != nil {
				t.Logf("Warning: failed to restore GOMODCACHE: %v", err)
			}
		} else {
			if err := os.Unsetenv("GOMODCACHE"); err != nil {
				t.Logf("Warning: failed to unset GOMODCACHE: %v", err)
			}
		}
	}()

	// 2. Setup main test module that requires the dependency
	mainModuleName := "example.com/mainmod"
	mainModuleFiles := map[string]string{
		"main.go": "package main\n\nimport _ \"" + depModulePath + "/core\"\n\nfunc main() {}",
	}
	testModuleDir := setupTestModule(t, mainModuleName, mainModuleFiles) // This creates a basic go.mod
	defer os.RemoveAll(testModuleDir)

	// Add require directive to the main module's go.mod
	mainGoModPath := filepath.Join(testModuleDir, "go.mod")
	mainGoModContent, err := os.ReadFile(mainGoModPath)
	if err != nil {
		t.Fatalf("Failed to read main module's go.mod: %v", err)
	}
	newGoModContent := string(mainGoModContent) + "\nrequire " + depModulePath + " " + depVersion + "\n"
	if err := os.WriteFile(mainGoModPath, []byte(newGoModContent), 0644); err != nil {
		t.Fatalf("Failed to write updated go.mod for main module: %v", err)
	}

	// 3. Initialize locator and perform test
	locator := &GoModLocator{workingDir: testModuleDir}
	buildCtx := BuildContext{}

	escapedDepModulePathForCache, err := module.EscapePath(depModulePath)
	if err != nil {
		t.Fatalf("Test setup: Error escaping module path for expected values: %v", err)
	}

	testCases := []struct {
		name         string
		pattern      string // Import path of a package from the dependency
		expectedMeta *PackageMetaInfo
		expectErr    bool
	}{
		{
			name:    "locate package from external dependency",
			pattern: depModulePath + "/core",
			expectedMeta: &PackageMetaInfo{
				ImportPath:    depModulePath + "/core",
				Name:          "core",
				Dir:           filepath.Join(mockCacheRoot, escapedDepModulePathForCache+"@"+depVersion, "core"),
				GoFiles:       []string{"utils.go"},
				TestGoFiles:   []string{},
				XTestGoFiles:  []string{},
				DirectImports: []string{},
				ModulePath:    depModulePath,
				ModuleDir:     filepath.Join(mockCacheRoot, escapedDepModulePathForCache+"@"+depVersion),
			},
			expectErr: false,
		},
		{
			name:    "locate another package from external dependency",
			pattern: depModulePath + "/another",
			expectedMeta: &PackageMetaInfo{
				ImportPath:    depModulePath + "/another",
				Name:          "another",
				Dir:          filepath.Join(mockCacheRoot, escapedDepModulePathForCache+"@"+depVersion, "another"),
				GoFiles:       []string{"another.go"},
				TestGoFiles:   []string{},
				XTestGoFiles:  []string{},
				DirectImports: []string{},
				ModulePath:    depModulePath,
				ModuleDir:    filepath.Join(mockCacheRoot, escapedDepModulePathForCache+"@"+depVersion),
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := locator.Locate(tc.pattern, buildCtx)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected an error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("Expected 1 package result, got %d", len(results))
			}
			meta := results[0]

			tc.expectedMeta.Dir = filepath.Clean(tc.expectedMeta.Dir)
			meta.Dir = filepath.Clean(meta.Dir)
			tc.expectedMeta.ModuleDir = filepath.Clean(tc.expectedMeta.ModuleDir)
			meta.ModuleDir = filepath.Clean(meta.ModuleDir)

			if tc.expectedMeta.GoFiles == nil {
				tc.expectedMeta.GoFiles = []string{}
			}
			if meta.GoFiles == nil {
				meta.GoFiles = []string{}
			}
			if tc.expectedMeta.TestGoFiles == nil {
				tc.expectedMeta.TestGoFiles = []string{}
			}
			if meta.TestGoFiles == nil {
				meta.TestGoFiles = []string{}
			}
			if tc.expectedMeta.XTestGoFiles == nil {
				tc.expectedMeta.XTestGoFiles = []string{}
			}
			if meta.XTestGoFiles == nil {
				meta.XTestGoFiles = []string{}
			}
			if tc.expectedMeta.DirectImports == nil {
				tc.expectedMeta.DirectImports = []string{}
			}
			if meta.DirectImports == nil {
				meta.DirectImports = []string{}
			}

			if !reflect.DeepEqual(*tc.expectedMeta, meta) {
				t.Errorf("Result mismatch for %s.\nExpected: %+v\nGot:      %+v", tc.pattern, *tc.expectedMeta, meta)
			}
		})
	}
}

func TestGoModLocator_Locate_CurrentModule(t *testing.T) {
	moduleName := "example.com/currentmod"
	files := map[string]string{
		"internalpkg/code.go": "package internalpkg\n\n// Internal logic\nfunc Helper() {}",
		"main.go":             "package main\n\nimport \"example.com/currentmod/internalpkg\"\n\nfunc main() { internalpkg.Helper() }",
	}
	testModuleDir := setupTestModule(t, moduleName, files)
	defer os.RemoveAll(testModuleDir)

	locator := &GoModLocator{workingDir: testModuleDir}
	buildCtx := BuildContext{} // Minimal context

	testCases := []struct {
		name         string
		pattern      string
		expectedMeta *PackageMetaInfo
		expectErr    bool
	}{
		{
			name:    "package in current module by full import path",
			pattern: "example.com/currentmod/internalpkg",
			expectedMeta: &PackageMetaInfo{
				ImportPath:    "example.com/currentmod/internalpkg",
				Name:          "internalpkg",
				Dir:           filepath.Join(testModuleDir, "internalpkg"),
				GoFiles:       []string{"code.go"},
				TestGoFiles:   []string{},
				XTestGoFiles:  []string{},
				DirectImports: []string{},
				ModulePath:    moduleName,
				ModuleDir:     testModuleDir,
			},
			expectErr: false,
		},
		{
			name:    "main package in current module by module path",
			pattern: "example.com/currentmod", // This should resolve to the main package at the root
			expectedMeta: &PackageMetaInfo{
				ImportPath:    "example.com/currentmod",
				Name:          "main", // Assuming package name in main.go is 'main'
				Dir:           testModuleDir,
				GoFiles:       []string{"main.go"},
				TestGoFiles:   []string{},
				XTestGoFiles:  []string{},
				DirectImports: []string{}, // Adjust if main.go has imports parsed by locator
				ModulePath:    moduleName,
				ModuleDir:     testModuleDir,
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results, err := locator.Locate(tc.pattern, buildCtx)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected an error, got nil")
				}
				// Add more detailed error string checking if necessary
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("Expected 1 package result, got %d", len(results))
			}
			meta := results[0]

			// Normalize paths
			tc.expectedMeta.Dir = filepath.Clean(tc.expectedMeta.Dir)
			meta.Dir = filepath.Clean(meta.Dir)
			tc.expectedMeta.ModuleDir = filepath.Clean(tc.expectedMeta.ModuleDir)
			meta.ModuleDir = filepath.Clean(meta.ModuleDir)

			// Ensure slices are non-nil for comparison if they are empty
			if tc.expectedMeta.GoFiles == nil {
				tc.expectedMeta.GoFiles = []string{}
			}
			if meta.GoFiles == nil {
				meta.GoFiles = []string{}
			}
			if tc.expectedMeta.TestGoFiles == nil {
				tc.expectedMeta.TestGoFiles = []string{}
			}
			if meta.TestGoFiles == nil {
				meta.TestGoFiles = []string{}
			}
			if tc.expectedMeta.XTestGoFiles == nil {
				tc.expectedMeta.XTestGoFiles = []string{}
			}
			if meta.XTestGoFiles == nil {
				meta.XTestGoFiles = []string{}
			}
			if tc.expectedMeta.DirectImports == nil {
				tc.expectedMeta.DirectImports = []string{}
			}
			if meta.DirectImports == nil {
				meta.DirectImports = []string{}
			}

			if !reflect.DeepEqual(*tc.expectedMeta, meta) {
				t.Errorf("Result mismatch.\nExpected: %+v\nGot:      %+v", *tc.expectedMeta, meta)
				// Detailed field comparison can be added here if needed, similar to TestGoModLocator_Locate_RelativePaths
			}
		})
	}
}
