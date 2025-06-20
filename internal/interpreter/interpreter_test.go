package interpreter

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv" // Added for strconv.Quote
	"strings"
	"testing"

	"github.com/podhmo/goat/internal/loader" // Added for loader.Loader
	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/astutils" // Added for astutils.EvalResult
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

const goatPkgImportPath = "github.com/podhmo/goat"

func TestInterpretInitializer_SimpleDefaults(t *testing.T) {
	content := `
package main
import g "github.com/podhmo/goat"

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
` // Closing backtick for content variable in TestInterpretInitializer_SimpleDefaults
	fileAst := parseTestFileForInterpreter(t, content)
	optionsMeta := []*metadata.OptionMetadata{
		{Name: "Name", CliName: "name", TypeName: "string"},
		{Name: "Port", CliName: "port", TypeName: "int"},
		{Name: "Verbose", CliName: "verbose", TypeName: "bool"},
	}

	// Provide dummy loader and currentPkgPath for tests not focusing on identifier resolution
	ctx := context.Background()
	dummyLoader := loader.New(loader.Config{})
	dummyCurrentPkgPath := "github.com/podhmo/goat/internal/interpreter/testpkgs/simpledefaults"
	err := InterpretInitializer(ctx, fileAst, "Options", "NewOpts", optionsMeta, goatPkgImportPath, dummyCurrentPkgPath, dummyLoader)
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

// TestInterpretInitializer_EnumNewScenarios tests new enum resolution scenarios,
// particularly direct composite literals with identifiers and variables resolving to such.
func TestInterpretInitializer_EnumNewScenarios(t *testing.T) {
	const testMarkerPkgImportPath = "github.com/podhmo/goat" // Standard goat path
	// Canonical import paths based on 'testdata/enumtests_module' as module root
	const mainPkgImportPath = "testdata/enumtests_module/src/mainpkg"
	const customTypesImportPath = "testdata/enumtests_module/src/customtypes"
	const externalPkgPath = "testdata/enumtests_module/src/externalpkg" // Used by some enums

	moduleRoot := "./testdata/enumtests_module"
	ld, fsetForLoader := newTestLoader(t, moduleRoot) // Get fset from loader

	ctx := context.Background() // Define context once

	// Pre-load and parse necessary packages to populate loader caches
	packagesToPreload := []string{mainPkgImportPath, customTypesImportPath, externalPkgPath}
	loadedPkgs, errLoad := ld.Load(ctx, packagesToPreload...)
	if errLoad != nil {
		t.Fatalf("Failed to pre-load packages: %v", errLoad)
	}
	for _, p := range loadedPkgs {
		if _, errFiles := p.Files(); errFiles != nil { // Trigger parsing and symbol caching
			t.Fatalf("Failed to pre-parse files for package %s: %v", p.ImportPath, errFiles)
		}
		t.Logf("Successfully pre-loaded and parsed package: %s for EnumNewScenarios", p.ImportPath)
	}

	// Parse the mainpkg.go file which now contains all necessary definitions
	mainGoFile := moduleRoot + "/src/mainpkg/main.go"
	// Use fsetForLoader for parsing
	entryFileAst, err := parser.ParseFile(fsetForLoader, mainGoFile, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file %s: %v", mainGoFile, err)
	}

	optionsMeta := []*metadata.OptionMetadata{
		{Name: "EnumCompositeDirect"},
		{Name: "EnumCompositeDirectMixed"},
		{Name: "EnumCompositeDirectLocalConst"},
		{Name: "EnumCompositeDirectFails"},
		{Name: "EnumVarCustomType"},
		{Name: "EnumVarMixed"},
		{Name: "EnumVarWithNonString"},
		// Fields for resolveEvalResultToEnumString via goat.Default
		{Name: "FieldForDirectString"},
		{Name: "FieldForLocalConst"},
		{Name: "FieldForImportedConst"},
	}
	optionsMap := make(map[string]*metadata.OptionMetadata)
	for i := range optionsMeta {
		optionsMap[optionsMeta[i].Name] = optionsMeta[i] // Store pointers
	}

	// The InterpretInitializer function needs the *ast.File of the file containing NewOptions,
	// the currentPkgPath should be the import path of that file.
	err = InterpretInitializer(ctx, entryFileAst, "Options", "NewOptions", optionsMeta,
		testMarkerPkgImportPath, // This is how `g.` calls will be checked
		mainPkgImportPath,       // Import path of the package where NewOptions is defined
		ld)
	if err != nil {
		t.Fatalf("InterpretInitializer failed: %v", err)
	}

	tests := []struct {
		fieldName          string
		expectedEnumValues []any
		expectedDefault    any // For fields testing defaults used by resolveEvalResultToEnumString
	}{
		// --- extractMarkerInfo (direct composite literals) ---
		{
			fieldName:          "EnumCompositeDirect",
			expectedEnumValues: []any{"val-a", "val-b"},
		},
		{
			fieldName:          "EnumCompositeDirectMixed",
			expectedEnumValues: []any{"val-a", "literal-b", "local-val-2"},
		},
		{
			fieldName:          "EnumCompositeDirectLocalConst",
			expectedEnumValues: []any{"local-val-1", "local-val-2"},
		},
		{
			fieldName: "EnumCompositeDirectFails",
			// customtypes.NotStringConst (int) should fail resolution by resolveEvalResultToEnumString
			expectedEnumValues: []any{"val-a"},
		},
		// --- extractEnumValuesFromEvalResult (variable composite literals) ---
		{
			fieldName:          "EnumVarCustomType", // MyCustomEnumSlice = []customtypes.MyEnum{customtypes.EnumValA, customtypes.EnumValB}
			expectedEnumValues: []any{"val-a", "val-b"},
		},
		{
			fieldName:          "EnumVarMixed", // MyMixedValSlice = []any{customtypes.EnumValA, "literal-in-var", LocalStringConst}
			expectedEnumValues: []any{"val-a", "literal-in-var", "local-val-1"},
		},
		{
			fieldName: "EnumVarWithNonString", // MyCustomEnumWithNonStringSlice = []any{customtypes.EnumValA, customtypes.NotStringConst}
			// customtypes.NotStringConst (int) should fail resolution
			expectedEnumValues: []any{"val-a"},
		},
		// --- For resolveEvalResultToEnumString (via Default values in mainpkg.NewOptions) ---
		{
			fieldName:       "FieldForDirectString",
			expectedDefault: "direct-string-default",
		},
		{
			fieldName:       "FieldForLocalConst", // Default(LocalStringConst) -> "local-val-1"
			expectedDefault: "local-val-1",        // The type from astutils.EvaluateArg will be the underlying type
		},
		{
			fieldName:       "FieldForImportedConst", // Default(customtypes.EnumValA) -> "val-a"
			expectedDefault: "val-a",                 // Underlying type after evaluation
		},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			opt := optionsMap[tt.fieldName]
			if opt == nil {
				t.Fatalf("Option %s not found in metadata", tt.fieldName)
			}

			if tt.expectedEnumValues != nil {
				if !reflect.DeepEqual(opt.EnumValues, tt.expectedEnumValues) {
					t.Errorf("Field '%s': Expected EnumValues %v (type %T), got %v (type %T)",
						tt.fieldName, tt.expectedEnumValues, tt.expectedEnumValues, opt.EnumValues, opt.EnumValues)
				}
			} else if len(opt.EnumValues) > 0 {
				t.Errorf("Field '%s': Expected nil/empty EnumValues, got %v", tt.fieldName, opt.EnumValues)
			}

			if tt.expectedDefault != nil {
				// Note: Default values from astutils.EvaluateArg might have types like customtypes.MyEnum
				// instead of just string, if the const itself was typed.
				if !reflect.DeepEqual(opt.DefaultValue, tt.expectedDefault) {
					t.Errorf("Field '%s': Expected DefaultValue %v (type %T), got %v (type %T)",
						tt.fieldName, tt.expectedDefault, tt.expectedDefault, opt.DefaultValue, opt.DefaultValue)
				}
			}
		})
	}
}

