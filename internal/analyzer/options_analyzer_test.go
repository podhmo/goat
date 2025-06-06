package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath" // For writing temp files
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/metadata"
)

// TestPackageFile represents a single file in a test package.
type TestPackageFile struct {
	Name    string // e.g., "main.go", "external.go"
	Content string
}

// TestModulePackages represents the package structure for a test module.
// Key: package import path suffix (e.g., "example.com/mainpkg")
// Value: List of files in that package
type TestModulePackages map[string][]TestPackageFile

// createTestModuleInTempDir sets up a temporary module on disk.
// It returns the root directory of the module, a list of ASTs for the created Go files, and their FileSet.
func createTestModuleInTempDir(t *testing.T, moduleName string, packages TestModulePackages) (string, []*ast.File, *token.FileSet) {
	t.Helper()
	tempModRoot := t.TempDir()

	// Create go.mod
	goModContent := fmt.Sprintf("module %s\n\ngo 1.18\n", moduleName)
	// Example for adding replace directives if sub-packages are treated as separate modules locally:
	// for pkgImportPathSuffix := range packages {
	// 	 if pkgImportPathSuffix != "." && pkgImportPathSuffix != "" { // Don't add replace for root package if any
	// 	    fullImportPath := moduleName + "/" + pkgImportPathSuffix
	// 	    localPath := "./" + pkgImportPathSuffix
	// 	    goModContent += fmt.Sprintf("replace %s => %s\n", fullImportPath, localPath)
	//   }
	// }

	if err := os.WriteFile(filepath.Join(tempModRoot, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	var createdFileFullPaths []string
	for pkgImportPathSuffix, filesInPkg := range packages {
		// pkgDir is absolute path to the package directory
		var pkgDir string
		if pkgImportPathSuffix == "." || pkgImportPathSuffix == "" { // For files in module root
			pkgDir = tempModRoot
		} else {
			pkgDir = filepath.Join(tempModRoot, pkgImportPathSuffix)
		}

		if err := os.MkdirAll(pkgDir, 0755); err != nil {
			t.Fatalf("Failed to create package directory %s: %v", pkgDir, err)
		}
		for _, file := range filesInPkg {
			filePath := filepath.Join(pkgDir, file.Name)
			if err := os.WriteFile(filePath, []byte(file.Content), 0644); err != nil {
				t.Fatalf("Failed to write file %s: %v", filePath, err)
			}
			createdFileFullPaths = append(createdFileFullPaths, filePath)
		}
	}

	fset := token.NewFileSet()
	var astFiles []*ast.File
	for _, path := range createdFileFullPaths {
		fileAst, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("Failed to parse created file %s: %v", path, err)
		}
		astFiles = append(astFiles, fileAst)
	}
	return tempModRoot, astFiles, fset
}


// parseSingleFileAst is a helper to parse string content into an AST file.
// DEPRECATED for multi-file/multi-package tests. Use createTestModuleInTempDir.
func parseSingleFileAst(t *testing.T, content string) (*token.FileSet, *ast.File) {
	t.Helper()
	// Create a temporary directory and file for parsing to ensure path info is available.
	// This is a simplified on-disk approach for single files if needed.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "testfile.go")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temporary test file: %v", err)
	}

	fset := token.NewFileSet()
	fileAst, err := parser.ParseFile(fset, tmpFile, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("Failed to parse test file content from %s: %v", tmpFile, err)
	}
	return fset, fileAst
}

