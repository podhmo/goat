package mainpkg

import (
	"testcmdmodule/internal/goat" // Assuming this is the marker package path used in tests
	ext "testdata/enumtests_module/src/externalpkg"
)

// Scenario 1: Enum defined as a var in the same package
var SamePkgEnum = []string{"alpha", "beta", "gamma"}

// Scenario 2: Enum from an external package is referenced via ext.ExternalEnumValues

// Local constants for testing resolution within the same package
type MyLocalEnum string
const LocalStringConst MyLocalEnum = "local-val-1"
const LocalStringConst2 MyLocalEnum = "local-val-2"
const LocalIntConst int = 10


type Options struct {
	// Existing fields for TestInterpretInitializer_EnumResolution
	FieldSamePkg         string `json:"fieldSamePkg"`
	FieldExternalPkg     string `json:"fieldExternalPkg"`
	FieldDefaultSamePkg  string `json:"fieldDefaultSamePkg"`
	FieldDefaultExtPkg   string `json:"fieldDefaultExtPkg"`
	FieldDefaultIdent    string `json:"fieldDefaultIdent"`
	FieldUnresolvedIdent string `json:"fieldUnresolvedIdent"`

	// --- New fields for new tests ---
	// --- For resolveEvalResultToEnumString (indirectly via default) ---
	FieldForDirectString         string
	FieldForLocalConst           string
	FieldForImportedConst        string
	FieldForNonStringConst       string
	FieldForNonExistentConst     string
	FieldForUnresolvablePkgConst string

	// --- For extractMarkerInfo (direct composite literals) ---
	EnumCompositeDirect           string
	EnumCompositeDirectMixed      string
	EnumCompositeDirectLocalConst string
	EnumCompositeDirectFails      string

	// --- For extractEnumValuesFromEvalResult (variable composite literals) ---
	EnumVarCustomType      string
	EnumVarMixed           string
	EnumVarWithNonString   string
}

// Variables for testing enums resolved from variables (new tests)
// Note: "enumtests_module/src/customtypes" is the import path for customtypes package
// Ensure customtypes.MyEnum is defined as `type MyEnum string` for this to work easily.
var MyCustomEnumSlice = []customtypes.MyEnum{customtypes.EnumValA, customtypes.EnumValB}
var MyMixedValSlice = []any{customtypes.EnumValA, "literal-in-var", LocalStringConst} // customtypes.EnumValA needs to be string-compatible
var MyCustomEnumWithNonStringSlice = []any{customtypes.EnumValA, customtypes.NotStringConst}


func NewOptions() *Options {
	// Marker package alias 'goat' should point to "testcmdmodule/internal/goat" as per existing test.
	// New test for resolveEvalResultToEnumString uses 'g' as "github.com/podhmo/goat".
	// For InterpretInitializer tests, the markerPkgImportPath param matters.
	// The import "testcmdmodule/internal/goat" is aliased to `goat` in this file.

	return &Options{
		// Existing fields
		FieldSamePkg:        goat.Enum(SamePkgEnum),
		FieldExternalPkg:    goat.Enum(ext.ExternalEnumValues),
		FieldDefaultSamePkg: goat.Default("defaultAlpha", goat.Enum(SamePkgEnum)),
		FieldDefaultExtPkg:  goat.Default("defaultDelta", goat.Enum(ext.ExternalEnumValues)),
		FieldDefaultIdent:   goat.Default("defaultBeta", SamePkgEnum),
		FieldUnresolvedIdent: goat.Enum(NonExistentVar),

		// --- New initializations for new test fields ---
		FieldForDirectString:  goat.Default("direct-string-default"), // For resolveEvalResultToEnumString test for direct value
		FieldForLocalConst:    goat.Default(LocalStringConst),   // For resolveEvalResultToEnumString test for local const
		FieldForImportedConst: goat.Default(customtypes.EnumValA), // For resolveEvalResultToEnumString test for imported const

		EnumCompositeDirect:           goat.Enum(nil, []customtypes.MyEnum{customtypes.EnumValA, customtypes.EnumValB}),
		EnumCompositeDirectMixed:      goat.Enum(nil, []any{customtypes.EnumValA, "literal-b", LocalStringConst2}),
		EnumCompositeDirectLocalConst: goat.Enum(nil, []MyLocalEnum{LocalStringConst, LocalStringConst2}),
		EnumCompositeDirectFails:      goat.Enum(nil, []any{customtypes.EnumValA, customtypes.NotStringConst}),

		EnumVarCustomType:    goat.Enum(MyCustomEnumSlice),
		EnumVarMixed:         goat.Enum(MyMixedValSlice),
		EnumVarWithNonString: goat.Enum(MyCustomEnumWithNonStringSlice),
	}
}

// Minimal main func to make it a runnable package if needed by loader
func main() {}

// NonExistentVar is intentionally not defined to test resolution failure.
// var NonExistentVar = []string{"fail"}
