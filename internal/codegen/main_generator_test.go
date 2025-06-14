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
			InitializerFunc:            "NewPointerTestOptions",
		},
		Options: []*metadata.OptionMetadata{
			{
				Name:     "OptionalAge",
				CliName:  "optional-age",
				TypeName: "*int",
				HelpText: "Optional age",
			},
			{
				Name:     "ExtraToggle",
				CliName:  "extra-toggle",
				TypeName: "*bool",
				HelpText: "Extra toggle", // DefaultValue is nil, so help should show (default: false)
			},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "Test Pointer Flags Nil Handling", true)
	if err != nil {
		t.Fatalf("GenerateMain for pointer flags nil handling failed: %v\nActual Code:\n%s", err, actualCode)
	}

	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = RunPointerTest(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Runtime error from command function", "error", err)`)

	t.Run("*int option (OptionalAge)", func(t *testing.T) {
		assertCodeContains(t, actualCode, "isOptionalageNilInitially := options.OptionalAge == nil")                                                             // Corrected: Age -> age
		assertCodeContains(t, actualCode, "var tempOptionalageVal int")                                                                                          // Corrected: Age -> age
		assertCodeContains(t, actualCode, "var defaultOptionalageValForFlag int")                                                                                // Corrected: Age -> age
		assertCodeContains(t, actualCode, "if !isOptionalageNilInitially { defaultOptionalageValForFlag = *options.OptionalAge }")                               // Corrected: Age -> age
		assertCodeContains(t, actualCode, "if isOptionalageNilInitially { flag.IntVar(&tempOptionalageVal, \"optional-age\", 0, \"Optional age\")")              // Corrected: Age -> age
		assertCodeContains(t, actualCode, "} else { flag.IntVar(options.OptionalAge, \"optional-age\", defaultOptionalageValForFlag, \"Optional age\")")         // Corrected: Age -> age
		assertCodeContains(t, actualCode, "if isOptionalageNilInitially && isFlagExplicitlySet[\"optional-age\"] { options.OptionalAge = &tempOptionalageVal }") // Corrected: Age -> age
	})

	t.Run("*bool option (ExtraToggle)", func(t *testing.T) {
		assertCodeContains(t, actualCode, "isExtratoggleNilInitially := options.ExtraToggle == nil")                                                                       // Corrected: Toggle -> toggle
		assertCodeContains(t, actualCode, "var tempExtratoggleVal bool")                                                                                                   // Corrected: Toggle -> toggle
		assertCodeContains(t, actualCode, "var defaultExtratoggleValForFlag bool")                                                                                         // Corrected: Toggle -> toggle
		assertCodeContains(t, actualCode, "if !isExtratoggleNilInitially { defaultExtratoggleValForFlag = *options.ExtraToggle }")                                         // Corrected: Toggle -> toggle
		assertCodeContains(t, actualCode, "if isExtratoggleNilInitially { flag.BoolVar(&tempExtratoggleVal, \"extra-toggle\", false, \"Extra toggle (default: false)\")")  // Corrected: Toggle -> toggle
		assertCodeContains(t, actualCode, "} else { flag.BoolVar(options.ExtraToggle, \"extra-toggle\", defaultExtratoggleValForFlag, \"Extra toggle (default: false)\")") // Corrected: Toggle -> toggle
		assertCodeContains(t, actualCode, "if isExtratoggleNilInitially && isFlagExplicitlySet[\"extra-toggle\"] { options.ExtraToggle = &tempExtratoggleVal }")           // Corrected: Toggle -> toggle
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
	assertCodeContains(t, actualCode, `flag.Func("field-a", "Help for FieldA (env: FIELD_A_ENV)"`)
	assertCodeContains(t, actualCode, `slog.WarnContext(ctx, "Failed to unmarshal environment variable for TextUnmarshaler option", "variable", "FIELD_A_ENV", "value", fieldaEnvVal, "error", err)`)
	assertCodeContains(t, actualCode, `if options.FieldB == nil { options.FieldB = new(textvar_pkg.MyPtrTextValue) }`)
	assertCodeContains(t, actualCode, `flag.StringVar(&tempFieldbStr, "field-b", "", "Help for FieldB (env: FIELD_B_ENV)")`)
	assertCodeContains(t, actualCode, `slog.WarnContext(ctx, "Failed to unmarshal environment variable for *TextUnmarshaler option", "variable", "FIELD_B_ENV", "value", fieldbEnvVal, "error", err)`)
	assertCodeContains(t, actualCode, `flag.Func("field-f", "Help for FieldF - only unmarshaler (env: FIELD_F_ENV)"`)
	assertCodeContains(t, actualCode, `slog.WarnContext(ctx, "Failed to unmarshal environment variable for TextUnmarshaler option", "variable", "FIELD_F_ENV", "value", fieldfEnvVal, "error", err)`)
	assertCodeContains(t, actualCode, "new(textvar_pkg.MyPtrTextValue)")
	// For TextUnmarshalerPtrHandler, the temp variable is temp<OptionName>Str, e.g. tempFieldbStr
	// The context variable used in the generated code is `ctx`, not `ctxVarName` from the handler params.
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Failed to unmarshal flag value for *TextUnmarshaler option", "option", "field-b", "value", tempFieldbStr, "error", err)`) // Corrected: tempFieldBStr -> tempFieldbStr
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
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = Run(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeContains(t, actualCode, "func main() {")
	assertCodeContains(t, actualCode, "ctx := context.Background()")
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Runtime error from command function", "error", err)`)
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
			{Name: "Name", CliName: "name", TypeName: "string", HelpText: "Name of the user", DefaultValue: "guest"},
			{Name: "Age", CliName: "age", TypeName: "int", HelpText: "Age of the user", DefaultValue: 30.0},
			{Name: "Verbose", CliName: "verbose", TypeName: "bool", HelpText: "Enable verbose output", DefaultValue: false},
		},
	}
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(MyOptionsType)")
	assertCodeContains(t, actualCode, `options.Name = "guest"`)
	assertCodeContains(t, actualCode, `options.Age = 30`)
	assertCodeContains(t, actualCode, `options.Verbose = false`)
	expectedFlagParsing := `
	flag.StringVar(&options.Name, "name", "guest", "Name of the user (default: guest)")
	flag.IntVar(&options.Age, "age", 30, "Age of the user (default: 30)")
	flag.BoolVar(&options.Verbose, "verbose", false, "Enable verbose output (default: false)")
	flag.Parse()
`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = RunWithOptions(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Runtime error from command function", "error", err)`)
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
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Name, "name", "guest", "Name of the user (default: guest)")`)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = run(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	// options.InputFile will be "" by default (zero-value for string)
	assertCodeContains(t, actualCode, `options.OutputDirectory = "/tmp"`)
	assertCodeContains(t, actualCode, `options.MaximumRetries = 3`)
	expectedFlagParsing := `
	flag.StringVar(&options.InputFile, "input-file", "", "Input file path")
	flag.StringVar(&options.OutputDirectory, "output-directory", "/tmp", "Output directory path (default: /tmp)")
	flag.IntVar(&options.MaximumRetries, "maximum-retries", 3, "Maximum number of retries (default: 3)")
	flag.Parse()
`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = ProcessData(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	// options.ConfigFile will be "" by default (zero-value for string)
	assertCodeContains(t, actualCode, `options.Retries = 0`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.ConfigFile, "config-file", "", "Path to config file")`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Retries, "retries", 0, "Number of retries (default: 0)")`)

	assertCodeContains(t, actualCode, `initialDefault_Configfile := ""`) // Corrected: ConfigFile -> Configfile
	assertCodeContains(t, actualCode, `env_Configfile_WasSet := false`)  // Corrected: ConfigFile -> Configfile
	assertCodeContains(t, actualCode, `initialDefault_Retries := 0`)
	assertCodeContains(t, actualCode, `env_Retries_WasSet := false`)

	assertCodeContains(t, actualCode, `if options.ConfigFile == initialDefault_Configfile && !isFlagExplicitlySet["config-file"] && !env_Configfile_WasSet {`) // Corrected: ConfigFile -> Configfile
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Missing required option", "flag", "config-file", "option", "ConfigFile")`)
	assertCodeContains(t, actualCode, `return fmt.Errorf("missing required option: --config-file / ")`)

	assertCodeContains(t, actualCode, `if options.Retries == initialDefault_Retries && !isFlagExplicitlySet["retries"] && !env_Retries_WasSet {`)
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Missing required option", "flag", "retries", "option", "Retries")`)
	assertCodeContains(t, actualCode, `return fmt.Errorf("missing required option: --retries / ")`)

	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = DoSomething(*options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
			{Name: "Mode", CliName: "mode", TypeName: "string", HelpText: "Mode of operation", EnumValues: []any{"auto", "manual", "standby"}, DefaultValue: "auto"},
		},
	}
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(ModeOptions)")
	assertCodeContains(t, actualCode, `options.Mode = "auto"`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Mode, "mode", "auto", "Mode of operation (default: auto) (allowed: auto, manual, standby)")`)

	expectedEnumLogic := `
