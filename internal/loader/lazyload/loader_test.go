package lazyload

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// newTestdataLocator creates a PackageLocator closure that captures the true root of the testdata directory.
func newTestdataLocator(t *testing.T, testdataRoot string) PackageLocator {
	return func(pattern string, baseDir string, buildCtx BuildContext) ([]PackageMetaInfo, error) {
		t.Logf("testdataLocator: pattern='%s', baseDirFromLoaderArgument='%s', capturedTestdataRoot='%s'", pattern, baseDir, testdataRoot)

		parts := strings.Split(pattern, "/")
		if len(parts) == 0 {
			return nil, fmt.Errorf("testdataLocator: invalid pattern %q", pattern)
		}
		pkgSubDirName := parts[len(parts)-1]
		pkgActualName := pkgSubDirName

		pkgDiskPath := filepath.Join(testdataRoot, pkgSubDirName)
		absPkgPath, err := filepath.Abs(pkgDiskPath)
		if err != nil {
			return nil, fmt.Errorf("testdataLocator: could not get absolute path for %s (from pattern %s): %w", pkgDiskPath, pattern, err)
		}

		dirEntries, err := os.ReadDir(absPkgPath)
		if err != nil {
			return nil, fmt.Errorf("testdataLocator: could not read dir %s (for pattern %s): %w", absPkgPath, pattern, err)
		}

		var goFiles []string
		var directImports []string
		fset := token.NewFileSet()

		for _, entry := range dirEntries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				goFiles = append(goFiles, entry.Name())
				filePath := filepath.Join(absPkgPath, entry.Name())
				src, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly|parser.PackageClauseOnly)
				if err != nil {
					return nil, fmt.Errorf("testdataLocator: failed to parse %s: %w", filePath, err)
				}
				if pkgActualName == pkgSubDirName && src.Name != nil { // Use actual package name from source
					pkgActualName = src.Name.Name
				}
				for _, importSpec := range src.Imports {
					importedPath := strings.Trim(importSpec.Path.Value, `"`)
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

		meta := PackageMetaInfo{
			ImportPath:    pattern,
			Name:          pkgActualName,
			Dir:           absPkgPath,
			GoFiles:       goFiles,
			DirectImports: directImports,
		}
		return []PackageMetaInfo{meta}, nil
	}
}

// getTestdataPath returns the absolute path to the testdata directory.
func getTestdataPath(t *testing.T) string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get caller information")
	}
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestLoadSimplePackage(t *testing.T) {
	testdataPath := getTestdataPath(t)
	expectedPkgDir := filepath.Join(testdataPath, "simplepkg")
	absExpectedPkgDir, err := filepath.Abs(expectedPkgDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for expectedPkgDir: %v", err)
	}

	cfg := Config{
		Context: BuildContext{},
		Locator: newTestdataLocator(t, testdataPath),
	}
	loader := NewLoader(cfg)
	pkgs, err := loader.Load(testdataPath, "example.com/simplepkg")
	if err != nil {
		t.Fatalf("Failed to load package: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("Expected 1 package, got %d", len(pkgs))
	}
	pkg := pkgs[0]
	if pkg.ImportPath != "example.com/simplepkg" {
		t.Errorf("Expected import path %s, got %s", "example.com/simplepkg", pkg.ImportPath)
	}
	if pkg.Name != "simplepkg" {
		t.Errorf("Expected package name simplepkg, got %s", pkg.Name)
	}
	if len(pkg.GoFiles) != 1 || filepath.Base(pkg.GoFiles[0]) != "simple.go" {
		t.Errorf("Expected one Go file named simple.go, got %v", pkg.GoFiles)
	}
	if pkg.Dir != absExpectedPkgDir {
		t.Errorf("Expected Dir to be %s, got %s", absExpectedPkgDir, pkg.Dir)
	}
}

func TestGetStruct(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: newTestdataLocator(t, getTestdataPath(t)),
	}
	loader := NewLoader(cfg)
	pkgs, err := loader.Load(getTestdataPath(t), "example.com/simplepkg")
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/simplepkg': %v", err)
	}
	pkg := pkgs[0]
	structInfo, err := pkg.GetStruct("MyStruct")
	if err != nil {
		t.Fatalf("Failed to get struct 'MyStruct': %v", err)
	}
	if structInfo.Name != "MyStruct" {
		t.Errorf("Expected struct name 'MyStruct', got '%s'", structInfo.Name)
	}
	if len(structInfo.Fields) != 3 {
		t.Fatalf("Expected 3 fields in MyStruct, got %d. Fields: %+v", len(structInfo.Fields), structInfo.Fields)
	}
}

