package codegen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/stringutils"
)

// Helper function to create a basic OptionMetadata for testing
func makeOptionMeta(name, typeName, cliName, help, defaultValue, envVar string, required bool, enumValues []any) *metadata.OptionMetadata {
	return &metadata.OptionMetadata{
		Name:           name,
		TypeName:       typeName,
		CliName:        cliName,
		HelpText:       help,
		DefaultValue:   defaultValue,
		EnvVar:         envVar,
		IsRequired:     required,
		EnumValues:     enumValues,
		UnderlyingKind: typeName, // Simplified for these tests
	}
}

func TestStringHandler_GenerateDefaultValueInitializationCode(t *testing.T) {
	handler := &StringHandler{}
	opt := makeOptionMeta("MyString", "string", "my-string", "help", "defaultVal", "", false, nil)
	optionsVarName := "opts"

	// With default value
	snippets := handler.GenerateDefaultValueInitializationCode(opt, optionsVarName)
	expectedLogic := fmt.Sprintf("%s.%s = %q\n", optionsVarName, opt.Name, "defaultVal")
	if snippets.Logic != expectedLogic {
		t.Errorf("Expected logic %q, got %q", expectedLogic, snippets.Logic)
	}
	if snippets.Declarations != "" {
		t.Errorf("Expected no declarations, got %q", snippets.Declarations)
	}

	// Without default value
	opt.DefaultValue = ""
	snippets = handler.GenerateDefaultValueInitializationCode(opt, optionsVarName)
	if snippets.Logic != "" { // Expect empty string, not "" = ""
		t.Errorf("Expected empty logic for no default, got %q", snippets.Logic)
	}

	// With different default value type (should be coerced to string)
	opt.DefaultValue = 123
	snippets = handler.GenerateDefaultValueInitializationCode(opt, optionsVarName)
	expectedLogic = fmt.Sprintf("%s.%s = %q\n", optionsVarName, opt.Name, "123")
	if snippets.Logic != expectedLogic {
		t.Errorf("Expected logic %q for coerced int default, got %q", expectedLogic, snippets.Logic)
	}
}

func TestStringHandler_GenerateEnvVarProcessingCode(t *testing.T) {
	handler := &StringHandler{}
	opt := makeOptionMeta("MyString", "string", "my-string", "help", "", "MY_ENV_VAR", false, nil)
	optionsVarName := "opts"
	envValVarName := "envVal"
	ctxVarName := "ctx"

	snippets := handler.GenerateEnvVarProcessingCode(opt, optionsVarName, envValVarName, ctxVarName)
	expectedLogic := fmt.Sprintf("%s.%s = %s\n", optionsVarName, opt.Name, envValVarName)
	if snippets.Logic != expectedLogic {
		t.Errorf("Expected logic %q, got %q", expectedLogic, snippets.Logic)
	}
	if snippets.Declarations != "" {
		t.Errorf("Expected no declarations, got %q", snippets.Declarations)
	}
}

func TestStringHandler_GenerateFlagRegistrationCode(t *testing.T) {
	handler := &StringHandler{}
	optionsVarName := "opts"
	isFlagExplicitlySetMapName := "isSet" // Not used by StringHandler directly in output
	globalTempVarPrefix := "temp"         // Not used by StringHandler

	tests := []struct {
		name         string
		opt          *metadata.OptionMetadata
		expectedCode string
	}{
		{
			name: "simple string flag",
			opt:  makeOptionMeta("MyString", "string", "my-string", "help text", "defaultVal", "", false, nil),
			expectedCode: `flag.StringVar(&opts.MyString, "my-string", "defaultVal", "help text (default: defaultVal)")
`,
		},
		{
			name: "string flag with no default",
			opt:  makeOptionMeta("MyString", "string", "my-string", "help text", "", "", false, nil),
			expectedCode: `flag.StringVar(&opts.MyString, "my-string", "", "help text")
`,
		},
		{
			name: "string flag with enum",
			opt:  makeOptionMeta("MyString", "string", "my-string", "help text", "a", "", false, []any{"a", "b", "c"}),
			expectedCode: `flag.StringVar(&opts.MyString, "my-string", "a", "help text (default: a) (allowed: a, b, c)")
`,
		},
		{
			name:         "string flag with quotes in help",
			opt:          makeOptionMeta("MyString", "string", "my-string", "help 'text'", "defaultVal", "", false, nil),
			expectedCode: "flag.StringVar(&opts.MyString, \"my-string\", \"defaultVal\", \"help `text` (default: defaultVal)\")\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snippets := handler.GenerateFlagRegistrationCode(tt.opt, optionsVarName, isFlagExplicitlySetMapName, globalTempVarPrefix)
			// Normalize whitespace for comparison if necessary, though here direct match is expected
			actualCode := strings.TrimSpace(snippets.Logic)
			expectedCode := strings.TrimSpace(tt.expectedCode)
			if actualCode != expectedCode {
				t.Errorf("Expected logic:\n%s\nGot:\n%s", expectedCode, actualCode)
			}
		})
	}
}