var modeEnumValues = []string{"auto", "manual", "standby"}
if err = func() error {
found := false
for _, validVal := range modeEnumValues {
	if options.Mode == validVal {
		found = true
		break
	}
}
if !found {
	slog.ErrorContext(ctx, "Invalid value for option", "option", "mode", "value", options.Mode, "allowed", modeEnumValues)
	return fmt.Errorf("invalid value for --mode: got %q, expected one of %v", options.Mode, modeEnumValues)
}
return nil
}(); err != nil {
	slog.ErrorContext(ctx, "Error validating enum for option", "option", "mode", "error", err)
	os.Exit(1)
}
`
	assertCodeContains(t, actualCode, expectedEnumLogic)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = SetMode(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	// options.APIKey will be "" by default (zero-value for string)
	assertCodeContains(t, actualCode, `options.Timeout = 60`)
	assertCodeContains(t, actualCode, `options.EnableFeature = false`)

	expectedApiKeyEnv := `
	if apikeyEnvVal, ok := os.LookupEnv("API_KEY"); ok {
		options.APIKey = apikeyEnvVal
	}
`
	assertCodeContains(t, actualCode, expectedApiKeyEnv)

	expectedTimeoutEnv := `
	if timeoutEnvVal, ok := os.LookupEnv("TIMEOUT_SECONDS"); ok {
		var timeoutEnvValConverted int
		if v, err := strconv.Atoi(timeoutEnvVal); err == nil {
			timeoutEnvValConverted = v
			options.Timeout = timeoutEnvValConverted
		} else {
			slog.WarnContext(ctx, "Invalid integer value for environment variable", "variable", "TIMEOUT_SECONDS", "value", timeoutEnvVal, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedTimeoutEnv)

	expectedEnableFeatureEnv := `
	if enablefeatureEnvVal, ok := os.LookupEnv("ENABLE_MY_FEATURE"); ok {
		if v, err := strconv.ParseBool(enablefeatureEnvVal); err == nil {
			options.EnableFeature = v
		} else {
			slog.WarnContext(ctx, "Invalid boolean value for environment variable", "variable", "ENABLE_MY_FEATURE", "value", enablefeatureEnvVal, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnableFeatureEnv)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.APIKey, "api-key", "", "API Key (env: API_KEY)")`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Timeout, "timeout", 60, "Timeout in seconds (default: 60) (env: TIMEOUT_SECONDS)")`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.EnableFeature, "enable-feature", false, "Enable new feature (default: false) (env: ENABLE_MY_FEATURE)")`)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = Configure(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	assertCodeContains(t, actualCode, `options.SmartParsing = true`)
	expectedEnvLogic := `
	if smartparsingEnvVal, ok := os.LookupEnv("SMART_PARSING_ENABLED"); ok {
		if v, err := strconv.ParseBool(smartparsingEnvVal); err == nil {
			options.SmartParsing = v
		} else {
			slog.WarnContext(ctx, "Invalid boolean value for environment variable", "variable", "SMART_PARSING_ENABLED", "value", smartparsingEnvVal, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnvLogic)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.SmartParsing, "smart-parsing", true, "Enable smart parsing (default: true) (env: SMART_PARSING_ENABLED)")`)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = ProcessWithFeature(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	assertCodeContains(t, actualCode, `options.ForceOverwrite = false`)
	expectedFlagParsing := `flag.BoolVar(&options.ForceOverwrite, "force-overwrite", false, "Force overwrite of existing files (default: false)")`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, `initialDefault_Forceoverwrite := false`)                                                                                                // Corrected: ForceOverwrite -> Forceoverwrite
	assertCodeContains(t, actualCode, `if options.ForceOverwrite == initialDefault_Forceoverwrite && !isFlagExplicitlySet["force-overwrite"] && !env_Forceoverwrite_WasSet {`) // Corrected: ForceOverwrite -> Forceoverwrite (twice)
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Missing required boolean option (must be explicitly set)", "flag", "force-overwrite", "option", "ForceOverwrite")`)
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
			{Name: "EnableSync", CliName: "enable-sync", TypeName: "bool", HelpText: "Enable synchronization", IsRequired: true, DefaultValue: true},
		},
	}
	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(TaskConfig)")
	assertCodeContains(t, actualCode, `options.EnableSync = true`)
	expectedFlagDefinition := `
	var tempEnablesyncNoFlagPresent bool
	flag.BoolVar(&tempEnablesyncNoFlagPresent, "no-enable-sync", false, "Set enable-sync to false, overriding default true")
	flag.BoolVar(&options.EnableSync, "enable-sync", true, "Enable synchronization (default: true)")
`
	assertCodeContains(t, actualCode, expectedFlagDefinition)

	expectedPostParseLogic := `
	if isFlagExplicitlySet["no-enable-sync"] {
		options.EnableSync = false
	}
`
	assertCodeContains(t, actualCode, expectedPostParseLogic)

	assertCodeContains(t, actualCode, `initialDefault_Enablesync := true`)                                                                                     // Corrected: EnableSync -> Enablesync
	assertCodeContains(t, actualCode, `if options.EnableSync == initialDefault_Enablesync && !isFlagExplicitlySet["enable-sync"] && !env_Enablesync_WasSet {`) // Corrected: EnableSync -> Enablesync (twice)
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Missing required boolean option (must be explicitly set)", "flag", "enable-sync", "option", "EnableSync")`)
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
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "err = DefaultRun(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Runtime error from command function", "error", err)`)
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
		t.Fatalf("GenerateMain failed for cmdMetaNoStrconv: %v", err)
	}
	assertCodeContains(t, actualCodeNoStrconv, `flag.StringVar(&options.Name, "name", "", "app name (env: APP_NAME)")`)
	assertCodeNotContains(t, actualCodeNoStrconv, `strconv.Atoi`)

	cmdMetaWithStrconv := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "MyOtherFunc",
			PackageName:                "main",
			OptionsArgTypeNameStripped: "ServerConfig",
			OptionsArgIsPointer:        false,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "Port", CliName: "port", TypeName: "int", EnvVar: "APP_PORT", HelpText: "app port (default: 0)", DefaultValue: 0.0},
		},
	}
	actualCodeWithStrconv, err := GenerateMain(cmdMetaWithStrconv, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed for cmdMetaWithStrconv: %v", err)
	}
	assertCodeContains(t, actualCodeWithStrconv, `flag.IntVar(&options.Port, "port", 0, "app port (default: 0) (env: APP_PORT)")`)
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
			{Name: "UserId", CliName: "user-id", TypeName: "int", HelpText: "User ID", IsRequired: true, EnvVar: "USER_ID", DefaultValue: 0.0},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(UserData)")
	assertCodeContains(t, actualCode, `options.UserId = 0`)
	envLogic := `
	if useridEnvVal, ok := os.LookupEnv("USER_ID"); ok {
		var useridEnvValConverted int
		if v, err := strconv.Atoi(useridEnvVal); err == nil {
			useridEnvValConverted = v
			options.UserId = useridEnvValConverted
		} else {
			slog.WarnContext(ctx, "Invalid integer value for environment variable", "variable", "USER_ID", "value", useridEnvVal, "error", err)
		}
	}
