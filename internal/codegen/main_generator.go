package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/stringutils"
)

// formatHelpText formats the help text string for inclusion in the generated Go code.
// It handles escaped newlines (\\n) and placeholder single quotes (') for backticks (`).
// It then chooses the best Go string literal representation.
func formatHelpText(text string) string {
	// Initial transformations
	// 1. Replace literal "\\n" with actual newline character '\n'.
	processedText := strings.ReplaceAll(text, "\\n", "\n")
	// 2. Replace placeholder single quote "'" with actual backtick '`'.
	processedText = strings.ReplaceAll(processedText, "'", "`")

	hasNewlines := strings.Contains(processedText, "\n")
	hasBackticks := strings.Contains(processedText, "`")

	if hasNewlines && hasBackticks {
		// Case 1: String contains both newlines and backticks.
		// Must be represented as a concatenation of raw string literals and quoted backticks.
		// Example: "line1\n`code`\nline3" becomes "`line1\n` + "`" + `code` + "`" + `\nline3`"
		var sb strings.Builder
		sb.WriteString("`") // Start the first raw string segment
		last := 0
		for i, r := range processedText {
			if r == '`' {
				sb.WriteString(processedText[last:i]) // Write content before the backtick
				sb.WriteString("`")                   // Close current raw string segment
				sb.WriteString(" + \"`\" + ")         // Concatenate a quoted backtick
				sb.WriteString("`")                   // Start a new raw string segment
				last = i + 1
			}
		}
		sb.WriteString(processedText[last:]) // Write the remaining content after the last backtick
		sb.WriteString("`")                  // Close the final raw string segment
		return sb.String()

	} else if hasNewlines {
		// Case 2: String contains newlines but no backticks.
		// Safe to use a single raw string literal.
		return "`" + processedText + "`"
	} else {
		// Case 3: String contains no newlines. It might contain backticks, or it might not.
		// `fmt.Sprintf("%q", ...)` handles this correctly.
		// It will produce a standard quoted string, escaping backticks (e.g., as `)
		// and other necessary characters.
		return fmt.Sprintf("%q", processedText)
	}
}

