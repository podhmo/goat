package interpreter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/metadata"
)

func parseTestFileForInterpreter(t *testing.T, content string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	// Ensure comments are parsed if markers.go uses them, though not typical for func calls
	fileAst, err := parser.ParseFile(fset, "test.go", content, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file content: %v", err)
	}
	return fileAst
}

const goatPkgImportPath = "github.com/podhmo/goat/goat"

func TestInterpretInitializer_SimpleDefaults(t *testing.T) {
	content := `
package main
import g "github.com/podhmo/goat/goat"

type Options struct {
	Name string
	Port int
	Verbose bool
}

func NewOpts() *Options {
	return &Options{
		Name: g.Default("guest"),
		Port: g.Default(8080),
		Verbose: g.Default(true),
	}
}
`
	fileAst := parseTestFileForInterpreter(t, content)
	optionsMeta := []*metadata.OptionMetadata{
		{Name: "Name", CliName: "name", TypeName: "string"},
		{Name: "Port", CliName: "port", TypeName: "int"},
		{Name: "Verbose", CliName: "verbose", TypeName: "bool"},
	}

	err := InterpretInitializer(fileAst, "Options", "NewOpts", optionsMeta, goatPkgImportPath)
	if err != nil {
		t.Fatalf("InterpretInitializer failed: %v", err)
	}

	expectedDefaults := map[string]any{
		"Name":    "guest",
		"Port":    int64(8080), // parser reads numbers as int64 initially
		"Verbose": true,
	}

	for _, opt := range optionsMeta {
		expected, ok := expectedDefaults[opt.Name]
		if !ok {
			t.Errorf("Unexpected option %s found in results", opt.Name)
			continue
		}
		if !reflect.DeepEqual(opt.DefaultValue, expected) {
			t.Errorf("For option %s, expected default %v (type %T), got %v (type %T)",
				opt.Name, expected, expected, opt.DefaultValue, opt.DefaultValue)
		}
	}
}

func TestInterpretInitializer_EnumAndCombined(t *testing.T) {
	content := `
package main
import "github.com/podhmo/goat/goat"

type Options struct {
	Level string
	Mode  string
}

func InitOptions() *Options {
	return &Options{
		Level: goat.Default("info", goat.Enum([]string{"debug", "info", "warn", "error"})),
		Mode:  goat.Enum([]string{"fast", "slow"}),
	}
}
`
	fileAst := parseTestFileForInterpreter(t, content)
	optionsMeta := []*metadata.OptionMetadata{
		{Name: "Level", CliName: "level", TypeName: "string"},
		{Name: "Mode", CliName: "mode", TypeName: "string"},
	}

	err := InterpretInitializer(fileAst, "Options", "InitOptions", optionsMeta, goatPkgImportPath)
	if err != nil {
		t.Fatalf("InterpretInitializer failed: %v", err)
	}

	levelOpt := optionsMeta[0]
	if levelOpt.DefaultValue != "info" {
		t.Errorf("Level: Expected default 'info', got '%v'", levelOpt.DefaultValue)
	}
	expectedLevelEnum := []any{"debug", "info", "warn", "error"}
	if !reflect.DeepEqual(levelOpt.EnumValues, expectedLevelEnum) {
		t.Errorf("Level: Expected enum %v, got %v", expectedLevelEnum, levelOpt.EnumValues)
	}

	modeOpt := optionsMeta[1]
	if modeOpt.DefaultValue != nil { // Mode only has Enum, no Default value explicitly set by goat.Default
		t.Errorf("Mode: Expected no default value, got '%v'", modeOpt.DefaultValue)
	}
	expectedModeEnum := []any{"fast", "slow"}
	if !reflect.DeepEqual(modeOpt.EnumValues, expectedModeEnum) {
		t.Errorf("Mode: Expected enum %v, got %v", expectedModeEnum, modeOpt.EnumValues)
	}
}

func TestInterpretInitializer_AssignmentStyle(t *testing.T) {
	content := `
package main
import customgoat "github.com/podhmo/goat/goat"

type Options struct {
	Path string
}

func New() *Options {
	opts := &Options{}
	opts.Path = customgoat.Default("/tmp")
	return opts
}
`
	fileAst := parseTestFileForInterpreter(t, content)
	optionsMeta := []*metadata.OptionMetadata{
		{Name: "Path", CliName: "path", TypeName: "string"},
	}

	err := InterpretInitializer(fileAst, "Options", "New", optionsMeta, goatPkgImportPath)
	if err != nil {
		t.Fatalf("InterpretInitializer with assignment style failed: %v", err)
	}

	if optionsMeta[0].DefaultValue != "/tmp" {
		t.Errorf("Path: Expected default '/tmp', got '%v'", optionsMeta[0].DefaultValue)
	}
}