func TestAnalyzeOptions_WithMixedPackageAsts(t *testing.T) {
	moduleName := "testmixedpkgs"
	mainPkgImportSuffix := "example.com/mainpkg" // Relative to module root
	externalPkgImportSuffix := "example.com/myexternalpkg"
	anotherPkgImportSuffix := "example.com/anotherpkg"

	// Note: Help texts are simplified/removed as they are not the focus of this structural test.
	// The AST parsing from string literals without comments will lose them anyway unless handled.
	mainContent := `package mainpkg // Package name matches last part of import path suffix

import (
	"` + moduleName + `/example.com/myexternalpkg" // Adjusted import path
	"` + moduleName + `/example.com/anotherpkg"    // Adjusted import path
)

// MainConfig is the top-level configuration.
type MainConfig struct {
    LocalName string ` + "`env:\"LOCAL_NAME\"`" + `
    myexternalpkg.ExternalEmbedded
    *myexternalpkg.PointerPkgConfig
    *anotherpkg.AnotherExternalEmbedded
}
`
	externalPkgContent := `package myexternalpkg
type ExternalEmbedded struct { IsRemote bool ` + "`env:\"IS_REMOTE_TAG\"`" + `}
type PointerPkgConfig struct { APIKey string ` + "`env:\"API_KEY_TAG\"`" + `}`

	anotherPkgContent := `package anotherpkg
type AnotherExternalEmbedded struct { Token string }`

	packages := TestModulePackages{
		mainPkgImportSuffix: {
			{Name: "main.go", Content: mainContent},
		},
		externalPkgImportSuffix: {
			{Name: "externalpkg.go", Content: externalPkgContent},
		},
		anotherPkgImportSuffix: {
			{Name: "anotherpkg.go", Content: anotherPkgContent},
		},
	}

	tempModRoot, astFiles, fset := createTestModuleInTempDir(t, moduleName, packages)

	expectedOptions := []*metadata.OptionMetadata{
		{Name: "LocalName", CliName: "local-name", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: "LOCAL_NAME"},
		{Name: "IsRemote", CliName: "is-remote", TypeName: "bool", HelpText: "", IsRequired: true, EnvVar: "IS_REMOTE_TAG"},    // Help text from comments in original strings would be lost here.
		{Name: "APIKey", CliName: "api-key", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: "API_KEY_TAG"},      // Help text lost.
		{Name: "Token", CliName: "token", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: ""},                     // Help text lost.
	}

	// Temporarily comment out the actual call to AnalyzeOptions until it's refactored.
	// The goal here is to ensure the test setup (createTestModuleInTempDir) works.
	// Once AnalyzeOptions (or AnalyzeOptionsV2) is ready, this will be:
	// options, structName, err := AnalyzeOptionsV2(fset, astFiles, "MainConfig", moduleName+"/"+mainPkgImportSuffix, tempModRoot)
	// if err != nil {
	// 	t.Fatalf("AnalyzeOptionsV2 with mixed package ASTs failed: %v\nTemp module root: %s", err, tempModRoot)
	// }
	// ... rest of the assertions ...

	// Dummy assertion to make test pass for now, focusing on setup.
	if tempModRoot == "" {
		t.Error("createTestModuleInTempDir failed to return a module root.")
	}
	if len(astFiles) != 3 {
		t.Errorf("Expected 3 AST files, got %d", len(astFiles))
	}
	if fset == nil {
		t.Error("FileSet is nil")
	}
	// Print for verification, remove later
	// t.Logf("Temp module root: %s", tempModRoot)
	// for _, astFile := range astFiles {
	// 	t.Logf("Parsed AST for file: %s (Package: %s)", fset.File(astFile.Pos()).Name(), astFile.Name.Name)
	// }

	targetPackageID := moduleName + "/" + mainPkgImportSuffix
	options, structNameOut, err := AnalyzeOptionsV2(fset, astFiles, "MainConfig", targetPackageID, tempModRoot)
	if err != nil {
		t.Fatalf("AnalyzeOptionsV2 with mixed package ASTs failed: %v\nTemp module root: %s", err, tempModRoot)
	}

	if structNameOut != "MainConfig" {
		t.Errorf("Expected struct name 'MainConfig', got '%s'", structNameOut)
	}

	if len(options) != len(expectedOptions) {
		t.Fatalf("Expected %d options, got %d. Options: %+v", len(expectedOptions), len(options), options)
	}

	for i, opt := range options {
		expected := expectedOptions[i]
		// HelpText is tricky with current string-based AST generation. It might be lost if comments are not part of the string content.
		// The test source strings for this test do not have comments for fields.
		if opt.Name != expected.Name || opt.CliName != expected.CliName ||
			opt.TypeName != expected.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expected.HelpText) ||
			opt.IsPointer != expected.IsPointer || opt.IsRequired != expected.IsRequired ||
			opt.EnvVar != expected.EnvVar {
			t.Errorf("Option %d (%s) Mismatch:\nExpected: %+v\nGot:      %+v (HelpText was: '%s')", i, opt.Name, expected, opt, opt.HelpText)
		}
	}
}