// GenerateMain creates the Go code string for the new main() function
// based on the extracted command metadata.
// If generateFullFile is true, it returns a complete Go file content including package and imports.
// Otherwise, it returns only the main function body.
func GenerateMain(cmdMeta *metadata.CommandMetadata, helpText string, generateFullFile bool) (string, error) {
	templateFuncs := template.FuncMap{
		"KebabCase":      stringutils.ToKebabCase,
		"FormatHelpText": formatHelpText, // Add this line
	}

	tmpl := template.Must(template.New("main").Funcs(templateFuncs).Parse(`
func main() {
	{{if .HelpText}}
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, {{FormatHelpText .HelpText}})
	}
	{{end}}

	{{if .HasOptions}}
	var options = &{{.RunFunc.OptionsArgTypeNameStripped}}{}
	{{range .Options}}
	{{if eq .TypeName "string"}}
	flag.StringVar(&options.{{.Name}}, "{{ KebabCase .Name }}", {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}}, {{FormatHelpText .HelpText}}   {{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{else if eq .TypeName "int"}}
	flag.IntVar(&options.{{.Name}}, "{{ KebabCase .Name }}", {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}}, {{FormatHelpText .HelpText}}{{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{else if eq .TypeName "bool"}}
	{{if and .IsRequired (ne .DefaultValue nil) (eq .DefaultValue true)}}
	var {{.Name}}_NoFlagIsPresent bool
	flag.BoolVar(&{{.Name}}_NoFlagIsPresent, "no-{{ KebabCase .Name }}", false, {{FormatHelpText .HelpText}})
	{{else}}
	flag.BoolVar(&options.{{.Name}}, "{{ KebabCase .Name }}", {{if ne .DefaultValue nil}}{{.DefaultValue}}{{else}}false{{end}}, {{FormatHelpText .HelpText}}{{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{end}}
	{{else if eq .TypeName "*string"}}
	flag.StringVar(options.{{.Name}}, "{{ KebabCase .Name }}", {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}}, {{FormatHelpText .HelpText}}   {{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{else if eq .TypeName "*int"}}
	flag.IntVar(options.{{.Name}}, "{{ KebabCase .Name }}", {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}}, {{FormatHelpText .HelpText}}{{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{else if eq .TypeName "*bool"}}
	flag.BoolVar(options.{{.Name}}, "{{ KebabCase .Name }}", {{if ne .DefaultValue nil}}{{.DefaultValue}}{{else}}false{{end}}, {{FormatHelpText .HelpText}}{{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{end}}
	{{end}}
	{{end}}

	flag.Parse()

	{{if .HasOptions}}
	{{range .Options}}
	{{if eq .TypeName "bool"}}
	{{if and .IsRequired (ne .DefaultValue nil) (eq .DefaultValue true)}}
	options.{{.Name}} = true // Default to true
	if {{.Name}}_NoFlagIsPresent { // If --no-{{KebabCase .Name}} was present
		options.{{.Name}} = false
	}
	{{end}}
	{{end}}
	{{if .EnvVar}}
	if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		// If flag was set, it takes precedence. Only use env if flag is still its zero value.
		// This check is tricky for bools where false is a valid value AND the default.
		// And for numbers where 0 is a valid value AND the default.
		// A more robust way might involve checking if the flag was explicitly set using flag.Visit().
		// For now, if default is zero-value, env var will override if set.
		// If default is non-zero, flag value (even if it's the default) takes precedence.
		{{if eq .TypeName "string"}}
		if options.{{.Name}} == {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}} { // only override if flag is still at default
			options.{{.Name}} = val
		}
		{{else if eq .TypeName "int"}}
		if options.{{.Name}} == {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}} {
			if v, err := strconv.Atoi(val); err == nil {
				options.{{.Name}} = v
			} else {
				slog.Warn("Could not parse environment variable as int", "envVar", "{{.EnvVar}}", "value", val, "error", err)
			}
		}
		{{else if eq .TypeName "bool"}}
		// For bools, if the default is false, and the env var is "true", we set it.
		// If the default is true, we only change if env var is explicitly "false".
		// This avoids overriding a true default with a missing or invalid env var.
		if defaultValForBool_{{.Name}} := {{if .DefaultValue}}{{.DefaultValue}}{{else}}false{{end}}; !defaultValForBool_{{.Name}} {
			{{if not .DefaultValue}} // Only generate this block if DefaultValue is actually false
			if v, err := strconv.ParseBool(val); err == nil && v { // Only set to true if default is false
				options.{{.Name}} = true
			} else if err != nil {
				slog.Warn("Could not parse environment variable as bool", "envVar", "{{.EnvVar}}", "value", val, "error", err)
			}
			{{end}}
		} else { // Default is true
			if v, err := strconv.ParseBool(val); err == nil && !v { // Only set to false if default is true and env is "false"
				options.{{.Name}} = false
			} else if err != nil && val != "" { // Don't warn if env var is just not set for a true default
				slog.Warn("Could not parse environment variable as bool", "envVar", "{{.EnvVar}}", "value", val, "error", err)
			}
		}
		{{end}}
	}
	{{end}}

	{{if .IsRequired}}
	{{if eq .TypeName "string"}}
	if options.{{.Name}} == "" {
		slog.Error("Missing required flag", "flag", "{{ KebabCase .Name }}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}})
		os.Exit(1)
	}
	{{else if eq .TypeName "int"}}
	// Check if the int flag was explicitly set or came from an env var,
	// especially if its value is the same as the default.
	// This is important if the default is 0, which is also the zero value for int.
	isSetOrFromEnv_{{.Name}} := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "{{ KebabCase .Name }}" {
			isSetOrFromEnv_{{.Name}} = true
		}
	})
	{{if .EnvVar}}
	if !isSetOrFromEnv_{{.Name}} {
		if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
			// Check if env var could have been the source
			if parsedVal, err := strconv.Atoi(val); err == nil && parsedVal == options.{{.Name}} {
				isSetOrFromEnv_{{.Name}} = true
			}
		}
	}
	{{end}}
	if !isSetOrFromEnv_{{.Name}} && options.{{.Name}} == {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}} {
		// If it wasn't set via flag or a matching env var, and it's still the default value,
		// then it's considered missing.
		slog.Error("Missing required flag", "flag", "{{ KebabCase .Name }}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}})
		os.Exit(1)
	}
	{{end}}

	{{end}}

	{{if .EnumValues}}
	isValidChoice_{{.Name}} := false
	allowedChoices_{{.Name}} := []string{ {{range $i, $e := .EnumValues}}{{if $i}}, {{end}}{{printf "%q" $e}}{{end}} }
	currentValue_{{.Name}}Str := fmt.Sprintf("%v", options.{{.Name}})
	for _, choice := range allowedChoices_{{.Name}} {
		if currentValue_{{.Name}}Str == choice {
			isValidChoice_{{.Name}} = true
			break
		}
	}
	if !isValidChoice_{{.Name}} {
		slog.Error("Invalid value for flag", "flag", "{{ KebabCase .Name }}", "value", options.{{.Name}}, "allowedChoices", strings.Join(allowedChoices_{{.Name}}, ", "))
		os.Exit(1)
	}
	{{end}}
	{{end}}
	{{end}}

	{{if .HasOptions}}
	err := {{.RunFunc.Name}}( {{if .RunFunc.OptionsArgIsPointer}} options {{else}} *options {{end}} )
	{{else}}
	err := {{.RunFunc.Name}}()
	{{end}}
	if err != nil {
		slog.Error("Runtime error", "error", err)
		os.Exit(1)
	}
}
`))

	if len(cmdMeta.Options) > 0 && cmdMeta.RunFunc.OptionsArgTypeNameStripped == "" {
		return "", fmt.Errorf("OptionsArgTypeNameStripped is empty for command %s, but options are present. This indicates an issue with parsing the run function's options struct type", cmdMeta.Name)
	}

	data := struct {
		RunFunc    *metadata.RunFuncInfo
		Options    []*metadata.OptionMetadata
		HasOptions bool
		HelpText   string
	}{
		RunFunc:    cmdMeta.RunFunc,
		Options:    cmdMeta.Options,
		HasOptions: len(cmdMeta.Options) > 0,
		HelpText:   helpText,
	}

	var generatedCode bytes.Buffer
	if err := tmpl.Execute(&generatedCode, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	if generateFullFile {
		// Construct the full Go source file content
		var sb strings.Builder
		sb.WriteString("package main\n\n")
		sb.WriteString("import (\n")
		for _, name := range []string{
			"flag",
			"fmt",
			"log/slog",
			"os",
			"strconv",
			"strings", // strings might be used by generated code for e.g. enum validation
		} {
			sb.WriteString(fmt.Sprintf("\t\"%s\"\n", name))
		}
		sb.WriteString(")\n\n")
		sb.WriteString(generatedCode.String())
		return sb.String(), nil
	}
	return generatedCode.String(), nil
}