func TestStringHandler_GenerateRequiredCheckCode(t *testing.T) {
	handler := &StringHandler{}
	opt := makeOptionMeta("MyString", "string", "my-string", "help", "defaultVal", "MY_ENV_VAR", true, nil)
	optionsVarName := "opts"
	isFlagExplicitlySetMapName := "isSet"
	initialDefaultVarName := "initialDefault_MyString" // Assumed to be created by main_generator
	envWasSetVarName := "env_MyString_WasSet"          // Assumed to be created by main_generator
	ctxVarName := "ctx"

	snippets := handler.GenerateRequiredCheckCode(opt, optionsVarName, isFlagExplicitlySetMapName, initialDefaultVarName, envWasSetVarName, ctxVarName)

	kebabCaseName := stringutils.ToKebabCase(opt.Name)
	envVarLogIfPresent := fmt.Sprintf(`, "envVar", %q`, opt.EnvVar)
	condition := fmt.Sprintf("%s.%s == %s && !%s[%q] && !%s",
		optionsVarName, opt.Name, initialDefaultVarName, isFlagExplicitlySetMapName, kebabCaseName, envWasSetVarName)

	expectedLogicFormat := `if %s {
	slog.ErrorContext(%s, "Missing required option", "flag", %q%s, "option", %q)
	return fmt.Errorf("missing required option: --%s / %s")
}
return nil
`
	expectedLogic := fmt.Sprintf(expectedLogicFormat, condition, ctxVarName, kebabCaseName, envVarLogIfPresent, opt.Name, kebabCaseName, opt.EnvVar)

	if strings.TrimSpace(snippets.Logic) != strings.TrimSpace(expectedLogic) {
		t.Errorf("Expected logic:\n%s\nGot:\n%s", strings.TrimSpace(expectedLogic), strings.TrimSpace(snippets.Logic))
	}
}

func TestStringHandler_GenerateEnumValidationCode(t *testing.T) {
	handler := &StringHandler{}
	optionsVarName := "opts"
	ctxVarName := "ctx"

	optWithEnum := makeOptionMeta("MyEnumString", "string", "my-enum", "help", "a", "", false, []any{"a", "b", "c"})
	optWithoutEnum := makeOptionMeta("MyString", "string", "my-string", "help", "", "", false, nil)

	// With enum values
	snippetsWithEnum := handler.GenerateEnumValidationCode(optWithEnum, optionsVarName, ctxVarName)
	if snippetsWithEnum.Declarations == "" {
		t.Error("Expected declarations for enum validation, got none")
	}
	if snippetsWithEnum.Logic == "" {
		t.Error("Expected logic for enum validation, got none")
	}
	expectedEnumVar := stringutils.ToCamelCase(optWithEnum.Name) + "EnumValues"
	if !strings.Contains(snippetsWithEnum.Declarations, expectedEnumVar) {
		t.Errorf("Expected declarations to contain %q, got %q", expectedEnumVar, snippetsWithEnum.Declarations)
	}
	if !strings.Contains(snippetsWithEnum.Logic, fmt.Sprintf("%s.%s", optionsVarName, optWithEnum.Name)) {
		t.Errorf("Expected logic to check option value, got %q", snippetsWithEnum.Logic)
	}
	if !strings.Contains(snippetsWithEnum.Logic, expectedEnumVar) {
		t.Errorf("Expected logic to use enum values var %q, got %q", expectedEnumVar, snippetsWithEnum.Logic)
	}

	// Without enum values
	snippetsWithoutEnum := handler.GenerateEnumValidationCode(optWithoutEnum, optionsVarName, ctxVarName)
	if snippetsWithoutEnum.Declarations != "" {
		t.Errorf("Expected no declarations without enum, got %q", snippetsWithoutEnum.Declarations)
	}
	if snippetsWithoutEnum.Logic != "" {
		t.Errorf("Expected no logic without enum, got %q", snippetsWithoutEnum.Logic)
	}
}

// TODO: Add tests for IntHandler, BoolHandler, Ptr Handlers, Slice Handlers, TextUnmarshaler Handlers, Unsupported Handler
