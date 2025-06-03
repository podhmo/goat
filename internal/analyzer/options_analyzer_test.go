package analyzer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/metadata"
)

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
	// Test with and without tags to ensure tags are parsed if present
	testCases := []struct {
		nameTag         string
		ageTag          string
		adminTag        string
		featTag         string
		expectedOptions []*metadata.OptionMetadata
	}{
		{
			nameTag: ` %s`, ageTag: ` %s`, adminTag: ` %s`, featTag: ` %s`,
			expectedOptions: []*metadata.OptionMetadata{
				{Name: "Name", CliName: "name", TypeName: "string", HelpText: "Name of the user.", IsRequired: true},
				{Name: "Age", CliName: "age", TypeName: "*int", HelpText: "Age of the user, optional.", IsPointer: true, IsRequired: false},
				{Name: "IsAdmin", CliName: "is-admin", TypeName: "bool", HelpText: "IsAdmin flag.", IsRequired: true},
				{Name: "Features", CliName: "features", TypeName: "[]string", HelpText: "Features list.", IsRequired: true},
			},
		},
		{
			nameTag: ` %s`, ageTag: ` %s`, adminTag: ` %s`, featTag: ` %s`,
			expectedOptions: []*metadata.OptionMetadata{
				{Name: "Name", CliName: "name", TypeName: "string", HelpText: "Name of the user.", IsRequired: true, EnvVar: "APP_NAME"},
				{Name: "Age", CliName: "age", TypeName: "*int", HelpText: "Age of the user, optional.", IsPointer: true, IsRequired: false, EnvVar: "USER_AGE"},
				{Name: "IsAdmin", CliName: "is-admin", TypeName: "bool", HelpText: "IsAdmin flag.", IsRequired: true}, // No env tag
				{Name: "Features", CliName: "features", TypeName: "[]string", HelpText: "Features list.", IsRequired: true, EnvVar: "APP_FEATURES"},
			},
		},
	}

	// Inject tags into content format string
	testCases[0].nameTag = ""
	testCases[0].ageTag = ""
	testCases[0].adminTag = ""
	testCases[0].featTag = ""

	testCases[1].nameTag = "`env:\"APP_NAME\"`"
	testCases[1].ageTag = "`env:\"USER_AGE\"`"
	testCases[1].adminTag = ""
	testCases[1].featTag = "`env:\"APP_FEATURES\"`"

	for i, tc := range testCases {
		formattedContent := fmt.Sprintf(content, tc.nameTag, tc.ageTag, tc.adminTag, tc.featTag)
		fileAst := parseTestFile(t, formattedContent) // Assuming parseTestFile is available

		options, structName, err := AnalyzeOptions(fileAst, "Config", "main")
		if err != nil {
			t.Fatalf("Test case %d: AnalyzeOptions failed: %v", i, err)
		}
		if structName != "Config" {
			t.Errorf("Test case %d: Expected struct name 'Config', got '%s'", i, structName)
		}

		if len(options) != len(tc.expectedOptions) {
			t.Fatalf("Test case %d: Expected %d options, got %d. Options: %+v", i, len(tc.expectedOptions), len(options), options)
		}

		for j, opt := range options {
			expectedOpt := tc.expectedOptions[j]
			// Compare relevant fields, reflect.DeepEqual might be too strict for uninitialized fields
			if opt.Name != expectedOpt.Name || opt.CliName != expectedOpt.CliName ||
				opt.TypeName != expectedOpt.TypeName || strings.TrimSpace(opt.HelpText) != strings.TrimSpace(expectedOpt.HelpText) ||
				opt.IsPointer != expectedOpt.IsPointer || opt.IsRequired != expectedOpt.IsRequired ||
				opt.EnvVar != expectedOpt.EnvVar {
				t.Errorf("Test case %d, Option %d: Mismatch.\nExpected: %+v\nGot:      %+v", i, j, expectedOpt, opt)
			}
		}
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
	fileAst := parseTestFile(t, content)
	options, _, err := AnalyzeOptions(fileAst, "Config", "main")
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
	fileAst := parseTestFile(t, content)
	_, _, err := AnalyzeOptions(fileAst, "NonExistentConfig", "main")
	if err == nil {
		t.Fatal("AnalyzeOptions should have failed for a non-existent struct")
	}
	if !strings.Contains(err.Error(), "NonExistentConfig' not found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}
