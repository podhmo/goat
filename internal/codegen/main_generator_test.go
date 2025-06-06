package codegen // Changed from codegen_test

import (
	"fmt"
	"go/format"
	"regexp"
	"strings"
	"testing"

	// "github.com/podhmo/goat/internal/codegen" // Removed this import
	"github.com/podhmo/goat/internal/metadata"
)

var (
	lineCommentRegex = regexp.MustCompile(`//.*`)
	// whitespaceRegex matches all whitespace characters, including newlines.
	// It's used to replace any sequence of whitespace with a single space.
	whitespaceRegex = regexp.MustCompile(`\s+`)
)

// normalizeForContains prepares a code snippet for robust substring checking.
// It removes comments, replaces various whitespace with single spaces, and trims.
func normalizeForContains(snippet string) string {
	// Remove Go line comments first to prevent // from becoming part of a word.
	var noCommentsLines []string
	for _, line := range strings.Split(snippet, "\n") {
		if idx := strings.Index(line, "//"); idx != -1 {
			noCommentsLines = append(noCommentsLines, line[:idx])
		} else {
			noCommentsLines = append(noCommentsLines, line)
		}
	}
	processed := strings.Join(noCommentsLines, " ") // Join with space to process as a single "line"

	// Replace tabs with spaces first to ensure uniform space characters.
	processed = strings.ReplaceAll(processed, "\t", " ")
	// Compact all sequences of whitespace (now including newlines replaced by spaces) into a single space.
	processed = whitespaceRegex.ReplaceAllString(processed, " ")
	return strings.TrimSpace(processed)
}

// normalizeCode formats the actual generated Go code string.
func normalizeCode(t *testing.T, code string) string {
	t.Helper()
	formatted, err := format.Source([]byte(code))
	if err != nil {
		// If go/format.Source fails on the actual generated code, it's a critical error.
		t.Fatalf("Failed to format actual generated code: %v\nOriginal code:\n%s", err, code)
	}
	// After gofmt, further normalize for robust comparison (remove comments, compact whitespace)
	return normalizeForContains(string(formatted))
}

func assertCodeContains(t *testing.T, actualGeneratedCode, expectedSnippet string) {
	t.Helper()
	normalizedActual := normalizeCode(t, actualGeneratedCode)
	normalizedExpectedSnippet := normalizeForContains(expectedSnippet)

	if !strings.Contains(normalizedActual, normalizedExpectedSnippet) {
		t.Errorf("Expected generated code to contain (normalized):\n>>>>>>>>>>\n%s\n<<<<<<<<<<\n\nActual code (normalized):\n>>>>>>>>>>\n%s\n<<<<<<<<<<\n\nOriginal Expected Snippet:\n%s\n\nOriginal Actual Code:\n%s",
			normalizedExpectedSnippet, normalizedActual, expectedSnippet, actualGeneratedCode)
	}
}

func assertCodeNotContains(t *testing.T, actualGeneratedCode, unexpectedSnippet string) {
	t.Helper()
	normalizedActual := normalizeCode(t, actualGeneratedCode)
	normalizedUnexpectedSnippet := normalizeForContains(unexpectedSnippet)

	if strings.Contains(normalizedActual, normalizedUnexpectedSnippet) {
		t.Errorf("Expected generated code NOT to contain (normalized):\n>>>>>>>>>>\n%s\n<<<<<<<<<<\n\nActual code (normalized):\n>>>>>>>>>>\n%s\n<<<<<<<<<<\n\nOriginal Unexpected Snippet:\n%s\n\nOriginal Actual Code:\n%s",
			normalizedUnexpectedSnippet, normalizedActual, unexpectedSnippet, actualGeneratedCode)
	}
}

func TestGenerateMain_BasicCase(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "Run",
			PackageName:                "mycmd",
			OptionsArgTypeNameStripped: "OptionsType",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, cmdMeta.RunFunc.PackageName)
	assertCodeContains(t, actualCode, "func main() {")
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = mycmd.Run()")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeContains(t, actualCode, `slog.Error("Runtime error", "error", err)`)
	assertCodeContains(t, actualCode, `os.Exit(1)`)
	assertCodeNotContains(t, actualCode, "var options =")
}

