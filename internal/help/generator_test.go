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
				TypeName:   "*bool", // Pointer bool
				HelpText:   "Enable verbose output.",
				IsPointer:  true,
				IsRequired: false,
			},
		},
	}

	helpMsg := GenerateHelp(cmdMeta)

	// Check for command name and description
	if !strings.Contains(helpMsg, "mytool - A super useful tool.") {
		t.Errorf("Help message missing or incorrect command title. Got:\n%s", helpMsg)
	}
	if !strings.Contains(helpMsg, "\n         Does amazing things.") { // Check multiline indent for desc
		t.Errorf("Help message missing or incorrect multiline description. Got:\n%s", helpMsg)
	}

	// Check for username option
	if !strings.Contains(helpMsg, "--username string") {
		t.Errorf("Missing --username string. Got:\n%s", helpMsg)
	}
	if !strings.Contains(helpMsg, "The username for login. (required) (env: APP_USER)") {
		t.Errorf("Missing or incorrect help for username. Got:\n%s", helpMsg)
	}

	// Check for port option
	if !strings.Contains(helpMsg, "--port int") {
		t.Errorf("Missing --port int. Got:\n%s", helpMsg)
	}
	if !strings.Contains(helpMsg, "Port number to listen on. (default: 8080)") {
		t.Errorf("Missing or incorrect help for port. Got:\n%s", helpMsg)
	}

	// Check for mode option
	if !strings.Contains(helpMsg, "--mode string") {
		t.Errorf("Missing --mode string. Got:\n%s", helpMsg)
	}
	if !strings.Contains(helpMsg, "Operation mode. (default: \"dev\") (allowed: dev, prod, test)") {
		t.Errorf("Missing or incorrect help for mode. Got:\n%s", helpMsg)
	}

	// Check for verbose option
	if !strings.Contains(helpMsg, "--verbose bool") { // Type indicator becomes "bool"
		t.Errorf("Missing --verbose bool. Got:\n%s", helpMsg)
	}
	if !strings.Contains(helpMsg, "Enable verbose output.") { // No (required), (default)
		t.Errorf("Missing or incorrect help for verbose. Got:\n%s", helpMsg)
	}
	if strings.Contains(helpMsg, "--verbose bool (required)") || strings.Contains(helpMsg, "--verbose bool (default:") {
		t.Errorf("Verbose option should not be marked as required or have a default in help text. Got:\n%s", helpMsg)
	}

	// Check for standard help flag
	if !strings.Contains(helpMsg, "-h, --help             Show this help message and exit") {
		t.Errorf("Standard help flag -h, --help is missing. Got:\n%s", helpMsg)
	}

	// t.Log(helpMsg) // For manual inspection if needed
}

func TestGenerateHelp_NilMetadata(t *testing.T) {
	helpMsg := GenerateHelp(nil)
	if !strings.Contains(helpMsg, "Error: Command metadata is nil.") {
		t.Errorf("Expected error message for nil metadata, got: %s", helpMsg)
	}
}
