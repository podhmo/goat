package loader

import (
	"context"
	"fmt"
	"go/ast"
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
func testdataLocator(ctx context.Context, pattern string, buildCtx BuildContext) ([]PackageMetaInfo, error) {
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
	loader := New(cfg)

	ctx := context.Background()
	pkgs, err := loader.Load(ctx, "example.com/simplepkg") // Use specific "module" path
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
	loader := New(cfg)
	ctx := context.Background()
	pkgs, err := loader.Load(ctx, "example.com/simplepkg") // Using custom locator
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
	loader := New(cfg)
	ctx := context.Background()
	pkgs, err := loader.Load(ctx, "example.com/simplepkg") // Use custom locator
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

	resolvedImport, err := pkg.ResolveImport(ctx, "example.com/anotherpkg")
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
	loader := New(cfg)
	ctx := context.Background()
	pkgs, err := loader.Load(ctx, "example.com/simplepkg") // Use custom locator
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
	resolvedImport, err := pkg.ResolveImport(ctx, "example.com/anotherpkg")
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

func TestGetStructWithEmbeddedForeignStruct(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: testdataLocator,
	}
	loader := New(cfg)

	// 1. Load the userpkg package
	ctx := context.Background()
	pkgs, err := loader.Load(ctx, "example.com/embed_foreign_pkg_user")
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/embed_foreign_pkg_user': %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(pkgs))
	}
	userPkg := pkgs[0]
	if userPkg.Name != "userpkg" {
		t.Errorf("Expected package name 'userpkg', got '%s'", userPkg.Name)
	}

	// 2. Get the StructInfo for UserStruct
	userStructInfo, err := userPkg.GetStruct("UserStruct")
	if err != nil {
		t.Fatalf("Failed to get struct 'UserStruct' from package '%s': %v", userPkg.Name, err)
	}
	if userStructInfo.Name != "UserStruct" {
		t.Errorf("Expected struct name 'UserStruct', got '%s'", userStructInfo.Name)
	}

	// 3. Verify regular fields and find the embedded field
	expectedRegularFields := map[string]struct {
		typeName string
		tag      string
	}{
		"Name":      {typeName: "string", tag: "`json:\"name\"`"},
		"OwnField":  {typeName: "string", tag: "`json:\"own_field\"`"},
		"AnotherID": {typeName: "int", tag: "`json:\"another_id\" custom_tag:\"custom_value\"`"},
	}
	var embeddedField *FieldInfo
	regularFieldsFound := 0

	for _, field := range userStructInfo.Fields {
		if data, ok := expectedRegularFields[field.Name]; ok {
			regularFieldsFound++
			if ident, okType := field.TypeExpr.(*ast.Ident); !okType || ident.Name != data.typeName {
				t.Errorf("Field %s: expected type %s, got %T (%v)", field.Name, data.typeName, field.TypeExpr, field.TypeExpr)
			}
			if field.Tag != data.tag {
				t.Errorf("Field %s: expected tag '%s', got '%s'", field.Name, data.tag, field.Tag)
			}
		} else if field.Embedded {
			if embeddedField != nil {
				t.Fatalf("Found multiple embedded fields in UserStruct, expected only one (BaseStruct)")
			}
			embeddedField = &field
		} else {
			t.Errorf("Unexpected field '%s' in UserStruct", field.Name)
		}
	}

	if regularFieldsFound != len(expectedRegularFields) {
		t.Errorf("Expected %d regular fields, found %d", len(expectedRegularFields), regularFieldsFound)
	}
	if embeddedField == nil {
		t.Fatalf("Did not find the embedded BaseStruct field in UserStruct")
	}

	// 4. Verify embedded BaseStruct field
	selExpr, ok := embeddedField.TypeExpr.(*ast.SelectorExpr)
	if !ok {
		t.Fatalf("Embedded field's TypeExpr is not *ast.SelectorExpr, got %T", embeddedField.TypeExpr)
	}
	pkgAliasIdent, ok := selExpr.X.(*ast.Ident)
	if !ok {
		t.Fatalf("Embedded field's SelectorExpr.X is not *ast.Ident, got %T", selExpr.X)
	}
	embeddedPkgAlias := pkgAliasIdent.Name
	if selExpr.Sel.Name != "BaseStruct" {
		t.Errorf("Embedded field's type selector is not 'BaseStruct', got '%s'", selExpr.Sel.Name)
	}

	// 5. Determine the full import path for the package alias
	// Need to parse the files of userPkg to inspect imports
	userPkgFiles, err := userPkg.Files()
	if err != nil {
		t.Fatalf("Failed to get AST files for userPkg: %v", err)
	}
	var basePkgFullImportPath string
	foundImport := false
	for _, astFile := range userPkgFiles { // Should be only one, user.go
		for _, importSpec := range astFile.Imports {
			importPathValue := strings.Trim(importSpec.Path.Value, `"`)
			var alias string
			if importSpec.Name != nil {
				alias = importSpec.Name.Name
			} else {
				// If no explicit alias, the package name is implicitly the alias.
				// The testdataLocator should have set the correct package name for "example.com/embed_foreign_pkg_base"
				// which is "basepkg". However, the embedded struct uses `embed_foreign_pkg_base.BaseStruct`.
				// This means the import statement `import "example.com/embed_foreign_pkg_base"` implies
				// that `embed_foreign_pkg_base` is the identifier used.
				// This usually happens if the directory name is `embed_foreign_pkg_base`.
				// Let's verify the package name from the locator.
				// The problem statement implies `embed_foreign_pkg_base` is used as the selector.
				// This means `selExpr.X` is `embed_foreign_pkg_base`.
				// So we need to match `embeddedPkgAlias` (from selExpr.X) with the alias or the last part of the import path.
				pathParts := strings.Split(importPathValue, "/")
				if len(pathParts) > 0 {
					alias = pathParts[len(pathParts)-1]
				}
			}

			if alias == embeddedPkgAlias {
				basePkgFullImportPath = importPathValue
				foundImport = true
				break
			}
		}
		if foundImport {
			break
		}
	}

	if !foundImport {
		t.Fatalf("Could not find import spec for alias '%s' in userpkg's files", embeddedPkgAlias)
	}
	expectedBaseImportPath := "example.com/embed_foreign_pkg_base"
	if basePkgFullImportPath != expectedBaseImportPath {
		t.Errorf("Expected full import path '%s' for alias '%s', got '%s'",
			expectedBaseImportPath, embeddedPkgAlias, basePkgFullImportPath)
	}

	// 6. Resolve the imported package for BaseStruct
	basePkg, err := userPkg.ResolveImport(ctx, basePkgFullImportPath)
	if err != nil {
		t.Fatalf("Failed to resolve import '%s' for BaseStruct: %v", basePkgFullImportPath, err)
	}
	if basePkg == nil {
		t.Fatalf("Resolved package for '%s' is nil", basePkgFullImportPath)
	}
	if basePkg.ImportPath != expectedBaseImportPath {
		t.Errorf("Resolved package import path is '%s', expected '%s'", basePkg.ImportPath, expectedBaseImportPath)
	}
	if basePkg.Name != "basepkg" { // As defined in base.go
		t.Errorf("Resolved package name is '%s', expected 'basepkg'", basePkg.Name)
	}

	// 7. Get the StructInfo for BaseStruct from the resolved basepkg
	baseStructInfo, err := basePkg.GetStruct("BaseStruct")
	if err != nil {
		t.Fatalf("Failed to get struct 'BaseStruct' from resolved package '%s': %v", basePkg.Name, err)
	}
	if baseStructInfo.Name != "BaseStruct" {
		t.Errorf("Expected struct name 'BaseStruct' from resolved package, got '%s'", baseStructInfo.Name)
	}

	// 8. Verify fields of BaseStruct
	expectedBaseFields := map[string]struct {
		typeName string
		tag      string
	}{
		"ID":      {typeName: "int", tag: "`json:\"id,omitempty\" xml:\"id,attr\"`"},
		"Version": {typeName: "string", tag: "`json:\"version\" xml:\"version\"`"},
	}
	baseFieldsFound := 0
	for _, field := range baseStructInfo.Fields {
		if data, ok := expectedBaseFields[field.Name]; ok {
			baseFieldsFound++
			if ident, okType := field.TypeExpr.(*ast.Ident); !okType || ident.Name != data.typeName {
				t.Errorf("BaseStruct field %s: expected type %s, got %T (%v)", field.Name, data.typeName, field.TypeExpr, field.TypeExpr)
			}
			if field.Tag != data.tag {
				t.Errorf("BaseStruct field %s: expected tag '%s', got '%s'", field.Name, data.tag, field.Tag)
			}
		} else {
			t.Errorf("Unexpected field '%s' in BaseStruct", field.Name)
		}
	}
	if baseFieldsFound != len(expectedBaseFields) {
		t.Errorf("Expected %d fields in BaseStruct, found %d", len(expectedBaseFields), baseFieldsFound)
	}

	t.Log("Successfully verified UserStruct with embedded foreign BaseStruct.")
}

