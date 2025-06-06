package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/podhmo/goat/internal/metadata"
	"github.com/podhmo/goat/internal/utils/stringutils"
)

const mainFuncTmpl = `
func main() {
	isFlagExplicitlySet := make(map[string]bool)

	{{if .HelpText}}
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, {{FormatHelpText .HelpText}})
	}
	{{end}}

	{{if .HasOptions}}
	var options = &{{.RunFunc.OptionsArgTypeNameStripped}}{}
	{{range .Options}}
	{{if eq .TypeName "string"}}
	var default{{.Name}} string = {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}}
	{{if .EnvVar}}
	if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		default{{.Name}} = val
	}
	{{end}}
	flag.StringVar(&options.{{.Name}}, "{{ KebabCase .Name }}", default{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if eq .TypeName "int"}}
	var default{{.Name}} int = {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}}
	{{if .EnvVar}}
	if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		if v, err := strconv.Atoi(val); err == nil {
			default{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as int for default value", "envVar", "{{.EnvVar}}", "value", val, "error", err)
		}
	}
	{{end}}
	flag.IntVar(&options.{{.Name}}, "{{ KebabCase .Name }}", default{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if eq .TypeName "bool"}}
	var default{{.Name}} bool = {{if .DefaultValue}}{{.DefaultValue}}{{else}}false{{end}}
	{{if .EnvVar}}
	if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		if v, err := strconv.ParseBool(val); err == nil {
			default{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as bool for default value", "envVar", "{{.EnvVar}}", "value", val, "error", err)
		}
	}
	{{end}}
	{{if and .IsRequired (eq (.DefaultValue | printf "%v") "true") }}
	options.{{.Name}} = default{{.Name}}
	var {{.Name}}_NoFlagIsPresent bool
	flag.BoolVar(&{{.Name}}_NoFlagIsPresent, "no-{{ KebabCase .Name }}", false, {{FormatHelpText .HelpText}})
	{{else}}
	flag.BoolVar(&options.{{.Name}}, "{{ KebabCase .Name }}", default{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{end}}
	{{else if eq .TypeName "*string"}}
	options.{{.Name}} = new(string)
	{{if .DefaultValue}}*options.{{.Name}} = {{printf "%q" .DefaultValue}}{{end}}
	flag.StringVar(options.{{.Name}}, "{{ KebabCase .Name }}", {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}}, {{FormatHelpText .HelpText}}   {{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{else if eq .TypeName "*int"}}
	options.{{.Name}} = new(int)
	{{if .DefaultValue}}*options.{{.Name}} = {{.DefaultValue}}{{end}}
	flag.IntVar(options.{{.Name}}, "{{ KebabCase .Name }}", {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}}, {{FormatHelpText .HelpText}}{{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{else if eq .TypeName "*bool"}}
	options.{{.Name}} = new(bool)
	{{if .DefaultValue}}*options.{{.Name}} = {{.DefaultValue}}{{end}}
	flag.BoolVar(options.{{.Name}}, "{{ KebabCase .Name }}", {{if ne .DefaultValue nil}}{{.DefaultValue}}{{else}}false{{end}}, {{FormatHelpText .HelpText}}{{- if ne .DefaultValue nil -}}/* Default: {{.DefaultValue}} */{{- end -}})
	{{end}}
	{{end}}
	{{end}}

	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	{{if .HasOptions}}
	{{range .Options}}
	{{if eq .TypeName "bool"}}
	{{if and .IsRequired (eq (.DefaultValue | printf "%v") "true") }}
	if {{.Name}}_NoFlagIsPresent {
		options.{{.Name}} = false
	}
	{{end}}
	{{end}}
	{{end}}

	{{range .Options}}
	{{if or (eq .TypeName "*string") (eq .TypeName "*int") (eq .TypeName "*bool")}}
	{{if .EnvVar}}
	if !isFlagExplicitlySet["{{ KebabCase .Name }}"] {
		if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
			{{if eq .TypeName "*string"}}
			*options.{{.Name}} = val
			{{else if eq .TypeName "*int"}}
			if v, err := strconv.Atoi(val); err == nil {
				*options.{{.Name}} = v
			} else {
				slog.Warn("Could not parse environment variable as *int", "envVar", "{{.EnvVar}}", "value", val, "error", err)
			}
			{{else if eq .TypeName "*bool"}}
			if v, err := strconv.ParseBool(val); err == nil {
				*options.{{.Name}} = v
			} else {
				slog.Warn("Could not parse environment variable as *bool", "envVar", "{{.EnvVar}}", "value", val, "error", err)
			}
			{{end}}
		}
	}
	{{end}}
	{{end}}
	{{end}}

	{{range .Options}}
	{{if .IsRequired}}
	{{if eq .TypeName "string"}}
	var env{{.Name}}IsSet bool
	{{if .EnvVar}}
	if _, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		env{{.Name}}IsSet = true
	}
	{{end}}
	if options.{{.Name}} == {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}} &&
	   !isFlagExplicitlySet["{{KebabCase .Name}}"] &&
	   !env{{.Name}}IsSet {
		slog.Error("Missing required flag", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}})
		os.Exit(1)
	}
	{{else if eq .TypeName "int"}}
	var env{{.Name}}IsSet bool
	{{if .EnvVar}}
	if _, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		env{{.Name}}IsSet = true
	}
	{{end}}
	if options.{{.Name}} == {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}} &&
	   !isFlagExplicitlySet["{{KebabCase .Name}}"] &&
	   !env{{.Name}}IsSet {
		slog.Error("Missing required flag", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}})
		os.Exit(1)
	}
	{{else if eq .TypeName "*string"}}
	if options.{{.Name}} == nil || *options.{{.Name}} == "" {
		slog.Error("Missing required flag or empty value for", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}})
		os.Exit(1)
	}
	{{else if or (eq .TypeName "*int") (eq .TypeName "*bool")}}
	if options.{{.Name}} == nil {
		slog.Error("Missing required flag", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}})
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
`

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

	tmpl := template.Must(template.New("main").Funcs(templateFuncs).Parse(mainFuncTmpl))

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