func TestAnalyzeOptions_WithTextVarTypes(t *testing.T) {
	// This test loads a single, existing file.
	// It can be adapted to use createTestModuleInTempDir if needed,
	// or AnalyzeOptionsV2 can handle single file paths directly.
	// For now, keeping its existing structure but noting it will use AnalyzeOptionsV2.

	fset := token.NewFileSet()
	// Original path relative to test file: "testdata/src/example.com/textvar_pkg/textvar_types.go"
	// To make it absolute for consistency with new approach:
	absPath, err := filepath.Abs("testdata/src/example.com/textvar_pkg/textvar_types.go")
	if err != nil {
		t.Fatalf("Failed to get absolute path for testdata: %v", err)
	}

	// Create a minimal module for this single file
	moduleName := "testtextvartypes"
	// The "package textvar_pkg" implies its import path could be "testtextvartypes/textvar_pkg"
	// or just "textvar_pkg" if it's at the root of a conceptual "example.com"
	// For a single file test, we can place it at the module root.
	// Let package path suffix be "." for module root.
	pkgPathSuffix := "."
	// pkgName := "textvar_pkg" // from "package textvar_pkg" // This variable is unused.

	actualTestdataFileContent, ioErr := os.ReadFile(absPath)
	if ioErr != nil {
		t.Fatalf("Failed to read testdata file %s: %v", absPath, ioErr)
	}

	packages := TestModulePackages{
		pkgPathSuffix: { // Place "textvar_types.go" in the module root
			{Name: "textvar_types.go", Content: string(actualTestdataFileContent)},
		},
	}
	tempModRoot, astFiles, fset := createTestModuleInTempDir(t, moduleName, packages)


	expected := []struct {
		name              string
		isTextUnmarshaler bool
		isTextMarshaler   bool
		typeName          string
	}{
		{"FieldA", true, true, "MyTextValue"}, // TypeName will be simple if in same package
		{"FieldB", true, true, "*MyPtrTextValue"},
		{"FieldC", true, true, "MyPtrTextValue"},
		{"FieldD", false, false, "string"},
		{"FieldE", true, true, "*MyTextValue"},
		{"FieldF", true, false, "MyOnlyUnmarshaler"},
		{"FieldG", false, true, "MyOnlyMarshaler"},
	}

	targetPackageID := moduleName
	if pkgPathSuffix != "." && pkgPathSuffix != "" { // Adjust if pkg is in a subdirectory of the module
		targetPackageID = moduleName + "/" + pkgPathSuffix
	}

	options, structNameOut, errAnalyze := AnalyzeOptionsV2(fset, astFiles, "TextVarOptions", targetPackageID, tempModRoot)
	if errAnalyze != nil {
		t.Fatalf("AnalyzeOptionsV2 failed for TextVarOptions: %v (module root: %s, target pkg: %s)", errAnalyze, tempModRoot, targetPackageID)
	}

	if structNameOut != "TextVarOptions" {
		t.Errorf("Expected struct name 'TextVarOptions', got '%s'", structNameOut)
	}
	if len(options) != len(expected) {
		t.Fatalf("Expected %d options, got %d. Options: %+v", len(expected), len(options), options)
	}
	for i, opt := range options {
		exp := expected[i]
		if opt.Name != exp.name || opt.IsTextUnmarshaler != exp.isTextUnmarshaler || opt.IsTextMarshaler != exp.isTextMarshaler || opt.TypeName != exp.typeName {
			t.Errorf("Field %s Mismatch:\nExpected: name=%s, unmarsh=%v, marsh=%v, type=%s\nGot:      name=%s, unmarsh=%v, marsh=%v, type=%s",
				exp.name, exp.name, exp.isTextUnmarshaler, exp.isTextMarshaler, exp.typeName,
				opt.Name, opt.IsTextUnmarshaler, opt.IsTextMarshaler, opt.TypeName)
		}
	}
}

// Other tests (TestAnalyzeOptions_Simple, _UnexportedFields, etc.) would be refactored similarly.
// For brevity, only _WithMixedPackageAsts and _WithTextVarTypes are shown with the new setup.
// The rest of the file remains unchanged for now, but will need similar refactoring
// or will fail if AnalyzeOptions's signature changes.

