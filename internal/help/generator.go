package help

import (
	"fmt"
	"io"
	"strings"

	"github.com/podhmo/goat/internal/metadata"
)

// GenerateHelp writes a formatted help message to the given io.Writer from CommandMetadata.
func GenerateHelp(cmdMeta *metadata.CommandMetadata) string {
	if cmdMeta == nil {
		return "<error>" // Handle nil case gracefully
	}

	var sb strings.Builder
	generateHelp(&sb, cmdMeta)
	return sb.String()
}

func generateHelp(w io.Writer, cmdMeta *metadata.CommandMetadata) {
	fmt.Fprintf(w, "%s - %s\n\n", cmdMeta.Name, strings.ReplaceAll(cmdMeta.Description, "\n", "\n         "))
	fmt.Fprintf(w, "Usage:\n  %s [flags]\n\n", cmdMeta.Name) // Removed CommandArgsPlaceholder and trailing space
	fmt.Fprintln(w, "Flags:")

	// Find max length of option names for alignment (include -h, --help)
	maxNameLen := len("h, --help") // Length of "h, --help"
	for _, opt := range cmdMeta.Options {
		currentCliName := opt.CliName
		// Check if the flag is a boolean, required, and defaults to true
		if opt.TypeName == "bool" && opt.IsRequired && opt.DefaultValueAsBool() {
			currentCliName = "no-" + opt.CliName
		}
		if l := len(currentCliName); l > maxNameLen {
			maxNameLen = l
		}
	}

	for _, opt := range cmdMeta.Options {
		baseType := strings.TrimPrefix(opt.TypeName, "*")
		baseType = strings.TrimPrefix(baseType, "[]")
		parts := strings.Split(baseType, ".")
		typeIndicator := strings.ToLower(parts[len(parts)-1])
		if strings.HasPrefix(opt.TypeName, "[]") {
			typeIndicator += "s"
		}

		displayName := opt.CliName
		// Check if the flag is a boolean, required, and defaults to true for display
		if opt.TypeName == "bool" && opt.IsRequired && opt.DefaultValueAsBool() {
			displayName = "no-" + opt.CliName
		}

		helpText := strings.ReplaceAll(opt.HelpText, "\n", "\n"+strings.Repeat(" ", maxNameLen+15))
		fmt.Fprintf(w, "  --%-*s %s %s", maxNameLen, displayName, typeIndicator, helpText)
		if opt.IsRequired && !(opt.TypeName == "bool" || opt.TypeName == "*bool") {
			fmt.Fprint(w, " (required)")
		}
		if opt.DefaultValue != nil && opt.DefaultValue != "" {
			if s, ok := opt.DefaultValue.(string); ok {
				fmt.Fprintf(w, " (default: %q)", s)
			} else {
				fmt.Fprintf(w, " (default: %v)", opt.DefaultValue)
			}
		}
		if opt.EnvVar != "" {
			fmt.Fprintf(w, " (env: %s)", opt.EnvVar)
		}
		if len(opt.EnumValues) > 0 {
			var enumStrs []string
			for _, v := range opt.EnumValues {
				if s, ok := v.(string); ok {
					enumStrs = append(enumStrs, fmt.Sprintf("%q", s))
				} else {
					enumStrs = append(enumStrs, fmt.Sprintf("%v", v))
				}
			}
			fmt.Fprintf(w, " (allowed: %s)", strings.Join(enumStrs, ", "))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "")
	helpName := "h, --help"
	helpText := "Show this help message and exit"
	fmt.Fprintf(w, "  -%-*s %s\n", maxNameLen, helpName, helpText)
}