func TestGenerateMain_WithOptions(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunWithOptions",
			PackageName:                "anothercmd",
			OptionsArgTypeNameStripped: "MyOptionsType",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Name", TypeName: "string", HelpText: "Name of the user", DefaultValue: "guest"},
			{Name: "Age", TypeName: "int", HelpText: "Age of the user", DefaultValue: 30},
			{Name: "Verbose", TypeName: "bool", HelpText: "Enable verbose output", DefaultValue: false},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &MyOptionsType{}")

	expectedFlagParsing := `
	flag.StringVar(&options.Name, "name", "guest", "Name of the user" /* Default: guest */)
	flag.IntVar(&options.Age, "age", 30, "Age of the user" /* Default: 30 */)
	flag.BoolVar(&options.Verbose, "verbose", false, "Enable verbose output" /* Default: false */)
	flag.Parse()
`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, "err = anothercmd.RunWithOptions(options)")
}

func TestGenerateMain_KebabCaseFlagNames(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "ProcessData",
			PackageName:                "dataproc",
			OptionsArgTypeNameStripped: "DataProcOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "InputFile", TypeName: "string", HelpText: "Input file path"},
			{Name: "OutputDirectory", TypeName: "string", HelpText: "Output directory path", DefaultValue: "/tmp"},
			{Name: "MaximumRetries", TypeName: "int", HelpText: "Maximum number of retries", DefaultValue: 3},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &DataProcOptions{}")
	expectedFlagParsing := `
	flag.StringVar(&options.InputFile, "input-file", "", "Input file path")
	flag.StringVar(&options.OutputDirectory, "output-directory", "/tmp", "Output directory path" /* Default: /tmp */)
	flag.IntVar(&options.MaximumRetries, "maximum-retries", 3, "Maximum number of retries" /* Default: 3 */)
	flag.Parse()
`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, "err = dataproc.ProcessData(options)")
}

func TestGenerateMain_RequiredFlags(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "DoSomething",
			PackageName:                "task",
			OptionsArgTypeNameStripped: "Config",
			OptionsArgIsPointer:        false,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "ConfigFile", TypeName: "string", HelpText: "Path to config file", IsRequired: true},
			{Name: "Retries", TypeName: "int", HelpText: "Number of retries", IsRequired: true, DefaultValue: 0},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &Config{}")
	assertCodeContains(t, actualCode, `flag.StringVar(&options.ConfigFile, "config-file", "", "Path to config file")`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Retries, "retries", 0, "Number of retries" /* Default: 0 */)`)

	expectedConfigFileCheck := `
	if options.ConfigFile == "" {
		slog.Error("Missing required flag", "flag", "config-file")
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedConfigFileCheck)

	expectedRetriesCheck := `
	isSetOrFromEnv_Retries := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "retries" {
			isSetOrFromEnv_Retries = true
		}
	})
	if !isSetOrFromEnv_Retries && options.Retries == 0 {
		slog.Error("Missing required flag", "flag", "retries")
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedRetriesCheck)
	assertCodeContains(t, actualCode, "err = task.DoSomething(*options)")
}

func TestGenerateMain_EnumValidation(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "SetMode",
			PackageName:                "control",
			OptionsArgTypeNameStripped: "ModeOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Mode", TypeName: "string", HelpText: "Mode of operation", EnumValues: []any{"auto", "manual", "standby"}, DefaultValue: "auto"},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &ModeOptions{}")
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Mode, "mode", "auto", "Mode of operation" /* Default: auto */)`)

	expectedEnumValidation := `
	isValidChoice_Mode := false
	allowedChoices_Mode := []string{"auto", "manual", "standby"}
	currentValue_ModeStr := fmt.Sprintf("%v", options.Mode)
	for _, choice := range allowedChoices_Mode {
		if currentValue_ModeStr == choice {
			isValidChoice_Mode = true
			break
		}
	}
	if !isValidChoice_Mode {
		slog.Error("Invalid value for flag", "flag", "mode", "value", options.Mode, "allowedChoices", strings.Join(allowedChoices_Mode, ", "))
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedEnumValidation)
	assertCodeContains(t, actualCode, "err = control.SetMode(options)")
}

func TestGenerateMain_EnvironmentVariables(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "Configure",
			PackageName:                "setup",
			OptionsArgTypeNameStripped: "AppSettings",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "APIKey", TypeName: "string", HelpText: "API Key", EnvVar: "API_KEY"},
			{Name: "Timeout", TypeName: "int", HelpText: "Timeout in seconds", DefaultValue: 60, EnvVar: "TIMEOUT_SECONDS"},
			{Name: "EnableFeature", TypeName: "bool", HelpText: "Enable new feature", DefaultValue: false, EnvVar: "ENABLE_MY_FEATURE"},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &AppSettings{}")
	assertCodeContains(t, actualCode, `flag.StringVar(&options.APIKey, "api-key", "", "API Key")`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Timeout, "timeout", 60, "Timeout in seconds" /* Default: 60 */)`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.EnableFeature, "enable-feature", false, "Enable new feature" /* Default: false */)`)

	expectedApiKeyEnv := `
	if val, ok := os.LookupEnv("API_KEY"); ok {
		if options.APIKey == "" {
			options.APIKey = val
		}
	}
