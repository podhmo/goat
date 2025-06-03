package codegen_test

import (
	"fmt"
	"go/format"
	"regexp"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/codegen"
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
		RunFunc: &metadata.RunFuncInfo{ // Changed to *RunFuncInfo
			Name:        "Run",
			PackageName: "mycmd",
			// Imports field removed
		},
		Options: []*metadata.OptionMetadata{}, // Changed to []*OptionMetadata
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	// Assertions use normalizeCode for actualCode, and normalizeForContains for snippets (done inside helpers)
	// For the RunFuncPackage, assert that the quoted package name exists.
	// This check is for the call site like `mycmd.Run()`, not for an import statement.
	assertCodeContains(t, actualCode, cmdMeta.RunFunc.PackageName) // Check for usage, not the import string literal
	assertCodeContains(t, actualCode, "func main() {")
	assertCodeContains(t, actualCode, "err := mycmd.Run()")
	assertCodeContains(t, actualCode, "if err != nil {")
	assertCodeContains(t, actualCode, "log.Fatal(err)")
	assertCodeNotContains(t, actualCode, "type Options struct")
}

func TestGenerateMain_WithOptions(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:        "RunWithOptions",
			PackageName: "anothercmd",
		},
		Options: []*metadata.OptionMetadata{ // Changed to []*OptionMetadata
			// Name used for Go var and CLI flag (per instruction), TypeName, HelpText, DefaultValue
			{Name: "name", CliName: "name", TypeName: "string", HelpText: "Name of the user", DefaultValue: "guest"},
			{Name: "age", CliName: "age", TypeName: "int", HelpText: "Age of the user", DefaultValue: 30},                   // DefaultValue is int
			{Name: "verbose", CliName: "verbose", TypeName: "bool", HelpText: "Enable verbose output", DefaultValue: false}, // DefaultValue is bool
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	assertCodeNotContains(t, actualCode, "type Options struct")

	expectedVarDeclarations := `
	var NameFlag string
	var AgeFlag int
	var VerboseFlag bool
`
	assertCodeContains(t, actualCode, expectedVarDeclarations)

	expectedFlagParsing := `
	flag.StringVar(&NameFlag, "name", "guest", "Name of the user")
	flag.IntVar(&AgeFlag, "age", 30, "Age of the user")
	flag.BoolVar(&VerboseFlag, "verbose", false, "Enable verbose output")
	flag.Parse()
`
	assertCodeContains(t, actualCode, expectedFlagParsing)
	assertCodeContains(t, actualCode, "err := anothercmd.RunWithOptions(NameFlag, AgeFlag, VerboseFlag)")
}