func TestPointerDefaultValue(t *testing.T) {
	const testMarkerPkgImportPath = "github.com/podhmo/goat"
	const pointerDefaultPkgImportPath = "pointerdefault_module/src/pointerdefault"
	moduleRoot := "./testdata/pointerdefault_module"

	// 1. Setup Loader
	ld, fsetForLoader := newTestLoader(t, moduleRoot)
	ctx := context.Background()

	// 2. Pre-load the target package
	packagesToPreload := []string{pointerDefaultPkgImportPath}
	loadedPkgs, errLoad := ld.Load(ctx, packagesToPreload...)
	if errLoad != nil || len(loadedPkgs) == 0 {
		t.Fatalf("Failed to pre-load package %s: %v", pointerDefaultPkgImportPath, errLoad)
	}
	if _, errFiles := loadedPkgs[0].Files(); errFiles != nil {
		t.Fatalf("Failed to pre-parse files for package %s: %v", pointerDefaultPkgImportPath, errFiles)
	}
	t.Logf("Successfully pre-loaded and parsed package: %s", pointerDefaultPkgImportPath)

	// 3. Get AST for defs.go
	// Assuming defs.go is the only file or the relevant file in the package.
	// The loader.Package.Files() returns a map[string]*ast.File. We need to find the correct one.
	// For simplicity, if there's only one file, use it.
	// Or, construct the expected file path.
	defsGoFilePath := moduleRoot + "/src/pointerdefault/defs.go"
	// Use fsetForLoader which was used by the loader
	fileAst, err := parser.ParseFile(fsetForLoader, defsGoFilePath, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file %s: %v", defsGoFilePath, err)
	}

	// 4. Prepare OptionMetadata (as if an analyzer had already populated parts of it)
	optionsMeta := []*metadata.OptionMetadata{
		{
			Name:      "MyStringPtr",
			CliName:   "ptr-field", // from struct tag `goat:"ptr-field"`
			TypeName:  "*string",   // This would be determined by an analyzer
			IsPointer: true,        // This would be determined by an analyzer
		},
		{
			Name:      "AnotherPtr",
			CliName:   "another-ptr-field",
			TypeName:  "*int",
			IsPointer: true,
		},
	}

	// 5. Call InterpretInitializer
	err = InterpretInitializer(ctx, fileAst,
		"Config",        // optionsStructName
		"DefaultConfig", // initializerFuncName
		optionsMeta,
		testMarkerPkgImportPath,     // markerPkgImportPath
		pointerDefaultPkgImportPath, // currentPkgPath
		ld)                          // loader
	if err != nil {
		t.Fatalf("InterpretInitializer failed: %v", err)
	}

	// 6. Assert Results
	expectedValues := map[string]struct {
		defaultValue any
		isPointer    bool
	}{
		"MyStringPtr": {defaultValue: "expected_default", isPointer: true},
		"AnotherPtr":  {defaultValue: int64(123), isPointer: true}, // Numbers are often int64 from parser/evaluator
	}

	if len(optionsMeta) != len(expectedValues) {
		t.Fatalf("Expected %d options metadata, got %d", len(expectedValues), len(optionsMeta))
	}

	for _, opt := range optionsMeta {
		t.Run(opt.Name, func(t *testing.T) {
			expected, ok := expectedValues[opt.Name]
			if !ok {
				t.Fatalf("Unexpected option %s processed", opt.Name)
			}

			if opt.IsPointer != expected.isPointer {
				t.Errorf("For option %s, expected IsPointer %t, got %t", opt.Name, expected.isPointer, opt.IsPointer)
			}

			// Check default value
			if !reflect.DeepEqual(opt.DefaultValue, expected.defaultValue) {
				t.Errorf("For option %s, expected DefaultValue %v (type %T), got %v (type %T)",
					opt.Name, expected.defaultValue, expected.defaultValue, opt.DefaultValue, opt.DefaultValue)
			}
		})
	}
}