func TestAnalyzeOptions_Simple(t *testing.T) {
	contentTemplate := `
package main

// Config holds configuration.
type Config struct {
	// Name of the user.
	Name string %s
	// Age of the user, optional.
	Age *int %s
	// IsAdmin flag.
	IsAdmin bool %s
	// Features list.
	Features []string %s
}
`
	moduleName := "testsimple"

	testCases := []struct {
		name            string
		pkgName         string
		structName      string
		tags            []string
		expectedOptions []*metadata.OptionMetadata
	}{
		{
			name:            "All not required (required tag ignored)",
			pkgName:         "main", // Go package name in source
			structName:      "Config",
			tags:            []string{"`env:\"APP_NAME\"`", "`env:\"USER_AGE\"`", "", "`env:\"APP_FEATURES\"`"},
			expectedOptions: []*metadata.OptionMetadata{
				{Name: "Name", CliName: "name", TypeName: "string", HelpText: "Name of the user.", IsRequired: true, EnvVar: "APP_NAME"},
				{Name: "Age", CliName: "age", TypeName: "*int", HelpText: "Age of the user, optional.", IsPointer: true, IsRequired: false, EnvVar: "USER_AGE"},
				{Name: "IsAdmin", CliName: "is-admin", TypeName: "bool", HelpText: "IsAdmin flag.", IsRequired: true},
				{Name: "Features", CliName: "features", TypeName: "[]string", HelpText: "Features list.", IsRequired: true, EnvVar: "APP_FEATURES"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var formatArgs []interface{}
			for _, tag := range tc.tags {
				formatArgs = append(formatArgs, tag)
			}
			formattedContent := fmt.Sprintf(contentTemplate, formatArgs...)

			currentPkgPathSuffix := "." // Place in module root
			packages := TestModulePackages{
				currentPkgPathSuffix: { {Name: strings.ToLower(tc.structName) + ".go", Content: formattedContent} },
			}
			tempModRoot, astFiles, fset := createTestModuleInTempDir(t, moduleName, packages)

			targetPackageID := moduleName
			if currentPkgPathSuffix != "." && currentPkgPathSuffix != "" {
				targetPackageID = moduleName + "/" + currentPkgPathSuffix
			}

			options, structNameOut, err := AnalyzeOptionsV2(fset, astFiles, tc.structName, targetPackageID, tempModRoot)
			if err != nil {
				t.Fatalf("AnalyzeOptionsV2 failed for %s: %v. Content:\n%s", tc.name, err, formattedContent)
			}
			if structNameOut != tc.structName {
				t.Errorf("Expected struct name '%s', got '%s' for test %s", tc.structName, structNameOut, tc.name)
			}

			if len(options) != len(tc.expectedOptions) {
				t.Fatalf("Expected %d options, got %d for test %s. Options: %+v", len(tc.expectedOptions), len(options), tc.name, options)
			}

			for j, opt := range options {
				expectedOpt := tc.expectedOptions[j]
				// HelpText might be an issue if comments are not correctly in formattedContent / parsed.
				if opt.Name != expectedOpt.Name || opt.CliName != expectedOpt.CliName ||
					opt.TypeName != expectedOpt.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expectedOpt.HelpText) ||
					opt.IsPointer != expectedOpt.IsPointer || opt.IsRequired != expectedOpt.IsRequired ||
					opt.EnvVar != expectedOpt.EnvVar {
					t.Errorf("Option %d (%s) Mismatch for test %s:\nExpected: %+v\nGot:      %+v", j, opt.Name, tc.name, expectedOpt, opt)
				}
			}
		})
	}
}

