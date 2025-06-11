package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
		"pkg/foo/foo.go": "package foo\n\nfunc Foo() {}",
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
				ImportPath:   "example.com/testmodule/pkg/foo",
				Name:         "foo",
				Dir:          filepath.Join(testModuleDir, "pkg/foo"),
				GoFiles:      []string{"foo.go"},
				TestGoFiles:  []string{"foo_test.go"},
				XTestGoFiles: []string{}, // As per current listGoFiles simplification
				DirectImports:[]string{}, // Explicitly initialize
				ModulePath:   moduleName,
				ModuleDir:    testModuleDir,
			},
			expectErr: false,
		},
		{
			name:    "valid relative path ./bar",
			pattern: "./bar",
			expectedMeta: &PackageMetaInfo{
				ImportPath:   "example.com/testmodule/bar",
				Name:         "bar",
				Dir:          filepath.Join(testModuleDir, "bar"),
				GoFiles:      []string{"bar.go"},
				TestGoFiles:  []string{}, // Ensure empty slice, not nil
				XTestGoFiles: []string{}, // Ensure empty slice, not nil
				DirectImports:[]string{}, // Explicitly initialize
				ModulePath:   moduleName,
				ModuleDir:    testModuleDir,
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