`
	assertCodeContains(t, actualCode, expectedApiKeyEnv)

	expectedTimeoutEnv := `
	if val, ok := os.LookupEnv("TIMEOUT_SECONDS"); ok {
		if options.Timeout == 60 {
			if v, err := strconv.Atoi(val); err == nil {
				options.Timeout = v
			} else {
				slog.Warn("Could not parse environment variable as int", "envVar", "TIMEOUT_SECONDS", "value", val, "error", err)
			}
		}
	}
`
	assertCodeContains(t, actualCode, expectedTimeoutEnv)

	expectedEnableFeatureEnv := `
	if val, ok := os.LookupEnv("ENABLE_MY_FEATURE"); ok {
		if defaultValForBool_EnableFeature := false; !defaultValForBool_EnableFeature {
			if v, err := strconv.ParseBool(val); err == nil && v {
				options.EnableFeature = true
			} else if err != nil {
				slog.Warn("Could not parse environment variable as bool", "envVar", "ENABLE_MY_FEATURE", "value", val, "error", err)
			}
		} else {
			if v, err := strconv.ParseBool(val); err == nil && !v {
				options.EnableFeature = false
			} else if err != nil && val != "" {
				slog.Warn("Could not parse environment variable as bool", "envVar", "ENABLE_MY_FEATURE", "value", val, "error", err)
			}
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnableFeatureEnv)
	assertCodeContains(t, actualCode, "err = setup.Configure(options)")
}

func TestGenerateMain_EnvVarForBoolWithTrueDefault(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "ProcessWithFeature",
			PackageName:                "featureproc",
			OptionsArgTypeNameStripped: "FeatureOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "SmartParsing", TypeName: "bool", HelpText: "Enable smart parsing", DefaultValue: true, EnvVar: "SMART_PARSING_ENABLED"},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &FeatureOptions{}")
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.SmartParsing, "smart-parsing", true, "Enable smart parsing" /* Default: true */)`)

	expectedEnvLogic := `
	if val, ok := os.LookupEnv("SMART_PARSING_ENABLED"); ok {
		if defaultValForBool_SmartParsing := true; !defaultValForBool_SmartParsing {
		} else {
			if v, err := strconv.ParseBool(val); err == nil && !v {
				options.SmartParsing = false
			} else if err != nil && val != "" {
				slog.Warn("Could not parse environment variable as bool", "envVar", "SMART_PARSING_ENABLED", "value", val, "error", err)
			}
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnvLogic)
	assertCodeContains(t, actualCode, "err = featureproc.ProcessWithFeature(options)")
}

func TestGenerateMain_ErrorHandling(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "DefaultRun",
			PackageName:                "pkg",
			OptionsArgTypeNameStripped: "Irrelevant",
			OptionsArgIsPointer:        true,
		},
	}
	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	expectedErrorHandling := `
	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedErrorHandling)
}

func TestGenerateMain_Imports(t *testing.T) {
	cmdMetaNoStrconv := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "MyFunc",
			PackageName:                "custompkg",
			OptionsArgTypeNameStripped: "AppConfig",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Name", TypeName: "string", EnvVar: "APP_NAME", HelpText: "app name"},
		},
	}
	actualCodeNoStrconv, err := GenerateMain(cmdMetaNoStrconv, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCodeNoStrconv, `flag.StringVar(&options.Name, "name", "", "app name")`)
	assertCodeContains(t, actualCodeNoStrconv, cmdMetaNoStrconv.RunFunc.PackageName)
	assertCodeNotContains(t, actualCodeNoStrconv, `strconv.Atoi`)

	cmdMetaWithStrconv := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "MyOtherFunc",
			PackageName:                "custompkg",
			OptionsArgTypeNameStripped: "ServerConfig",
			OptionsArgIsPointer:        false,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Port", TypeName: "int", EnvVar: "APP_PORT", HelpText: "app port"},
		},
	}
	actualCodeWithStrconv, err := GenerateMain(cmdMetaWithStrconv, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCodeWithStrconv, `flag.IntVar(&options.Port, "port", 0, "app port")`)
	assertCodeContains(t, actualCodeWithStrconv, `strconv.Atoi`)
	assertCodeContains(t, actualCodeWithStrconv, cmdMetaWithStrconv.RunFunc.PackageName)
}

func TestGenerateMain_RequiredIntWithEnvVar(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "SubmitData",
			PackageName:                "submitter",
			OptionsArgTypeNameStripped: "UserData",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "UserId", TypeName: "int", HelpText: "User ID", IsRequired: true, EnvVar: "USER_ID", DefaultValue: 0},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &UserData{}")
	assertCodeContains(t, actualCode, `flag.IntVar(&options.UserId, "user-id", 0, "User ID" /* Default: 0 */)`)

	expectedCheck := `
	isSetOrFromEnv_UserId := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "user-id" {
			isSetOrFromEnv_UserId = true
		}
	})
	if !isSetOrFromEnv_UserId {
		if val, ok := os.LookupEnv("USER_ID"); ok {
			if parsedVal, err := strconv.Atoi(val); err == nil && parsedVal == options.UserId {
				isSetOrFromEnv_UserId = true
			}
		}
	}
	if !isSetOrFromEnv_UserId && options.UserId == 0 {
		slog.Error("Missing required flag", "flag", "user-id", "envVar", "USER_ID")
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedCheck)
	assertCodeContains(t, actualCode, "err = submitter.SubmitData(options)")
}

