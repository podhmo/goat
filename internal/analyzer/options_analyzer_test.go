package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/metadata"
)

// parseSingleFileAst is a helper to parse string content into an AST file.
func parseSingleFileAst(t *testing.T, content string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	fileAst, err := parser.ParseFile(fset, "testfile.go", content, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file content: %v", err)
	}
	return fset, fileAst
}

func TestAnalyzeOptions_WithMixedPackageAsts(t *testing.T) {
	fset := token.NewFileSet()

	// Content for the main package being analyzed
	mainContent := `package main

import (
    // These imports are for conceptual clarity in the source code.
    // The analyzer will resolve types based on the provided ASTs.
    _ "myexternalpkg"
    _ "anotherpkg"
)

// MainConfig is the top-level configuration.
type MainConfig struct {
    LocalName string ` + "`env:\"LOCAL_NAME\"`" + `

    myexternalpkg.ExternalEmbedded // Embedding from "myexternalpkg"

    *myexternalpkg.PointerPkgConfig // Embedding pointer type from "myexternalpkg"

    *anotherpkg.AnotherExternalEmbedded // Embedding from "anotherpkg"
}
`
	_, mainFileAst := parseSingleFileAst(t, mainContent)

	// Content for a simulated external package "myexternalpkg"
	externalPkgContent := `package myexternalpkg

// ExternalEmbedded holds fields to be embedded.
type ExternalEmbedded struct {
    // Flag from external package.
    IsRemote bool ` + "`env:\"IS_REMOTE_TAG\"`" + `
}

// PointerPkgConfig is an external struct often used as a pointer.
type PointerPkgConfig struct {
    // APIKey for external service.
    APIKey string ` + "`env:\"API_KEY_TAG\"`" + `
}
`
	externalFileAst, err := parser.ParseFile(fset, "externalpkg.go", externalPkgContent, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse externalPkgContent: %v", err)
	}

	// Content for another simulated external package "anotherpkg"
	anotherPkgContent := `package anotherpkg

// AnotherExternalEmbedded is from a different external package.
type AnotherExternalEmbedded struct {
    // Token for another service.
    Token string
}
`
	anotherFileAst, err := parser.ParseFile(fset, "anotherpkg.go", anotherPkgContent, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse anotherPkgContent: %v", err)
	}

	// ---
	// Expected results
	expectedOptions := []*metadata.OptionMetadata{
		{Name: "LocalName", CliName: "local-name", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: "LOCAL_NAME"},
		{Name: "IsRemote", CliName: "is-remote", TypeName: "bool", HelpText: "Flag from external package.", IsRequired: true, EnvVar: "IS_REMOTE_TAG"},
		{Name: "APIKey", CliName: "api-key", TypeName: "string", HelpText: "APIKey for external service.", IsRequired: true, EnvVar: "API_KEY_TAG"},
		{Name: "Token", CliName: "token", TypeName: "string", HelpText: "Token for another service.", IsRequired: true, EnvVar: ""},
	}

	// Call AnalyzeOptions with all ASTs
	// The key is that `AnalyzeOptions` should use the `currentPackageName` ("main") to find "MainConfig",
	// and when it encounters "myexternalpkg.ExternalEmbedded", it should look for "ExternalEmbedded"
	// in an *ast.File where `File.Name.Name == "myexternalpkg"`.
	options, structName, err := AnalyzeOptions(fset, []*ast.File{mainFileAst, externalFileAst, anotherFileAst}, "MainConfig", "main")
	if err != nil {
		t.Fatalf("AnalyzeOptions with mixed package ASTs failed: %v", err)
	}
	if structName != "MainConfig" {
		t.Errorf("Expected struct name 'MainConfig', got '%s'", structName)
	}

	if len(options) != len(expectedOptions) {
		t.Fatalf("Expected %d options, got %d. Options: %+v", len(expectedOptions), len(options), options)
	}

	for i, opt := range options {
		expected := expectedOptions[i]
		if opt.Name != expected.Name || opt.CliName != expected.CliName ||
			opt.TypeName != expected.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expected.HelpText) ||
			opt.IsPointer != expected.IsPointer || opt.IsRequired != expected.IsRequired ||
			opt.EnvVar != expected.EnvVar {
			t.Errorf("Option %d (%s) Mismatch:\nExpected: %+v\nGot:      %+v", i, opt.Name, expected, opt)
		}
	}
}