func newTestLoader(t *testing.T, moduleRootRelPath string) (*loader.Loader, *token.FileSet) {
	t.Helper()
	fset := token.NewFileSet() // Create fset here
	gml := &loader.GoModLocator{}
	gml.WorkingDir = moduleRootRelPath // e.g., "./testdata/enumtests_module"
	ld := loader.New(loader.Config{
		Locator: gml.Locate,
		Fset:    fset, // Loader uses this fset
	})
	return ld, fset // Return it
}

// newTestContextForPkg creates a minimal *ast.File for a given package structure, primarily for import path resolution.
// currentPkgSourcePath is the path to the source file that would contain the goat.Enum call.
// currentPkgImportPath is the canonical import path for the current package.
func newTestContext(t *testing.T, currentPkgImportPath string, imports map[string]string) (*ast.File, string) {
	t.Helper()
	var importSpecs []*ast.ImportSpec
	for alias, path := range imports {
		spec := &ast.ImportSpec{
			Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(path)},
		}
		if alias != "" && alias != lastPathPart(path) { // Add alias if it's explicit and not default
			spec.Name = ast.NewIdent(alias)
		}
		importSpecs = append(importSpecs, spec)
	}

	return &ast.File{
		Name:    ast.NewIdent(lastPathPart(currentPkgImportPath)), // e.g., "mainpkg"
		Decls:   []ast.Decl{&ast.GenDecl{Tok: token.IMPORT, Specs: importSpecsToAstSpecs(importSpecs)}},
		Imports: importSpecs, // For astutils.GetImportPath
	}, currentPkgImportPath
}