func TestGenerateMain_RequiredFlags(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{Name: "DoSomething", PackageName: "task"},
		Options: []*metadata.OptionMetadata{
			{Name: "configFile", CliName: "configFile", TypeName: "string", HelpText: "Path to config file", IsRequired: true},       // Required -> IsRequired
			{Name: "retries", CliName: "retries", TypeName: "int", HelpText: "Number of retries", IsRequired: true, DefaultValue: 0}, // Default -> DefaultValue (as int)
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	expectedConfigFileCheck := `
	if ConfigFileFlag == "" {
		log.Fatalf("Missing required flag: -configFile")
	}
`
	assertCodeContains(t, actualCode, expectedConfigFileCheck)

	expectedRetriesCheck := `
	if RetriesFlag == 0 {
		isSet_Retries := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "retries" { // .Name used for CLI flag name
				isSet_Retries = true
			}
		})
		envIsSource_Retries := false
		if !isSet_Retries && !envIsSource_Retries {
			log.Fatalf("Missing required flag: -retries")
		}
	}
`
	assertCodeContains(t, actualCode, expectedRetriesCheck)
	assertCodeContains(t, actualCode, "err := task.DoSomething(ConfigFileFlag, RetriesFlag)")
}

func TestGenerateMain_EnumValidation(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{Name: "SetMode", PackageName: "control"},
		Options: []*metadata.OptionMetadata{
			// Enum -> EnumValues (as []any)
			{Name: "mode", CliName: "mode", TypeName: "string", HelpText: "Mode of operation", EnumValues: []any{"auto", "manual", "standby"}},
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	expectedEnumValidation := `
	isValidChoice_ModeFlag := false
	allowedChoices_ModeFlag := []string{"auto", "manual", "standby"}
	for _, choice := range allowedChoices_ModeFlag {
		if ModeFlag == choice {
			isValidChoice_ModeFlag = true
			break
		}
	}
	if !isValidChoice_ModeFlag {
		log.Fatalf("Invalid value for -mode: %s. Allowed choices are: %s", ModeFlag, strings.Join(allowedChoices_ModeFlag, ", "))
	}
`
	assertCodeContains(t, actualCode, expectedEnumValidation)
	assertCodeContains(t, actualCode, "err := control.SetMode(ModeFlag)")
}

func TestGenerateMain_EnvironmentVariables(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{Name: "Configure", PackageName: "setup"},
		Options: []*metadata.OptionMetadata{
			{Name: "apiKey", CliName: "apiKey", TypeName: "string", HelpText: "API Key", EnvVar: "API_KEY"}, // Envvar -> EnvVar
			{Name: "timeout", CliName: "timeout", TypeName: "int", HelpText: "Timeout in seconds", DefaultValue: 60, EnvVar: "TIMEOUT_SECONDS"},
			{Name: "enableFeature", CliName: "enableFeature", TypeName: "bool", HelpText: "Enable new feature", DefaultValue: false, EnvVar: "ENABLE_MY_FEATURE"},
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	expectedApiKeyEnv := `
	if val, ok := os.LookupEnv("API_KEY"); ok {
		if ApiKeyFlag == "" { 
			ApiKeyFlag = val
		}
	}
`
	assertCodeContains(t, actualCode, expectedApiKeyEnv)

	expectedTimeoutEnv := `
	if val, ok := os.LookupEnv("TIMEOUT_SECONDS"); ok {
		if TimeoutFlag == 60 {
			if v, err := strconv.Atoi(val); err == nil {
				TimeoutFlag = v
			} else {
				log.Printf("Warning: could not parse environment variable TIMEOUT_SECONDS as int: %v", err)
			}
		}
	}
`
	assertCodeContains(t, actualCode, expectedTimeoutEnv)

	expectedEnableFeatureEnv := `
	if val, ok := os.LookupEnv("ENABLE_MY_FEATURE"); ok {
		if EnableFeatureFlag == false {
			if v, err := strconv.ParseBool(val); err == nil {
				EnableFeatureFlag = v
			} else {
				log.Printf("Warning: could not parse environment variable ENABLE_MY_FEATURE as bool: %v", err)
			}
		}
	}
`
	assertCodeContains(t, actualCode, expectedEnableFeatureEnv)
	// "strconv" import is no longer generated by GenerateMain; imports.Process handles it.
	assertCodeContains(t, actualCode, "err := setup.Configure(ApiKeyFlag, TimeoutFlag, EnableFeatureFlag)")
}

func TestGenerateMain_RunFuncInvocation(t *testing.T) {
	cmdMetaNoOpts := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{Name: "Execute", PackageName: "action"},
	}
	actualCodeNoOpts, err := codegen.GenerateMain(cmdMetaNoOpts, "")
	if err != nil {
		t.Fatalf("GenerateMain (no opts) failed: %v", err)
	}
	assertCodeContains(t, actualCodeNoOpts, "err := action.Execute()")

	cmdMetaWithOptions := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{Name: "Process", PackageName: "dataflow"},
		Options: []*metadata.OptionMetadata{
			{Name: "input", CliName: "input", TypeName: "string", HelpText: ""}, // Added CliName, HelpText for consistency
			{Name: "level", CliName: "level", TypeName: "int", HelpText: ""},    // Added CliName, HelpText
		},
	}
	actualCodeWithOptions, err := codegen.GenerateMain(cmdMetaWithOptions, "")
	if err != nil {
		t.Fatalf("GenerateMain (with opts) failed: %v", err)
	}
	assertCodeContains(t, actualCodeWithOptions, "err := dataflow.Process(InputFlag, LevelFlag)")
}

func TestGenerateMain_ErrorHandling(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{Name: "DefaultRun", PackageName: "pkg"},
	}
	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	expectedErrorHandling := `
	if err != nil {
		log.Fatal(err)
	}
`
	assertCodeContains(t, actualCode, expectedErrorHandling)
}

