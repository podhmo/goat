package codegen // Changed from codegen_test

import (
	"fmt"
	"go/format"
	"regexp"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/metadata"
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

func TestGenerateMain_PointerFlagsNilHandling(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "ptrflagsnilcmd",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunPointerTest",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "PointerTestOptions",
			OptionsArgIsPointer:        true,
			InitializerFunc:            "NewPointerTestOptions", // Assumed initializer
		},
		Options: []*metadata.OptionMetadata{
			{
				Name:     "OptionalAge",
				TypeName: "*int",
				HelpText: "Optional age",
				// DefaultValue is nil (omitted)
			},
			{
				Name:     "ExtraToggle",
				TypeName: "*bool",
				HelpText: "Extra toggle",
				// DefaultValue is nil (omitted)
			},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "Test Pointer Flags Nil Handling", true)
	if err != nil {
		t.Fatalf("GenerateMain for pointer flags nil handling failed: %v", err)
	}

	// Assertions for *int option (OptionalAge)
	t.Run("*int option (OptionalAge)", func(t *testing.T) {
		assertCodeContains(t, actualCode, "isOptionalAgeNilInitially := options.OptionalAge == nil")
		assertCodeContains(t, actualCode, "var tempOptionalAgeVal int")
		assertCodeContains(t, actualCode, "var defaultOptionalAgeValForFlag int")
		assertCodeContains(t, actualCode, "if options.OptionalAge != nil { defaultOptionalAgeValForFlag = *options.OptionalAge }")
		assertCodeContains(t, actualCode, "if isOptionalAgeNilInitially { flag.IntVar(&tempOptionalAgeVal, \"optional-age\", 0, \"Optional age\")")      // Removed trailing space in help text
		assertCodeContains(t, actualCode, "} else { flag.IntVar(options.OptionalAge, \"optional-age\", defaultOptionalAgeValForFlag, \"Optional age\")") // Removed trailing space
		assertCodeContains(t, actualCode, "if isOptionalAgeNilInitially && isFlagExplicitlySet[\"optional-age\"] { options.OptionalAge = &tempOptionalAgeVal }")
		assertCodeNotContains(t, actualCode, "if options.OptionalAge == nil { options.OptionalAge = new(int) }")
	})

	// Assertions for *bool option (ExtraToggle)
	t.Run("*bool option (ExtraToggle)", func(t *testing.T) {
		assertCodeContains(t, actualCode, "isExtraToggleNilInitially := options.ExtraToggle == nil")
		assertCodeContains(t, actualCode, "var tempExtraToggleVal bool")
		assertCodeContains(t, actualCode, "var defaultExtraToggleValForFlag bool")
		assertCodeContains(t, actualCode, "if options.ExtraToggle != nil { defaultExtraToggleValForFlag = *options.ExtraToggle }")
		assertCodeContains(t, actualCode, "if isExtraToggleNilInitially { flag.BoolVar(&tempExtraToggleVal, \"extra-toggle\", false, \"Extra toggle\")")  // Removed trailing space
		assertCodeContains(t, actualCode, "} else { flag.BoolVar(options.ExtraToggle, \"extra-toggle\", defaultExtraToggleValForFlag, \"Extra toggle\")") // Removed trailing space
		assertCodeContains(t, actualCode, "if isExtraToggleNilInitially && isFlagExplicitlySet[\"extra-toggle\"] { options.ExtraToggle = &tempExtraToggleVal }")
		assertCodeNotContains(t, actualCode, "if options.ExtraToggle == nil { options.ExtraToggle = new(bool) }")
	})
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
	actualCode, err := GenerateMain(cmdMeta, "Test TextVar functionality", true)
	if err != nil {
		t.Fatalf("GenerateMain for TextVar options failed: %v", err)
	}
	assertCodeContains(t, actualCode, `flag.TextVar(&options.FieldA, "field-a", options.FieldA, "Help for FieldA" /* Env: FIELD_A_ENV */)`)
	assertCodeContains(t, actualCode, `err := (&options.FieldA).UnmarshalText([]byte(val))`)
	assertCodeContains(t, actualCode, `slog.WarnContext(ctx, "Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "FIELD_A_ENV", "option", "field-a"`)
	assertCodeContains(t, actualCode, `if options.FieldB == nil { options.FieldB = new(textvar_pkg.MyPtrTextValue) }`)
	assertCodeContains(t, actualCode, `flag.TextVar(options.FieldB, "field-b", options.FieldB, "Help for FieldB" /* Env: FIELD_B_ENV */)`)
	assertCodeContains(t, actualCode, `if options.FieldB == nil { options.FieldB = new(textvar_pkg.MyPtrTextValue) }`)
	assertCodeContains(t, actualCode, `err := options.FieldB.UnmarshalText([]byte(val))`)
	assertCodeContains(t, actualCode, `slog.WarnContext(ctx, "Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "FIELD_B_ENV", "option", "field-b"`)
	assertCodeNotContains(t, actualCode, `flag.TextVar(&options.FieldF, "field-f"`)
	assertCodeNotContains(t, actualCode, `flag.TextVar(options.FieldF, "field-f"`)
	assertCodeContains(t, actualCode, `err := (&options.FieldF).UnmarshalText([]byte(val))`)
	assertCodeContains(t, actualCode, `slog.WarnContext(ctx, "Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "FIELD_F_ENV", "option", "field-f"`)
	assertCodeContains(t, actualCode, "new(textvar_pkg.MyPtrTextValue)")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := Run(options); err != nil {")
	assertCodeContains(t, actualCode, "func main() {")
	assertCodeContains(t, actualCode, "ctx := context.Background()")
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Runtime error", "error", err)`)
	assertCodeContains(t, actualCode, `os.Exit(1)`)
	assertCodeNotContains(t, actualCode, "var options =")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(MyOptionsType)")
	// The line "options = new(MyOptionsType)" is no longer generated here after "options :="
	assertCodeNotContains(t, actualCode, "options = new(MyOptionsType)")
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
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := RunWithOptions(options); err != nil {")
	assertCodeNotContains(t, actualCode, "import . \"anothercmd\"")
	assertCodeNotContains(t, actualCode, "import \"anothercmd\"")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, `options.Name = "guest"`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Name, "name", options.Name, "Name of the user" /* Original Default: guest, Env: */)`)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := run(options); err != nil {")
	assertCodeNotContains(t, actualCode, "main.run(")
	assertCodeNotContains(t, actualCode, "main.Run(")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(DataProcOptions)")
	assertCodeNotContains(t, actualCode, "options = new(DataProcOptions)")
	assertCodeNotContains(t, actualCode, `options.InputFile = ""`)
	assertCodeContains(t, actualCode, `options.OutputDirectory = "/tmp"`)
	assertCodeContains(t, actualCode, `options.MaximumRetries = 3`)
	expectedFlagParsing := `
	flag.StringVar(&options.InputFile, "input-file", options.InputFile, "Input file path")
	flag.StringVar(&options.OutputDirectory, "output-directory", options.OutputDirectory, "Output directory path" /* Original Default: /tmp, Env: */)
	flag.IntVar(&options.MaximumRetries, "maximum-retries", options.MaximumRetries, "Maximum number of retries" /* Original Default: 3, Env: */)
	flag.Parse()
`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := ProcessData(options); err != nil {")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(Config)")
	assertCodeNotContains(t, actualCode, "options = new(Config)")
	assertCodeNotContains(t, actualCode, `options.ConfigFile = ""`)
	assertCodeContains(t, actualCode, `options.Retries = 0`)
	assertCodeNotContains(t, actualCode, `os.LookupEnv("CONFIG_FILE")`)
	assertCodeNotContains(t, actualCode, `os.LookupEnv("RETRIES")`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.ConfigFile, "config-file", options.ConfigFile, "Path to config file")`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Retries, "retries", options.Retries, "Number of retries" /* Original Default: 0, Env: */)`)
	assertCodeContains(t, actualCode, `initialDefaultConfigFile := ""`)
	assertCodeContains(t, actualCode, `envConfigFileWasSet := false`)
	assertCodeContains(t, actualCode, `if options.ConfigFile == initialDefaultConfigFile && !isFlagExplicitlySet["config-file"] && !envConfigFileWasSet {`)
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "config-file", "option", "ConfigFile")`)
	assertCodeContains(t, actualCode, `initialDefaultRetries := 0`)
	assertCodeContains(t, actualCode, `envRetriesWasSet := false`)
	assertCodeContains(t, actualCode, `if options.Retries == initialDefaultRetries && !isFlagExplicitlySet["retries"] && !envRetriesWasSet {`)
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "retries", "option", "Retries")`)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := DoSomething(*options); err != nil {")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(ModeOptions)")
	assertCodeNotContains(t, actualCode, "options = new(ModeOptions)")
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
		slog.ErrorContext(ctx, "Invalid value for flag", errors.New("Invalid value for flag"), "flag", "mode", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_Mode, ", "))
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedEnumValidation)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := SetMode(options); err != nil {")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(AppSettings)")
	assertCodeNotContains(t, actualCode, "options = new(AppSettings)")
	assertCodeNotContains(t, actualCode, `options.APIKey = ""`)
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
			slog.WarnContext(ctx, "Could not parse environment variable as int for option", "envVar", "TIMEOUT_SECONDS", "option", "Timeout", "value", val, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedTimeoutEnv)
	expectedEnableFeatureEnv := `
	if val, ok := os.LookupEnv("ENABLE_MY_FEATURE"); ok {
		if v, err := strconv.ParseBool(val); err == nil {
			options.EnableFeature = v
		} else {
			slog.WarnContext(ctx, "Could not parse environment variable as bool for option", "envVar", "ENABLE_MY_FEATURE", "option", "EnableFeature", "value", val, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnableFeatureEnv)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.APIKey, "api-key", options.APIKey, "API Key" /* Env: API_KEY */)`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Timeout, "timeout", options.Timeout, "Timeout in seconds" /* Original Default: 60, Env: TIMEOUT_SECONDS */)`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.EnableFeature, "enable-feature", options.EnableFeature, "Enable new feature" /* Original Default: false, Env: ENABLE_MY_FEATURE */)`)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := Configure(options); err != nil {")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(FeatureOptions)")
	assertCodeNotContains(t, actualCode, "options = new(FeatureOptions)")
	assertCodeContains(t, actualCode, `options.SmartParsing = true`)
	expectedEnvLogic := `
	if val, ok := os.LookupEnv("SMART_PARSING_ENABLED"); ok {
		if v, err := strconv.ParseBool(val); err == nil {
			options.SmartParsing = v
		} else {
			slog.WarnContext(ctx, "Could not parse environment variable as bool for option", "envVar", "SMART_PARSING_ENABLED", "option", "SmartParsing", "value", val, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnvLogic)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.SmartParsing, "smart-parsing", options.SmartParsing, "Enable smart parsing" /* Original Default: true, Env: SMART_PARSING_ENABLED */)`)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := ProcessWithFeature(options); err != nil {")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(DataOptions)")
	assertCodeNotContains(t, actualCode, "options = new(DataOptions)")
	assertCodeContains(t, actualCode, `options.ForceOverwrite = false`)
	assertCodeNotContains(t, actualCode, `os.LookupEnv("FORCE_OVERWRITE")`)
	expectedFlagParsing := `flag.BoolVar(&options.ForceOverwrite, "force-overwrite", options.ForceOverwrite, "Force overwrite of existing files" /* Original Default: false, Env: */)`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeNotContains(t, actualCode, "var ForceOverwrite_NoFlagIsPresent bool")
	assertCodeNotContains(t, actualCode, "options.ForceOverwrite = true")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(TaskConfig)")
	assertCodeNotContains(t, actualCode, "options = new(TaskConfig)")
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeNotContains(t, actualCode, "var err error")
	// For this test, we are checking the specific error handling block structure
	// which is now part of the if err := ... block.
	// The previous assertCodeContains for "err = DefaultRun(options)" is implicitly covered by the new format.
	assertCodeContains(t, actualCode, "if err := DefaultRun(options); err != nil {")
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Runtime error", "error", err)`)
	assertCodeContains(t, actualCode, `os.Exit(1)`)
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
	actualCodeNoStrconv, err := GenerateMain(cmdMetaNoStrconv, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCodeNoStrconv, `flag.StringVar(&options.Name, "name", options.Name, "app name" /* Env: APP_NAME */)`)
	assertCodeNotContains(t, actualCodeNoStrconv, `strconv.Atoi`)

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
	actualCodeWithStrconv, err := GenerateMain(cmdMetaWithStrconv, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCodeWithStrconv, `flag.IntVar(&options.Port, "port", options.Port, "app port" /* Env: APP_PORT */)`)
	assertCodeContains(t, actualCodeWithStrconv, `strconv.Atoi`)
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

	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(UserData)")
	assertCodeNotContains(t, actualCode, "options = new(UserData)")
	assertCodeContains(t, actualCode, `options.UserId = 0`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("USER_ID"); ok { if v, err := strconv.Atoi(val); err == nil { options.UserId = v } else { slog.WarnContext(ctx, "Could not parse environment variable as int for option", "envVar", "USER_ID", "option", "UserId", "value", val, "error", err) } }`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.UserId, "user-id", options.UserId, "User ID" /* Original Default: 0, Env: USER_ID */)`)
	expectedRequiredCheck := `
	initialDefaultUserId := 0
	envUserIdWasSet := false
	if _, ok := os.LookupEnv("USER_ID"); ok { envUserIdWasSet = true }
	if options.UserId == initialDefaultUserId && !isFlagExplicitlySet["user-id"] && !envUserIdWasSet {
		slog.ErrorContext(ctx, "Missing required flag or environment variable not set", errors.New("Missing required flag or environment variable not set"), "flag", "user-id", "envVar", "USER_ID", "option", "UserId")
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedRequiredCheck)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := SubmitData(options); err != nil {")
}

func TestGenerateMain_EnvVarPrecendenceStrategy(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunStrategyTest",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "StrategyOptions",
			OptionsArgIsPointer:        true,
			// InitializerFunc is NOT specified, so options will be new'd and zero-value defaults applied first
		},
		Options: []*metadata.OptionMetadata{
			{Name: "StringOpt", TypeName: "string", DefaultValue: "original_string", EnvVar: "ENV_STRING", HelpText: "String option"},
			{Name: "IntOpt", TypeName: "int", DefaultValue: 123, EnvVar: "ENV_INT", HelpText: "Int option"},
			{Name: "BoolOpt", TypeName: "bool", DefaultValue: false, EnvVar: "ENV_BOOL", HelpText: "Bool option"},
			{Name: "BoolTrueOpt", TypeName: "bool", DefaultValue: true, EnvVar: "ENV_BOOL_TRUE", IsRequired: true, HelpText: "Bool true option"},
			{Name: "StringPtrOpt", TypeName: "*string", EnvVar: "ENV_STRING_PTR", HelpText: "String pointer option"}, // No DefaultValue in metadata
			{Name: "IntPtrOpt", TypeName: "*int", EnvVar: "ENV_INT_PTR", HelpText: "Int pointer option"},             // No DefaultValue in metadata
			{Name: "BoolPtrOpt", TypeName: "*bool", EnvVar: "ENV_BOOL_PTR", HelpText: "Bool pointer option"},         // No DefaultValue in metadata
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "Test help text", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, `isFlagExplicitlySet := make(map[string]bool)`)
	assertCodeContains(t, actualCode, `flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })`)

	// Non-pointer types (existing assertions should be fine)
	assertCodeContains(t, actualCode, `options.StringOpt = "original_string"`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("ENV_STRING"); ok { options.StringOpt = val }`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.StringOpt, "string-opt", options.StringOpt, "String option" /* Original Default: original_string, Env: ENV_STRING */)`)
	assertCodeContains(t, actualCode, `options.IntOpt = 123`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("ENV_INT"); ok { if v, err := strconv.Atoi(val); err == nil { options.IntOpt = v } else { slog.WarnContext(ctx, "Could not parse environment variable as int for option", "envVar", "ENV_INT", "option", "IntOpt", "value", val, "error", err) } }`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.IntOpt, "int-opt", options.IntOpt, "Int option" /* Original Default: 123, Env: ENV_INT */)`)
	assertCodeContains(t, actualCode, `options.BoolOpt = false`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("ENV_BOOL"); ok { if v, err := strconv.ParseBool(val); err == nil { options.BoolOpt = v } else { slog.WarnContext(ctx, "Could not parse environment variable as bool for option", "envVar", "ENV_BOOL", "option", "BoolOpt", "value", val, "error", err) } }`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.BoolOpt, "bool-opt", options.BoolOpt, "Bool option" /* Original Default: false, Env: ENV_BOOL */)`)
	assertCodeContains(t, actualCode, `options.BoolTrueOpt = true`)
	assertCodeContains(t, actualCode, `if val, ok := os.LookupEnv("ENV_BOOL_TRUE"); ok { if v, err := strconv.ParseBool(val); err == nil { options.BoolTrueOpt = v } else { slog.WarnContext(ctx, "Could not parse environment variable as bool for option", "envVar", "ENV_BOOL_TRUE", "option", "BoolTrueOpt", "value", val, "error", err) } }`)
	assertCodeContains(t, actualCode, `var BoolTrueOpt_NoFlagIsPresent bool`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&BoolTrueOpt_NoFlagIsPresent, "no-bool-true-opt", false, "Set bool-true-opt to false")`)
	assertCodeContains(t, actualCode, `if BoolTrueOpt_NoFlagIsPresent { options.BoolTrueOpt = false }`)

	// Pointer types - asserting new logic
	// StringPtrOpt
	assertCodeContains(t, actualCode, `options.StringPtrOpt = new(string)`) // Initialized to new(string) because no InitializerFunc
	stringPtrEnvLogic := `
	if val, ok := os.LookupEnv("ENV_STRING_PTR"); ok {
		if options.StringPtrOpt == nil { options.StringPtrOpt = new(string) } // This check is fine for env vars
		*options.StringPtrOpt = val
	}
`
	assertCodeContains(t, actualCode, stringPtrEnvLogic)
	assertCodeContains(t, actualCode, "isStringPtrOptNilInitially := options.StringPtrOpt == nil")
	assertCodeContains(t, actualCode, "var tempStringPtrOptVal string")
	assertCodeContains(t, actualCode, "var defaultStringPtrOptValForFlag string")
	assertCodeContains(t, actualCode, "if options.StringPtrOpt != nil { defaultStringPtrOptValForFlag = *options.StringPtrOpt }")
	assertCodeNotContains(t, actualCode, "if options.StringPtrOpt == nil { options.StringPtrOpt = new(string) } flag.StringVar(options.StringPtrOpt, ") // Ensure old flag-specific init is gone
	newStringPtrFlagLogic := `
	if isStringPtrOptNilInitially {
		flag.StringVar(&tempStringPtrOptVal, "string-ptr-opt", "", "String pointer option" /* Env: ENV_STRING_PTR */)
	} else {
		flag.StringVar(options.StringPtrOpt, "string-ptr-opt", defaultStringPtrOptValForFlag, "String pointer option" /* Env: ENV_STRING_PTR */)
	}
`
	assertCodeContains(t, actualCode, newStringPtrFlagLogic)
	assertCodeContains(t, actualCode, "if isStringPtrOptNilInitially && isFlagExplicitlySet[\"string-ptr-opt\"] { options.StringPtrOpt = &tempStringPtrOptVal }")

	// IntPtrOpt
	assertCodeContains(t, actualCode, `options.IntPtrOpt = new(int)`) // Initialized to new(int)
	intPtrEnvLogic := `
	if val, ok := os.LookupEnv("ENV_INT_PTR"); ok {
		if options.IntPtrOpt == nil { options.IntPtrOpt = new(int) }
		if v, err := strconv.Atoi(val); err == nil { *options.IntPtrOpt = v
		} else { slog.WarnContext(ctx, "Could not parse environment variable as *int for option", "envVar", "ENV_INT_PTR", "option", "IntPtrOpt", "value", val, "error", err) }
	}
`
	assertCodeContains(t, actualCode, intPtrEnvLogic)
	assertCodeContains(t, actualCode, "isIntPtrOptNilInitially := options.IntPtrOpt == nil")
	assertCodeContains(t, actualCode, "var tempIntPtrOptVal int")
	assertCodeNotContains(t, actualCode, "if options.IntPtrOpt == nil { options.IntPtrOpt = new(int) } flag.IntVar(options.IntPtrOpt, ")
	newIntPtrFlagLogic := `
	if isIntPtrOptNilInitially {
		flag.IntVar(&tempIntPtrOptVal, "int-ptr-opt", 0, "Int pointer option" /* Env: ENV_INT_PTR */)
	} else {
		flag.IntVar(options.IntPtrOpt, "int-ptr-opt", defaultIntPtrOptValForFlag, "Int pointer option" /* Env: ENV_INT_PTR */)
	}
`
	assertCodeContains(t, actualCode, newIntPtrFlagLogic)
	assertCodeContains(t, actualCode, "if isIntPtrOptNilInitially && isFlagExplicitlySet[\"int-ptr-opt\"] { options.IntPtrOpt = &tempIntPtrOptVal }")

	// BoolPtrOpt
	assertCodeContains(t, actualCode, `options.BoolPtrOpt = new(bool)`) // Initialized to new(bool)
	boolPtrEnvLogic := `
	if val, ok := os.LookupEnv("ENV_BOOL_PTR"); ok {
		if options.BoolPtrOpt == nil { options.BoolPtrOpt = new(bool) }
		if v, err := strconv.ParseBool(val); err == nil { *options.BoolPtrOpt = v
		} else { slog.WarnContext(ctx, "Could not parse environment variable as *bool for option", "envVar", "ENV_BOOL_PTR", "option", "BoolPtrOpt", "value", val, "error", err) }
	}
`
	assertCodeContains(t, actualCode, boolPtrEnvLogic)
	assertCodeContains(t, actualCode, "isBoolPtrOptNilInitially := options.BoolPtrOpt == nil")
	assertCodeContains(t, actualCode, "var tempBoolPtrOptVal bool")
	assertCodeNotContains(t, actualCode, "if options.BoolPtrOpt == nil { options.BoolPtrOpt = new(bool) } flag.BoolVar(options.BoolPtrOpt, ")
	newBoolPtrFlagLogic := `
	if isBoolPtrOptNilInitially {
		flag.BoolVar(&tempBoolPtrOptVal, "bool-ptr-opt", false, "Bool pointer option" /* Env: ENV_BOOL_PTR */)
	} else {
		flag.BoolVar(options.BoolPtrOpt, "bool-ptr-opt", defaultBoolPtrOptValForFlag, "Bool pointer option" /* Env: ENV_BOOL_PTR */)
	}
`
	assertCodeContains(t, actualCode, newBoolPtrFlagLogic)
	assertCodeContains(t, actualCode, "if isBoolPtrOptNilInitially && isFlagExplicitlySet[\"bool-ptr-opt\"] { options.BoolPtrOpt = &tempBoolPtrOptVal }")

	assertCodeNotContains(t, actualCode, `var defaultStringOpt string =`) // This assertion is still valid
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
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(PrintOpts)")
	assertCodeNotContains(t, actualCode, "options = new(PrintOpts)")
	assertCodeContains(t, actualCode, `options.Greeting = "hello \"world\""`)
	expectedFlagParsing := `flag.StringVar(&options.Greeting, "greeting", options.Greeting, "A greeting message" /* Original Default: hello "world", Env: */)`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeNotContains(t, actualCode, "var err error") // Added this line
	assertCodeContains(t, actualCode, "if err := PrintString(options); err != nil {")
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

	actualCode, err := GenerateMain(cmdMeta, helpText, true)
	if err != nil {
		t.Fatalf("GenerateMain with help text failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(ToolOptions)")
	assertCodeNotContains(t, actualCode, "options = new(ToolOptions)")
	expectedHelpTextSnippet := `
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, ` + "`" + helpText + "`" + `)
	}`
	assertCodeContains(t, actualCode, expectedHelpTextSnippet)
	oldManualHelpLogic := `for _, arg := range os.Args[1:] { if arg == "-h" || arg == "--help" {`
	assertCodeNotContains(t, actualCode, oldManualHelpLogic)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Input, "input", options.Input, "Input file")`)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := RunMyTool(options); err != nil {")
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

	actualCode, err := GenerateMain(cmdMeta, "", true)
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
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := AnotherTool(); err != nil {")
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
		actualCode, err := GenerateMain(baseCmdMeta, helpTextWithNewlines, true)
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
	})

	t.Run("WithoutNewlines", func(t *testing.T) {
		helpTextWithoutNewlines := "This is a single line."
		expectedFormattedText := fmt.Sprintf("%q", helpTextWithoutNewlines)
		expectedSnippet := fmt.Sprintf("fmt.Fprint(os.Stderr, %s)", expectedFormattedText)
		actualCode, err := GenerateMain(baseCmdMeta, helpTextWithoutNewlines, true)
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
	actualCode, err := GenerateMain(cmdMeta, helpText, true)
	if err != nil {
		t.Fatalf("GenerateMain with initializer failed: %v\nGenerated code:\n%s", err, actualCode)
	}
	assertCodeNotContains(t, actualCode, `import . "example.com/user/usercmd"`)
	assertCodeNotContains(t, actualCode, `import "example.com/user/usercmd"`)
	assertCodeContains(t, actualCode, "options := NewMyOptions()")
	assertCodeNotContains(t, actualCode, "options = NewMyOptions()") // Ensure old form is not present
	assertCodeNotContains(t, actualCode, "options = new(MyOptions)")
	assertCodeNotContains(t, actualCode, "var options = &MyOptions{}")
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := Run(options); err != nil {")
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
	actualCode, err := GenerateMain(cmdMeta, helpText, true)
	if err != nil {
		t.Fatalf("GenerateMain without initializer failed: %v\nGenerated code:\n%s", err, actualCode)
	}
	assertCodeNotContains(t, actualCode, `import . "example.com/user/usercmd"`)
	assertCodeNotContains(t, actualCode, `import "example.com/user/usercmd"`)
	assertCodeNotContains(t, actualCode, "usercmd.NewMyOptions()")
	// This test is for NO initializer, so options := new(MyOptions) is expected
	assertCodeContains(t, actualCode, "options := new(MyOptions)")
	assertCodeNotContains(t, actualCode, "options = new(MyOptions)") // Ensure old form is not present
	assertCodeContains(t, actualCode, `options.Mode = "test"`)
	assertCodeContains(t, actualCode, `options.Count = 42`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Mode, "mode", options.Mode, "Operation mode" /* Original Default: test, Env: */)`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Count, "count", options.Count, "A number" /* Original Default: 42, Env: */)`)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := Run(options); err != nil {")
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
	actualCode, err := GenerateMain(cmdMeta, helpText, true)
	if err != nil {
		t.Fatalf("GenerateMain with initializer in main package failed: %v\nGenerated code:\n%s", err, actualCode)
	}
	assertCodeNotContains(t, actualCode, `import "main"`)
	assertCodeContains(t, actualCode, "options := NewMyOptions()")
	assertCodeNotContains(t, actualCode, "options = NewMyOptions()") // Ensure old form is not present
	assertCodeNotContains(t, actualCode, "options = new(MyOptions)")
	assertCodeNotContains(t, actualCode, "var options = &MyOptions{}")
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := Run(options); err != nil {")
}

func TestGenerateMain_WithContextOnly(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "ctxonlycmd",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunCtxOnly",
			PackageName:                "main",
			ContextArgName:             "ctx",
			ContextArgType:             "context.Context",
			OptionsArgTypeNameStripped: "", // No options
		},
		Options: []*metadata.OptionMetadata{},
	}
	actualCode, err := GenerateMain(cmdMeta, "Test context only", true)
	if err != nil {
		t.Fatalf("GenerateMain for context only failed: %v", err)
	}
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := RunCtxOnly(ctx); err != nil {")
	assertCodeNotContains(t, actualCode, "var options") // Ensure no options struct is declared
}

func TestGenerateMain_WithContextAndOptionsPointer(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "ctxoptscmd",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunCtxOptsPtr",
			PackageName:                "main",
			ContextArgName:             "ctx",
			ContextArgType:             "context.Context",
			OptionsArgTypeNameStripped: "MyOptions",
			OptionsArgIsPointer:        true,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "TestOption", TypeName: "string", DefaultValue: "hello"},
		},
	}
	actualCode, err := GenerateMain(cmdMeta, "Test context and options pointer", true)
	if err != nil {
		t.Fatalf("GenerateMain for context and options pointer failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(MyOptions)")
	assertCodeContains(t, actualCode, `options.TestOption = "hello"`)
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := RunCtxOptsPtr(ctx, options); err != nil {")
}

func TestGenerateMain_WithContextAndOptionsValue(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "ctxoptsvalcmd",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunCtxOptsVal",
			PackageName:                "main",
			ContextArgName:             "ctx",
			ContextArgType:             "context.Context",
			OptionsArgTypeNameStripped: "MyOptions",
			OptionsArgIsPointer:        false, // Value type
		},
		Options: []*metadata.OptionMetadata{
			{Name: "AnotherOption", TypeName: "int", DefaultValue: 123},
		},
	}
	actualCode, err := GenerateMain(cmdMeta, "Test context and options value", true)
	if err != nil {
		t.Fatalf("GenerateMain for context and options value failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(MyOptions)") // Still a pointer internally for setup
	assertCodeContains(t, actualCode, "options.AnotherOption = 123")
	assertCodeNotContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "if err := RunCtxOptsVal(ctx, *options); err != nil {")
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
			want:  "`Pre-existing newline\nAnd pre-existing ` + \"`\" + `backtick` + \"`\" + `.`",
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
