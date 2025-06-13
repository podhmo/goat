package metadata

import "go/token"

// CommandMetadata holds all extracted information about a CLI command
// that goat needs to generate code or help messages.
type CommandMetadata struct {
	Name             string // Name of the command (e.g., from package or explicitly set)
	Description      string // Overall help description for the command (from run func doc)
	RunFunc          *RunFuncInfo
	Options          []*OptionMetadata
	MainFuncPosition *token.Position // TODO: For knowing where to replace main func content
}

// RunFuncInfo describes the target 'run' function.
type RunFuncInfo struct {
	Name                       string // Name of the run function (e.g., "run")
	PackageName                string // Package where the run function is defined
	OptionsArgName             string // Name of the options struct parameter (e.g., "opts")
	OptionsArgType             string // Type name of the options struct (e.g., "Options", "main.Options")
	OptionsArgTypeNameStripped string // Base type name of the options struct (e.g., "Options" from "*Options")
	OptionsArgIsPointer        bool   // True if OptionsArgType is a pointer
	ContextArgName             string // Name of the context.Context parameter (if present)
	ContextArgType             string // Type name of the context.Context parameter (if present)
	InitializerFunc            string // Name of the function that initializes the options struct (e.g., NewOptions)
}

// OptionMetadata holds information about a single command-line option.
type OptionMetadata struct {
	Name                   string // Original field name in the Options struct (e.g., "UserName")
	CliName                string // CLI flag name (e.g., "user-name")
	TypeName               string // Go type of the field (e.g., "string", "*int", "[]string")
	HelpText               string // Description for the option (from field comment)
	IsPointer              bool   // True if the field is a pointer type (often implies optional)
	IsRequired             bool   // True if the option must be provided
	EnvVar                 string // Environment variable name to read from (from `env` tag)
	DefaultValue           any    // Default value (from goat.Default or struct tag)
	EnumValues             []any  // Allowed enum values (from goat.Enum or struct tag)
	IsTextUnmarshaler      bool   // True if the field's type implements encoding.TextUnmarshaler
	IsTextMarshaler        bool   // True if the field's type implements encoding.TextMarshaler
	UnderlyingKindIsString bool   // True if the field type is a named type whose underlying kind is string

	// File-specific options
	FileMustExist   bool `json:"fileMustExist,omitempty"`
	FileGlobPattern bool `json:"fileGlobPattern,omitempty"`
}

// DefaultValueAsBool checks if the DefaultValue is a boolean and true.
func (om *OptionMetadata) DefaultValueAsBool() bool {
	if b, ok := om.DefaultValue.(bool); ok {
		return b
	}
	return false
}