func TestGenerateMain_Imports(t *testing.T) {
	cmdMetaNoStrconv := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:        "MyFunc",
			PackageName: "custompkg", // This will be imported
		},
		Options: []*metadata.OptionMetadata{ // EnvVar without int/bool type, so no strconv needed
			{Name: "name", CliName: "name", TypeName: "string", EnvVar: "APP_NAME", HelpText: "app name"},
		},
	}

	actualCodeNoStrconv, err := codegen.GenerateMain(cmdMetaNoStrconv, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	// Import checks removed as GenerateMain no longer outputs them.
	// Check for usage of the package name.
	assertCodeContains(t, actualCodeNoStrconv, cmdMetaNoStrconv.RunFunc.PackageName)
	assertCodeNotContains(t, actualCodeNoStrconv, `strconv.Atoi`) // Check for actual usage of strconv

	cmdMetaWithStrconv := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:        "MyOtherFunc",
			PackageName: "custompkg",
		},
		Options: []*metadata.OptionMetadata{ // EnvVar with int type, so strconv IS needed
			{Name: "port", CliName: "port", TypeName: "int", EnvVar: "APP_PORT", HelpText: "app port"},
		},
	}
	actualCodeWithStrconv, err := codegen.GenerateMain(cmdMetaWithStrconv, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	// "strconv" import is no longer generated by GenerateMain; imports.Process handles it.
	// We can check for actual usage of strconv functions if necessary, e.g., strconv.Atoi
	assertCodeContains(t, actualCodeWithStrconv, `strconv.Atoi`)
	assertCodeContains(t, actualCodeWithStrconv, cmdMetaWithStrconv.RunFunc.PackageName) // Check for usage
}

func TestGenerateMain_RequiredIntWithEnvVar(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{Name: "SubmitData", PackageName: "submitter"},
		Options: []*metadata.OptionMetadata{
			{Name: "userId", CliName: "userId", TypeName: "int", HelpText: "User ID", IsRequired: true, EnvVar: "USER_ID", DefaultValue: 0},
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	expectedCheck := `
	if UserIdFlag == 0 {
		isSet_UserId := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "userId" { // .Name used for CLI flag name
				isSet_UserId = true
			}
		})
		envIsSource_UserId := false
		if val, ok := os.LookupEnv("USER_ID"); ok {
			if parsedVal, err := strconv.Atoi(val); err == nil && parsedVal == UserIdFlag {
				envIsSource_UserId = true
			}
		}
		if !isSet_UserId && !envIsSource_UserId {
			log.Fatalf("Missing required flag: -userId or environment variable USER_ID")
		}
	}
`
	assertCodeContains(t, actualCode, expectedCheck)
	assertCodeContains(t, actualCode, "err := submitter.SubmitData(UserIdFlag)")
	// "strconv" import is no longer generated by GenerateMain; imports.Process handles it.
}

func TestGenerateMain_StringFlagWithQuotesInDefault(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{Name: "PrintString", PackageName: "printer"},
		Options: []*metadata.OptionMetadata{
			{Name: "greeting", CliName: "greeting", TypeName: "string", HelpText: "A greeting message", DefaultValue: `hello "world"`},
		},
	}
	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}

	expectedFlagParsing := `flag.StringVar(&GreetingFlag, "greeting", "hello \"world\"", "A greeting message")`
	assertCodeContains(t, actualCode, expectedFlagParsing)
}

func TestGenerateMain_WithHelpText(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:        "RunMyTool",
			PackageName: "mytool",
		},
		Options: []*metadata.OptionMetadata{
			{Name: "input", CliName: "input", TypeName: "string", HelpText: "Input file"},
		},
	}
	helpText := "This is my custom help message.\nUsage: mytool -input <file>"

	actualCode, err := codegen.GenerateMain(cmdMeta, helpText)
	if err != nil {
		t.Fatalf("GenerateMain with help text failed: %v", err)
	}

	expectedHelpTextSnippet := fmt.Sprintf(`fmt.Fprintln(os.Stdout, %q)`, helpText)
	assertCodeContains(t, actualCode, expectedHelpTextSnippet)

	expectedArgParsingLogic := `
	// Handle -h/--help flags
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintln(os.Stdout, %q)
			os.Exit(0)
		}
	}
`
	assertCodeContains(t, actualCode, fmt.Sprintf(expectedArgParsingLogic, helpText))
	assertCodeContains(t, actualCode, "os.Exit(0)")
	assertCodeContains(t, actualCode, `flag.StringVar(&InputFlag, "input", "", "Input file")`)
	assertCodeContains(t, actualCode, "err := mytool.RunMyTool(InputFlag)")
}

func TestGenerateMain_WithEmptyHelpText(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: &metadata.RunFuncInfo{
			Name:        "AnotherTool",
			PackageName: "othertool",
		},
		Options: []*metadata.OptionMetadata{},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "")
	if err != nil {
		t.Fatalf("GenerateMain with empty help text failed: %v", err)
	}

	unexpectedHelpLogic := `
	// Handle -h/--help flags
	for _, arg := range os.Args[1:] {
`
	assertCodeNotContains(t, actualCode, unexpectedHelpLogic)

	assertCodeContains(t, actualCode, "func main() {")
	assertCodeContains(t, actualCode, "err := othertool.AnotherTool()")
	assertCodeNotContains(t, actualCode, "os.Exit(0)") // This specific os.Exit(0) from help
}