func TestAnalyzeOptions_UnexportedFields(t *testing.T) {
	content := `
package main
type Config struct {
	Exported   string
	unexported string // Should be ignored
}
`
	moduleName := "testunexported"
	packages := TestModulePackages{
		".": { {Name: "config.go", Content: content} },
	}
	tempModRoot, astFiles, fset := createTestModuleInTempDir(t, moduleName, packages)

	targetPackageID := moduleName // Package is at module root
	options, _, err := AnalyzeOptionsV2(fset, astFiles, "Config", targetPackageID, tempModRoot)
	if err != nil {
		t.Fatalf("AnalyzeOptionsV2 failed for UnexportedFields: %v. Content:\n%s", err, content)
	}
	if len(options) != 1 {
		t.Fatalf("Expected 1 option, got %d. Unexported field was not ignored. Options: %+v", len(options), options)
	}
	if options[0].Name != "Exported" {
		t.Errorf("Expected option name 'Exported', got '%s'", options[0].Name)
	}
}

func TestAnalyzeOptions_StructNotFound(t *testing.T) {
	content := `package main; type OtherStruct struct{}`
	moduleName := "teststructnotfound"
	packages := TestModulePackages{
		".": { {Name: "other.go", Content: content} },
	}
	tempModRoot, astFiles, fset := createTestModuleInTempDir(t, moduleName, packages)

	targetPackageID := moduleName // Package is at module root
	_, _, err := AnalyzeOptionsV2(fset, astFiles, "NonExistentConfig", targetPackageID, tempModRoot)
	if err == nil {
		t.Fatal("AnalyzeOptionsV2 should have failed for a non-existent struct")
	}
	// Expected error: "options struct type 'NonExistentConfig' (simple name 'NonExistentConfig') not found in package 'teststructnotfound'"
	expectedErrorSubstring := "options struct type 'NonExistentConfig' (simple name 'NonExistentConfig') not found in package"
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error message to contain '%s', but got: %v", expectedErrorSubstring, err)
	}
}

func TestAnalyzeOptions_WithEmbeddedStructs(t *testing.T) {
	moduleName := "testembedded"

	content1 := `
package main
type EmbeddedConfig struct {
	EmbeddedString string %s
	EmbeddedInt *int %s
}
type ParentConfig struct {
	ParentField bool %s
	EmbeddedConfig
	AnotherField string
}`
	formattedContent1 := fmt.Sprintf(content1, "`env:\"EMBEDDED_STRING\"`", "`env:\"EMBEDDED_INT\"`", "`env:\"PARENT_FIELD\"`")
	packages1 := TestModulePackages{ ".": { {Name: "config1.go", Content: formattedContent1} } }
	tempModRoot1, astFiles1, fset1 := createTestModuleInTempDir(t, moduleName+"1", packages1)

	targetPackageID1 := moduleName + "1" // moduleName is "testembedded", so "testembedded1"
	options1, structName1, err1Analyze := AnalyzeOptionsV2(fset1, astFiles1, "ParentConfig", targetPackageID1, tempModRoot1)
	if err1Analyze != nil {
		t.Fatalf("Scenario 1: AnalyzeOptionsV2 failed: %v. Content:\n%s", err1Analyze, formattedContent1)
	}
	expectedOptions1 := []*metadata.OptionMetadata{
		{Name: "ParentField", CliName: "parent-field", TypeName: "bool", HelpText: "", IsRequired: true, EnvVar: "PARENT_FIELD"},
		{Name: "EmbeddedString", CliName: "embedded-string", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: "EMBEDDED_STRING"},
		{Name: "EmbeddedInt", CliName: "embedded-int", TypeName: "*int", HelpText: "", IsPointer: true, IsRequired: false, EnvVar: "EMBEDDED_INT"},
		{Name: "AnotherField", CliName: "another-field", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: ""},
	}
	if structName1 != "ParentConfig" {
		t.Errorf("Scenario 1: Expected struct name 'ParentConfig', got '%s'", structName1)
	}
	if len(options1) != len(expectedOptions1) {
		t.Fatalf("Scenario 1: Expected %d options, got %d. Options: %+v", len(expectedOptions1), len(options1), options1)
	}
	for i, opt := range options1 {
		expectedOpt := expectedOptions1[i]
		if opt.Name != expectedOpt.Name || opt.CliName != expectedOpt.CliName || opt.TypeName != expectedOpt.TypeName ||
			strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expectedOpt.HelpText) || opt.IsPointer != expectedOpt.IsPointer ||
			opt.IsRequired != expectedOpt.IsRequired || opt.EnvVar != expectedOpt.EnvVar {
			t.Errorf("Scenario 1, Option %d Mismatch:\nExpected: %+v\nGot:      %+v", i, expectedOpt, opt)
		}
	}

	content2 := `
package main
type EmbeddedPointerConfig struct { PtrEmbeddedField float64 %s }
type ParentWithPointerEmbedded struct { ParentOwn string; *EmbeddedPointerConfig }`
	formattedContent2 := fmt.Sprintf(content2, "`env:\"PTR_EMBEDDED_FLOAT\"`")
	packages2 := TestModulePackages{".": {{Name: "config2.go", Content: formattedContent2}}}
	tempModRoot2, astFiles2, fset2 := createTestModuleInTempDir(t, moduleName+"2", packages2)

	targetPackageID2 := moduleName + "2"
	options2, structName2, err2Analyze := AnalyzeOptionsV2(fset2, astFiles2, "ParentWithPointerEmbedded", targetPackageID2, tempModRoot2)
	if err2Analyze != nil {
		t.Fatalf("Scenario 2: AnalyzeOptionsV2 failed: %v. Content:\n%s", err2Analyze, formattedContent2)
	}
	expectedOptions2 := []*metadata.OptionMetadata{
		{Name: "ParentOwn", CliName: "parent-own", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: ""},
		{Name: "PtrEmbeddedField", CliName: "ptr-embedded-field", TypeName: "float64", HelpText: "", IsRequired: true, EnvVar: "PTR_EMBEDDED_FLOAT"},
	}
	if structName2 != "ParentWithPointerEmbedded" {
		t.Errorf("Scenario 2: Expected struct name 'ParentWithPointerEmbedded', got '%s'", structName2)
	}
	if len(options2) != len(expectedOptions2) {
		t.Fatalf("Scenario 2: Expected %d options, got %d. Options: %+v", len(expectedOptions2), len(options2), options2)
	}
	for i, opt := range options2 {
		expectedOpt := expectedOptions2[i]
		if opt.Name != expectedOpt.Name || opt.CliName != expectedOpt.CliName || opt.TypeName != expectedOpt.TypeName ||
			strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expectedOpt.HelpText) || opt.IsPointer != expectedOpt.IsPointer ||
			opt.IsRequired != expectedOpt.IsRequired || opt.EnvVar != expectedOpt.EnvVar {
			t.Errorf("Scenario 2, Option %d Mismatch:\nExpected: %+v\nGot:      %+v", i, expectedOpt, opt)
		}
	}
}

