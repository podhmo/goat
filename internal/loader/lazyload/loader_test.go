package lazyload

import (
	"fmt"
	"go/ast" // Ensure this is present and uncommented
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testdataLocator is a custom PackageLocator for tests.
// It expects 'pattern' to be a module-like path such as "example.com/simplepkg"
// which it maps to a directory in "./testdata/" (e.g., "./testdata/simplepkg").
func testdataLocator(pattern string, buildCtx BuildContext) ([]PackageMetaInfo, error) {
	// Determine the base directory for testdata relative to this file.
	// _, currentFilePath, _, ok := runtime.Caller(0) // This might be tricky if locator is not in _test.go
	// For simplicity, assume tests are run from a context where "./testdata" is accessible
	// or use a fixed relative path from where `go test` is expected to run.
	// Let's try to make it relative to the current working directory from which `go test` is run.
	// The `getTestdataPath` function in tests already computes this, but locator won't have `*testing.T`.
	// A common approach is to assume `go test` runs from the package directory `internal/loader/lazyload`.

	baseTestdataDir := filepath.Join(".", "testdata") // Assumes test runs from internal/loader/lazyload

	// Extract the package name from the pattern, e.g., "simplepkg" from "example.com/simplepkg"
	parts := strings.Split(pattern, "/")
	if len(parts) == 0 {
		return nil, fmt.Errorf("testdataLocator: invalid pattern %q", pattern)
	}
	pkgDirName := parts[len(parts)-1]
	pkgActualName := pkgDirName // Usually package name matches directory name

	pkgPath := filepath.Join(baseTestdataDir, pkgDirName)
	absPkgPath, err := filepath.Abs(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("testdataLocator: could not get absolute path for %s: %w", pkgPath, err)
	}

	dirEntries, err := os.ReadDir(pkgPath)
	if err != nil {
		return nil, fmt.Errorf("testdataLocator: could not read dir %s: %w", pkgPath, err)
	}

	var goFiles []string
	var directImports []string
	fset := token.NewFileSet()

	for _, entry := range dirEntries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			goFiles = append(goFiles, entry.Name())
			filePath := filepath.Join(absPkgPath, entry.Name())

			// Parse file to get package name (if not already set) and imports
			// Only parsing imports should be enough and faster.
			src, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly|parser.PackageClauseOnly)
			if err != nil {
				return nil, fmt.Errorf("testdataLocator: failed to parse %s: %w", filePath, err)
			}

			if pkgActualName == pkgDirName && src.Name != nil { // Use actual package name from source
				pkgActualName = src.Name.Name
			}

			for _, importSpec := range src.Imports {
				importedPath := strings.Trim(importSpec.Path.Value, `"`)
				// Avoid duplicates
				found := false
				for _, imp := range directImports {
					if imp == importedPath {
						found = true
						break
					}
				}
				if !found {
					directImports = append(directImports, importedPath)
				}
			}
		}
	}

	if len(goFiles) == 0 {
		// This might be an issue if we expect a package but find no Go files.
		// However, go list also returns metadata for packages with no .go files (e.g. just subdirs)
		// For our testdata, we expect .go files.
		// return nil, fmt.Errorf("testdataLocator: no .go files found in %s", pkgPath)
	}

	meta := PackageMetaInfo{
		ImportPath:    pattern, // Use the provided pattern as the canonical import path
		Name:          pkgActualName,
		Dir:           absPkgPath,
		GoFiles:       goFiles,
		DirectImports: directImports,
		// TestGoFiles, XTestGoFiles, ModulePath, ModuleDir, Error can be empty/zero for this locator
	}
	return []PackageMetaInfo{meta}, nil
}

