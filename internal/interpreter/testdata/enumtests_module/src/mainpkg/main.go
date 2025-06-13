package mainpkg

import (
	"testcmdmodule/internal/goat" // Assuming this is the marker package path used in tests
	ext "testdata/enumtests_module/src/externalpkg"
)

// Scenario 1: Enum defined as a var in the same package
var SamePkgEnum = []string{"alpha", "beta", "gamma"}

// Scenario 2: Enum from an external package is referenced via ext.ExternalEnumValues

type Options struct {
	FieldSamePkg         string `json:"fieldSamePkg"`
	FieldExternalPkg     string `json:"fieldExternalPkg"`
	FieldDefaultSamePkg  string `json:"fieldDefaultSamePkg"`
	FieldDefaultExtPkg   string `json:"fieldDefaultExtPkg"`
	FieldDefaultIdent    string `json:"fieldDefaultIdent"`    // For testing goat.Default("val", SamePkgEnum)
	FieldUnresolvedIdent string `json:"fieldUnresolvedIdent"` // For testing goat.Enum(NonExistentVar)
}

func NewOptions() *Options {
	return &Options{
		FieldSamePkg:        goat.Enum(SamePkgEnum),
		FieldExternalPkg:    goat.Enum(ext.ExternalEnumValues),
		FieldDefaultSamePkg: goat.Default("defaultAlpha", goat.Enum(SamePkgEnum)),
		FieldDefaultExtPkg:  goat.Default("defaultDelta", goat.Enum(ext.ExternalEnumValues)),
		FieldDefaultIdent:   goat.Default("defaultBeta", SamePkgEnum), // Test logging for this case
		FieldUnresolvedIdent: goat.Enum(NonExistentVar), // This should log an error but not panic
	}
}

// Minimal main func to make it a runnable package if needed by loader
func main() {}

// NonExistentVar is intentionally not defined to test resolution failure.
// var NonExistentVar = []string{"fail"}