func TestAnalyzeOptions_WithExternalPackages(t *testing.T) {
	moduleName := "testexternalpkgs"
	mainPkgImportSuffix := "example.com/mainpkg"
	externalPkgImportSuffix := "example.com/myexternalpkg"
	anotherPkgImportSuffix := "example.com/anotherpkg"

	mainContent := `package mainpkg
import (
	"` + moduleName + `/example.com/myexternalpkg"
	"` + moduleName + `/example.com/anotherpkg"
)
type MainConfig struct {
	LocalName string ` + "`env:\"LOCAL_NAME\"`" + `
	myexternalpkg.ExternalEmbedded
	*myexternalpkg.PointerPkgConfig
	*anotherpkg.AnotherExternalEmbedded
}`
	externalPkgContent := `package myexternalpkg
type ExternalEmbedded struct { IsRemote bool ` + "`env:\"IS_REMOTE_TAG\"`" + `}
type PointerPkgConfig struct { APIKey string ` + "`env:\"API_KEY_TAG\"`" + `}`
	anotherPkgContent := `package anotherpkg
type AnotherExternalEmbedded struct { Token string }`

	packages := TestModulePackages{
		mainPkgImportSuffix:       { {Name: "main.go", Content: mainContent} },
		externalPkgImportSuffix:   { {Name: "external.go", Content: externalPkgContent} },
		anotherPkgImportSuffix:    { {Name: "another.go", Content: anotherPkgContent} },
	}
	tempModRoot, astFiles, fset := createTestModuleInTempDir(t, moduleName, packages)

	targetPackageID := moduleName + "/" + mainPkgImportSuffix
	options, structNameOut, errAnalyze := AnalyzeOptionsV2(fset, astFiles, "MainConfig", targetPackageID, tempModRoot)
	if errAnalyze != nil {
		t.Fatalf("AnalyzeOptionsV2 with external packages failed: %v. Content paths:\nMain: %s\nExternal: %s\nAnother: %s",
			errAnalyze,
			filepath.Join(tempModRoot, mainPkgImportSuffix, "main.go"),
			filepath.Join(tempModRoot, externalPkgImportSuffix, "external.go"),
			filepath.Join(tempModRoot, anotherPkgImportSuffix, "another.go"),
		)
	}
	expectedOptions := []*metadata.OptionMetadata{
		{Name: "LocalName", CliName: "local-name", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: "LOCAL_NAME"},
		{Name: "IsRemote", CliName: "is-remote", TypeName: "bool", HelpText: "", IsRequired: true, EnvVar: "IS_REMOTE_TAG"},
		{Name: "APIKey", CliName: "api-key", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: "API_KEY_TAG"},
		{Name: "Token", CliName: "token", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: ""},
	}
	if structNameOut != "MainConfig" {
		t.Errorf("Expected struct name 'MainConfig', got '%s'", structNameOut)
	}
	if len(options) != len(expectedOptions) {
		t.Fatalf("Expected %d options, got %d. Options: %+v", len(expectedOptions), len(options), options)
	}
	for i, opt := range options {
		expected := expectedOptions[i]
		if opt.Name != expected.Name || opt.CliName != expected.CliName || opt.TypeName != expected.TypeName ||
			strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expected.HelpText) || opt.IsPointer != expected.IsPointer ||
			opt.IsRequired != expected.IsRequired || opt.EnvVar != expected.EnvVar {
			t.Errorf("Option %d (%s) Mismatch:\nExpected: %+v\nGot:      %+v", i, opt.Name, expected, opt)
		}
	}
}