// getTestdataPath returns the absolute path to the testdata directory.
func getTestdataPath(t *testing.T) string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get caller information")
	}
	// Construct path to testdata relative to this test file
	// Expected structure: internal/loader/lazyload/loader_test.go -> internal/loader/lazyload/testdata
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestLoadSimplePackage(t *testing.T) {
	testdataPath := getTestdataPath(t) // This gives path to .../testdata
	expectedPkgDir := filepath.Join(testdataPath, "simplepkg")
	absExpectedPkgDir, err := filepath.Abs(expectedPkgDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for expectedPkgDir: %v", err)
	}

	cfg := Config{
		Context: BuildContext{}, // UseGoModule might not be relevant with custom locator
		Locator: testdataLocator,
	}
	loader := NewLoader(cfg)

	pkgs, err := loader.Load("example.com/simplepkg") // Use specific "module" path
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}

	if len(pkgs) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(pkgs))
	}

	pkg := pkgs[0]
	expectedImportPath := "example.com/simplepkg"

	if pkg.ImportPath != expectedImportPath {
		t.Errorf("Expected import path %s, got %s", expectedImportPath, pkg.ImportPath)
	}
	if pkg.Name != "simplepkg" { // Assuming testdataLocator correctly determines name from dir or file
		t.Errorf("Expected package name simplepkg, got %s", pkg.Name)
	}
	// Check GoFiles count; names might be tricky if locator returns full paths
	if len(pkg.GoFiles) != 1 || filepath.Base(pkg.GoFiles[0]) != "simple.go" {
		// This check depends on how testdataLocator populates GoFiles (base names or full paths)
		// The current testdataLocator returns base names.
		t.Errorf("Expected one Go file named simple.go, got %v", pkg.GoFiles)
	}
	if pkg.Dir != absExpectedPkgDir {
		t.Errorf("Expected Dir to be %s, got %s", absExpectedPkgDir, pkg.Dir)
	}

	t.Logf("Successfully loaded package: %s", pkg.ImportPath)
	t.Logf("Package directory: %s", pkg.Dir)
	t.Logf("Go files: %v", pkg.GoFiles)
}

func TestGetStruct(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: testdataLocator,
	}
	loader := NewLoader(cfg)
	pkgs, err := loader.Load("example.com/simplepkg") // Using custom locator
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/simplepkg': %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(pkgs))
	}
	pkg := pkgs[0]

	// Test GetStruct
	structInfo, err := pkg.GetStruct("MyStruct")
	if err != nil {
		t.Fatalf("Failed to get struct 'MyStruct': %v", err)
	}
	if structInfo.Name != "MyStruct" {
		t.Errorf("Expected struct name 'MyStruct', got '%s'", structInfo.Name)
	}
	// MyStruct in testdata/simplepkg/simple.go is:
	// type MyStruct struct {
	// 	Name      string
	// 	Age       int
	// 	OtherData anotherpkg.AnotherStruct
	// }
	if len(structInfo.Fields) != 3 { // Name, Age, OtherData
		t.Fatalf("Expected 3 fields in MyStruct, got %d. Fields: %+v", len(structInfo.Fields), structInfo.Fields)
	}

	expectedFields := map[string]string{
		"Name":      "string",        // ast.Ident.Name
		"Age":       "int",           // ast.Ident.Name
		"OtherData": "AnotherStruct", // ast.SelectorExpr.Sel.Name
	}
	fieldFound := map[string]bool{}

	for _, field := range structInfo.Fields {
		fieldFound[field.Name] = true
		expectedType, ok := expectedFields[field.Name]
		if !ok {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}

		switch expr := field.TypeExpr.(type) {
		case *ast.Ident: // For types like string, int
			if expr.Name != expectedType {
				t.Errorf("Field %s: expected type %s, got %s", field.Name, expectedType, expr.Name)
			}
		case *ast.SelectorExpr: // For types like anotherpkg.AnotherStruct
			// expr.X is the package identifier (e.g., "anotherpkg")
			// expr.Sel is the type identifier (e.g., "AnotherStruct")
			if expr.Sel.Name != expectedType {
				t.Errorf("Field %s: expected type %s, got selector %s.%s", field.Name, expectedType, expr.X.(*ast.Ident).Name, expr.Sel.Name)
			}
		default:
			t.Errorf("Field %s: unexpected type expression %T", field.Name, field.TypeExpr)
		}
	}

	for name := range expectedFields {
		if !fieldFound[name] {
			t.Errorf("Expected field %s not found", name)
		}
	}
}

func TestResolveImport(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: testdataLocator,
	}
	loader := NewLoader(cfg)
	pkgs, err := loader.Load("example.com/simplepkg") // Use custom locator
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/simplepkg': %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(pkgs))
	}
	pkg := pkgs[0]

	// Ensure simplepkg's AST is parsed to know its declared imports for ResolveImport to check.
	// Calling GetStruct or Files() would parse it.
	if _, err := pkg.GetStruct("MyStruct"); err != nil {
		t.Fatalf("Prerequisite GetStruct on simplepkg failed: %v", err)
	}

	resolvedImport, err := pkg.ResolveImport("example.com/anotherpkg")
	if err != nil {
		t.Fatalf("Failed to resolve import 'example.com/anotherpkg': %v", err)
	}
	if resolvedImport.ImportPath != "example.com/anotherpkg" {
		t.Errorf("Expected resolved import path 'example.com/anotherpkg', got '%s'", resolvedImport.ImportPath)
	}
	if resolvedImport.Name != "anotherpkg" { // Assuming testdataLocator sets Name correctly from dir or file
		t.Errorf("Expected resolved package name 'anotherpkg', got '%s'", resolvedImport.Name)
	}
	// Check if we can get a struct from the resolved package
	_, err = resolvedImport.GetStruct("AnotherStruct")
	if err != nil {
		t.Errorf("Failed to get struct 'AnotherStruct' from resolved package 'anotherpkg': %v", err)
	}
}