func TestInterpretInitializer_NonGoatPackageCall(t *testing.T) {
	content := `
package main
import g "github.com/some/other/pkg"

type Options struct { Name string }
func New() *Options {
	// This call should be ignored by the interpreter if markerPkgImportPath is specific
	return &Options{ Name: g.Default("ignored") }
}
`
	fileAst := parseTestFileForInterpreter(t, content)
	optionsMeta := []*metadata.OptionMetadata{{Name: "Name"}}

	err := InterpretInitializer(fileAst, "Options", "New", optionsMeta, goatPkgImportPath) // goatPkgImportPath is for "github.com/podhmo/goat/goat"
	if err != nil {
		t.Fatalf("InterpretInitializer failed: %v", err)
	}
	if optionsMeta[0].DefaultValue != nil {
		t.Errorf("Expected DefaultValue to be nil as g.Default is not from goat package, got %v", optionsMeta[0].DefaultValue)
	}
}

func TestInterpretInitializer_InitializerNotFound(t *testing.T) {
	content := `package main; type Options struct{}`
	fileAst := parseTestFileForInterpreter(t, content)
	err := InterpretInitializer(fileAst, "Options", "NonExistentInit", nil, goatPkgImportPath)
	if err == nil {
		t.Fatal("InterpretInitializer should fail if initializer func not found")
	}
	if !strings.Contains(err.Error(), "NonExistentInit' not found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestInterpretInitializer_FileMarkers(t *testing.T) {
	tests := []struct {
		name              string
		content           string
		optionsName       string
		initializerName   string
		initialOptMeta    []*metadata.OptionMetadata // Input metadata to InterpretInitializer
		expectedOptMeta   []*metadata.OptionMetadata // Expected metadata after InterpretInitializer
		expectError       bool
		expectedErrorMsg  string
	}{
		{
			name: "File with default path only",
			content: `
package main
import g "github.com/podhmo/goat/goat"
type Config struct { InputFile string }
func NewConfig() *Config { return &Config{ InputFile: g.File("/path/to/default.txt") } }`,
			optionsName:     "Config",
			initializerName: "NewConfig",
			initialOptMeta:  []*metadata.OptionMetadata{{Name: "InputFile"}},
			expectedOptMeta: []*metadata.OptionMetadata{
				{Name: "InputFile", DefaultValue: "/path/to/default.txt", TypeName: "string", FileMustExist: false, FileGlobPattern: false},
			},
		},
		{
			name: "File with MustExist",
			content: `
package main
import g "github.com/podhmo/goat/goat"
type Config struct { DataFile string }
func NewConfig() *Config { return &Config{ DataFile: g.File("data.csv", g.MustExist()) } }`,
			optionsName:     "Config",
			initializerName: "NewConfig",
			initialOptMeta:  []*metadata.OptionMetadata{{Name: "DataFile"}},
			expectedOptMeta: []*metadata.OptionMetadata{
				{Name: "DataFile", DefaultValue: "data.csv", TypeName: "string", FileMustExist: true, FileGlobPattern: false},
			},
		},
		{
			name: "File with GlobPattern",
			content: `
package main
import g "github.com/podhmo/goat/goat"
type Config struct { Pattern string }
func NewConfig() *Config { return &Config{ Pattern: g.File("*.json", g.GlobPattern()) } }`,
			optionsName:     "Config",
			initializerName: "NewConfig",
			initialOptMeta:  []*metadata.OptionMetadata{{Name: "Pattern"}},
			expectedOptMeta: []*metadata.OptionMetadata{
				{Name: "Pattern", DefaultValue: "*.json", TypeName: "string", FileMustExist: false, FileGlobPattern: true},
			},
		},
		{
			name: "File with MustExist and GlobPattern",
			content: `
package main
import g "github.com/podhmo/goat/goat"
type Config struct { AssetsDir string }
func NewConfig() *Config { return &Config{ AssetsDir: g.File("./assets", g.MustExist(), g.GlobPattern()) } }`,
			optionsName:     "Config",
			initializerName: "NewConfig",
			initialOptMeta:  []*metadata.OptionMetadata{{Name: "AssetsDir"}},
			expectedOptMeta: []*metadata.OptionMetadata{
				{Name: "AssetsDir", DefaultValue: "./assets", TypeName: "string", FileMustExist: true, FileGlobPattern: true},
			},
		},
		{
			name: "File with assignment style",
			content: `
package main
import goat "github.com/podhmo/goat/goat"
type Settings struct { ConfigFile string }
func LoadSettings() *Settings {
	s := &Settings{}
	s.ConfigFile = goat.File("config.yaml", goat.MustExist())
	return s
}`,
			optionsName:     "Settings",
			initializerName: "LoadSettings",
			initialOptMeta:  []*metadata.OptionMetadata{{Name: "ConfigFile"}},
			expectedOptMeta: []*metadata.OptionMetadata{
				{Name: "ConfigFile", DefaultValue: "config.yaml", TypeName: "string", FileMustExist: true, FileGlobPattern: false},
			},
		},
		{
			name: "File with option from different package",
			content: `
package main
import g "github.com/podhmo/goat/goat"
import other "github.com/some/other/pkg"
type Config struct { Input string }
func New() *Config { return &Config{ Input: g.File("in.txt", other.SomeOption()) } }`, // other.SomeOption() should be ignored
			optionsName:     "Config",
			initializerName: "New",
			initialOptMeta:  []*metadata.OptionMetadata{{Name: "Input"}},
			expectedOptMeta: []*metadata.OptionMetadata{
				// Only DefaultValue from g.File should be processed. FileMustExist/FileGlobPattern should remain false.
				{Name: "Input", DefaultValue: "in.txt", TypeName: "string", FileMustExist: false, FileGlobPattern: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileAst := parseTestFileForInterpreter(t, tt.content)
			// Make a deep copy of initialOptMeta for each test run if it can be modified by InterpretInitializer
			currentOptionsMeta := make([]*metadata.OptionMetadata, len(tt.initialOptMeta))
			for i, opt := range tt.initialOptMeta {
				// Shallow copy is enough if fields of OptionMetadata are value types or not modified deeply
				// If opt itself or its fields (like slices/maps if any were complex) were modified, a deep copy would be needed
				metaCopy := *opt
				currentOptionsMeta[i] = &metaCopy
			}


			err := InterpretInitializer(fileAst, tt.optionsName, tt.initializerName, currentOptionsMeta, goatPkgImportPath)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected an error but got none")
				}
				if tt.expectedErrorMsg != "" && !strings.Contains(err.Error(), tt.expectedErrorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedErrorMsg, err.Error())
				}
				return // Test ends here if an error is expected and occurred
			}
			if err != nil {
				t.Fatalf("InterpretInitializer failed: %v", err)
			}

			if len(currentOptionsMeta) != len(tt.expectedOptMeta) {
				t.Fatalf("Expected %d option metadata, got %d", len(tt.expectedOptMeta), len(currentOptionsMeta))
			}

			for i, expectedOpt := range tt.expectedOptMeta {
				actualOpt := currentOptionsMeta[i]
				// Fill in CliName if not specified in expected, as it's auto-generated if empty
				// For these tests, we are focused on interpreter part, not analyzer's CliName generation.
				// However, to make DeepEqual work, all fields must match.
				if expectedOpt.CliName == "" {
					expectedOpt.CliName = actualOpt.CliName
				}


				if !reflect.DeepEqual(actualOpt, expectedOpt) {
					t.Errorf("OptionMetadata mismatch for '%s':\nExpected: %+v (type %T)\nActual:   %+v (type %T)",
						expectedOpt.Name, expectedOpt, expectedOpt, actualOpt, actualOpt)
					// Detailed field comparison for debugging
					if actualOpt.Name != expectedOpt.Name {t.Logf(" Name: expected '%s', got '%s'", expectedOpt.Name, actualOpt.Name)}
					if actualOpt.DefaultValue != expectedOpt.DefaultValue {t.Logf(" DefaultValue: expected '%v', got '%v'", expectedOpt.DefaultValue, actualOpt.DefaultValue)}
					if actualOpt.TypeName != expectedOpt.TypeName {t.Logf(" TypeName: expected '%s', got '%s'", expectedOpt.TypeName, actualOpt.TypeName)}
					if actualOpt.FileMustExist != expectedOpt.FileMustExist {t.Logf(" FileMustExist: expected '%t', got '%t'", expectedOpt.FileMustExist, actualOpt.FileMustExist)}
					if actualOpt.FileGlobPattern != expectedOpt.FileGlobPattern {t.Logf(" FileGlobPattern: expected '%t', got '%t'", expectedOpt.FileGlobPattern, actualOpt.FileGlobPattern)}
				}
			}
		})
	}
}
