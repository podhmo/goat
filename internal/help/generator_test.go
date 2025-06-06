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

	expected := `mytool - A super useful tool.
         Does amazing things.

Usage:
  mytool [flags]

Flags:
  --username            string The username for login. (required) (env: APP_USER)
  --port                int Port number to listen on. (default: 8080)
  --mode                string Operation mode. (default: "dev") (allowed: "dev", "prod", "test")
  --verbose             bool Enable verbose output.
  --force-push          bool Force push changes.
  --no-enable-auto-sync bool Enable automatic synchronization.
  --strict-validation   bool Enable strict validation.

  -h, --help Show this help message and exit
`
	// Adjust spacing for alignment based on the longest flag name "no-enable-auto-sync" (20) vs "username" (8)
	// Original max was username (8). New max is no-enable-auto-sync (20).
	// The help generator should handle this alignment automatically.
	// We need to ensure the expected string matches the auto-alignment.

	// Re-aligning the expected string manually for the test comparison:
	// Max flag name length is now `no-enable-auto-sync` (20 chars)
	// Help flag `h, --help` (10 chars)
	// All flag lines need to be padded to this new max length.

	expected = `mytool - A super useful tool.
         Does amazing things.

Usage:
  mytool [flags]

Flags:
  --username            string The username for login. (required) (env: APP_USER)
  --port                int    Port number to listen on. (default: 8080)
  --mode                string Operation mode. (default: "dev") (allowed: "dev", "prod", "test")
  --verbose             bool   Enable verbose output.
  --force-push          bool   Force push changes.
  --no-enable-auto-sync bool   Enable automatic synchronization.
  --strict-validation   bool   Enable strict validation.

  -h, --help            Show this help message and exit
`

	helpMsg = strings.ReplaceAll(helpMsg, "\r\n", "\n")
	expected = strings.ReplaceAll(expected, "\r\n", "\n")

	// スペースを@に変換して比較
	helpMsgAt := strings.ReplaceAll(helpMsg, " ", "@")
	expectedAt := strings.ReplaceAll(expected, " ", "@")

	if helpMsgAt != expectedAt {
		t.Errorf("help message mismatch (spaces shown as @)\n--- expected ---\n%s\n--- got ---\n%s", expectedAt, helpMsgAt)
	}
}

func TestGenerateHelp_NilMetadata(t *testing.T) {
	helpMsg := GenerateHelp(nil)
	if !strings.Contains(helpMsg, "<error>") {
		t.Errorf("Expected error message for nil metadata, got: %s", helpMsg)
	}
}