func TestResolveImport(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: newTestdataLocator(t, getTestdataPath(t)),
	}
	loader := NewLoader(cfg)
	pkgs, err := loader.Load(getTestdataPath(t), "example.com/simplepkg")
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/simplepkg': %v", err)
	}
	pkg := pkgs[0]
	if _, err := pkg.GetStruct("MyStruct"); err != nil { // ensure parsed
		t.Fatalf("Prerequisite GetStruct on simplepkg failed: %v", err)
	}
	resolvedImport, err := pkg.ResolveImport("example.com/anotherpkg")
	if err != nil {
		t.Fatalf("Failed to resolve import 'example.com/anotherpkg': %v", err)
	}
	if resolvedImport.ImportPath != "example.com/anotherpkg" {
		t.Errorf("Expected resolved import path 'example.com/anotherpkg', got '%s'", resolvedImport.ImportPath)
	}
	if resolvedImport.Name != "anotherpkg" {
		t.Errorf("Expected resolved package name 'anotherpkg', got '%s'", resolvedImport.Name)
	}
	if _, err = resolvedImport.GetStruct("AnotherStruct"); err != nil {
		t.Errorf("Failed to get struct 'AnotherStruct' from resolved package 'anotherpkg': %v", err)
	}
}

func TestLazyLoading(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: newTestdataLocator(t, getTestdataPath(t)),
	}
	loader := NewLoader(cfg)
	pkgs, err := loader.Load(getTestdataPath(t), "example.com/simplepkg")
	if err != nil {
		t.Fatalf("Failed to load package 'example.com/simplepkg': %v", err)
	}
	pkg := pkgs[0]
	if len(pkg.parsedFiles) != 0 {
		t.Errorf("Expected parsedFiles to be empty initially for simplepkg, got %d files. Files: %v", len(pkg.parsedFiles), pkg.parsedFiles)
	}
	if _, err = pkg.GetStruct("MyStruct"); err != nil {
		t.Fatalf("Failed to get struct 'MyStruct' from simplepkg: %v", err)
	}
	if len(pkg.parsedFiles) == 0 {
		t.Errorf("Expected simplepkg.parsedFiles to be populated after GetStruct, but it's empty")
	}
	if len(pkg.resolvedImports) != 0 {
		t.Errorf("Expected simplepkg.resolvedImports to be empty initially, got %v", pkg.resolvedImports)
	}
	resolvedImport, err := pkg.ResolveImport("example.com/anotherpkg")
	if err != nil {
		t.Fatalf("Failed to resolve import 'example.com/anotherpkg': %v", err)
	}
	if _, ok := pkg.resolvedImports["example.com/anotherpkg"]; !ok {
		t.Errorf("'example.com/anotherpkg' not found in simplepkg.resolvedImports after ResolveImport call")
	}
	if len(resolvedImport.parsedFiles) != 0 {
		t.Errorf("Expected anotherpkg.parsedFiles to be empty initially, got %d files. Files: %v", len(resolvedImport.parsedFiles), resolvedImport.parsedFiles)
	}
	if _, err = resolvedImport.GetStruct("AnotherStruct"); err != nil {
		t.Fatalf("Failed to get struct 'AnotherStruct' from 'anotherpkg': %v", err)
	}
	if len(resolvedImport.parsedFiles) == 0 {
		t.Errorf("Expected anotherpkg.parsedFiles to be populated after GetStruct, but it's empty")
	}
}

/*
func TestGetStructWithEmbeddedForeignStruct(t *testing.T) {
	cfg := Config{
		Context: BuildContext{},
		Locator: newTestdataLocator(t, getTestdataPath(t)),
	}
	loader := NewLoader(cfg)

	// 1. Load the userpkg package
	pkgs, err := loader.Load(getTestdataPath(t), "example.com/embed_foreign_pkg_user")
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
	userPkgFiles, err := userPkg.Files()
	if err != nil {
		t.Fatalf("Failed to get AST files for userPkg: %v", err)
	}
	var basePkgFullImportPath string
	foundImport := false
	for _, astFile := range userPkgFiles {
		for _, importSpec := range astFile.Imports {
			importPathValue := strings.Trim(importSpec.Path.Value, `"`)
			var alias string
			if importSpec.Name != nil {
				alias = importSpec.Name.Name
			} else {
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
	basePkg, err := userPkg.ResolveImport(basePkgFullImportPath)
	if err != nil {
		t.Fatalf("Failed to resolve import '%s' for BaseStruct: %v", basePkgFullImportPath, err)
	}
	if basePkg.ImportPath != expectedBaseImportPath {
		t.Errorf("Resolved package import path is '%s', expected '%s'", basePkg.ImportPath, expectedBaseImportPath)
	}
	if basePkg.Name != "basepkg" {
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
*/