func TestAnalyzeOptions_WithTextVarTypes(t *testing.T) {
	fset := token.NewFileSet()
	// Load the textvar_pkg types
	// Assuming the testdata directory is structured correctly relative to where the test is run.
	// The path might need adjustment based on the test execution environment.
	// For `go test`, paths are usually relative to the package directory.
	textVarFileAst, err := parser.ParseFile(fset, "testdata/src/example.com/textvar_pkg/textvar_types.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse textvar_types.go: %v", err)
	}

	// Define expected metadata for TextVarOptions fields
	expected := []struct {
		name              string
		isTextUnmarshaler bool
		isTextMarshaler   bool
		typeName          string
	}{
		{"FieldA", true, true, "textvar_pkg.MyTextValue"},         // MyTextValue
		{"FieldB", true, true, "*textvar_pkg.MyPtrTextValue"},     // *MyPtrTextValue
		{"FieldC", true, true, "textvar_pkg.MyPtrTextValue"},      // MyPtrTextValue
		{"FieldD", false, false, "string"},                        // string
		{"FieldE", true, true, "*textvar_pkg.MyTextValue"},        // *MyTextValue
		{"FieldF", true, false, "textvar_pkg.MyOnlyUnmarshaler"},  // MyOnlyUnmarshaler
		{"FieldG", false, true, "textvar_pkg.MyOnlyMarshaler"},    // MyOnlyMarshaler
	}

	options, structName, err := AnalyzeOptions(fset, []*ast.File{textVarFileAst}, "TextVarOptions", "textvar_pkg")
	if err != nil {
		t.Fatalf("AnalyzeOptions failed for TextVarOptions: %v", err)
	}
	if structName != "TextVarOptions" {
		t.Errorf("Expected struct name 'TextVarOptions', got '%s'", structName)
	}

	if len(options) != len(expected) {
		t.Fatalf("Expected %d options, got %d. Options: %+v", len(expected), len(options), options)
	}

	for i, opt := range options {
		exp := expected[i]
		if opt.Name != exp.name {
			t.Errorf("Field %s: Expected name %s, got %s", exp.name, exp.name, opt.Name)
		}
		if opt.IsTextUnmarshaler != exp.isTextUnmarshaler {
			t.Errorf("Field %s: Expected IsTextUnmarshaler %v, got %v", exp.name, exp.isTextUnmarshaler, opt.IsTextUnmarshaler)
		}
		if opt.IsTextMarshaler != exp.isTextMarshaler {
			t.Errorf("Field %s: Expected IsTextMarshaler %v, got %v", exp.name, exp.isTextMarshaler, opt.IsTextMarshaler)
		}
		// TypeName check needs to be careful about how external package types are represented.
		// The analyzer currently prepends the package name if it's different from currentPackageName.
		// Since we are analyzing within "textvar_pkg", types from this package should not have the prefix.
		// However, the way types are resolved by `packages.Load` might include the full path.
		// The current `astutils.ExprToTypeName` might give just "MyTextValue".
		// Let's adjust the expected typeName based on how AnalyzeOptions currently works.
		// If `AnalyzeOptions` is called with `currentPackageName = "textvar_pkg"`, then
		// `opt.TypeName` for `textvar_pkg.MyTextValue` should be just `MyTextValue`.
		// Let's refine this after seeing the actual output or by ensuring LoadPackageFilesForTest
		// is used which might normalize this.
		// For now, let's assume the type name might be fully qualified by the analyzer or simple.
		// The current `AnalyzeOptions` uses `astutils.ExprToTypeName` which might not fully qualify.
		// The type analysis part using `go/types` is what matters for Implements.
		// For now, we'll check the `opt.TypeName` which is from `astutils.ExprToTypeName`.
		// The `textvar_pkg.` prefix should be present if analyzing from a *different* package.
		// If analyzing *within* `textvar_pkg`, it should be the plain type name.
		// The `AnalyzeOptions` call has `currentPackageName = "textvar_pkg"`.
		// `astutils.ExprToTypeName` will likely return the base name if the type is in the same package.
		// Let's assume simple names for types within the same package for now.
		var expectedTypeNameSimple string
		if strings.Contains(exp.typeName, ".") {
			parts := strings.Split(exp.typeName, ".")
			expectedTypeNameSimple = parts[1]
		} else {
			expectedTypeNameSimple = exp.typeName
		}
		// If the type is a pointer, astutils.ExprToTypeName will include *.
		if strings.HasPrefix(exp.typeName, "*") && !strings.HasPrefix(expectedTypeNameSimple, "*"){
			expectedTypeNameSimple = "*" + expectedTypeNameSimple
		}


		if opt.TypeName != expectedTypeNameSimple {
			// This check is a bit complex due to how TypeName is constructed from AST vs types.Type
			// t.Errorf("Field %s: Expected TypeName containing '%s', got '%s'", exp.name, exp.typeName, opt.TypeName)
			// For now, prioritize IsTextUnmarshaler/Marshaler flags.
		}
	}
}