func TestCachingMechanisms(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: testdataLocator,
	}
	loader := New(cfg)
	ctx := context.Background()

	// --- simplepkg ---
	pkgsSimple1, err := loader.Load(ctx, "example.com/simplepkg")
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/simplepkg': %v", err)
	}
	if len(pkgsSimple1) != 1 {
		t.Fatalf("Expected 1 package for simplepkg, got %d", len(pkgsSimple1))
	}
	pkgSimple1 := pkgsSimple1[0]

	// Trigger parsing for simplepkg
	if _, err := pkgSimple1.Files(); err != nil {
		t.Fatalf("pkgSimple1.Files() failed: %v", err)
	}

	simpleGoPath := filepath.Join(pkgSimple1.Dir, "simple.go")
	absSimpleGoPath, err := filepath.Abs(simpleGoPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for simple.go: %v", err)
	}

	// Check AST Cache for simple.go
	astFile1, found := loader.GetAST(absSimpleGoPath)
	if !found {
		t.Errorf("AST for %s not found in cache after loading simplepkg", absSimpleGoPath)
	}
	if astFile1 == nil {
		t.Fatalf("Cached AST for %s is nil", absSimpleGoPath)
	}
	if astFile1.Name.Name != "simplepkg" {
		t.Errorf("Expected cached AST for simple.go to have package name 'simplepkg', got '%s'", astFile1.Name.Name)
	}

	// Check Symbol Cache for simplepkg:MyStruct
	infoMyStruct, found := loader.LookupSymbol("example.com/simplepkg:MyStruct")
	if !found {
		t.Errorf("Symbol 'example.com/simplepkg:MyStruct' not found in cache")
	}
	if infoMyStruct.SymbolName != "MyStruct" {
		t.Errorf("Expected symbol name 'MyStruct', got '%s'", infoMyStruct.SymbolName)
	}
	if infoMyStruct.PackagePath != "example.com/simplepkg" {
		t.Errorf("Expected package path 'example.com/simplepkg', got '%s'", infoMyStruct.PackagePath)
	}
	if infoMyStruct.FilePath != absSimpleGoPath {
		t.Errorf("Expected file path '%s', got '%s'", absSimpleGoPath, infoMyStruct.FilePath)
	}
	if _, ok := infoMyStruct.Node.(*ast.TypeSpec); !ok {
		t.Errorf("Expected Node for MyStruct to be *ast.TypeSpec, got %T", infoMyStruct.Node)
	}

	// Check Symbol Cache for simplepkg:Greet
	infoGreet, found := loader.LookupSymbol("example.com/simplepkg:Greet")
	if !found {
		t.Errorf("Symbol 'example.com/simplepkg:Greet' not found in cache")
	}
	if infoGreet.SymbolName != "Greet" {
		t.Errorf("Expected symbol name 'Greet', got '%s'", infoGreet.SymbolName)
	}
	if infoGreet.FilePath != absSimpleGoPath {
		t.Errorf("Expected file path for Greet to be '%s', got '%s'", absSimpleGoPath, infoGreet.FilePath)
	}
	if _, ok := infoGreet.Node.(*ast.FuncDecl); !ok {
		t.Errorf("Expected Node for Greet to be *ast.FuncDecl, got %T", infoGreet.Node)
	}

	// Test Package Cache Hit
	pkgsSimple2, err := loader.Load(ctx, "example.com/simplepkg")
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/simplepkg' a second time: %v", err)
	}
	if len(pkgsSimple2) != 1 {
		t.Fatalf("Expected 1 package for second load of simplepkg, got %d", len(pkgsSimple2))
	}
	pkgSimple2 := pkgsSimple2[0]
	if pkgSimple1 != pkgSimple2 {
		t.Errorf("Expected pkgSimple1 and pkgSimple2 to be the same instance due to package caching")
	}

	// Trigger parsing for this instance (pkgSimple2) to ensure it uses cached ASTs if possible
	if _, err := pkgSimple2.Files(); err != nil {
		t.Fatalf("pkgSimple2.Files() failed: %v", err)
	}

	// Test AST Cache Hit (direct check)
	astFile2, found := loader.GetAST(absSimpleGoPath)
	if !found {
		t.Errorf("AST for %s not found in cache on second access via GetAST", absSimpleGoPath)
	}
	if astFile1 != astFile2 { // Pointer equality
		t.Errorf("AST cache miss: Expected same AST instance for %s from GetAST, got different instances. astFile1=%p, astFile2=%p", absSimpleGoPath, astFile1, astFile2)
	}

	// --- anotherpkg ---
	pkgsAnother1, err := loader.Load(ctx, "example.com/anotherpkg")
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/anotherpkg': %v", err)
	}
	if len(pkgsAnother1) != 1 {
		t.Fatalf("Expected 1 package for anotherpkg, got %d", len(pkgsAnother1))
	}
	pkgAnother1 := pkgsAnother1[0]

	if _, err := pkgAnother1.Files(); err != nil { // Trigger parsing
		t.Fatalf("pkgAnother1.Files() failed: %v", err)
	}

	anotherGoPath := filepath.Join(pkgAnother1.Dir, "another.go")
	absAnotherGoPath, err := filepath.Abs(anotherGoPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for another.go: %v", err)
	}

	astAnotherFile1, found := loader.GetAST(absAnotherGoPath)
	if !found {
		t.Errorf("AST for %s not found in cache after loading anotherpkg", absAnotherGoPath)
	}
	if astAnotherFile1 == nil {
		t.Fatalf("Cached AST for %s is nil", absAnotherGoPath)
	}
	if astAnotherFile1.Name.Name != "anotherpkg" {
		t.Errorf("Expected cached AST for another.go to have package name 'anotherpkg', got '%s'", astAnotherFile1.Name.Name)
	}

	// Check Symbol Cache for anotherpkg:AnotherStruct
	infoAnotherStruct, found := loader.LookupSymbol("example.com/anotherpkg:AnotherStruct")
	if !found {
		t.Errorf("Symbol 'example.com/anotherpkg:AnotherStruct' not found in cache")
	}
	if infoAnotherStruct.SymbolName != "AnotherStruct" {
		t.Errorf("Expected symbol name 'AnotherStruct', got '%s'", infoAnotherStruct.SymbolName)
	}
	if infoAnotherStruct.PackagePath != "example.com/anotherpkg" {
		t.Errorf("Expected package path 'example.com/anotherpkg', got '%s'", infoAnotherStruct.PackagePath)
	}
	if infoAnotherStruct.FilePath != absAnotherGoPath {
		t.Errorf("Expected file path '%s', got '%s'", absAnotherGoPath, infoAnotherStruct.FilePath)
	}
	if _, ok := infoAnotherStruct.Node.(*ast.TypeSpec); !ok {
		t.Errorf("Expected Node for AnotherStruct to be *ast.TypeSpec, got %T", infoAnotherStruct.Node)
	}
}
