package help

import (
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/metadata"
)

func TestGenerateHelp_Basic(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name:        "mytool",
		Description: "A super useful tool.\nDoes amazing things.",
		RunFunc: &metadata.RunFuncInfo{
			Name: "Run",
		},
		Options: []*metadata.OptionMetadata{
			{
				Name:       "Username",
				CliName:    "username",
				TypeName:   "string",
				HelpText:   "The username for login.",
				IsRequired: true,
				EnvVar:     "APP_USER",
			},
			{
				Name:         "Port",
				CliName:      "port",
				TypeName:     "int",
				HelpText:     "Port number to listen on.",
				IsRequired:   false,
				DefaultValue: 8080,
			},
			{
				Name:         "Mode",
				CliName:      "mode",
				TypeName:     "string",
				HelpText:     "Operation mode.",
				DefaultValue: "dev",
				EnumValues:   []any{"dev", "prod", "test"},
			},
			{
				Name:       "Verbose",
				CliName:    "verbose",
				TypeName:   "*bool",
				HelpText:   "Enable verbose output.",
				IsPointer:  true,
				IsRequired: false,
			},
			{
				Name:         "ForcePush",
				CliName:      "force-push",
				TypeName:     "bool",
				HelpText:     "Force push changes.",
				IsRequired:   true,
				DefaultValue: false,
			},
			{
				Name:         "EnableAutoSync",
				CliName:      "enable-auto-sync",
				TypeName:     "bool",
				HelpText:     "Enable automatic synchronization.",
				IsRequired:   true,
				DefaultValue: true, // This will become --no-enable-auto-sync
			},
			{
				Name:         "StrictValidation",
				CliName:      "strict-validation",
				TypeName:     "*bool",
				HelpText:     "Enable strict validation.",
				IsPointer:    true,
				IsRequired:   true,
				DefaultValue: false,
			},
			{ // New option to test the core change
				Name:         "Region",
				CliName:      "region",
				TypeName:     "string",
				HelpText:     "AWS region.",
				IsRequired:   true,
				DefaultValue: "us-east-1",
			},
		},
	}

	helpMsg := GenerateHelp(cmdMeta)

	expected := `mytool - A super useful tool.
         Does amazing things.

Usage:
  mytool [flags]

Flags:
  --username            string   The username for login. (required) (env: APP_USER)
  --port                int      Port number to listen on. (default: 8080)
  --mode                string   Operation mode. (default: "dev") (allowed: "dev", "prod", "test")
  --verbose             bool     Enable verbose output.
  --force-push          bool     Force push changes.
  --no-enable-auto-sync bool     Enable automatic synchronization.
  --strict-validation   bool     Enable strict validation.
  --region              string   AWS region. (default: "us-east-1")

  -h, --help                    Show this help message and exit
`
	helpMsg = strings.ReplaceAll(helpMsg, "\r\n", "\n")
	expected = strings.ReplaceAll(expected, "\r\n", "\n")

	if helpMsg != expected {
		t.Errorf("help message mismatch:\n---EXPECTED---\n%s\n\n---ACTUAL---\n%s", expected, helpMsg)
	}
}

func TestGenerateHelp_NilMetadata(t *testing.T) {
	helpMsg := GenerateHelp(nil)
	if !strings.Contains(helpMsg, "<error>") {
		t.Errorf("Expected error message for nil metadata, got: %s", helpMsg)
	}
}

func TestGenerateHelp_fooMainGo(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name:        "foo/main.go",
		Description: "Test for foo/main.go.",
	}
	expectedName := "foo"
	actualHelp := GenerateHelp(cmdMeta)

	// Normalize line endings
	actualHelp = strings.ReplaceAll(actualHelp, "\r\n", "\n")

	expectedHeader := expectedName + " - Test for foo/main.go."
	if !strings.Contains(actualHelp, expectedHeader) {
		t.Errorf("GenerateHelp() with 'foo/main.go' did not display name correctly in header.\nExpected to contain: %q\nGot:\n%s", expectedHeader, actualHelp)
	}

	expectedUsage := "Usage:\n  " + expectedName + " [flags]"
	if !strings.Contains(actualHelp, expectedUsage) {
		t.Errorf("GenerateHelp() with 'foo/main.go' did not display name correctly in usage.\nExpected to contain: %q\nGot:\n%s", expectedUsage, actualHelp)
	}
}

func TestGenerateHelp_fooBarGo(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name:        "foo/bar.go",
		Description: "Test for foo/bar.go.",
	}
	expectedName := "bar"
	actualHelp := GenerateHelp(cmdMeta)

	// Normalize line endings
	actualHelp = strings.ReplaceAll(actualHelp, "\r\n", "\n")

	expectedHeader := expectedName + " - Test for foo/bar.go."
	if !strings.Contains(actualHelp, expectedHeader) {
		t.Errorf("GenerateHelp() with 'foo/bar.go' did not display name correctly in header.\nExpected to contain: %q\nGot:\n%s", expectedHeader, actualHelp)
	}

	expectedUsage := "Usage:\n  " + expectedName + " [flags]"
	if !strings.Contains(actualHelp, expectedUsage) {
		t.Errorf("GenerateHelp() with 'foo/bar.go' did not display name correctly in usage.\nExpected to contain: %q\nGot:\n%s", expectedUsage, actualHelp)
	}
}

func TestGenerateHelp_bazGo(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name:        "baz.go",
		Description: "Test for baz.go.",
	}
	expectedName := "baz"
	actualHelp := GenerateHelp(cmdMeta)

	// Normalize line endings
	actualHelp = strings.ReplaceAll(actualHelp, "\r\n", "\n")

	expectedHeader := expectedName + " - Test for baz.go."
	if !strings.Contains(actualHelp, expectedHeader) {
		t.Errorf("GenerateHelp() with 'baz.go' did not display name correctly in header.\nExpected to contain: %q\nGot:\n%s", expectedHeader, actualHelp)
	}

	expectedUsage := "Usage:\n  " + expectedName + " [flags]"
	if !strings.Contains(actualHelp, expectedUsage) {
		t.Errorf("GenerateHelp() with 'baz.go' did not display name correctly in usage.\nExpected to contain: %q\nGot:\n%s", expectedUsage, actualHelp)
	}
}

func TestGenerateHelp_qux(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name:        "qux",
		Description: "Test for qux.",
	}
	expectedName := "qux"
	actualHelp := GenerateHelp(cmdMeta)

	// Normalize line endings
	actualHelp = strings.ReplaceAll(actualHelp, "\r\n", "\n")

	expectedHeader := expectedName + " - Test for qux."
	if !strings.Contains(actualHelp, expectedHeader) {
		t.Errorf("GenerateHelp() with 'qux' did not display name correctly in header.\nExpected to contain: %q\nGot:\n%s", expectedHeader, actualHelp)
	}

	expectedUsage := "Usage:\n  " + expectedName + " [flags]"
	if !strings.Contains(actualHelp, expectedUsage) {
		t.Errorf("GenerateHelp() with 'qux' did not display name correctly in usage.\nExpected to contain: %q\nGot:\n%s", expectedUsage, actualHelp)
	}
}