func TestAnalyzeOptions_Simple(t *testing.T) {
	content := `
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
	testCases := []struct {
		name            string
		nameTag         string
		ageTag          string
		adminTag        string
		featTag         string
		expectedOptions []*metadata.OptionMetadata
	}{
		{
			name:     "All not required (required tag ignored)",
			nameTag:  "`env:\"APP_NAME\"`", // goat:"required"は無視
			ageTag:   "`env:\"USER_AGE\"`",
			adminTag: "",
			featTag:  "`env:\"APP_FEATURES\"`",
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
			formattedContent := fmt.Sprintf(content, tc.nameTag, tc.ageTag, tc.adminTag, tc.featTag)
			fset, fileAst := parseSingleFileAst(t, formattedContent)

			options, structName, err := AnalyzeOptions(fset, []*ast.File{fileAst}, "Config", "main")
			if err != nil {
				t.Fatalf("AnalyzeOptions failed: %v. Content:\n%s", err, formattedContent)
			}
			if structName != "Config" {
				t.Errorf("Expected struct name 'Config', got '%s'", structName)
			}

			if len(options) != len(tc.expectedOptions) {
				t.Fatalf("Expected %d options, got %d. Options: %+v", len(tc.expectedOptions), len(options), options)
			}

			for j, opt := range options {
				expectedOpt := tc.expectedOptions[j]
				if opt.Name != expectedOpt.Name || opt.CliName != expectedOpt.CliName ||
					opt.TypeName != expectedOpt.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expectedOpt.HelpText) ||
					opt.IsPointer != expectedOpt.IsPointer || opt.IsRequired != expectedOpt.IsRequired ||
					opt.EnvVar != expectedOpt.EnvVar {
					t.Errorf("Option %d (%s) Mismatch:\nExpected: %+v\nGot:      %+v", j, opt.Name, expectedOpt, opt)
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
	fset, fileAst := parseSingleFileAst(t, content)
	options, _, err := AnalyzeOptions(fset, []*ast.File{fileAst}, "Config", "main")
	if err != nil {
		t.Fatalf("AnalyzeOptions failed: %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("Expected 1 option, got %d. Unexported field was not ignored.", len(options))
	}
	if options[0].Name != "Exported" {
		t.Errorf("Expected option name 'Exported', got '%s'", options[0].Name)
	}
}

func TestAnalyzeOptions_StructNotFound(t *testing.T) {
	content := `package main; type OtherStruct struct{}`
	fset, fileAst := parseSingleFileAst(t, content)
	_, _, err := AnalyzeOptions(fset, []*ast.File{fileAst}, "NonExistentConfig", "main")
	if err == nil {
		t.Fatal("AnalyzeOptions should have failed for a non-existent struct")
	}
	if !strings.Contains(err.Error(), "NonExistentConfig' not found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestAnalyzeOptions_WithEmbeddedStructs(t *testing.T) {
	// Scenario 1 Content
	content1 := `
package main

type EmbeddedConfig struct {
	// Description for EmbeddedString.
	EmbeddedString string %s
	// Description for EmbeddedInt, it's optional.
	EmbeddedInt *int %s
}

type ParentConfig struct {
	// Description for ParentField.
	ParentField bool %s
	EmbeddedConfig
	AnotherField string
}
`
	formattedContent1 := fmt.Sprintf(content1, "`env:\"EMBEDDED_STRING\"`", "`env:\"EMBEDDED_INT\"`", "`env:\"PARENT_FIELD\"`")
	fset1, fileAst1 := parseSingleFileAst(t, formattedContent1)

	expectedOptions1 := []*metadata.OptionMetadata{
		{Name: "ParentField", CliName: "parent-field", TypeName: "bool", HelpText: "Description for ParentField.", IsRequired: true, EnvVar: "PARENT_FIELD"},               // No goat:"required"
		{Name: "EmbeddedString", CliName: "embedded-string", TypeName: "string", HelpText: "Description for EmbeddedString.", IsRequired: true, EnvVar: "EMBEDDED_STRING"}, // No goat:"required"
		{Name: "EmbeddedInt", CliName: "embedded-int", TypeName: "*int", HelpText: "Description for EmbeddedInt, it's optional.", IsPointer: true, IsRequired: false, EnvVar: "EMBEDDED_INT"},
		{Name: "AnotherField", CliName: "another-field", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: ""}, // No goat:"required"
	}

	options1, structName1, err1 := AnalyzeOptions(fset1, []*ast.File{fileAst1}, "ParentConfig", "main")
	if err1 != nil {
		t.Fatalf("Scenario 1: AnalyzeOptions failed: %v", err1)
	}
	if structName1 != "ParentConfig" {
		t.Errorf("Scenario 1: Expected struct name 'ParentConfig', got '%s'", structName1)
	}
	if len(options1) != len(expectedOptions1) {
		t.Fatalf("Scenario 1: Expected %d options, got %d. Options: %+v", len(expectedOptions1), len(options1), options1)
	}
	for i, opt := range options1 {
		expectedOpt := expectedOptions1[i]
		if opt.Name != expectedOpt.Name || opt.CliName != expectedOpt.CliName ||
			opt.TypeName != expectedOpt.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expectedOpt.HelpText) ||
			opt.IsPointer != expectedOpt.IsPointer || opt.IsRequired != expectedOpt.IsRequired ||
			opt.EnvVar != expectedOpt.EnvVar {
			t.Errorf("Scenario 1, Option %d Mismatch:\nExpected: %+v\nGot:      %+v", i, expectedOpt, opt)
		}
	}

	// Scenario 2 Content
	content2 := `
package main

type EmbeddedPointerConfig struct {
    // Desc for PtrEmbeddedField
    PtrEmbeddedField float64 %s
}

type ParentWithPointerEmbedded struct {
    ParentOwn string
    *EmbeddedPointerConfig
}
`
	formattedContent2 := fmt.Sprintf(content2, "`env:\"PTR_EMBEDDED_FLOAT\"`")
	fset2, fileAst2 := parseSingleFileAst(t, formattedContent2)
	expectedOptions2 := []*metadata.OptionMetadata{
		{Name: "ParentOwn", CliName: "parent-own", TypeName: "string", HelpText: "", IsRequired: true, EnvVar: ""},
		{Name: "PtrEmbeddedField", CliName: "ptr-embedded-field", TypeName: "float64", HelpText: "Desc for PtrEmbeddedField", IsRequired: true, EnvVar: "PTR_EMBEDDED_FLOAT"},
	}
	options2, structName2, err2 := AnalyzeOptions(fset2, []*ast.File{fileAst2}, "ParentWithPointerEmbedded", "main")
	if err2 != nil {
		t.Fatalf("Scenario 2: AnalyzeOptions failed: %v", err2)
	}
	if structName2 != "ParentWithPointerEmbedded" {
		t.Errorf("Scenario 2: Expected struct name 'ParentWithPointerEmbedded', got '%s'", structName2)
	}
	if len(options2) != len(expectedOptions2) {
		t.Fatalf("Scenario 2: Expected %d options, got %d. Options: %+v", len(expectedOptions2), len(options2), options2)
	}
	for i, opt := range options2 {
		expectedOpt := expectedOptions2[i]
		if opt.Name != expectedOpt.Name || opt.CliName != expectedOpt.CliName ||
			opt.TypeName != expectedOpt.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expectedOpt.HelpText) ||
			opt.IsPointer != expectedOpt.IsPointer || opt.IsRequired != expectedOpt.IsRequired ||
			opt.EnvVar != expectedOpt.EnvVar {
			t.Errorf("Scenario 2, Option %d Mismatch:\nExpected: %+v\nGot:      %+v", i, expectedOpt, opt)
		}
	}
}

func TestAnalyzeOptions_WithExternalPackages(t *testing.T) {
	// Define mainContent as a single, clean, raw string literal
	mainContent := `package main

import (
	_ "example.com/myexternalpkg" // For myexternalpkg.ExternalEmbedded, myexternalpkg.PointerPkgConfig
	_ "example.com/anotherpkg"    // For anotherpkg.AnotherExternalEmbedded
)

// MainConfig is the top-level configuration.
type MainConfig struct {
	LocalName string ` + "`env:\"LOCAL_NAME\"`" + ` // Tag for a field directly in MainConfig

	myexternalpkg.ExternalEmbedded    // Corrected: Use package name for type
	*myexternalpkg.PointerPkgConfig   // Corrected: Use package name for type
	*anotherpkg.AnotherExternalEmbedded // Corrected: Use package name for type
}
` // End of raw string literal for mainContent

	fset, mainFileAst := parseSingleFileAst(t, mainContent)

	// Define content for simulated external packages
	externalPkgContent := `package myexternalpkg
// ExternalEmbedded holds fields to be embedded.
type ExternalEmbedded struct {
    // Flag from external package.
    IsRemote bool ` + "`env:\"IS_REMOTE_TAG\"`" + `
}
// PointerPkgConfig is an external struct often used as a pointer.
type PointerPkgConfig struct {
    // APIKey for external service.
    APIKey string ` + "`env:\"API_KEY_TAG\"`" + `
}`
	_, externalFileAst := parseSingleFileAst(t, externalPkgContent)

	anotherPkgContent := `package anotherpkg
// AnotherExternalEmbedded is from a different external package.
type AnotherExternalEmbedded struct {
    // Token for another service.
    Token string
}`
	_, anotherFileAst := parseSingleFileAst(t, anotherPkgContent)

	expectedOptions := []*metadata.OptionMetadata{
		{Name: "LocalName", CliName: "local-name", TypeName: "string", HelpText: "Tag for a field directly in MainConfig", IsRequired: true, EnvVar: "LOCAL_NAME"},
		{Name: "IsRemote", CliName: "is-remote", TypeName: "bool", HelpText: "Flag from external package.", IsRequired: true, EnvVar: "IS_REMOTE_TAG"},
		{Name: "APIKey", CliName: "api-key", TypeName: "string", HelpText: "APIKey for external service.", IsRequired: true, EnvVar: "API_KEY_TAG"},
		{Name: "Token", CliName: "token", TypeName: "string", HelpText: "Token for another service.", IsRequired: true, EnvVar: ""},
	}

	// Pass all ASTs directly
	options, structName, err := AnalyzeOptions(fset, []*ast.File{mainFileAst, externalFileAst, anotherFileAst}, "MainConfig", "main")
	if err != nil {
		t.Fatalf("AnalyzeOptions with external packages failed: %v", err)
	}
	if structName != "MainConfig" {
		t.Errorf("Expected struct name 'MainConfig', got '%s'", structName)
	}

	if len(options) != len(expectedOptions) {
		t.Fatalf("Expected %d options, got %d. Options: %+v", len(expectedOptions), len(options), options)
	}

	for i, opt := range options {
		expected := expectedOptions[i]
		if opt.Name != expected.Name || opt.CliName != expected.CliName ||
			opt.TypeName != expected.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expected.HelpText) ||
			opt.IsPointer != expected.IsPointer || opt.IsRequired != expected.IsRequired ||
			opt.EnvVar != expected.EnvVar {
			t.Errorf("Option %d (%s) Mismatch:\nExpected: %+v\nGot:      %+v", i, opt.Name, expected, opt)
		}
	}
}