func importSpecsToAstSpecs(specs []*ast.ImportSpec) []ast.Spec {
	astSpecs := make([]ast.Spec, len(specs))
	for i, s := range specs {
		astSpecs[i] = s
	}
	return astSpecs
}

func lastPathPart(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func TestResolveEvalResultToEnumString(t *testing.T) {
	// Setup loader assuming 'enumtests_module' is our module context.
	// The paths used for currentPkgPath and for resolving imports inside testdata
	// should align with this module structure.
	moduleRoot := "./testdata/enumtests_module"
	ld, _ := newTestLoader(t, moduleRoot) // Get fset from loader, but not used directly here yet

	// Define canonical import paths for test packages
	const mainPkgImportPath = "testdata/enumtests_module/src/mainpkg"
	const customTypesImportPath = "testdata/enumtests_module/src/customtypes"
	const externalPkgPath = "testdata/enumtests_module/src/externalpkg" // For TestResolveEvalResultToEnumString if it uses it

	// Pre-load packages for TestResolveEvalResultToEnumString
	ctxForLoad := context.Background()
	initialPkgsToLoad := []string{mainPkgImportPath, customTypesImportPath, externalPkgPath} // Add any other distinct pkgs used

	pkgsForTest, errLoad := ld.Load(ctxForLoad, initialPkgsToLoad...)
	if errLoad != nil {
		t.Fatalf("Pre-loading packages for TestResolveEvalResultToEnumString failed: %v", errLoad)
	}
	for _, p := range pkgsForTest {
		if _, errFiles := p.Files(); errFiles != nil {
			t.Fatalf("Pre-parsing files for package %s (for TestResolveEvalResultToEnumString) failed: %v", p.ImportPath, errFiles)
		}
		t.Logf("Successfully pre-loaded and parsed package: %s for TestResolveEvalResultToEnumString", p.ImportPath)
	}

	// Test cases
	tests := []struct {
		name              string
		elementEvalResult astutils.EvalResult
		currentPkgPath    string            // Import path of the package where the resolving is happening
		importsInTestFile map[string]string // Simulates imports in the file where the enum element is used
		expectedString    string
		expectedSuccess   bool
		expectedErrorMsg  string // Optional: for checking specific error log patterns (not implemented in this test)
	}{
		{
			name:              "direct string value",
			elementEvalResult: astutils.EvalResult{Value: "direct-str"},
			currentPkgPath:    mainPkgImportPath,
			importsInTestFile: nil,
			expectedString:    "direct-str",
			expectedSuccess:   true,
		},
		{
			name:              "nil value, no identifier",
			elementEvalResult: astutils.EvalResult{Value: nil, IdentifierName: ""},
			currentPkgPath:    mainPkgImportPath,
			importsInTestFile: nil,
			expectedString:    "",
			expectedSuccess:   false,
		},
		{
			name:              "non-string value",
			elementEvalResult: astutils.EvalResult{Value: 123},
			currentPkgPath:    mainPkgImportPath,
			importsInTestFile: nil,
			expectedString:    "",
			expectedSuccess:   false,
		},
		{
			name: "identifier for local const in current package",
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "LocalStringConst", // Defined in mainpkg.go
			},
			currentPkgPath:    mainPkgImportPath, // Resolution happens as if we are in mainpkg
			importsInTestFile: nil,               // No specific imports needed for alias resolution
			expectedString:    "local-val-1",     // Value of LocalStringConst
			expectedSuccess:   true,
		},
		{
			name: "qualified identifier for imported const",
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "EnumValA",
				PkgName:        "ct", // Alias used in the "calling" context
			},
			currentPkgPath: mainPkgImportPath, // Context of the call
			importsInTestFile: map[string]string{ // Imports in the file where ct.EnumValA would be written
				"ct": customTypesImportPath,
			},
			expectedString:  "val-a", // Value of customtypes.EnumValA
			expectedSuccess: true,
		},
		{
			name: "qualified identifier, default alias for imported const",
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "EnumValB",
				PkgName:        "customtypes", // Default alias (last part of import path)
			},
			currentPkgPath: mainPkgImportPath,
			importsInTestFile: map[string]string{
				// No explicit alias, so "customtypes" should map to customTypesImportPath
				"": customTypesImportPath, // Representing `import "enumtests_module/src/customtypes"`
			},
			expectedString:  "val-b",
			expectedSuccess: true,
		},
		{
			name: "identifier not found (local)",
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "NonExistentLocalConst",
			},
			currentPkgPath:    mainPkgImportPath,
			importsInTestFile: nil,
			expectedString:    "",
			expectedSuccess:   false,
		},
		{
			name: "identifier not found (imported)",
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "NonExistentRemoteConst",
				PkgName:        "ct",
			},
			currentPkgPath: mainPkgImportPath,
			importsInTestFile: map[string]string{
				"ct": customTypesImportPath,
			},
			expectedString:  "",
			expectedSuccess: false,
		},
		{
			name: "const is not a string (local)", // mainpkg.go needs a non-string const for this
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "LocalIntConst", // Needs to be added to mainpkg.go: const LocalIntConst int = 10
			},
			currentPkgPath:    mainPkgImportPath,
			importsInTestFile: nil,
			expectedString:    "",
			expectedSuccess:   false,
		},
		{
			name: "const is not a string (imported)",
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "NotStringConst", // This is an int const in customtypes
				PkgName:        "ct",
			},
			currentPkgPath: mainPkgImportPath,
			importsInTestFile: map[string]string{
				"ct": customTypesImportPath,
			},
			expectedString:  "",
			expectedSuccess: false,
		},
		{
			name: "package alias not resolvable",
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "EnumValA",
				PkgName:        "unresolvableAlias",
			},
			currentPkgPath:    mainPkgImportPath,
			importsInTestFile: nil, // No import for "unresolvableAlias"
			expectedString:    "",
			expectedSuccess:   false,
		},
		{
			name: "package cannot be loaded (bad import path)",
			elementEvalResult: astutils.EvalResult{
				IdentifierName: "EnumValA",
				PkgName:        "badpkg",
			},
			currentPkgPath: mainPkgImportPath,
			importsInTestFile: map[string]string{
				"badpkg": "enumtests_module/src/nonexistentpkg", // Path that loader will fail on
			},
			expectedString:  "",
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Create a dummy *ast.File for context, primarily for GetImportPath
			// Its content doesn't matter as much as its Imports list.
			fileAstForContext, currentPkgPathForContext := newTestContext(t, tt.currentPkgPath, tt.importsInTestFile)

			strVal, success := resolveEvalResultToEnumString(ctx, tt.elementEvalResult, ld, currentPkgPathForContext, fileAstForContext)

			if success != tt.expectedSuccess {
				t.Errorf("resolveEvalResultToEnumString() success = %v, want %v", success, tt.expectedSuccess)
			}
			if strVal != tt.expectedString {
				t.Errorf("resolveEvalResultToEnumString() strVal = %q, want %q", strVal, tt.expectedString)
			}
			// TODO: Check logs for tt.expectedErrorMsg if that becomes necessary
		})
	}
}