func TestAnalyzeOptions_ExternalPackageDirectly(t *testing.T) {
	moduleName := "testdirectexternal"
	pkgSuffix := "example.com/myexternalpkg"
	externalPkgContent := `package myexternalpkg
type ExternalConfig struct { ExternalURL string; ExternalRetryCount int }`

	packages := TestModulePackages{
		pkgSuffix: { {Name: "external.go", Content: externalPkgContent} },
	}
	tempModRoot, astFiles, fset := createTestModuleInTempDir(t, moduleName, packages)

	targetPackageID := moduleName + "/" + pkgSuffix
	options, structNameOut, errAnalyze := AnalyzeOptionsV2(fset, astFiles, "ExternalConfig", targetPackageID, tempModRoot)
	if errAnalyze != nil {
		t.Fatalf("AnalyzeOptionsV2 for direct external package failed: %v. Content path: %s",
			errAnalyze, filepath.Join(tempModRoot, pkgSuffix, "external.go"))
	}
	expectedOptions := []*metadata.OptionMetadata{
		{Name: "ExternalURL", CliName: "external-url", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: ""},
		{Name: "ExternalRetryCount", CliName: "external-retry-count", TypeName: "int", HelpText: "", IsRequired: true, EnvVar: ""},
	}
	if structNameOut != "ExternalConfig" {
		t.Errorf("Expected struct name 'ExternalConfig', got '%s'", structNameOut)
	}
	if len(options) != len(expectedOptions) {
		t.Fatalf("Expected %d options, got %d. Options: %+v", len(expectedOptions), len(options), options)
	}
	for i, opt := range options {
		expected := expectedOptions[i]
		if opt.Name != expected.Name || opt.CliName != expected.CliName || opt.TypeName != expected.TypeName ||
			strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expected.HelpText) || opt.IsPointer != expected.IsPointer ||
			opt.IsRequired != expected.IsRequired || opt.EnvVar != expected.EnvVar {
			t.Errorf("Option %d (%s) Mismatch:\nExpected: %+v\nGot:      %+v", i, opt.Name, expected, opt)
		}
	}
}

// NOTE: The original TestAnalyzeOptions_WithMixedPackageAsts had detailed assertions for option metadata.
// These will need to be reinstated and potentially adjusted once AnalyzeOptionsV2 is fully integrated.
// Specifically, HelpText might be lost if not properly handled during AST parsing/recreation or if comments are stripped.
// The focus of this refactoring step is the on-disk module setup.
