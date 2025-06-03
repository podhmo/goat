package codegen_test

import (
	"fmt"
	"go/format"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/codegen"
	"github.com/podhmo/goat/internal/metadata"
)

// normalizeCode formats the code string using go/format and removes leading/trailing whitespace.
// This helps in making comparisons less brittle.
func normalizeCode(t *testing.T, code string) string {
	t.Helper()
	formatted, err := format.Source([]byte(code))
	if err != nil {
		t.Fatalf("Failed to format code: %v\nOriginal code:\n%s", err, code)
	}
	return strings.TrimSpace(string(formatted))
}

func assertCodeContains(t *testing.T, actualCode, expectedSnippet string) {
	t.Helper()
	// Normalize the snippet too, so we are comparing formatted code with formatted code.
	normalizedExpectedSnippet := normalizeCode(t, expectedSnippet)
	if !strings.Contains(actualCode, normalizedExpectedSnippet) {
		// For easier debugging, show the normalized expected snippet
		// and the (already normalized) actual code.
		t.Errorf("Expected generated code to contain:\n>>>>>>>>>>\n%s\n<<<<<<<<<<\n\nActual code:\n>>>>>>>>>>\n%s\n<<<<<<<<<<",
			normalizedExpectedSnippet, actualCode)
	}
}

func assertCodeNotContains(t *testing.T, actualCode, unexpectedSnippet string) {
	t.Helper()
	// Normalize the snippet to ensure we're not failing due to formatting.
	normalizedUnexpectedSnippet := normalizeCode(t, unexpectedSnippet)
	if strings.Contains(actualCode, normalizedUnexpectedSnippet) {
		t.Errorf("Expected generated code NOT to contain:\n>>>>>>>>>>\n%s\n<<<<<<<<<<\n\nActual code:\n>>>>>>>>>>\n%s\n<<<<<<<<<<",
			normalizedUnexpectedSnippet, actualCode)
	}
}


func TestGenerateMain_BasicCase(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{
			Name:        "Run",
			PackageName: "mycmd",
			Imports:     []string{"github.com/example/mycmd"},
		},
		Options: []metadata.Option{},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Corrected line
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	expectedImports := `
import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/example/mycmd"
)
`
	assertCodeContains(t, normalizedActualCode, expectedImports)
	assertCodeContains(t, normalizedActualCode, "func main() {")
	assertCodeContains(t, normalizedActualCode, "err := mycmd.Run()")
	assertCodeContains(t, normalizedActualCode, "if err != nil {")
	assertCodeContains(t, normalizedActualCode, "log.Fatal(err)")
	assertCodeNotContains(t, normalizedActualCode, "type Options struct")
}