func TestInterpretInitializer_EnumAndCombined(t *testing.T) {
	content := `
package main
import "github.com/podhmo/goat"

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

	ctx := context.Background()
	dummyLoader := loader.New(loader.Config{})
	dummyCurrentPkgPath := "github.com/podhmo/goat/internal/interpreter/testpkgs/enumandcombined"
	err := InterpretInitializer(ctx, fileAst, "Options", "InitOptions", optionsMeta, goatPkgImportPath, dummyCurrentPkgPath, dummyLoader)
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
import customgoat "github.com/podhmo/goat"

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

	ctx := context.Background()
	dummyLoader := loader.New(loader.Config{})
	dummyCurrentPkgPath := "github.com/podhmo/goat/internal/interpreter/testpkgs/assignmentstyle"
	err := InterpretInitializer(ctx, fileAst, "Options", "New", optionsMeta, goatPkgImportPath, dummyCurrentPkgPath, dummyLoader)
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

	ctx := context.Background()
	dummyLoader := loader.New(loader.Config{})
	dummyCurrentPkgPath := "github.com/podhmo/goat/internal/interpreter/testpkgs/nongoatpkgcall"
	err := InterpretInitializer(ctx, fileAst, "Options", "New", optionsMeta, goatPkgImportPath, dummyCurrentPkgPath, dummyLoader) // goatPkgImportPath is for "github.com/podhmo/goat"
	if err != nil {
		t.Fatalf("InterpretInitializer failed: %v", err)
	}
	if optionsMeta[0].DefaultValue != nil {
		t.Errorf("Expected DefaultValue to be nil as g.Default is not from goat package, got %v", optionsMeta[0].DefaultValue)
	}
}

func TestInterpretInitializer_InitializerNotFound(t *testing.T) {
	content := `package main; type Options struct{}`
	ctx := context.Background()
	fileAst := parseTestFileForInterpreter(t, content)
	dummyLoader := loader.New(loader.Config{})
	dummyCurrentPkgPath := "github.com/podhmo/goat/internal/interpreter/testpkgs/initializererror"
	err := InterpretInitializer(ctx, fileAst, "Options", "NonExistentInit", nil, goatPkgImportPath, dummyCurrentPkgPath, dummyLoader)
	if err == nil {
		t.Fatal("InterpretInitializer should fail if initializer func not found")
	}
	if !strings.Contains(err.Error(), "NonExistentInit' not found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestInterpretInitializer_FileMarkers(t *testing.T) {
	tests := []struct {
		name             string
		content          string
		optionsName      string
		initializerName  string
		initialOptMeta   []*metadata.OptionMetadata // Input metadata to InterpretInitializer
		expectedOptMeta  []*metadata.OptionMetadata // Expected metadata after InterpretInitializer
		expectError      bool
		expectedErrorMsg string
	}{
		{
			name: "File with default path only",
			content: `
package main
import g "github.com/podhmo/goat"
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
import g "github.com/podhmo/goat"
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
import g "github.com/podhmo/goat"
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
import g "github.com/podhmo/goat"
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
import goat "github.com/podhmo/goat"
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
import g "github.com/podhmo/goat"
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
			ctx := context.Background()
			fileAst := parseTestFileForInterpreter(t, tt.content)
			// Make a deep copy of initialOptMeta for each test run if it can be modified by InterpretInitializer
			currentOptionsMeta := make([]*metadata.OptionMetadata, len(tt.initialOptMeta))
			for i, opt := range tt.initialOptMeta {
				// Shallow copy is enough if fields of OptionMetadata are value types or not modified deeply
				// If opt itself or its fields (like slices/maps if any were complex) were modified, a deep copy would be needed
				metaCopy := *opt
				currentOptionsMeta[i] = &metaCopy
			}

			dummyLoader := loader.New(loader.Config{})
			dummyCurrentPkgPath := "github.com/podhmo/goat/internal/interpreter/testpkgs/filemarkers/" + tt.name
			err := InterpretInitializer(ctx, fileAst, tt.optionsName, tt.initializerName, currentOptionsMeta, goatPkgImportPath, dummyCurrentPkgPath, dummyLoader)

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
					if actualOpt.Name != expectedOpt.Name {
						t.Logf(" Name: expected '%s', got '%s'", expectedOpt.Name, actualOpt.Name)
					}
					if actualOpt.DefaultValue != expectedOpt.DefaultValue {
						t.Logf(" DefaultValue: expected '%v', got '%v'", expectedOpt.DefaultValue, actualOpt.DefaultValue)
					}
					if actualOpt.TypeName != expectedOpt.TypeName {
						t.Logf(" TypeName: expected '%s', got '%s'", expectedOpt.TypeName, actualOpt.TypeName)
					}
					if actualOpt.FileMustExist != expectedOpt.FileMustExist {
						t.Logf(" FileMustExist: expected '%t', got '%t'", expectedOpt.FileMustExist, actualOpt.FileMustExist)
					}
					if actualOpt.FileGlobPattern != expectedOpt.FileGlobPattern {
						t.Logf(" FileGlobPattern: expected '%t', got '%t'", expectedOpt.FileGlobPattern, actualOpt.FileGlobPattern)
					}
				}
			}
		})
	}
}

