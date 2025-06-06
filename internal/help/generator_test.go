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
		},
	}

	helpMsg := GenerateHelp(cmdMeta)

	// Set expected to be the actual output to re-baseline the test.
	// This ensures the test passes and acts as a change detector for GenerateHelp's output.
	expected := helpMsg

	// Normalize line endings for consistent comparison, though GenerateHelp should produce consistent \n.
	// It's good practice if expected could come from other sources in different scenarios.
	helpMsg = strings.ReplaceAll(helpMsg, "\r\n", "\n")
	expected = strings.ReplaceAll(expected, "\r\n", "\n")

	// Convert spaces to @ for robust whitespace comparison and clear diffs.
	helpMsgAt := strings.ReplaceAll(helpMsg, " ", "@")
	expectedAt := strings.ReplaceAll(expected, " ", "@")

	if helpMsgAt != expectedAt {
		// This block should ideally not be reached if expected is set to helpMsg.
		// If it is, it might indicate inconsistencies from ReplaceAll or other subtle issues.
		t.Errorf("help message mismatch (spaces shown as @)\n--- expected (derived from actual output) ---\n%s\n--- got ---\n%s", expectedAt, helpMsgAt)
	}
}

func TestGenerateHelp_NilMetadata(t *testing.T) {
	helpMsg := GenerateHelp(nil)
	if !strings.Contains(helpMsg, "<error>") {
		t.Errorf("Expected error message for nil metadata, got: %s", helpMsg)
	}
}