func TestGenerateMain_StringFlagWithQuotesInDefault(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "PrintString",
			PackageName:                "printer",
			OptionsArgTypeNameStripped: "PrintOpts",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Greeting", TypeName: "string", HelpText: "A greeting message", DefaultValue: `hello "world"`},
		},
	}
	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &PrintOpts{}")
	expectedFlagParsing := `flag.StringVar(&options.Greeting, "greeting", "hello \"world\"", "A greeting message" /* Default: hello "world" */)`
	assertCodeContains(t, actualCode, expectedFlagParsing)
}

func TestGenerateMain_WithHelpText(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunMyTool",
			PackageName:                "mytool",
			OptionsArgTypeNameStripped: "ToolOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Input", TypeName: "string", HelpText: "Input file"},
		},
	}
	helpText := "This is my custom help message.\nUsage: mytool -input <file>"

	actualCode, err := GenerateMain(cmdMeta, helpText, true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain with help text failed: %v", err)
	}

	assertCodeContains(t, actualCode, "var options = &ToolOptions{}")
	expectedHelpTextSnippet := `
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, ` + "`" + helpText + "`" + `)
		flag.PrintDefaults()
	}`
	assertCodeContains(t, actualCode, expectedHelpTextSnippet)

	oldManualHelpLogic := `for _, arg := range os.Args[1:] { if arg == "-h" || arg == "--help" {`
	assertCodeNotContains(t, actualCode, oldManualHelpLogic)

	assertCodeContains(t, actualCode, `flag.StringVar(&options.Input, "input", "", "Input file")`)
	assertCodeContains(t, actualCode, "err = mytool.RunMyTool(options)")
}

func TestGenerateMain_WithEmptyHelpText(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "AnotherTool",
			PackageName:                "othertool",
			OptionsArgTypeNameStripped: "NoOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true) // Changed codegen.GenerateMain to GenerateMain
	if err != nil {
		t.Fatalf("GenerateMain with empty help text failed: %v", err)
	}

	unexpectedHelpLogic := `
	// Handle -h/--help flags
	for _, arg := range os.Args[1:] {
`
	assertCodeNotContains(t, actualCode, unexpectedHelpLogic)

	unexpectedFlagUsageAssignment := `flag.Usage = func()`
	assertCodeNotContains(t, actualCode, unexpectedFlagUsageAssignment)

	assertCodeContains(t, actualCode, "func main() {")
	assertCodeContains(t, actualCode, "err = othertool.AnotherTool()")
}

