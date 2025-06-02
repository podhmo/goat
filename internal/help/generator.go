package help

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/podhmo/goat/internal/metadata"
)

const helpTemplate = `{{.CommandName}} - {{.CommandDescription}}

Usage:
  {{.CommandName}} [flags] {{.CommandArgsPlaceholder}}

Flags:
{{range .Options}}
  --{{.CliName}} {{.TypeIndicator}} {{.HelpText}}{{if .IsRequired}} (required){{end}}{{if .DefaultValue}} (default: {{.DefaultValue | QuoteIfString}}){{end}}{{if .EnvVar}} (env: {{.EnvVar}}){{end}}{{if .EnumValues}} (allowed: {{.EnumValues | JoinStrings ", "}}){{end}}
{{end}}
  -h, --help             Show this help message and exit
`

// FuncMap for the template
var funcMap = template.FuncMap{
	"QuoteIfString": func(v any) string {
		if s, ok := v.(string); ok {
			return fmt.Sprintf("%q", s)
		}
		return fmt.Sprintf("%v", v)
	},
	"JoinStrings": func(values []any, sep string) string {
		var s []string
		for _, v := range values {
			s = append(s, fmt.Sprintf("%v", v)) // QuoteIfString could be used here too if enums are strings
		}
		return strings.Join(s, sep)
	},
}

// GenerateHelp creates a formatted help message string from CommandMetadata.
func GenerateHelp(cmdMeta *metadata.CommandMetadata) string {
	if cmdMeta == nil {
		return "Error: Command metadata is nil."
	}

	type templateOption struct {
		CliName       string
		TypeIndicator string // e.g. "string", "int", "bool"
		HelpText      string
		IsRequired    bool
		DefaultValue  any
		EnvVar        string
		EnumValues    []any
	}

	var tplOptions []templateOption
	for _, opt := range cmdMeta.Options {
		tplOpt := templateOption{
			CliName:      opt.CliName,
			HelpText:     strings.ReplaceAll(opt.HelpText, "\n", "\n                           "), // Indent multi-line help
			IsRequired:   opt.IsRequired,
			DefaultValue: opt.DefaultValue,
			EnvVar:       opt.EnvVar,
			EnumValues:   opt.EnumValues,
		}
		// Simplify type indicator for help message
		baseType := strings.TrimPrefix(opt.TypeName, "*") // Remove pointer indicator for base type
		baseType = strings.TrimPrefix(baseType, "[]")     // Remove slice indicator
		parts := strings.Split(baseType, ".")
		tplOpt.TypeIndicator = strings.ToLower(parts[len(parts)-1]) // Show simple type like "string", "int"
		if strings.HasPrefix(opt.TypeName, "[]") {
			tplOpt.TypeIndicator += "s" // e.g. strings, ints
		}

		tplOptions = append(tplOptions, tplOpt)
	}

	// Prepare data for the template
	data := struct {
		CommandName            string
		CommandDescription     string
		CommandArgsPlaceholder string // TODO: if command takes positional args
		Options                []templateOption
	}{
		CommandName:            cmdMeta.Name,                                                 // Or a more specific CLI executable name
		CommandDescription:     strings.ReplaceAll(cmdMeta.Description, "\n", "\n         "), // Indent multi-line desc
		CommandArgsPlaceholder: "",                                                           // Placeholder for now
		Options:                tplOptions,
	}

	tmpl, err := template.New("help").Funcs(funcMap).Parse(helpTemplate)
	if err != nil {
		return fmt.Sprintf("Error parsing help template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Sprintf("Error executing help template: %v", err)
	}

	return buf.String()
}
