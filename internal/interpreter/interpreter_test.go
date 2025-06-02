package interpreter

import (
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
import "github.com/podhmo/goat/goat" // Direct import

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

	// Check Level
	levelOpt := optionsMeta[0]
	if levelOpt.DefaultValue != "info" {
		t.Errorf("Level: Expected default 'info', got '%v'", levelOpt.DefaultValue)
	}
	expectedLevelEnum := []any{"debug", "info", "warn", "error"}
	if !reflect.DeepEqual(levelOpt.EnumValues, expectedLevelEnum) {
		t.Errorf("Level: Expected enum %v, got %v", expectedLevelEnum, levelOpt.EnumValues)
	}

	// Check Mode
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
import g "github.com/some/other/pkg" // Different package

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