func TestLazyLoading(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: testdataLocator,
	}
	loader := NewLoader(cfg)
	pkgs, err := loader.Load("example.com/simplepkg") // Use custom locator
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/simplepkg': %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(pkgs))
	}
	pkg := pkgs[0]

	// 1. Check that files are not parsed initially
	// Accessing internal fields like pkg.parsedFiles for testing is okay.
	if len(pkg.parsedFiles) != 0 {
		t.Errorf("Expected parsedFiles to be empty initially for simplepkg, got %d files. Files: %v", len(pkg.parsedFiles), pkg.parsedFiles)
	}

	// 2. Trigger parsing by accessing a struct
	_, err = pkg.GetStruct("MyStruct")
	if err != nil {
		t.Fatalf("Failed to get struct 'MyStruct' from simplepkg: %v", err)
	}

	// 3. Check that files are now parsed for simplepkg
	if len(pkg.parsedFiles) == 0 {
		t.Errorf("Expected simplepkg.parsedFiles to be populated after GetStruct, but it's empty")
	}
	foundSimpleGo := false
	// The parsedFiles map keys are base filenames as per current Package.ensureParsed logic
	// if it joins with Dir and then stores only file name.
	// Let's re-check how ensureParsed stores keys in p.parsedFiles.
	// It is `p.parsedFiles[goFile] = fileAST` where goFile is relative to Dir.
	// And testdataLocator sets GoFiles to be relative names. So this check should be fine.
	for fileName := range pkg.parsedFiles {
		if fileName == "simple.go" {
			foundSimpleGo = true
			break
		}
	}
	if !foundSimpleGo {
		t.Errorf("simple.go was not found in simplepkg.parsedFiles. Found: %v", pkg.parsedFiles)
	}

	// 4. Check that imports are not resolved initially for the main package
	if len(pkg.resolvedImports) != 0 {
		t.Errorf("Expected simplepkg.resolvedImports to be empty initially, got %v", pkg.resolvedImports)
	}

	// 5. Trigger import resolution
	resolvedImport, err := pkg.ResolveImport("example.com/anotherpkg")
	if err != nil {
		t.Fatalf("Failed to resolve import 'example.com/anotherpkg': %v", err)
	}
	if resolvedImport == nil {
		t.Fatal("resolvedImport is nil after ResolveImport, cannot proceed with lazy check for anotherpkg")
	}

	// 6. Check that 'anotherpkg' is now in resolvedImports of 'simplepkg'
	if _, ok := pkg.resolvedImports["example.com/anotherpkg"]; !ok {
		t.Errorf("'example.com/anotherpkg' not found in simplepkg.resolvedImports after ResolveImport call")
	}

	// 7. Check that 'anotherpkg' itself has not parsed its files yet
	if len(resolvedImport.parsedFiles) != 0 {
		t.Errorf("Expected anotherpkg.parsedFiles to be empty initially, got %d files. Files: %v", len(resolvedImport.parsedFiles), resolvedImport.parsedFiles)
	}

	// 8. Trigger parsing of 'anotherpkg' files
	_, err = resolvedImport.GetStruct("AnotherStruct")
	if err != nil {
		t.Fatalf("Failed to get struct 'AnotherStruct' from 'anotherpkg': %v", err)
	}
	if len(resolvedImport.parsedFiles) == 0 {
		t.Errorf("Expected anotherpkg.parsedFiles to be populated after GetStruct, but it's empty")
	}
	foundAnotherGo := false
	for fileName := range resolvedImport.parsedFiles {
		if fileName == "another.go" {
			foundAnotherGo = true
			break
		}
	}
	if !foundAnotherGo {
		t.Errorf("another.go was not found in anotherpkg.parsedFiles. Found: %v", resolvedImport.parsedFiles)
	}
}