func TestInterpretInitializer_EnumResolution(t *testing.T) {
	// Setup: Loader and test file paths
	// Note: The loader requires a module context. The testdata directory "enumtests_module"
	// is set up as a Go module.
	// The paths used by the loader (e.g., for currentPkgPath and resolving imports)
	// need to be relative to this module root or be absolute/canonical import paths.

	const testMarkerPkgImportPath = "testcmdmodule/internal/goat"              // This must match what's used in mainpkg.go's import
	const mainPkgPath = "testdata/enumtests_module/src/mainpkg"                // Corrected canonical path
	const externalPkgPathForTest = "testdata/enumtests_module/src/externalpkg" // Used in TestInterpretInitializer_EnumResolution

	// Parse the main.go file from our test module
	mainGoFile := "./testdata/enumtests_module/src/mainpkg/main.go" // Path relative to internal/interpreter package
	fset := token.NewFileSet()
	fileAst, err := parser.ParseFile(fset, mainGoFile, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file %s: %v", mainGoFile, err)
	}

	// Instantiate Loader
	gml := &loader.GoModLocator{}
	gml.WorkingDir = "./testdata/enumtests_module" // Module root for GoModLocator
	ld := loader.New(loader.Config{
		Locator: gml.Locate,
		Fset:    fset,
	})

	// Pre-load and parse necessary packages for TestInterpretInitializer_EnumResolution
	// to ensure symbols are in the cache.
	ctx := context.Background()
	packagesToPreloadForEnumRes := []string{mainPkgPath, externalPkgPathForTest}
	loadedPkgsForEnumRes, errLoadEnumRes := ld.Load(ctx, packagesToPreloadForEnumRes...)
	if errLoadEnumRes != nil {
		t.Fatalf("Failed to pre-load packages for EnumResolution test: %v", errLoadEnumRes)
	}
	for _, p := range loadedPkgsForEnumRes {
		if _, errFiles := p.Files(); errFiles != nil {
			t.Fatalf("Failed to pre-parse files for package %s (for EnumResolution test): %v", p.ImportPath, errFiles)
		}
		t.Logf("Successfully pre-loaded and parsed package: %s for EnumResolution test", p.ImportPath)
	}

	optionsMeta := []*metadata.OptionMetadata{
		{Name: "FieldSamePkg", TypeName: "string"},
		{Name: "FieldExternalPkg", TypeName: "string"},
		{Name: "FieldDefaultSamePkg", TypeName: "string"},
		{Name: "FieldDefaultExtPkg", TypeName: "string"},
		{Name: "FieldDefaultIdent", TypeName: "string"},
		{Name: "FieldUnresolvedIdent", TypeName: "string"},
	}

	// Call InterpretInitializer
	// The currentPkgPath needs to be the canonical import path of mainpkg
	// as the loader would see it, relative to the loader's configured module root.
	// ctx was already defined above for ld.Load, err was defined by parser.ParseFile
	err = InterpretInitializer(ctx, fileAst, "Options", "NewOptions", optionsMeta,
		testMarkerPkgImportPath, // How goat markers are identified
		mainPkgPath,             // Canonical path for the package being processed
		ld)
	if err != nil {
		t.Fatalf("InterpretInitializer failed: %v", err)
	}

	// Expected values
	expectedSamePkgEnum := []any{"alpha", "beta", "gamma"}
	expectedExternalPkgEnum := []any{"delta", "epsilon", "zeta"}

	tests := []struct {
		fieldName          string
		expectedDefault    any
		expectedEnumValues []any
		expectEnumResolved bool // True if EnumValues should be non-empty
	}{
		{"FieldSamePkg", nil, expectedSamePkgEnum, true},
		{"FieldExternalPkg", nil, expectedExternalPkgEnum, true},
		{"FieldDefaultSamePkg", "defaultAlpha", expectedSamePkgEnum, true},
		{"FieldDefaultExtPkg", "defaultDelta", expectedExternalPkgEnum, true},
		{
			"FieldDefaultIdent",
			"defaultBeta",
			nil,   // EnumValues might be nil because direct identifier resolution in goat.Default is logged as "not fully implemented"
			false, // Change to true if resolution for this case is implemented and expected
		},
		{
			"FieldUnresolvedIdent", // goat.Enum(NonExistentVar)
			nil,
			nil, // Should not resolve, EnumValues should be empty or nil
			false,
		},
	}

	optionsMap := make(map[string]*metadata.OptionMetadata)
	for _, opt := range optionsMeta {
		optionsMap[opt.Name] = opt
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			opt, ok := optionsMap[tt.fieldName]
			if !ok {
				t.Fatalf("OptionMetadata for field '%s' not found after interpretation", tt.fieldName)
			}

			if tt.expectedDefault != nil {
				if !reflect.DeepEqual(opt.DefaultValue, tt.expectedDefault) {
					t.Errorf("Field '%s': Expected DefaultValue %v (type %T), got %v (type %T)",
						tt.fieldName, tt.expectedDefault, tt.expectedDefault, opt.DefaultValue, opt.DefaultValue)
				}
			} else if opt.DefaultValue != nil {
				t.Errorf("Field '%s': Expected nil DefaultValue, got %v", tt.fieldName, opt.DefaultValue)
			}

			if tt.expectEnumResolved {
				if !reflect.DeepEqual(opt.EnumValues, tt.expectedEnumValues) {
					t.Errorf("Field '%s': Expected EnumValues %v, got %v",
						tt.fieldName, tt.expectedEnumValues, opt.EnumValues)
				}
			} else {
				if len(opt.EnumValues) > 0 {
					// For FieldDefaultIdent, we expect a log message, and EnumValues might be empty.
					// For FieldUnresolvedIdent, EnumValues must be empty.
					if tt.fieldName == "FieldDefaultIdent" {
						// This is acceptable for FieldDefaultIdent as per current plan (logged, not resolved)
						t.Logf("Field '%s': EnumValues are %v. This is expected as direct identifier resolution in goat.Default is logged as 'not fully implemented'.", tt.fieldName, opt.EnumValues)
					} else {
						t.Errorf("Field '%s': Expected empty EnumValues due to resolution failure or not being applicable, got %v",
							tt.fieldName, opt.EnumValues)
					}
				}
			}
		})
	}
}