func TestGenerateMain_WithOptions(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{
			Name:        "RunWithOptions",
			PackageName: "anothercmd",
			Imports:     []string{"github.com/example/anothercmd"},
		},
		Options: []metadata.Option{
			{Name: "name", Type: "string", Description: "Name of the user", Default: "guest"},
			{Name: "age", Type: "int", Description: "Age of the user", Default: "30"},
			{Name: "verbose", Type: "bool", Description: "Enable verbose output", Default: "false"},
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Added ""
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	assertCodeNotContains(t, normalizedActualCode, "type Options struct")

	expectedVarDeclarations := `
	var NameFlag string
	var AgeFlag int
	var VerboseFlag bool
`
	assertCodeContains(t, normalizedActualCode, expectedVarDeclarations)

	expectedFlagParsing := `
	flag.StringVar(&NameFlag, "name", "guest", "Name of the user")
	flag.IntVar(&AgeFlag, "age", 30, "Age of the user")
	flag.BoolVar(&VerboseFlag, "verbose", false, "Enable verbose output")
	flag.Parse()
`
	assertCodeContains(t, normalizedActualCode, expectedFlagParsing)
	assertCodeContains(t, normalizedActualCode, "err := anothercmd.RunWithOptions(NameFlag, AgeFlag, VerboseFlag)")
}

func TestGenerateMain_RequiredFlags(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{Name: "DoSomething", PackageName: "task"},
		Options: []metadata.Option{
			{Name: "configFile", Type: "string", Description: "Path to config file", Required: true},
			{Name: "retries", Type: "int", Description: "Number of retries", Required: true, Default: "0"},
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Added ""
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	expectedConfigFileCheck := `
	if ConfigFileFlag == "" {
		log.Fatalf("Missing required flag: -configFile")
	}
`
	assertCodeContains(t, normalizedActualCode, expectedConfigFileCheck)

	expectedRetriesCheck := `
	if RetriesFlag == 0 {
		isSet_Retries := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "retries" {
				isSet_Retries = true
			}
		})
		envIsSource_Retries := false
		if !isSet_Retries && !envIsSource_Retries {
			log.Fatalf("Missing required flag: -retries")
		}
	}
`
	assertCodeContains(t, normalizedActualCode, expectedRetriesCheck)
	assertCodeContains(t, normalizedActualCode, "err := task.DoSomething(ConfigFileFlag, RetriesFlag)")
}

func TestGenerateMain_EnumValidation(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{Name: "SetMode", PackageName: "control"},
		Options: []metadata.Option{
			{Name: "mode", Type: "string", Description: "Mode of operation", Enum: []string{"auto", "manual", "standby"}},
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Added ""
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

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
	assertCodeContains(t, normalizedActualCode, expectedEnumValidation)
	assertCodeContains(t, normalizedActualCode, "err := control.SetMode(ModeFlag)")
}

func TestGenerateMain_EnvironmentVariables(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{Name: "Configure", PackageName: "setup"},
		Options: []metadata.Option{
			{Name: "apiKey", Type: "string", Description: "API Key", Envvar: "API_KEY"}, 
			{Name: "timeout", Type: "int", Description: "Timeout in seconds", Default: "60", Envvar: "TIMEOUT_SECONDS"},
			{Name: "enableFeature", Type: "bool", Description: "Enable new feature", Default: "false", Envvar: "ENABLE_MY_FEATURE"},
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Added ""
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	expectedApiKeyEnv := `
	if val, ok := os.LookupEnv("API_KEY"); ok {
		if ApiKeyFlag == "" { 
			ApiKeyFlag = val
		}
	}
`
	assertCodeContains(t, normalizedActualCode, expectedApiKeyEnv)

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
	assertCodeContains(t, normalizedActualCode, expectedTimeoutEnv)

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
	assertCodeContains(t, normalizedActualCode, expectedEnableFeatureEnv)
	assertCodeContains(t, normalizedActualCode, `import ("strconv")`)
	assertCodeContains(t, normalizedActualCode, "err := setup.Configure(ApiKeyFlag, TimeoutFlag, EnableFeatureFlag)")
}


func TestGenerateMain_RunFuncInvocation(t *testing.T) {
	// Case 1: No options
	cmdMetaNoOpts := &metadata.CommandMetadata{
		RunFunc: metadata.Func{Name: "Execute", PackageName: "action"},
	}
	actualCodeNoOpts, err := codegen.GenerateMain(cmdMetaNoOpts, "") // Already correct from previous partial apply
	if err != nil {
		t.Fatalf("GenerateMain (no opts) failed: %v", err)
	}
	normalizedActualCodeNoOpts := normalizeCode(t, actualCodeNoOpts)
	assertCodeContains(t, normalizedActualCodeNoOpts, "err := action.Execute()")

	// Case 2: With options
	cmdMetaWithOptions := &metadata.CommandMetadata{
		RunFunc: metadata.Func{Name: "Process", PackageName: "dataflow"},
		Options: []metadata.Option{
			{Name: "input", Type: "string"},
			{Name: "level", Type: "int"},
		},
	}
	actualCodeWithOptions, err := codegen.GenerateMain(cmdMetaWithOptions, "") // Already correct
	if err != nil {
		t.Fatalf("GenerateMain (with opts) failed: %v", err)
	}
	normalizedActualCodeWithOptions := normalizeCode(t, actualCodeWithOptions)
	assertCodeContains(t, normalizedActualCodeWithOptions, "err := dataflow.Process(InputFlag, LevelFlag)")
}

func TestGenerateMain_ErrorHandling(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{Name: "DefaultRun", PackageName: "pkg"},
	}
	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Already correct
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	expectedErrorHandling := `
	if err != nil {
		log.Fatal(err)
	}
`
	assertCodeContains(t, normalizedActualCode, expectedErrorHandling)
}

func TestGenerateMain_Imports(t *testing.T) {
	cmdMetaNoStrconv := &metadata.CommandMetadata{
		RunFunc: metadata.Func{
			Name:        "MyFunc",
			PackageName: "custompkg",
			Imports:     []string{"github.com/custom/lib1", "github.com/another/lib2"},
		},
		Options: []metadata.Option{
			{Name: "name", Type: "string", Envvar: "APP_NAME"}, 
		},
	}

	actualCodeNoStrconv, err := codegen.GenerateMain(cmdMetaNoStrconv, "") // Already correct
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedCodeNoStrconv := normalizeCode(t, actualCodeNoStrconv)

	standardImports := []string{`"flag"`, `"fmt"`, `"log"`, `"os"`, `"strings"`}
	for _, imp := range standardImports {
		assertCodeContains(t, normalizedCodeNoStrconv, imp)
	}
	customImports := []string{`"github.com/custom/lib1"`, `"github.com/another/lib2"`}
	for _, imp := range customImports {
		assertCodeContains(t, normalizedCodeNoStrconv, imp)
	}
	assertCodeNotContains(t, normalizedCodeNoStrconv, `"strconv"`) 

	cmdMetaWithStrconv := &metadata.CommandMetadata{
		RunFunc: metadata.Func{
			Name:        "MyOtherFunc",
			PackageName: "custompkg",
			Imports:     []string{"github.com/custom/lib1"}, 
		},
		Options: []metadata.Option{
			{Name: "port", Type: "int", Envvar: "APP_PORT"}, 
		},
	}
	actualCodeWithStrconv, err := codegen.GenerateMain(cmdMetaWithStrconv, "") // Already correct
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedCodeWithStrconv := normalizeCode(t, actualCodeWithStrconv)
	assertCodeContains(t, normalizedCodeWithStrconv, `"strconv"`) 

	cmdMetaWithUserStrconv := &metadata.CommandMetadata{
		RunFunc: metadata.Func{
			Name:        "MyOtherFunc",
			PackageName: "custompkg",
			Imports:     []string{"github.com/custom/lib1", "strconv"}, 
		},
		Options: []metadata.Option{
			{Name: "port", Type: "int", Envvar: "APP_PORT"}, 
		},
	}
	actualCodeWithUserStrconv, err := codegen.GenerateMain(cmdMetaWithUserStrconv, "") // Already correct
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedCodeWithUserStrconv := normalizeCode(t, actualCodeWithUserStrconv)
	assertCodeContains(t, normalizedCodeWithUserStrconv, `"strconv"`)
}


func TestGenerateMain_RequiredIntWithEnvVar(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{Name: "SubmitData", PackageName: "submitter"},
		Options: []metadata.Option{
			{Name: "userId", Type: "int", Description: "User ID", Required: true, Envvar: "USER_ID"}, 
		},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Already correct
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	expectedCheck := `
	if UserIdFlag == 0 {
		isSet_UserId := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "userId" {
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
	assertCodeContains(t, normalizedActualCode, expectedCheck)
	assertCodeContains(t, normalizedActualCode, "err := submitter.SubmitData(UserIdFlag)")
	assertCodeContains(t, normalizedActualCode, `import ("strconv")`) 
}

func TestGenerateMain_StringFlagWithQuotesInDefault(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{Name: "PrintString", PackageName: "printer"},
		Options: []metadata.Option{
			{Name: "greeting", Type: "string", Description: "A greeting message", Default: `hello "world"`},
		},
	}
	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Already correct
	if err != nil {
		t.Fatalf("GenerateMain failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	expectedFlagParsing := `flag.StringVar(&GreetingFlag, "greeting", "hello \"world\"", "A greeting message")`
	assertCodeContains(t, normalizedActualCode, expectedFlagParsing)
}

func TestGenerateMain_WithHelpText(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{
			Name:        "RunMyTool",
			PackageName: "mytool",
		},
		Options: []metadata.Option{
			{Name: "input", Type: "string", Description: "Input file"},
		},
	}
	helpText := "This is my custom help message.\nUsage: mytool -input <file>"

	actualCode, err := codegen.GenerateMain(cmdMeta, helpText)
	if err != nil {
		t.Fatalf("GenerateMain with help text failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	// Check for the help text itself, properly quoted for the template
	expectedHelpTextSnippet := fmt.Sprintf(`fmt.Fprintln(os.Stdout, %q)`, helpText)
	assertCodeContains(t, normalizedActualCode, expectedHelpTextSnippet)

	// Check for the argument parsing logic
	expectedArgParsingLogic := `
	// Handle -h/--help flags
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintln(os.Stdout, %q)
			os.Exit(0)
		}
	}
`
	assertCodeContains(t, normalizedActualCode, fmt.Sprintf(expectedArgParsingLogic, helpText))
	assertCodeContains(t, normalizedActualCode, "os.Exit(0)")
	assertCodeContains(t, normalizedActualCode, "flag.StringVar(&InputFlag, \"input\", \"\", \"Input file\")")
	assertCodeContains(t, normalizedActualCode, "err := mytool.RunMyTool(InputFlag)")
}

func TestGenerateMain_WithEmptyHelpText(t *testing.T) {
	cmdMeta := &metadata.CommandMetadata{
		RunFunc: metadata.Func{
			Name:        "AnotherTool",
			PackageName: "othertool",
		},
		Options: []metadata.Option{},
	}

	actualCode, err := codegen.GenerateMain(cmdMeta, "") // Empty help text
	if err != nil {
		t.Fatalf("GenerateMain with empty help text failed: %v", err)
	}
	normalizedActualCode := normalizeCode(t, actualCode)

	// Assert that the help text block is NOT present
	// This is a bit tricky as we need a snippet that's unique to that block
	// and wouldn't appear elsewhere. The loop and os.Args check is a good candidate.
	unexpectedHelpLogic := `
	// Handle -h/--help flags
	for _, arg := range os.Args[1:] {
`
	assertCodeNotContains(t, normalizedActualCode, unexpectedHelpLogic)

	// Ensure standard parts are still there
	assertCodeContains(t, normalizedActualCode, "func main() {")
	assertCodeContains(t, normalizedActualCode, "err := othertool.AnotherTool()")
	assertCodeNotContains(t, normalizedActualCode, "os.Exit(0)") // This specific os.Exit(0) from help
}
