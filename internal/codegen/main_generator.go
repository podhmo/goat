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

	{{/* Options struct handling block */}}
	{{if .RunFunc.OptionsArgTypeNameStripped}}
	var options *{{.RunFunc.OptionsArgTypeNameStripped}}

	{{if .RunFunc.InitializerFunc}}
	// 1. Create Options using the initializer function.
	options = {{.RunFunc.InitializerFunc}}()
	{{else}}
	// 1. Create Options with default values (no initializer function provided).
	options = new({{.RunFunc.OptionsArgTypeNameStripped}}) // options is now a valid pointer to a zeroed struct

	// The following block populates the fields of the options struct.
	// This logic is only executed if no InitializerFunc is provided.
	{{range .Options}}
	{{if eq .TypeName "string"}}
	options.{{.Name}} = {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}}
	{{else if eq .TypeName "int"}}
	options.{{.Name}} = {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}}
	{{else if eq .TypeName "bool"}}
	options.{{.Name}} = {{if .DefaultValue}}{{.DefaultValue}}{{else}}false{{end}}
	{{else if eq .TypeName "*string"}}
	options.{{.Name}} = new(string)
	{{if .DefaultValue}}*options.{{.Name}} = {{printf "%q" .DefaultValue}}{{end}}
	{{else if eq .TypeName "*int"}}
	options.{{.Name}} = new(int)
	{{if .DefaultValue}}*options.{{.Name}} = {{.DefaultValue}}{{end}}
	{{else if eq .TypeName "*bool"}}
	options.{{.Name}} = new(bool)
	{{if .DefaultValue}}*options.{{.Name}} = {{.DefaultValue}}{{end}}
	{{end}}
	{{end}} // End of range .Options (for non-initializer case)
	{{end}} // End of if/else .RunFunc.InitializerFunc for options assignment

	// 2. Override with environment variable values.
	// This section assumes 'options' is already initialized.
	{{range .Options}}
	{{if .EnvVar}}
	if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		{{if .IsTextUnmarshaler}}
			{{if .IsPointer}}
			if options.{{.Name}} == nil {
				options.{{.Name}} = new({{.TypeName | TrimStar}})
			}
			errUnmarshal := options.{{.Name}}.UnmarshalText([]byte(val))
			if errUnmarshal != nil {
				slog.Warn("Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "{{.EnvVar}}", "option", "{{.CliName}}", "value", val, "error", errUnmarshal)
			}
			{{else}}
			errUnmarshal := (&options.{{.Name}}).UnmarshalText([]byte(val))
			if errUnmarshal != nil {
				slog.Warn("Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "{{.EnvVar}}", "option", "{{.CliName}}", "value", val, "error", errUnmarshal)
			}
			{{end}}
		{{else if eq .TypeName "string"}}
		options.{{.Name}} = val
		{{else if eq .TypeName "int"}}
		if v, errConv := strconv.Atoi(val); errConv == nil {
			options.{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as int for option", "envVar", "{{.EnvVar}}", "option", "{{.Name}}", "value", val, "error", errConv)
		}
		{{else if eq .TypeName "bool"}}
		if v, errConv := strconv.ParseBool(val); errConv == nil {
			options.{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as bool for option", "envVar", "{{.EnvVar}}", "option", "{{.Name}}", "value", val, "error", errConv)
		}
		{{else if eq .TypeName "*string"}}
		if options.{{.Name}} == nil { options.{{.Name}} = new(string) }
		*options.{{.Name}} = val
		{{else if eq .TypeName "*int"}}
		if options.{{.Name}} == nil { options.{{.Name}} = new(int) }
		if v, errConv := strconv.Atoi(val); errConv == nil {
			*options.{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as *int for option", "envVar", "{{.EnvVar}}", "option", "{{.Name}}", "value", val, "error", errConv)
		}
		{{else if eq .TypeName "*bool"}}
		if options.{{.Name}} == nil { options.{{.Name}} = new(bool) }
		if v, errConv := strconv.ParseBool(val); errConv == nil {
			*options.{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as *bool for option", "envVar", "{{.EnvVar}}", "option", "{{.Name}}", "value", val, "error", errConv)
		}
		{{end}}
	}
	{{end}}
	{{end}} // End of range .Options for env vars

	// 3. Set flags.
	{{range .Options}}
	{{if eq .TypeName "string"}}
	flag.StringVar(&options.{{.Name}}, "{{ KebabCase .Name }}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if eq .TypeName "int"}}
	flag.IntVar(&options.{{.Name}}, "{{ KebabCase .Name }}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if eq .TypeName "bool"}}
	{{if and .IsRequired (eq (.DefaultValue | printf "%v") "true") }}
	var {{.Name}}_NoFlagIsPresent bool
	flag.BoolVar(&{{.Name}}_NoFlagIsPresent, "no-{{ KebabCase .Name }}", false, "Set {{ KebabCase .Name }} to false")
	{{else}}
	flag.BoolVar(&options.{{.Name}}, "{{ KebabCase .Name }}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{end}}
	{{else if eq .TypeName "*string"}}
	var default{{.Name}}ValForFlag string
	if options.{{.Name}} != nil { default{{.Name}}ValForFlag = *options.{{.Name}} }
	if options.{{.Name}} == nil { options.{{.Name}} = new(string) }
	flag.StringVar(options.{{.Name}}, "{{ KebabCase .Name }}", default{{.Name}}ValForFlag, {{FormatHelpText .HelpText}}   {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if eq .TypeName "*int"}}
	var default{{.Name}}ValForFlag int
	if options.{{.Name}} != nil { default{{.Name}}ValForFlag = *options.{{.Name}} }
	if options.{{.Name}} == nil { options.{{.Name}} = new(int) }
	flag.IntVar(options.{{.Name}}, "{{ KebabCase .Name }}", default{{.Name}}ValForFlag, {{FormatHelpText .HelpText}}{{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if eq .TypeName "*bool"}}
	var default{{.Name}}ValForFlag bool
	if options.{{.Name}} != nil { default{{.Name}}ValForFlag = *options.{{.Name}} }
	if options.{{.Name}} == nil { options.{{.Name}} = new(bool) }
	flag.BoolVar(options.{{.Name}}, "{{ KebabCase .Name }}", default{{.Name}}ValForFlag, {{FormatHelpText .HelpText}}{{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if and .IsTextUnmarshaler .IsTextMarshaler}}
	{{if .IsPointer}}
	if options.{{.Name}} == nil {
		options.{{.Name}} = new({{.TypeName | TrimStar}})
	}
	flag.TextVar(options.{{.Name}}, "{{.CliName}}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else}}
	flag.TextVar(&options.{{.Name}}, "{{.CliName}}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{end}}
	{{end}}
	{{end}} // End of range .Options for flags

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	// Handle special case for required bools defaulting to true with 'no-<flag>'
	{{range .Options}}
	{{if eq .TypeName "bool"}}
	{{if and .IsRequired (eq (.DefaultValue | printf "%v") "true") }}
	if {{.Name}}_NoFlagIsPresent {
		options.{{.Name}} = false
	}
	{{end}}
	{{end}}
	{{end}}

	// 5. Perform required checks (excluding booleans).
	{{range .Options}}
	{{if .IsRequired}}
	{{if eq .TypeName "string"}}
	initialDefault{{.Name}} := {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}}
	env{{.Name}}WasSet := false
	{{if .EnvVar}}
	if _, ok := os.LookupEnv("{{.EnvVar}}"); ok { env{{.Name}}WasSet = true }
	{{end}}
	if options.{{.Name}} == initialDefault{{.Name}} && !isFlagExplicitlySet["{{KebabCase .Name}}"] && !env{{.Name}}WasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
		os.Exit(1)
	}
	{{else if eq .TypeName "int"}}
	initialDefault{{.Name}} := {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}}
	env{{.Name}}WasSet := false
	{{if .EnvVar}}
	if _, ok := os.LookupEnv("{{.EnvVar}}"); ok { env{{.Name}}WasSet = true }
	{{end}}
	if options.{{.Name}} == initialDefault{{.Name}} && !isFlagExplicitlySet["{{KebabCase .Name}}"] && !env{{.Name}}WasSet {
		slog.Error("Missing required flag or environment variable not set", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
		os.Exit(1)
	}
	{{else if eq .TypeName "*string"}}
	env{{.Name}}WasSet := false
	{{if .EnvVar}}
	if _, ok := os.LookupEnv("{{.EnvVar}}"); ok { env{{.Name}}WasSet = true }
	{{end}}
	if !isFlagExplicitlySet["{{KebabCase .Name}}"] && !env{{.Name}}WasSet {
		{{if .DefaultValue}}
		{{else}}
		if options.{{.Name}} == nil || *options.{{.Name}} == "" {
			slog.Error("Missing required flag or environment variable, and no default provided", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
			os.Exit(1)
		}
		{{end}}
	} else if options.{{.Name}} == nil || *options.{{.Name}} == "" {
		slog.Error("Required flag was set to an empty value", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
		os.Exit(1)
	}
	{{else if eq .TypeName "*int"}}
	env{{.Name}}WasSet := false
	{{if .EnvVar}}
	if _, ok := os.LookupEnv("{{.EnvVar}}"); ok { env{{.Name}}WasSet = true }
	{{end}}
	if !isFlagExplicitlySet["{{KebabCase .Name}}"] && !env{{.Name}}WasSet {
		{{if .DefaultValue}}
		{{else}}
		if options.{{.Name}} == nil {
			slog.Error("Missing required flag or environment variable, and no default provided", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
			os.Exit(1)
		}
		{{end}}
	} else if options.{{.Name}} == nil {
		slog.Error("Required flag was not provided or set to nil", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
		os.Exit(1)
	}
	{{end}}
	{{end}}

	{{if .EnumValues}}
	isValidChoice_{{.Name}} := false
	allowedChoices_{{.Name}} := []string{ {{range $i, $e := .EnumValues}}{{if $i}}, {{end}}{{printf "%q" $e}}{{end}} }

	{{if or (eq .TypeName "*string") (eq .TypeName "*int") (eq .TypeName "*bool")}}
		if options.{{.Name}} != nil {
			currentValue_{{.Name}}Str := fmt.Sprintf("%v", *options.{{.Name}})
			isValidChoice_{{.Name}} = slices.Contains(allowedChoices_{{.Name}}, currentValue_{{.Name}}Str)
		} else {
			{{if .IsRequired}}
			slog.Error("Required enum flag is nil", "flag", "{{ KebabCase .Name }}", "option", "{{.Name}}")
			os.Exit(1)
			{{else}}
			isValidChoice_{{.Name}} = true
			{{end}}
		}
	{{else}}
		currentValue_{{.Name}}Str := fmt.Sprintf("%v", options.{{.Name}})
		isValidChoice_{{.Name}} = slices.Contains(allowedChoices_{{.Name}}, currentValue_{{.Name}}Str)
	{{end}}

	if !isValidChoice_{{.Name}} {
		var currentValueForMsg interface{} = options.{{.Name}}
		{{if or (eq .TypeName "*string") (eq .TypeName "*int") (eq .TypeName "*bool")}}
		if options.{{.Name}} != nil {
			currentValueForMsg = *options.{{.Name}}
		}
		{{end}}
		slog.Error("Invalid value for flag", "flag", "{{ KebabCase .Name }}", "value", currentValueForMsg, "allowedChoices", strings.Join(allowedChoices_{{.Name}}, ", "))
		os.Exit(1)
	}
	{{end}}
	{{end}} // End of range .Options for required checks
	{{end}} // End of if .RunFunc.OptionsArgTypeNameStripped (options handling block)

	{{/* Run the actual command */}}
	var err error
	{{if .RunFunc.OptionsArgTypeNameStripped}}
	// Run function expects an options argument
	err = {{if ne .RunFunc.PackageName "main"}}{{.RunFunc.PackageName}}.{{end}}{{.RunFunc.Name}}( {{if .RunFunc.OptionsArgIsPointer}} options {{else}} *options {{end}} )
	{{else}}
	// Run function does not expect an options argument
	err = {{if ne .RunFunc.PackageName "main"}}{{.RunFunc.PackageName}}.{{end}}{{.RunFunc.Name}}()
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
		"FormatHelpText": formatHelpText,
		"TrimStar": func(s string) string {
			if strings.HasPrefix(s, "*") {
				return s[1:]
			}
			return s
		},
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

		// Standard imports
		imports := []string{
			"flag",
			"fmt",
			"log/slog",
			"os",
			"slices",
			"strconv",
			"strings",
		}

		// Add user's package import if necessary
		userPkgImportPath := ""
		if cmdMeta.RunFunc != nil && cmdMeta.RunFunc.PackageName != "" && cmdMeta.RunFunc.PackageName != "main" {
			if cmdMeta.Name != "" { // cmdMeta.Name is the targetPackageID
				userPkgImportPath = cmdMeta.Name
			}
		}

		if userPkgImportPath != "" {
			alreadyPresent := false
			for _, imp := range imports {
				if imp == userPkgImportPath {
					alreadyPresent = true
					break
				}
			}
			if !alreadyPresent {
				imports = append(imports, userPkgImportPath)
			}
		}

		for _, importPath := range imports {
			sb.WriteString(fmt.Sprintf("\t%q\n", importPath))
		}
		sb.WriteString(")\n\n")
		sb.WriteString(generatedCode.String())
		return sb.String(), nil
	}
	return generatedCode.String(), nil
}
