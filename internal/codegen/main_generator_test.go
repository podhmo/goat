package codegen // Changed from codegen_test

import (
	"bytes" // Added for log capture
	"fmt"
	"go/format"
	"log/slog" // Added for slog
	"os"       // Added for Setenv
	"regexp"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/metadata"
	"github.com/stretchr/testify/assert" // Added for assertions
)

var (
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

func TestGenerateMain_WithTextVarOptions(t *testing.T) {
	runFuncInfo := &metadata.RunFuncInfo{
		Name:                       "RunTextVarTest",
		PackageName:                "main",
		OptionsArgTypeNameStripped: "TextVarCmdOptions",
		OptionsArgIsPointer:        true,
	}
	optMetaValue := &metadata.OptionMetadata{
		Name: "FieldA", CliName: "field-a", TypeName: "textvar_pkg.MyTextValue", IsTextUnmarshaler: true, IsTextMarshaler: true, HelpText: "Help for FieldA", EnvVar: "FIELD_A_ENV",
	}
	optMetaPtr := &metadata.OptionMetadata{
		Name: "FieldB", CliName: "field-b", TypeName: "*textvar_pkg.MyPtrTextValue", IsPointer: true, IsTextUnmarshaler: true, IsTextMarshaler: true, HelpText: "Help for FieldB", EnvVar: "FIELD_B_ENV",
	}
	optMetaOnlyUnmarshaler := &metadata.OptionMetadata{
		Name: "FieldF", CliName: "field-f", TypeName: "textvar_pkg.MyOnlyUnmarshaler", IsTextUnmarshaler: true, IsTextMarshaler: false, HelpText: "Help for FieldF - only unmarshaler", EnvVar: "FIELD_F_ENV",
	}
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: runFuncInfo, Options: []*metadata.OptionMetadata{optMetaValue, optMetaPtr, optMetaOnlyUnmarshaler},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "Test TextVar functionality", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain for TextVar options failed: %v", err)
	}
	assertCodeContains(t, actualCode, `flag.TextVar(&options.FieldA, "field-a", options.FieldA, "Help for FieldA" /* Env: FIELD_A_ENV */)`)
	assertCodeContains(t, actualCode, `err := (&options.FieldA).UnmarshalText([]byte(val))`)
	assertCodeContains(t, actualCode, `slog.Warn("Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "FIELD_A_ENV", "option", "field-a"`)
	assertCodeContains(t, actualCode, `if options.FieldB == nil { options.FieldB = new(textvar_pkg.MyPtrTextValue) }`)
	assertCodeContains(t, actualCode, `flag.TextVar(options.FieldB, "field-b", options.FieldB, "Help for FieldB" /* Env: FIELD_B_ENV */)`)
	assertCodeContains(t, actualCode, `if options.FieldB == nil { options.FieldB = new(textvar_pkg.MyPtrTextValue) }`)
	assertCodeContains(t, actualCode, `err := options.FieldB.UnmarshalText([]byte(val))`)
	assertCodeContains(t, actualCode, `slog.Warn("Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "FIELD_B_ENV", "option", "field-b"`)
	assertCodeNotContains(t, actualCode, `flag.TextVar(&options.FieldF, "field-f"`)
	assertCodeNotContains(t, actualCode, `flag.TextVar(options.FieldF, "field-f"`)
	assertCodeContains(t, actualCode, `err := (&options.FieldF).UnmarshalText([]byte(val))`)
	assertCodeContains(t, actualCode, `slog.Warn("Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "FIELD_F_ENV", "option", "field-f"`)
	assertCodeContains(t, actualCode, "new(textvar_pkg.MyPtrTextValue)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "\tValidating command metadata for template generation")
	assert.Contains(t, logOutput, "\tExecuting main function template")
	assert.Contains(t, logOutput, "\tGenerating full file with package and imports")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
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
		Name: "github.com/podhmo/goat/cmd/goat",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "Run",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "OptionsType",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "err = Run(options)")
	assertCodeContains(t, actualCode, "func main() {")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeContains(t, actualCode, `slog.Error("Runtime error", "error", err)`)
	assertCodeContains(t, actualCode, `os.Exit(1)`)
	assertCodeNotContains(t, actualCode, "var options =")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_WithOptions(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "anothercmd",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunWithOptions",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "MyOptionsType",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Name", TypeName: "string", HelpText: "Name of the user", DefaultValue: "guest"},
			{Name: "Age", TypeName: "int", HelpText: "Age of the user", DefaultValue: 30},
			{Name: "Verbose", TypeName: "bool", HelpText: "Enable verbose output", DefaultValue: false},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *MyOptionsType")
	assertCodeContains(t, actualCode, "options = new(MyOptionsType)")
	assertCodeContains(t, actualCode, `options.Name = "guest"`)
	assertCodeContains(t, actualCode, `options.Age = 30`)
	assertCodeContains(t, actualCode, `options.Verbose = false`)
	expectedFlagParsing := `
	flag.StringVar(&options.Name, "name", options.Name, "Name of the user" /* Original Default: guest, Env: */)
	flag.IntVar(&options.Age, "age", options.Age, "Age of the user" /* Original Default: 30, Env: */)
	flag.BoolVar(&options.Verbose, "verbose", options.Verbose, "Enable verbose output" /* Original Default: false, Env: */)
	flag.Parse()
`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, "err = RunWithOptions(options)")
	assertCodeNotContains(t, actualCode, "import . \"anothercmd\"")
	assertCodeNotContains(t, actualCode, "import \"anothercmd\"")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_NoPackagePrefixWhenMain(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "run",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "Options",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Name", TypeName: "string", HelpText: "Name of the user", DefaultValue: "guest"},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, `options.Name = "guest"`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Name, "name", options.Name, "Name of the user" /* Original Default: guest, Env: */)`)
	assertCodeContains(t, actualCode, "err = run(options)")
	assertCodeNotContains(t, actualCode, "main.run(")
	assertCodeNotContains(t, actualCode, "main.Run(")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_KebabCaseFlagNames(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "ProcessData",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "DataProcOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "InputFile", TypeName: "string", HelpText: "Input file path"},
			{Name: "OutputDirectory", TypeName: "string", HelpText: "Output directory path", DefaultValue: "/tmp"},
			{Name: "MaximumRetries", TypeName: "int", HelpText: "Maximum number of retries", DefaultValue: 3},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *DataProcOptions")
	assertCodeContains(t, actualCode, "options = new(DataProcOptions)")
	assertCodeContains(t, actualCode, `options.InputFile = ""`)
	assertCodeContains(t, actualCode, `options.OutputDirectory = "/tmp"`)
	assertCodeContains(t, actualCode, `options.MaximumRetries = 3`)
	expectedFlagParsing := `
	flag.StringVar(&options.InputFile, "input-file", options.InputFile, "Input file path")
	flag.StringVar(&options.OutputDirectory, "output-directory", options.OutputDirectory, "Output directory path" /* Original Default: /tmp, Env: */)
	flag.IntVar(&options.MaximumRetries, "maximum-retries", options.MaximumRetries, "Maximum number of retries" /* Original Default: 3, Env: */)
	flag.Parse()
`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, "err = ProcessData(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_RequiredFlags(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "DoSomething",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "Config",
			OptionsArgIsPointer:        false,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "ConfigFile", TypeName: "string", HelpText: "Path to config file", IsRequired: true},
			{Name: "Retries", TypeName: "int", HelpText: "Number of retries", IsRequired: true, DefaultValue: 0},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *Config")
	assertCodeContains(t, actualCode, "options = new(Config)")
	assertCodeContains(t, actualCode, `options.ConfigFile = ""`)
	assertCodeContains(t, actualCode, `options.Retries = 0`)
	assertCodeNotContains(t, actualCode, `os.LookupEnv("CONFIG_FILE")`)
	assertCodeNotContains(t, actualCode, `os.LookupEnv("RETRIES")`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.ConfigFile, "config-file", options.ConfigFile, "Path to config file")`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Retries, "retries", options.Retries, "Number of retries" /* Original Default: 0, Env: */)`)
	assertCodeContains(t, actualCode, `initialDefaultConfigFile := ""`)
	assertCodeContains(t, actualCode, `envConfigFileWasSet := false`)
	assertCodeContains(t, actualCode, `if options.ConfigFile == initialDefaultConfigFile && !isFlagExplicitlySet["config-file"] && !envConfigFileWasSet {`)
	assertCodeContains(t, actualCode, `slog.Error("Missing required flag or environment variable not set", "flag", "config-file", "option", "ConfigFile")`)
	assertCodeContains(t, actualCode, `initialDefaultRetries := 0`)
	assertCodeContains(t, actualCode, `envRetriesWasSet := false`)
	assertCodeContains(t, actualCode, `if options.Retries == initialDefaultRetries && !isFlagExplicitlySet["retries"] && !envRetriesWasSet {`)
	assertCodeContains(t, actualCode, `slog.Error("Missing required flag or environment variable not set", "flag", "retries", "option", "Retries")`)
	assertCodeContains(t, actualCode, "err = DoSomething(*options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_EnumValidation(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "SetMode",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "ModeOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Mode", TypeName: "string", HelpText: "Mode of operation", EnumValues: []any{"auto", "manual", "standby"}, DefaultValue: "auto"},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *ModeOptions")
	assertCodeContains(t, actualCode, "options = new(ModeOptions)")
	assertCodeContains(t, actualCode, `options.Mode = "auto"`)
	assertCodeNotContains(t, actualCode, `os.LookupEnv("MODE")`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Mode, "mode", options.Mode, "Mode of operation" /* Original Default: auto, Env: */)`)
	expectedEnumValidation := `
	isValidChoice_Mode := false
	allowedChoices_Mode := []string{"auto", "manual", "standby"}
	currentValue_ModeStr := fmt.Sprintf("%v", options.Mode)
	isValidChoice_Mode = slices.Contains(allowedChoices_Mode, currentValue_ModeStr)

	if !isValidChoice_Mode {
		var currentValueForMsg interface{} = options.Mode
		slog.Error("Invalid value for flag", "flag", "mode", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_Mode, ", "))
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedEnumValidation)
	assertCodeContains(t, actualCode, "err = SetMode(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_EnvironmentVariables(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "Configure",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "AppSettings",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "APIKey", TypeName: "string", HelpText: "API Key", EnvVar: "API_KEY"},
			{Name: "Timeout", TypeName: "int", HelpText: "Timeout in seconds", DefaultValue: 60, EnvVar: "TIMEOUT_SECONDS"},
			{Name: "EnableFeature", TypeName: "bool", HelpText: "Enable new feature", DefaultValue: false, EnvVar: "ENABLE_MY_FEATURE"},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *AppSettings")
	assertCodeContains(t, actualCode, "options = new(AppSettings)")
	assertCodeContains(t, actualCode, `options.APIKey = ""`)
	assertCodeContains(t, actualCode, `options.Timeout = 60`)
	assertCodeContains(t, actualCode, `options.EnableFeature = false`)
	expectedApiKeyEnv := `
	if val, ok := os.LookupEnv("API_KEY"); ok {
		options.APIKey = val
	}
`
	assertCodeContains(t, actualCode, expectedApiKeyEnv)
	expectedTimeoutEnv := `
	if val, ok := os.LookupEnv("TIMEOUT_SECONDS"); ok {
		if v, err := strconv.Atoi(val); err == nil {
			options.Timeout = v
		} else {
			slog.Warn("Could not parse environment variable as int for option", "envVar", "TIMEOUT_SECONDS", "option", "Timeout", "value", val, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedTimeoutEnv)
	expectedEnableFeatureEnv := `
	if val, ok := os.LookupEnv("ENABLE_MY_FEATURE"); ok {
		if v, err := strconv.ParseBool(val); err == nil {
			options.EnableFeature = v
		} else {
			slog.Warn("Could not parse environment variable as bool for option", "envVar", "ENABLE_MY_FEATURE", "option", "EnableFeature", "value", val, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnableFeatureEnv)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.APIKey, "api-key", options.APIKey, "API Key" /* Env: API_KEY */)`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Timeout, "timeout", options.Timeout, "Timeout in seconds" /* Original Default: 60, Env: TIMEOUT_SECONDS */)`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.EnableFeature, "enable-feature", options.EnableFeature, "Enable new feature" /* Original Default: false, Env: ENABLE_MY_FEATURE */)`)
	assertCodeContains(t, actualCode, "err = Configure(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_EnvVarForBoolWithTrueDefault(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "ProcessWithFeature",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "FeatureOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "SmartParsing", TypeName: "bool", HelpText: "Enable smart parsing", DefaultValue: true, EnvVar: "SMART_PARSING_ENABLED"},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *FeatureOptions")
	assertCodeContains(t, actualCode, "options = new(FeatureOptions)")
	assertCodeContains(t, actualCode, `options.SmartParsing = true`)
	expectedEnvLogic := `
	if val, ok := os.LookupEnv("SMART_PARSING_ENABLED"); ok {
		if v, err := strconv.ParseBool(val); err == nil {
			options.SmartParsing = v
		} else {
			slog.Warn("Could not parse environment variable as bool for option", "envVar", "SMART_PARSING_ENABLED", "option", "SmartParsing", "value", val, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnvLogic)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.SmartParsing, "smart-parsing", options.SmartParsing, "Enable smart parsing" /* Original Default: true, Env: SMART_PARSING_ENABLED */)`)
	assertCodeContains(t, actualCode, "err = ProcessWithFeature(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_RequiredBool_DefaultFalse(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "ProcessData",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "DataOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "ForceOverwrite", TypeName: "bool", HelpText: "Force overwrite of existing files", IsRequired: true, DefaultValue: false},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *DataOptions")
	assertCodeContains(t, actualCode, "options = new(DataOptions)")
	assertCodeContains(t, actualCode, `options.ForceOverwrite = false`)
	assertCodeNotContains(t, actualCode, `os.LookupEnv("FORCE_OVERWRITE")`)
	expectedFlagParsing := `flag.BoolVar(&options.ForceOverwrite, "force-overwrite", options.ForceOverwrite, "Force overwrite of existing files" /* Original Default: false, Env: */)`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeNotContains(t, actualCode, "var ForceOverwrite_NoFlagIsPresent bool")
	assertCodeNotContains(t, actualCode, "options.ForceOverwrite = true")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_RequiredBool_DefaultTrue(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunTask",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "TaskConfig",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "EnableSync", TypeName: "bool", HelpText: "Enable synchronization", IsRequired: true, DefaultValue: true},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *TaskConfig")
	assertCodeContains(t, actualCode, "options = new(TaskConfig)")
	assertCodeContains(t, actualCode, `options.EnableSync = true`)
	assertCodeNotContains(t, actualCode, `os.LookupEnv("ENABLE_SYNC")`)
	expectedFlagDefinition := `
	var EnableSync_NoFlagIsPresent bool
	flag.BoolVar(&EnableSync_NoFlagIsPresent, "no-enable-sync", false, "Set enable-sync to false")
`
	assertCodeContains(t, actualCode, expectedFlagDefinition)
	expectedPostParseLogic := `
	if EnableSync_NoFlagIsPresent {
		options.EnableSync = false
	}
`
	assertCodeContains(t, actualCode, expectedPostParseLogic)
	assertCodeNotContains(t, actualCode, "if EnableSync_NoFlagIsPresent { options.EnableSync = false } else { options.EnableSync = true }")
	assertCodeNotContains(t, actualCode, `flag.BoolVar(&options.EnableSync, "enable-sync"`)
	assertCodeNotContains(t, actualCode, `flag.BoolVar(&options.EnableSync, "no-enable-sync"`)
	assertCodeNotContains(t, actualCode, `slog.Error("Missing required flag", "flag", "no-enable-sync")`)
	assertCodeNotContains(t, actualCode, `slog.Error("Missing required flag", "flag", "enable-sync")`)

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_ErrorHandling(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "DefaultRun",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "Irrelevant",
			OptionsArgIsPointer:        true,
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
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

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_Imports(t *testing.T) {
	cmdMetaNoStrconv := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "MyFunc",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "AppConfig",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Name", TypeName: "string", EnvVar: "APP_NAME", HelpText: "app name"},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBufNoConv bytes.Buffer
	handlerNoConv := slog.NewTextHandler(&logBufNoConv, &slog.HandlerOptions{Level: slog.LevelDebug})
	loggerNoConv := slog.New(handlerNoConv)
	originalLoggerNoConv := slog.Default()
	slog.SetDefault(loggerNoConv)
	defer slog.SetDefault(originalLoggerNoConv)

	actualCodeNoStrconv, err := GenerateMain(cmdMetaNoStrconv, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCodeNoStrconv, `flag.StringVar(&options.Name, "name", options.Name, "app name" /* Env: APP_NAME */)`)
	assertCodeNotContains(t, actualCodeNoStrconv, `strconv.Atoi`)

	logOutputNoConv := logBufNoConv.String()
	assert.Contains(t, logOutputNoConv, "GenerateMain: start")
	assert.Contains(t, logOutputNoConv, "GenerateMain: end (full file)")

	cmdMetaWithStrconv := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "MyOtherFunc",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "ServerConfig",
			OptionsArgIsPointer:        false,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Port", TypeName: "int", EnvVar: "APP_PORT", HelpText: "app port"},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBufWithConv bytes.Buffer
	handlerWithConv := slog.NewTextHandler(&logBufWithConv, &slog.HandlerOptions{Level: slog.LevelDebug})
	loggerWithConv := slog.New(handlerWithConv)
	originalLoggerWithConv := slog.Default()
	slog.SetDefault(loggerWithConv)
	defer slog.SetDefault(originalLoggerWithConv)

	actualCodeWithStrconv, err := GenerateMain(cmdMetaWithStrconv, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCodeWithStrconv, `flag.IntVar(&options.Port, "port", options.Port, "app port" /* Env: APP_PORT */)`)
	assertCodeContains(t, actualCodeWithStrconv, `strconv.Atoi`)

	logOutputWithConv := logBufWithConv.String()
	assert.Contains(t, logOutputWithConv, "GenerateMain: start")
	assert.Contains(t, logOutputWithConv, "GenerateMain: end (full file)")
}

func TestGenerateMain_RequiredIntWithEnvVar(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "SubmitData",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "UserData",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "UserId", TypeName: "int", HelpText: "User ID", IsRequired: true, EnvVar: "USER_ID", DefaultValue: 0},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *UserData")
	assertCodeContains(t, actualCode, "options = new(UserData)")
	assertCodeContains(t, actualCode, `options.UserId = 0`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("USER_ID"); ok { if v, err := strconv.Atoi(val); err == nil { options.UserId = v } else { slog.Warn("Could not parse environment variable as int for option", "envVar", "USER_ID", "option", "UserId", "value", val, "error", err) } }`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.UserId, "user-id", options.UserId, "User ID" /* Original Default: 0, Env: USER_ID */)`)
	expectedRequiredCheck := `
	initialDefaultUserId := 0
	envUserIdWasSet := false
	if _, ok := os.LookupEnv("USER_ID"); ok { envUserIdWasSet = true }
	if options.UserId == initialDefaultUserId && !isFlagExplicitlySet["user-id"] && !envUserIdWasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "user-id", "envVar", "USER_ID", "option", "UserId")
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedRequiredCheck)
	assertCodeContains(t, actualCode, "err = SubmitData(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_EnvVarPrecendenceStrategy(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunStrategyTest",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "StrategyOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "StringOpt", TypeName: "string", DefaultValue: "original_string", EnvVar: "ENV_STRING", HelpText: "String option"},
			{Name: "IntOpt", TypeName: "int", DefaultValue: 123, EnvVar: "ENV_INT", HelpText: "Int option"},
			{Name: "BoolOpt", TypeName: "bool", DefaultValue: false, EnvVar: "ENV_BOOL", HelpText: "Bool option"},
			{Name: "BoolTrueOpt", TypeName: "bool", DefaultValue: true, EnvVar: "ENV_BOOL_TRUE", IsRequired: true, HelpText: "Bool true option"},
			{Name: "StringPtrOpt", TypeName: "*string", EnvVar: "ENV_STRING_PTR", HelpText: "String pointer option"},
			{Name: "IntPtrOpt", TypeName: "*int", EnvVar: "ENV_INT_PTR", HelpText: "Int pointer option"},
			{Name: "BoolPtrOpt", TypeName: "*bool", EnvVar: "ENV_BOOL_PTR", HelpText: "Bool pointer option"},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "Test help text", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, `isFlagExplicitlySet := make(map[string]bool)`)
	assertCodeContains(t, actualCode, `flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })`)
	assertCodeContains(t, actualCode, `options.StringOpt = "original_string"`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("ENV_STRING"); ok { options.StringOpt = val }`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.StringOpt, "string-opt", options.StringOpt, "String option" /* Original Default: original_string, Env: ENV_STRING */)`)
	assertCodeContains(t, actualCode, `options.IntOpt = 123`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("ENV_INT"); ok { if v, err := strconv.Atoi(val); err == nil { options.IntOpt = v } else { slog.Warn("Could not parse environment variable as int for option", "envVar", "ENV_INT", "option", "IntOpt", "value", val, "error", err) } }`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.IntOpt, "int-opt", options.IntOpt, "Int option" /* Original Default: 123, Env: ENV_INT */)`)
	assertCodeContains(t, actualCode, `options.BoolOpt = false`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("ENV_BOOL"); ok { if v, err := strconv.ParseBool(val); err == nil { options.BoolOpt = v } else { slog.Warn("Could not parse environment variable as bool for option", "envVar", "ENV_BOOL", "option", "BoolOpt", "value", val, "error", err) } }`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.BoolOpt, "bool-opt", options.BoolOpt, "Bool option" /* Original Default: false, Env: ENV_BOOL */)`)
	assertCodeContains(t, actualCode, `options.BoolTrueOpt = true`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("ENV_BOOL_TRUE"); ok { if v, err := strconv.ParseBool(val); err == nil { options.BoolTrueOpt = v } else { slog.Warn("Could not parse environment variable as bool for option", "envVar", "ENV_BOOL_TRUE", "option", "BoolTrueOpt", "value", val, "error", err) } }`)
	assertCodeContains(t, actualCode, `var BoolTrueOpt_NoFlagIsPresent bool`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&BoolTrueOpt_NoFlagIsPresent, "no-bool-true-opt", false, "Set bool-true-opt to false")`)
	assertCodeContains(t, actualCode, `if BoolTrueOpt_NoFlagIsPresent { options.BoolTrueOpt = false }`)
	assertCodeContains(t, actualCode, `options.StringPtrOpt = new(string)`)
	stringPtrEnvLogic := `
	if val, ok := os.LookupEnv("ENV_STRING_PTR"); ok {
		if options.StringPtrOpt == nil { options.StringPtrOpt = new(string) }
		*options.StringPtrOpt = val
	}
`
	assertCodeContains(t, actualCode, stringPtrEnvLogic)
	stringPtrFlagLogic := `
	var defaultStringPtrOptValForFlag string
	if options.StringPtrOpt != nil { defaultStringPtrOptValForFlag = *options.StringPtrOpt }
	if options.StringPtrOpt == nil { options.StringPtrOpt = new(string) }
	flag.StringVar(options.StringPtrOpt, "string-ptr-opt", defaultStringPtrOptValForFlag, "String pointer option" /* Env: ENV_STRING_PTR */)
`
	assertCodeContains(t, actualCode, stringPtrFlagLogic)
	assertCodeContains(t, actualCode, `options.IntPtrOpt = new(int)`)
	intPtrEnvLogic := `
	if val, ok := os.LookupEnv("ENV_INT_PTR"); ok {
		if options.IntPtrOpt == nil { options.IntPtrOpt = new(int) }
		if v, err := strconv.Atoi(val); err == nil {
			*options.IntPtrOpt = v
		} else {
			slog.Warn("Could not parse environment variable as *int for option", "envVar", "ENV_INT_PTR", "option", "IntPtrOpt", "value", val, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, intPtrEnvLogic)
	intPtrFlagLogic := `
	var defaultIntPtrOptValForFlag int
	if options.IntPtrOpt != nil { defaultIntPtrOptValForFlag = *options.IntPtrOpt }
	if options.IntPtrOpt == nil { options.IntPtrOpt = new(int) }
	flag.IntVar(options.IntPtrOpt, "int-ptr-opt", defaultIntPtrOptValForFlag, "Int pointer option" /* Env: ENV_INT_PTR */)
`
	assertCodeContains(t, actualCode, intPtrFlagLogic)
	assertCodeContains(t, actualCode, `options.BoolPtrOpt = new(bool)`)
	boolPtrEnvLogic := `
	if val, ok := os.LookupEnv("ENV_BOOL_PTR"); ok {
		if options.BoolPtrOpt == nil { options.BoolPtrOpt = new(bool) }
		if v, err := strconv.ParseBool(val); err == nil {
			*options.BoolPtrOpt = v
		} else {
			slog.Warn("Could not parse environment variable as *bool for option", "envVar", "ENV_BOOL_PTR", "option", "BoolPtrOpt", "value", val, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, boolPtrEnvLogic)
	boolPtrFlagLogic := `
	var defaultBoolPtrOptValForFlag bool
	if options.BoolPtrOpt != nil { defaultBoolPtrOptValForFlag = *options.BoolPtrOpt }
	if options.BoolPtrOpt == nil { options.BoolPtrOpt = new(bool) }
	flag.BoolVar(options.BoolPtrOpt, "bool-ptr-opt", defaultBoolPtrOptValForFlag, "Bool pointer option" /* Env: ENV_BOOL_PTR */)
`
	assertCodeContains(t, actualCode, boolPtrFlagLogic)
	assertCodeNotContains(t, actualCode, `var defaultStringOpt string =`)
	assertCodeNotContains(t, actualCode, `if !isFlagExplicitlySet["string-ptr-opt"] { if val, ok := os.LookupEnv("ENV_STRING_PTR"); ok {`)

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_StringFlagWithQuotesInDefault(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "PrintString",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "PrintOpts",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Greeting", TypeName: "string", HelpText: "A greeting message", DefaultValue: `hello "world"`},
		},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *PrintOpts")
	assertCodeContains(t, actualCode, "options = new(PrintOpts)")
	assertCodeContains(t, actualCode, `options.Greeting = "hello \"world\""`)
	expectedFlagParsing := `flag.StringVar(&options.Greeting, "greeting", options.Greeting, "A greeting message" /* Original Default: hello "world", Env: */)`
	assertCodeContains(t, actualCode, expectedFlagParsing)

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_WithHelpText(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunMyTool",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "ToolOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Input", TypeName: "string", HelpText: "Input file"},
		},
	}
	helpText := "This is my custom help message.\nUsage: mytool -input <file>"

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, helpText, true, 0)
	if err != nil {
		t.Fatalf("GenerateMain with help text failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var options *ToolOptions")
	assertCodeContains(t, actualCode, "options = new(ToolOptions)")
	expectedHelpTextSnippet := `
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, ` + "`" + helpText + "`" + `)
	}`
	assertCodeContains(t, actualCode, expectedHelpTextSnippet)
	oldManualHelpLogic := `for _, arg := range os.Args[1:] { if arg == "-h" || arg == "--help" {`
	assertCodeNotContains(t, actualCode, oldManualHelpLogic)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Input, "input", options.Input, "Input file")`)
	assertCodeContains(t, actualCode, "err = RunMyTool(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_WithEmptyHelpText(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "AnotherTool",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "",
			OptionsArgIsPointer:        false,
		},
		Options: []*metadata.OptionMetadata{},
	}

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, "", true, 0)
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
	assertCodeContains(t, actualCode, "err = AnotherTool()")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_HelpTextNewlineFormatting(t *testing.T) {
	baseCmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunMyTool",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "ToolOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{},
	}

	t.Run("WithNewlines", func(t *testing.T) {
		helpTextWithNewlines := "This is line one.\nThis is line two."

		t.Setenv("DEBUG", "1")
		var logBuf bytes.Buffer
		handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
		logger := slog.New(handler)
		originalLogger := slog.Default()
		slog.SetDefault(logger)
		defer slog.SetDefault(originalLogger)

		actualCode, err := GenerateMain(baseCmdMeta, helpTextWithNewlines, true, 0)
		if err != nil {
			t.Fatalf("GenerateMain with help text failed: %v", err)
		}
		formattedActualCode, err := format.Source([]byte(actualCode))
		if err != nil {
			t.Fatalf("Failed to format actual generated code: %v\nOriginal code:\n%s", err, actualCode)
		}
		expectedUsageFunc := "fmt.Fprint(os.Stderr, `This is line one.\nThis is line two.`)"
		if !strings.Contains(string(formattedActualCode), expectedUsageFunc) {
			t.Errorf("Expected generated code to contain exact snippet for multiline help text with raw string literal.\nExpected snippet:\n%s\n\nFormatted actual code:\n%s", expectedUsageFunc, string(formattedActualCode))
		}

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "GenerateMain: start")
		assert.Contains(t, logOutput, "GenerateMain: end (full file)")
	})

	t.Run("WithoutNewlines", func(t *testing.T) {
		helpTextWithoutNewlines := "This is a single line."
		expectedFormattedText := fmt.Sprintf("%q", helpTextWithoutNewlines)
		expectedSnippet := fmt.Sprintf("fmt.Fprint(os.Stderr, %s)", expectedFormattedText)

		t.Setenv("DEBUG", "1")
		var logBuf bytes.Buffer
		handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
		logger := slog.New(handler)
		originalLogger := slog.Default()
		slog.SetDefault(logger)
		defer slog.SetDefault(originalLogger)

		actualCode, err := GenerateMain(baseCmdMeta, helpTextWithoutNewlines, true, 0)
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

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "GenerateMain: start")
		assert.Contains(t, logOutput, "GenerateMain: end (full file)")
	})
}

func TestGenerateMain_WithInitializer(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "example.com/user/usercmd",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "Run",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "MyOptions",
			OptionsArgIsPointer:        true,
			InitializerFunc:            "NewMyOptions",
		},
		Options: []*metadata.OptionMetadata{},
	}

	helpText := "Test command with initializer"

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, helpText, true, 0)
	if err != nil {
		t.Fatalf("GenerateMain with initializer failed: %v\nGenerated code:\n%s", err, actualCode)
	}
	assertCodeNotContains(t, actualCode, `import . "example.com/user/usercmd"`)
	assertCodeNotContains(t, actualCode, `import "example.com/user/usercmd"`)
	assertCodeContains(t, actualCode, "options = NewMyOptions()")
	assertCodeNotContains(t, actualCode, "options = new(MyOptions)")
	assertCodeNotContains(t, actualCode, "var options = &MyOptions{}")
	assertCodeContains(t, actualCode, "err = Run(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_WithoutInitializer_Fallback(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "example.com/user/usercmd",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "Run",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "MyOptions",
			OptionsArgIsPointer:        true,
			InitializerFunc:            "",
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Mode", TypeName: "string", DefaultValue: "test", HelpText: "Operation mode"},
			{Name: "Count", TypeName: "int", DefaultValue: 42, HelpText: "A number"},
		},
	}

	helpText := "Test command without initializer"

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, helpText, true, 0)
	if err != nil {
		t.Fatalf("GenerateMain without initializer failed: %v\nGenerated code:\n%s", err, actualCode)
	}
	assertCodeNotContains(t, actualCode, `import . "example.com/user/usercmd"`)
	assertCodeNotContains(t, actualCode, `import "example.com/user/usercmd"`)
	assertCodeNotContains(t, actualCode, "usercmd.NewMyOptions()")
	assertCodeContains(t, actualCode, "options = new(MyOptions)")
	assertCodeContains(t, actualCode, `options.Mode = "test"`)
	assertCodeContains(t, actualCode, `options.Count = 42`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Mode, "mode", options.Mode, "Operation mode" /* Original Default: test, Env: */)`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Count, "count", options.Count, "A number" /* Original Default: 42, Env: */)`)
	assertCodeContains(t, actualCode, "err = Run(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
}

func TestGenerateMain_InitializerInMainPackage(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "main",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "Run",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "MyOptions",
			OptionsArgIsPointer:        true,
			InitializerFunc:            "NewMyOptions",
		},
		Options: []*metadata.OptionMetadata{},
	}

	helpText := "Test command with initializer in main package"

	t.Setenv("DEBUG", "1")
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	originalLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	actualCode, err := GenerateMain(cmdMeta, helpText, true, 0)
	if err != nil {
		t.Fatalf("GenerateMain with initializer in main package failed: %v\nGenerated code:\n%s", err, actualCode)
	}
	assertCodeNotContains(t, actualCode, `import "main"`)
	assertCodeContains(t, actualCode, "options = NewMyOptions()")
	assertCodeNotContains(t, actualCode, "options = new(MyOptions)")
	assertCodeNotContains(t, actualCode, "var options = &MyOptions{}")
	assertCodeContains(t, actualCode, "err = Run(options)")

	logOutput := logBuf.String()
	assert.Contains(t, logOutput, "GenerateMain: start")
	assert.Contains(t, logOutput, "GenerateMain: end (full file)")
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
			want: "`Pre-existing newline\nAnd pre-existing ` + \"`\" + `backtick` + \"`\" + `.`",
		},
		{
			name:  "empty string",
			input: "",
			want:  "\"\"",
		},
		{
			name:  "only escaped newline",
			input: "\\n",
			want:  "`\n`",
		},
		{
			name:  "only single quote",
			input: "'",
			want:  "\"`\"",
		},
		{
			name:  "multiple backticks and newlines",
			input: "A\\nB'C'D\\nE'F'G",
			want:  "`A\nB` + \"`\" + `C` + \"`\" + `D\nE` + \"`\" + `F` + \"`\" + `G`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatHelpText(tt.input); got != tt.want {
				t.Errorf("formatHelpText(%q) = %s, want %s", tt.input, fmt.Sprintf("%q", got), fmt.Sprintf("%q", tt.want))
			}
		})
	}
}
