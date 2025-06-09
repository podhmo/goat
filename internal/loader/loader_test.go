package loader

import (
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFile_Success(t *testing.T) {
	// Create a temporary Go file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	content := `
package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	fset := token.NewFileSet()
	fileAST, err := LoadFile(fset, tmpFile)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if fileAST == nil {
		t.Fatal("LoadFile returned nil AST")
	}
	if fileAST.Name.Name != "main" {
		t.Errorf("Expected package name 'main', got '%s'", fileAST.Name.Name)
	}
}

func TestLoadFile_NonExistentFile(t *testing.T) {
	fset := token.NewFileSet()
	_, err := LoadFile(fset, "non_existent_file.go")
	if err == nil {
		t.Fatal("LoadFile should have failed for a non-existent file, but it did not")
	}
	// Check if the error message is somewhat informative, though exact message depends on os
	if !strings.Contains(err.Error(), "no such file or directory") && !strings.Contains(err.Error(), "cannot find the file") {
		t.Logf("Warning: Error message might not be as expected: %v", err)
	}
}

func TestLoadFile_InvalidGoSyntax(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.go")
	content := `
package main
func main() { fmt.Println("Hello" // Missing closing parenthesis
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	fset := token.NewFileSet()
	_, err := LoadFile(fset, tmpFile)
	if err == nil {
		t.Fatal("LoadFile should have failed for a file with syntax errors, but it did not")
	}
	// Error message from parser usually contains line number and expected token
	if !strings.Contains(err.Error(), "expected ')'") && !strings.Contains(err.Error(), "expected declaration") { // depends on parser error detail
		t.Logf("Warning: Syntax error message might not be as expected: %v", err)
	}
}

func TestLoadPackageFiles_ImportPath_Success(t *testing.T) {
	fset := token.NewFileSet()
	// Using "os" package as it's a standard library package and should be available.
	files, err := LoadPackageFiles(fset, "os", "")
	if err != nil {
		t.Fatalf("LoadPackageFiles failed for import path 'os': %v", err)
	}
	if len(files) == 0 {
		t.Fatal("LoadPackageFiles returned no files for package 'os'")
	}
	for _, fileAST := range files {
		if fileAST.Name.Name != "os" {
			// This check might be too strict if "os" package has files with different package declarations (e.g. due to build constraints)
			// However, for a standard package like "os", it's generally expected they all declare `package os`.
			// Let's check the filename for a more robust test of origin.
			fileName := fset.Position(fileAST.Pos()).Filename
			if !strings.Contains(fileName, filepath.FromSlash("os")) { // Check if the file path contains "os"
				t.Errorf("Expected file from 'os' package, but got %s with package decl '%s'", fileName, fileAST.Name.Name)
			}
		}
	}
	t.Logf("Loaded %d files for package 'os'", len(files))
}

func TestLoadPackageFiles_AbsoluteDirPath_Success(t *testing.T) {
	tmpDir := t.TempDir()
	fset := token.NewFileSet()

	content1 := "package testpkg\n\nconst Val1 = 1"
	content2 := "package testpkg\n\nconst Val2 = 2"

	if err := os.WriteFile(filepath.Join(tmpDir, "c.go"), []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to write temp file c.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "d.go"), []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to write temp file d.go: %v", err)
	}

	files, err := LoadPackageFiles(fset, tmpDir, "")
	if err != nil {
		t.Fatalf("LoadPackageFiles failed for absolute path '%s': %v", tmpDir, err)
	}
	if len(files) != 2 {
		t.Errorf("Expected 2 AST files, got %d", len(files))
	}
	for _, fileAST := range files {
		if fileAST.Name.Name != "testpkg" {
			t.Errorf("Expected package name 'testpkg', got '%s'", fileAST.Name.Name)
		}
	}
}

func TestLoadPackageFiles_NonExistentPath(t *testing.T) {
	fset := token.NewFileSet()

	// Test non-existent import path
	_, err := LoadPackageFiles(fset, "example.com/nonexistentpkg", "")
	if err == nil {
		t.Error("LoadPackageFiles should have failed for non-existent import path, but it did not")
	}
	// The error from build.Import for non-existent packages can vary.
	// It might say "cannot find package" or similar.
	// For now, just checking for any error is sufficient.

	// Test non-existent absolute directory path
	nonExistentDir := filepath.Join(t.TempDir(), "nonexistentdir") // Ensure it's truly non-existent
	_, err = LoadPackageFiles(fset, nonExistentDir, "")
	if err == nil {
		t.Errorf("LoadPackageFiles should have failed for non-existent absolute dir path '%s', but it did not", nonExistentDir)
	}
	if !os.IsNotExist(err) {
		// More specific check for "no such file or directory" type of error
		// This can be tricky as the error might be wrapped or come from different places.
		// A general check for non-nil error is the primary goal.
		// t.Logf("Note: error for non-existent dir is not os.IsNotExist: %v", err)
	}
}

func TestLoadPackageFiles_TypeNameHint(t *testing.T) {
	tmpDir := t.TempDir()
	fset := token.NewFileSet()

	contentApple := "package fruitpkg\n\n// Apple related types\ntype Apple struct{}"
	contentBanana := "package fruitpkg\n\n// Banana related types\ntype Banana struct{}"
	contentOrange := "package fruitpkg\n\n// Orange related types\ntype Orange struct{}"

	if err := os.WriteFile(filepath.Join(tmpDir, "apple_types.go"), []byte(contentApple), 0644); err != nil {
		t.Fatalf("Failed to write temp file apple_types.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "banana_fruit.go"), []byte(contentBanana), 0644); err != nil {
		t.Fatalf("Failed to write temp file banana_fruit.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "orange.go"), []byte(contentOrange), 0644); err != nil {
		t.Fatalf("Failed to write temp file orange.go: %v", err)
	}

	// Test with typeNameHint "apple"
	filesAppleHint, err := LoadPackageFiles(fset, tmpDir, "apple")
	if err != nil {
		t.Fatalf("LoadPackageFiles failed with typeNameHint 'apple': %v", err)
	}
	if len(filesAppleHint) != 3 { // Expect all 3 files to be loaded
		t.Errorf("Expected 3 files with hint 'apple', got %d", len(filesAppleHint))
	}
	// Check if apple_types.go is prioritized (first in the list)
	// This depends on the implementation detail that priority files are prepended.
	appleFileName := fset.Position(filesAppleHint[0].Pos()).Filename
	if !strings.HasSuffix(appleFileName, "apple_types.go") {
		t.Errorf("Expected 'apple_types.go' to be prioritized with hint 'apple', but got '%s' first", filepath.Base(appleFileName))
	}

	// Test with typeNameHint "fruit" (matching a part of banana_fruit.go)
	fset2 := token.NewFileSet() // Use a new fset to be safe, or ensure fset is re-usable
	filesFruitHint, err := LoadPackageFiles(fset2, tmpDir, "fruit")
	if err != nil {
		t.Fatalf("LoadPackageFiles failed with typeNameHint 'fruit': %v", err)
	}
	if len(filesFruitHint) != 3 { // Expect all 3 files
		t.Errorf("Expected 3 files with hint 'fruit', got %d", len(filesFruitHint))
	}
	fruitFileName := fset2.Position(filesFruitHint[0].Pos()).Filename
	if !strings.HasSuffix(fruitFileName, "banana_fruit.go") {
		t.Errorf("Expected 'banana_fruit.go' to be prioritized with hint 'fruit', but got '%s' first", filepath.Base(fruitFileName))
	}
}

func TestLoadPackageFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	fset := token.NewFileSet()

	files, err := LoadPackageFiles(fset, tmpDir, "")
	if err != nil {
		t.Fatalf("LoadPackageFiles failed for an empty directory '%s': %v", tmpDir, err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 AST files for an empty directory, got %d", len(files))
	}
}

func TestLoadPackageFiles_SkipsTestGoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	fset := token.NewFileSet()

	realCodeContent := "package testpkg\n\nfunc MyFunction() {}"
	testCodeContent := "package testpkg\n\nimport \"testing\"\n\nfunc TestMyFunction(t *testing.T) {}"

	if err := os.WriteFile(filepath.Join(tmpDir, "realcode.go"), []byte(realCodeContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file realcode.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "realcode_test.go"), []byte(testCodeContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file realcode_test.go: %v", err)
	}

	files, err := LoadPackageFiles(fset, tmpDir, "")
	if err != nil {
		t.Fatalf("LoadPackageFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 AST file, got %d (should skip _test.go files)", len(files))
	}
	if files != nil && len(files) == 1 {
		loadedFileName := filepath.Base(fset.Position(files[0].Pos()).Filename)
		if loadedFileName != "realcode.go" {
			t.Errorf("Expected 'realcode.go' to be loaded, but got '%s'", loadedFileName)
		}
	}
}

func TestFindModuleRoot_Success_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "basedir")
	if err := os.Mkdir(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create baseDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "go.mod"), []byte("module example.com/basic"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}
	filePath := filepath.Join(baseDir, "somefile.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write somefile.go: %v", err)
	}

	foundPath, err := FindModuleRoot(filePath)
	if err != nil {
		t.Fatalf("FindModuleRoot failed: %v", err)
	}
	// Normalize paths for comparison, especially on Windows
	expectedPath, _ := filepath.Abs(baseDir)
	normalizedFoundPath, _ := filepath.Abs(foundPath)
	if normalizedFoundPath != expectedPath {
		t.Errorf("Expected module root '%s', got '%s'", expectedPath, normalizedFoundPath)
	}
}

func TestFindModuleRoot_Success_DeepNesting(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	if err := os.Mkdir(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create projectDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/project"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	deepPath := filepath.Join(projectDir, "pkg", "subpkg", "internal")
	if err := os.MkdirAll(deepPath, 0755); err != nil {
		t.Fatalf("Failed to create deepPath directories: %v", err)
	}
	filePath := filepath.Join(deepPath, "deepfile.go")
	if err := os.WriteFile(filePath, []byte("package internal"), 0644); err != nil {
		t.Fatalf("Failed to write deepfile.go: %v", err)
	}

	foundPath, err := FindModuleRoot(filePath)
	if err != nil {
		t.Fatalf("FindModuleRoot failed: %v", err)
	}
	expectedPath, _ := filepath.Abs(projectDir)
	normalizedFoundPath, _ := filepath.Abs(foundPath)
	if normalizedFoundPath != expectedPath {
		t.Errorf("Expected module root '%s', got '%s'", expectedPath, normalizedFoundPath)
	}
}

func TestFindModuleRoot_Success_CurrentDirIsRoot(t *testing.T) {
	tmpDir := t.TempDir()
	currentRootDir := filepath.Join(tmpDir, "currentroot")
	if err := os.Mkdir(currentRootDir, 0755); err != nil {
		t.Fatalf("Failed to create currentRootDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(currentRootDir, "go.mod"), []byte("module example.com/currentroot"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}
	// Test with a file directly in the root
	filePath := filepath.Join(currentRootDir, "dummy.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write dummy.go: %v", err)
	}

	foundPath, err := FindModuleRoot(filePath)
	if err != nil {
		t.Fatalf("FindModuleRoot failed: %v", err)
	}
	expectedPath, _ := filepath.Abs(currentRootDir)
	normalizedFoundPath, _ := filepath.Abs(foundPath)
	if normalizedFoundPath != expectedPath {
		t.Errorf("Expected module root '%s', got '%s'", expectedPath, normalizedFoundPath)
	}

	// Test with the go.mod file itself
	goModPath := filepath.Join(currentRootDir, "go.mod")
	foundPathGoMod, errGoMod := FindModuleRoot(goModPath)
	if errGoMod != nil {
		t.Fatalf("FindModuleRoot failed for go.mod path: %v", errGoMod)
	}
	normalizedFoundPathGoMod, _ := filepath.Abs(foundPathGoMod)
	if normalizedFoundPathGoMod != expectedPath {
		t.Errorf("Expected module root '%s' for go.mod path, got '%s'", expectedPath, normalizedFoundPathGoMod)
	}
}

func TestFindModuleRoot_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	noModProjectDir := filepath.Join(tmpDir, "no_mod_project", "some")
	if err := os.MkdirAll(noModProjectDir, 0755); err != nil {
		t.Fatalf("Failed to create noModProjectDir: %v", err)
	}
	filePath := filepath.Join(noModProjectDir, "file.go")
	if err := os.WriteFile(filePath, []byte("package some"), 0644); err != nil {
		t.Fatalf("Failed to write file.go: %v", err)
	}

	// Need to cd into a path that doesn't have go.mod in its parent hierarchy up to tmpDir
	// to avoid finding go.mod from the project running the test.
	// However, FindModuleRoot should stop at filesystem root or return error if path is odd.
	// The most reliable way is to ensure FindModuleRoot uses absolute paths internally
	// and stops correctly. Given the current implementation seems to rely on walking up,
	// this test should be fine if it starts deep enough in a temp dir.

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		t.Fatalf("Could not get absolute path for test: %v", err)
	}

	foundPath, err := FindModuleRoot(absFilePath)
	if err == nil {
		t.Errorf("FindModuleRoot should have failed, but found path '%s'", foundPath)
	}
	if foundPath != "" {
		t.Errorf("Expected empty path on failure, got '%s'", foundPath)
	}
	expectedErrorMsgPart := "go.mod not found upwards from" // Check for the more specific part of the error
	if !strings.Contains(err.Error(), expectedErrorMsgPart) {
		t.Errorf("Expected error message to contain '%s', got '%s'", expectedErrorMsgPart, err.Error())
	}
}

func TestFindModuleRoot_NotFound_EmptyPath(t *testing.T) {
	foundPath, err := FindModuleRoot("")
	if err == nil {
		t.Error("FindModuleRoot should have failed for an empty path, but it did not")
	}
	if foundPath != "" {
		t.Errorf("Expected empty path on failure for empty input, got '%s'", foundPath)
	}
	// The error might be from filepath.Abs or the "go.mod not found" error.
	// If filepath.Abs("") fails, it's an invalid input. If it becomes ".",
	// it might find a go.mod if the test is run from a module root.
	// The function should ideally handle empty string as an invalid argument.
	// For now, let's check for a generic error.
}

func TestFindModuleRoot_FilePathIsJustDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	modAtTopDir := filepath.Join(tmpDir, "mod_at_top")
	if err := os.Mkdir(modAtTopDir, 0755); err != nil {
		t.Fatalf("Failed to create modAtTopDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modAtTopDir, "go.mod"), []byte("module example.com/modattop"), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	subDir := filepath.Join(modAtTopDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subDir: %v", err)
	}
	// Path provided to FindModuleRoot is a directory, not a file.
	// filepath.Dir(subDir) will give modAtTopDir.
	// FindModuleRoot will then look for go.mod in modAtTopDir.
	// This test is slightly different from Success_DeepNesting as the input path is a directory.
	// However, FindModuleRoot internally calls filepath.Dir on the input.
	// If the input is `mod_at_top/subdir`, `filepath.Dir` is `mod_at_top`.
	// If the input is `mod_at_top/subdir/`, `filepath.Dir` is `mod_at_top/subdir`.
	// Let's test with `mod_at_top/subdir` as the input.

	// Scenario 1: filePath is a directory that contains a file (conceptually)
	// We are testing FindModuleRoot(mod_at_top/subdir/anotherfile.go) where anotherfile.go may not exist.
	// The function uses filepath.Dir, so it should resolve to "mod_at_top/subdir" then look upwards.
	conceptualFilePath := filepath.Join(subDir, "anotherfile.go")

	foundPath, err := FindModuleRoot(conceptualFilePath)
	if err != nil {
		t.Fatalf("FindModuleRoot failed for conceptual file path: %v", err)
	}
	expectedPath, _ := filepath.Abs(modAtTopDir)
	normalizedFoundPath, _ := filepath.Abs(foundPath)
	if normalizedFoundPath != expectedPath {
		t.Errorf("Expected module root '%s', got '%s' for path %s", expectedPath, normalizedFoundPath, conceptualFilePath)
	}

	// Scenario 2: filePath is a directory itself.
	// The original FindModuleRoot implementation takes `filePath` and calls `filepath.Dir(filePath)`.
	// If `filePath` is `mod_at_top/subdir`, then `filepath.Dir(filePath)` is `mod_at_top`.
	// This means `FindModuleRoot("mod_at_top/subdir")` should find `mod_at_top/go.mod`.
	foundPathDirInput, errDirInput := FindModuleRoot(subDir)
	if errDirInput != nil {
		t.Fatalf("FindModuleRoot failed for direct directory input: %v", errDirInput)
	}
	normalizedFoundPathDirInput, _ := filepath.Abs(foundPathDirInput)
	if normalizedFoundPathDirInput != expectedPath {
		t.Errorf("Expected module root '%s', got '%s' for path %s", expectedPath, normalizedFoundPathDirInput, subDir)
	}
}

func TestGetModuleName_Success(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := filepath.Join(tmpDir, "testmod_succ")
	if err := os.Mkdir(modulePath, 0755); err != nil {
		t.Fatalf("Failed to create modulePath: %v", err)
	}
	goModContent := "module example.com/mytestmodule\n\ngo 1.18"
	if err := os.WriteFile(filepath.Join(modulePath, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	moduleName, err := GetModuleName(modulePath)
	if err != nil {
		t.Fatalf("GetModuleName failed: %v", err)
	}
	if moduleName != "example.com/mytestmodule" {
		t.Errorf("Expected module name 'example.com/mytestmodule', got '%s'", moduleName)
	}
}

func TestGetModuleName_Success_WithOtherDirectives(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := filepath.Join(tmpDir, "testmod_other")
	if err := os.Mkdir(modulePath, 0755); err != nil {
		t.Fatalf("Failed to create modulePath: %v", err)
	}
	goModContent := `
// This is a comment
module example.com/anothermodule

require (
	golang.org/x/text v0.3.7
)

go 1.17
`
	if err := os.WriteFile(filepath.Join(modulePath, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	moduleName, err := GetModuleName(modulePath)
	if err != nil {
		t.Fatalf("GetModuleName failed: %v", err)
	}
	if moduleName != "example.com/anothermodule" {
		t.Errorf("Expected module name 'example.com/anothermodule', got '%s'", moduleName)
	}
}

func TestGetModuleName_NoModuleDirective(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := filepath.Join(tmpDir, "testmod_nomod")
	if err := os.Mkdir(modulePath, 0755); err != nil {
		t.Fatalf("Failed to create modulePath: %v", err)
	}
	goModContent := `
go 1.19

require (
	example.com/some/dep v1.2.3
)
`
	if err := os.WriteFile(filepath.Join(modulePath, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	moduleName, err := GetModuleName(modulePath)
	if err == nil {
		t.Errorf("GetModuleName should have failed, but returned module name '%s'", moduleName)
	}
	if moduleName != "" {
		t.Errorf("Expected empty module name on failure, got '%s'", moduleName)
	}
	if err != nil && !strings.Contains(err.Error(), "module directive not found in") { // Check for partial message
		t.Errorf("Expected error to contain 'module directive not found in', got '%v'", err)
	}
}

func TestGetModuleName_EmptyGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := filepath.Join(tmpDir, "testmod_empty")
	if err := os.Mkdir(modulePath, 0755); err != nil {
		t.Fatalf("Failed to create modulePath: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulePath, "go.mod"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write empty go.mod: %v", err)
	}

	moduleName, err := GetModuleName(modulePath)
	if err == nil {
		t.Errorf("GetModuleName should have failed for empty go.mod, but returned module name '%s'", moduleName)
	}
	if moduleName != "" {
		t.Errorf("Expected empty module name on failure, got '%s'", moduleName)
	}
	if err != nil && !strings.Contains(err.Error(), "module directive not found in") { // Check for partial message
		t.Errorf("Expected error to contain 'module directive not found in' for empty file, got '%v'", err)
	}
}

func TestGetModuleName_NonExistentGoModFile(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := filepath.Join(tmpDir, "testmod_nonexistent_gomod_dir")
	if err := os.Mkdir(modulePath, 0755); err != nil {
		t.Fatalf("Failed to create modulePath: %v", err)
	}

	moduleName, err := GetModuleName(modulePath)
	if err == nil {
		t.Errorf("GetModuleName should have failed as go.mod does not exist, but returned module name '%s'", moduleName)
	}
	if moduleName != "" {
		t.Errorf("Expected empty module name on failure, got '%s'", moduleName)
	}
	// Error should be about file not existing
	if err != nil && !os.IsNotExist(err) && !strings.Contains(err.Error(), "no such file or directory") { // More robust check for file system error
		t.Errorf("Expected a file system error (IsNotExist or similar), but got: %v", err)
	}
}

func TestGetModuleName_PathIsNotDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "not_a_dir.txt")
	if err := os.WriteFile(filePath, []byte("I am not a directory"), 0644); err != nil {
		t.Fatalf("Failed to write dummy file: %v", err)
	}

	moduleName, err := GetModuleName(filePath)
	if err == nil {
		t.Errorf("GetModuleName should have failed as path is a file, not a directory, but returned module name '%s'", moduleName)
	}
	if moduleName != "" {
		t.Errorf("Expected empty module name on failure, got '%s'", moduleName)
	}
	// The error will likely be because <filePath>/go.mod is not found,
	// or because the path component is not a directory.
	if err != nil && (!strings.Contains(err.Error(), "not a directory") && !strings.Contains(err.Error(), "could not read go.mod")) {
		t.Errorf("Expected error to contain 'not a directory' or 'could not read go.mod', but got: %v", err)
	}
}