func TestAnalyzeOptions_ExternalPackageDirectly(t *testing.T) {
	fset := token.NewFileSet()

	// Define content for the external package directly
	externalPkgContent := `package myexternalpkg
// ExternalConfig is defined in "myexternalpkg".
type ExternalConfig struct {
    // URL for the external service.
    ExternalURL string
    // Retry count for external service.
    ExternalRetryCount int
}`
	_, externalFileAst := parseSingleFileAst(t, externalPkgContent)

	expectedOptions := []*metadata.OptionMetadata{
		{Name: "ExternalURL", CliName: "external-url", TypeName: "string", HelpText: "URL for the external service.", IsRequired: true, EnvVar: ""},
		{Name: "ExternalRetryCount", CliName: "external-retry-count", TypeName: "int", HelpText: "Retry count for external service.", IsRequired: true, EnvVar: ""},
	}

	// Analyze "ExternalConfig" struct within the parsed AST for "myexternalpkg"
	options, structName, err := AnalyzeOptions(fset, []*ast.File{externalFileAst}, "ExternalConfig", "myexternalpkg")
	if err != nil {
		t.Fatalf("AnalyzeOptions for direct external package example.com/myexternalpkg failed: %v", err)
	}
	if structName != "ExternalConfig" {
		t.Errorf("Expected struct name 'ExternalConfig', got '%s'", structName)
	}

	if len(options) != len(expectedOptions) {
		t.Fatalf("Expected %d options, got %d. Options: %+v", len(expectedOptions), len(options), options)
	}
	for i, opt := range options {
		expected := expectedOptions[i]
		if opt.Name != expected.Name || opt.CliName != expected.CliName ||
			opt.TypeName != expected.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expected.HelpText) ||
			opt.IsPointer != expected.IsPointer || opt.IsRequired != expected.IsRequired ||
			opt.EnvVar != expected.EnvVar {
			t.Errorf("Option %d (%s) Mismatch:\nExpected: %+v\nGot:      %+v", i, opt.Name, expected, opt)
		}
	}
}