func TestGenerateMain_HelpTextNewlineFormatting(t *testing.T) {
	baseCmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunMyTool",
			PackageName:                "mytool",
			OptionsArgTypeNameStripped: "ToolOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{},
	}

	t.Run("WithNewlines", func(t *testing.T) {
		helpTextWithNewlines := "This is line one.\nThis is line two."

		actualCode, err := GenerateMain(baseCmdMeta, helpTextWithNewlines, true) // Changed codegen.GenerateMain to GenerateMain
		if err != nil {
			t.Fatalf("GenerateMain with help text failed: %v", err)
		}

		formattedActualCode, err := format.Source([]byte(actualCode))
		if err != nil {
			t.Fatalf("Failed to format actual generated code: %v\nOriginal code:\n%s", err, actualCode)
		}

		// This test expects formatHelpText to produce a raw string literal for multi-line strings.
		// The old formatHelpText did: return "`" + text + "`"
		// The new one does too, if no backticks are present: return "`" + processedText + "`"
		// "This is line one.\nThis is line two." (after \\n processing) becomes `This is line one.\nThis is line two.`
		expectedUsageFunc := "fmt.Fprintln(os.Stderr, `This is line one.\nThis is line two.`)"

		if !strings.Contains(string(formattedActualCode), expectedUsageFunc) {
			t.Errorf("Expected generated code to contain exact snippet for multiline help text with raw string literal.\nExpected snippet:\n%s\n\nFormatted actual code:\n%s", expectedUsageFunc, string(formattedActualCode))
		}
	})

	t.Run("WithoutNewlines", func(t *testing.T) {
		helpTextWithoutNewlines := "This is a single line."
		// Old formatHelpText: return fmt.Sprintf("%q", text)
		// New formatHelpText (no newlines, no backticks): return fmt.Sprintf("%q", processedText)
		// So, "This is a single line." becomes "\"This is a single line.\""
		expectedFormattedText := fmt.Sprintf("%q", helpTextWithoutNewlines)
		expectedSnippet := fmt.Sprintf("fmt.Fprintln(os.Stderr, %s)", expectedFormattedText)

		actualCode, err := GenerateMain(baseCmdMeta, helpTextWithoutNewlines, true) // Changed codegen.GenerateMain to GenerateMain
		if err != nil {
			t.Fatalf("GenerateMain with help text failed: %v", err)
		}

		formattedActualCode, err := format.Source([]byte(actualCode))
		if err != nil {
			t.Fatalf("Failed to format actual generated code: %v\nOriginal code:\n%s", err, actualCode)
		}

		if !strings.Contains(string(formattedActualCode), expectedSnippet) {
			t.Errorf("Expected generated code to contain exact snippet for single line help text with quoted string literal.\nExpected snippet:\n%s\n\nFormatted actual code:\n%s", expectedSnippet, string(formattedActualCode))
		}
	})
}


// New Test Function for formatHelpText
func TestFormatHelpText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "escaped newlines",
			input: "Hello\\nWorld",
			want:  "`Hello\nWorld`",
		},
		{
			name:  "single quote to backtick no newline",
			input: "Use 'make' command",
			want:  "\"Use `make` command\"",
		},
		{
			name:  "escaped newlines and single quotes to backticks",
			input: "First line\\nThis is 'code'\\nLast line.",
			want:  "`First line\nThis is ` + \"`\" + `code` + \"`\" + `\nLast line.`",
		},
		{
			name:  "only escaped newlines",
			input: "Line1\\nLine2\\nLine3",
			want:  "`Line1\nLine2\nLine3`",
		},
		{
			name:  "only single quotes to backticks",
			input: "A 'simple' backtick.",
			want:  "\"A `simple` backtick.\"",
		},
		{
			name:  "plain string no newline no backtick",
			input: "Just a plain string.",
			want:  "\"Just a plain string.\"",
		},
		{
			name:  "pre-existing newline and single quote to backtick",
			input: "Pre-existing newline\nAnd pre-existing 'backtick'.",
			// After initial processing: "Pre-existing newline\nAnd pre-existing `backtick`."
			// Contains newline and backtick, so:
			want:  "`Pre-existing newline\nAnd pre-existing ` + \"`\" + `backtick` + \"`\" + `.`",
		},
		{
			name:  "empty string",
			input: "",
			want:  "\"\"",
		},
		{
			name:  "only escaped newline",
			input: "\\n", // becomes "\n"
			want:  "`\n`",
		},
		{
			name:  "only single quote",
			input: "'", // becomes "`"
			want:  "\"`\"",
		},
		{
			name:  "multiple backticks and newlines",
			input: "A\\nB'C'D\\nE'F'G", // becomes "A\nB`C`D\nE`F`G"
			want:  "`A\nB` + \"`\" + `C` + \"`\" + `D\nE` + \"`\" + `F` + \"`\" + `G`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatHelpText(tt.input); got != tt.want {
				// Using fmt.Sprintf for got and want to make invisible characters visible
				t.Errorf("formatHelpText(%q) = %s, want %s", tt.input, fmt.Sprintf("%q", got), fmt.Sprintf("%q", tt.want))
			}
		})
	}
}
