package codegen

import (
	"github.com/podhmo/goat/internal/metadata"
)

// OptionCodeSnippets holds different parts of generated code for an option.
// For instance, flag registration might need variable declarations before flag.Parse()
// and assignment logic after flag.Parse().
type OptionCodeSnippets struct {
	Declarations   string // e.g., temp variables for pointer flags before Parse
	Logic          string // e.g., the flag.StringVar call, env var parsing logic
	PostProcessing string // e.g., assigning temp var to actual option field after Parse
}

// OptionHandler defines the contract for all type-specific code generation handlers.
type OptionHandler interface {
	// Generates code for default value assignment when no InitializerFunc is present.
	// optionsVarName is the name of the variable holding the options struct (e.g., "options").
	GenerateDefaultValueInitializationCode(opt *metadata.OptionMetadata, optionsVarName string) OptionCodeSnippets

	// Generates code for processing an environment variable.
	// optionsVarName is the name of the options struct variable.
	// envValVarName is the name of the variable holding the string value read from the environment.
	// ctxVarName is the name of the context variable (e.g., "ctx").
	GenerateEnvVarProcessingCode(opt *metadata.OptionMetadata, optionsVarName string, envValVarName string, ctxVarName string) OptionCodeSnippets

	// Generates code for flag registration.
	// optionsVarName is the name of the options struct variable.
	// isFlagExplicitlySetMapName is the name of the map tracking explicitly set flags (e.g., "isFlagExplicitlySet").
	// globalTempVarPrefix helps ensure uniqueness of temp vars for flags (e.g., "temp").
	GenerateFlagRegistrationCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets

	// Generates code for assignments needed after flag.Parse() (primarily for pointer flags).
	// optionsVarName is the name of the options struct variable.
	// isFlagExplicitlySetMapName is the name of the map tracking explicitly set flags.
	// globalTempVarPrefix is the prefix for temporary variables used in flag registration.
	GenerateFlagPostParseAssignmentCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, globalTempVarPrefix string) OptionCodeSnippets

	// Generates code for checking if a required option is missing.
	// optionsVarName is the name of the options struct variable.
	// isFlagExplicitlySetMapName is the name of the map tracking explicitly set flags.
	// initialDefaultVarName is the name of the variable holding the initial default value of the field.
	// envWasSetVarName is the name of the boolean variable indicating if the corresponding env var was set.
	// ctxVarName is the name of the context variable.
	GenerateRequiredCheckCode(opt *metadata.OptionMetadata, optionsVarName string, isFlagExplicitlySetMapName string, initialDefaultVarName string, envWasSetVarName string, ctxVarName string) OptionCodeSnippets

	// Generates code for validating enum values.
	// optionsVarName is the name of the options struct variable.
	// ctxVarName is the name of the context variable.
	GenerateEnumValidationCode(opt *metadata.OptionMetadata, optionsVarName string, ctxVarName string) OptionCodeSnippets
}

// GetOptionHandler is a factory function that returns the appropriate OptionHandler
// based on the characteristics of the OptionMetadata.
func GetOptionHandler(opt *metadata.OptionMetadata) OptionHandler {
	if opt.IsTextUnmarshaler {
		if opt.IsPointer {
			return &TextUnmarshalerPtrHandler{}
		}
		return &TextUnmarshalerHandler{}
	}

	switch opt.TypeName {
	case "string":
		return &StringHandler{}
	case "int":
		return &IntHandler{}
	case "bool":
		return &BoolHandler{}
	case "*string":
		return &StringPtrHandler{}
	case "*int":
		return &IntPtrHandler{}
	case "*bool":
		return &BoolPtrHandler{}
	case "[]string":
		return &StringSliceHandler{}
	// Add cases for other supported types like []int, *[]string, etc. if they get specific handlers.
	// For now, any other complex type not matching above will use UnsupportedTypeHandler.
	default:
		// This default might also catch types like "net.IP" if IsTextUnmarshaler is false,
		// or custom enum types that are not string/int based and don't implement TextUnmarshaler.
		// The original main_generator.go had specific logic for some of these.
		// For the refactoring, if they don't fit the primary handlers, they become "unsupported"
		// or would need new dedicated handlers.
		return &UnsupportedTypeHandler{}
	}
}