`
	assertCodeContains(t, actualCode, envLogic)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.UserId, "user-id", 0, "User ID (default: 0) (env: USER_ID)")`)

	expectedRequiredCheck := `
	initialDefault_Userid := 0 // Corrected: UserId -> Userid
	_, env_Userid_WasSet := os.LookupEnv("USER_ID") // Corrected: UserId -> Userid
	if err = func() error {
		if options.UserId == initialDefault_Userid && !isFlagExplicitlySet["user-id"] && !env_Userid_WasSet { // Corrected: UserId -> Userid (twice)
			slog.ErrorContext(ctx, "Missing required option", "flag", "user-id", "envVar", "USER_ID", "option", "UserId")
			return fmt.Errorf("missing required option: --user-id / USER_ID")
		}
		return nil
	}(); err != nil {
		slog.ErrorContext(ctx, "Error processing required option", "option", "user-id", "error", err)
		os.Exit(1)
	}
`
	assertCodeContains(t, actualCode, expectedRequiredCheck)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = SubmitData(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
			{Name: "StringOpt", CliName: "string-opt", TypeName: "string", DefaultValue: "original_string", EnvVar: "ENV_STRING", HelpText: "String option"},
			{Name: "IntOpt", CliName: "int-opt", TypeName: "int", DefaultValue: 123.0, EnvVar: "ENV_INT", HelpText: "Int option"},
			{Name: "BoolOpt", CliName: "bool-opt", TypeName: "bool", DefaultValue: false, EnvVar: "ENV_BOOL", HelpText: "Bool option"},
			{Name: "BoolTrueOpt", CliName: "bool-true-opt", TypeName: "bool", DefaultValue: true, EnvVar: "ENV_BOOL_TRUE", IsRequired: true, HelpText: "Bool true option"},
			{Name: "StringPtrOpt", CliName: "string-ptr-opt", TypeName: "*string", EnvVar: "ENV_STRING_PTR", HelpText: "String pointer option"},
			{Name: "IntPtrOpt", CliName: "int-ptr-opt", TypeName: "*int", EnvVar: "ENV_INT_PTR", HelpText: "Int pointer option"},
			{Name: "BoolPtrOpt", CliName: "bool-ptr-opt", TypeName: "*bool", EnvVar: "ENV_BOOL_PTR", HelpText: "Bool pointer option"},
		},
	}

	actualCode, err := GenerateMain(cmdMeta, "Test help text", true)
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	assertCodeContains(t, actualCode, `isFlagExplicitlySet := make(map[string]bool)`)
	assertCodeContains(t, actualCode, `flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })`)

	assertCodeContains(t, actualCode, `options.StringOpt = "original_string"`)
	assertCodeContains(t, actualCode, `options.IntOpt = 123`)
	assertCodeContains(t, actualCode, `options.BoolOpt = false`)
	assertCodeContains(t, actualCode, `options.BoolTrueOpt = true`)
	assertCodeNotContains(t, actualCode, `options.StringPtrOpt = new(string)`)
	assertCodeNotContains(t, actualCode, `options.IntPtrOpt = new(int)`)
	assertCodeNotContains(t, actualCode, `options.BoolPtrOpt = new(bool)`)

	assertCodeContains(t, actualCode, `if stringoptEnvVal, ok := os.LookupEnv("ENV_STRING"); ok { options.StringOpt = stringoptEnvVal }`)
	// For IntOpt (non-pointer)
	expectedIntEnvLogic := `
	if intoptEnvVal, ok := os.LookupEnv("ENV_INT"); ok {
		var intoptEnvValConverted int
		if v, err := strconv.Atoi(intoptEnvVal); err == nil {
			intoptEnvValConverted = v
			options.IntOpt = intoptEnvValConverted
		} else {
			slog.WarnContext(ctx, "Invalid integer value for environment variable", "variable", "ENV_INT", "value", intoptEnvVal, "error", err)
		}
	}`
	assertCodeContains(t, actualCode, expectedIntEnvLogic)
	assertCodeContains(t, actualCode, `if booloptEnvVal, ok := os.LookupEnv("ENV_BOOL"); ok { if v, err := strconv.ParseBool(booloptEnvVal); err == nil { options.BoolOpt = v } else { slog.WarnContext(ctx, "Invalid boolean value for environment variable", "variable", "ENV_BOOL", "value", booloptEnvVal, "error", err) } }`)
	assertCodeContains(t, actualCode, `if booltrueoptEnvVal, ok := os.LookupEnv("ENV_BOOL_TRUE"); ok { if v, err := strconv.ParseBool(booltrueoptEnvVal); err == nil { options.BoolTrueOpt = v } else { slog.WarnContext(ctx, "Invalid boolean value for environment variable", "variable", "ENV_BOOL_TRUE", "value", booltrueoptEnvVal, "error", err) } }`)
	assertCodeContains(t, actualCode, `if stringptroptEnvVal, ok := os.LookupEnv("ENV_STRING_PTR"); ok { { valCopy := stringptroptEnvVal options.StringPtrOpt = &valCopy } }`)
	// For IntPtrOpt
	expectedIntPtrEnvLogic := `
	if intptroptEnvVal, ok := os.LookupEnv("ENV_INT_PTR"); ok {
		if v, err := strconv.Atoi(intptroptEnvVal); err == nil {
			valCopy := v
			options.IntPtrOpt = &valCopy
		} else {
			slog.WarnContext(ctx, "Invalid integer value for environment variable", "variable", "ENV_INT_PTR", "value", intptroptEnvVal, "error", err)
		}
	}`
	assertCodeContains(t, actualCode, expectedIntPtrEnvLogic)
	// For BoolPtrOpt
	expectedBoolPtrEnvLogic := `
	if boolptroptEnvVal, ok := os.LookupEnv("ENV_BOOL_PTR"); ok {
		if v, err := strconv.ParseBool(boolptroptEnvVal); err == nil {
			valCopy := v
			options.BoolPtrOpt = &valCopy
		} else {
			slog.WarnContext(ctx, "Invalid boolean value for environment variable", "variable", "ENV_BOOL_PTR", "value", boolptroptEnvVal, "error", err)
		}
	}`
	assertCodeContains(t, actualCode, expectedBoolPtrEnvLogic)

	assertCodeContains(t, actualCode, `flag.StringVar(&options.StringOpt, "string-opt", "original_string", "String option (default: original_string) (env: ENV_STRING)")`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.IntOpt, "int-opt", 123, "Int option (default: 123) (env: ENV_INT)")`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.BoolOpt, "bool-opt", false, "Bool option (default: false) (env: ENV_BOOL)")`)
	assertCodeContains(t, actualCode, `var tempBooltrueoptNoFlagPresent bool`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&tempBooltrueoptNoFlagPresent, "no-bool-true-opt", false, "Set bool-true-opt to false, overriding default true")`)
	assertCodeContains(t, actualCode, `flag.BoolVar(&options.BoolTrueOpt, "bool-true-opt", true, "Bool true option (default: true) (env: ENV_BOOL_TRUE)")`)
	assertCodeContains(t, actualCode, `isStringptroptNilInitially := options.StringPtrOpt == nil`)
	assertCodeContains(t, actualCode, `var tempStringptroptVal string`)
	assertCodeContains(t, actualCode, `flag.StringVar(&tempStringptroptVal, "string-ptr-opt", "", "String pointer option (env: ENV_STRING_PTR)")`)
	assertCodeContains(t, actualCode, `if isStringptroptNilInitially && isFlagExplicitlySet["string-ptr-opt"] { options.StringPtrOpt = &tempStringptroptVal }`)

	assertCodeContains(t, actualCode, `initialDefault_Booltrueopt := true`)                                                                                                                                                                                                                                                         // Corrected: BoolTrueOpt -> Booltrueopt
	assertCodeContains(t, actualCode, `if options.BoolTrueOpt == initialDefault_Booltrueopt && !isFlagExplicitlySet["bool-true-opt"] && !env_Booltrueopt_WasSet { slog.ErrorContext(ctx, "Missing required boolean option (must be explicitly set)", "flag", "bool-true-opt", "envVar", "ENV_BOOL_TRUE", "option", "BoolTrueOpt")`) // Corrected: BoolTrueOpt -> Booltrueopt (twice)

	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = RunStrategyTest(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeContains(t, actualCode, `slog.ErrorContext(ctx, "Runtime error from command function", "error", err)`)

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
	assertCodeContains(t, actualCode, `options.Greeting = "hello \"world\""`)
	expectedFlagParsing := `flag.StringVar(&options.Greeting, "greeting", "hello \"world\"", "A greeting message (default: hello \"world\")")`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = PrintString(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	helpTextWithRawQuotes := strings.ReplaceAll(helpText, "`", "`+\"`\"+`")
	expectedHelpTextSnippet := "fmt.Fprint(os.Stderr, `" + helpTextWithRawQuotes + "`)"
	assertCodeContains(t, actualCode, expectedHelpTextSnippet)

	assertCodeContains(t, actualCode, `flag.StringVar(&options.Input, "input", "", "Input file")`)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = RunMyTool(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = AnotherTool()")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	assertCodeNotContains(t, actualCode, "options = NewMyOptions()")
	assertCodeNotContains(t, actualCode, "options = new(MyOptions)")
	assertCodeNotContains(t, actualCode, "var options = &MyOptions{}")
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = Run(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	assertCodeContains(t, actualCode, "options := new(MyOptions)")
	assertCodeContains(t, actualCode, `options.Mode = "test"`)
	assertCodeContains(t, actualCode, `options.Count = 42`)
	assertCodeContains(t, actualCode, `flag.StringVar(&options.Mode, "mode", "test", "Operation mode (default: test)")`)
	assertCodeContains(t, actualCode, `flag.IntVar(&options.Count, "count", 42, "A number (default: 42)")`)
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = Run(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
	assertCodeNotContains(t, actualCode, "options = NewMyOptions()")
	assertCodeNotContains(t, actualCode, "options = new(MyOptions)")
	assertCodeNotContains(t, actualCode, "var options = &MyOptions{}")
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = Run(options)")
	assertCodeContains(t, actualCode, "if err != nil {")
}

func TestGenerateMain_WithContextOnly(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		Name: "ctxonlycmd",
		RunFunc: &metadata.RunFuncInfo{
			Name:                       "RunCtxOnly",
			PackageName:                "main",
			ContextArgName:             "ctx",
			ContextArgType:             "context.Context",
			OptionsArgTypeNameStripped: "",
		},
		Options: []*metadata.OptionMetadata{},
	}
	actualCode, err := GenerateMain(cmdMeta, "Test context only", true)
	if err != nil {
		t.Fatalf("GenerateMain for context only failed: %v", err)
	}
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = RunCtxOnly(ctx)")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeNotContains(t, actualCode, "var options")
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
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = RunCtxOptsPtr(ctx, options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
			OptionsArgIsPointer:        false,
		},
		Options: []*metadata.OptionMetadata{
			{Name: "AnotherOption", TypeName: "int", DefaultValue: 123},
		},
	}
	actualCode, err := GenerateMain(cmdMeta, "Test context and options value", true)
	if err != nil {
		t.Fatalf("GenerateMain for context and options value failed: %v", err)
	}
	assertCodeContains(t, actualCode, "options := new(MyOptions)")
	assertCodeContains(t, actualCode, "options.AnotherOption = 123")
	assertCodeContains(t, actualCode, "var err error")
	assertCodeContains(t, actualCode, "_ = err")
	assertCodeContains(t, actualCode, "err = RunCtxOptsVal(ctx, *options)")
	assertCodeContains(t, actualCode, "if err != nil {")
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
