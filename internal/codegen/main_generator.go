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

	// 1. Create Options with default values.
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
	{{end}}

	// 2. Override with environment variable values.
	{{range .Options}}
	{{if .EnvVar}}
	if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		{{if .IsTextUnmarshaler}}
			{{if .IsPointer}}
			// options.{{.Name}} is type *CustomType. CustomType implements TextUnmarshaler (likely via *CustomType receiver)
			if options.{{.Name}} == nil {
				options.{{.Name}} = new({{.TypeName | TrimStar}})
			}
			err := options.{{.Name}}.UnmarshalText([]byte(val))
			if err != nil {
				slog.Warn("Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "{{.EnvVar}}", "option", "{{.CliName}}", "value", val, "error", err)
			}
			{{else}}
			// options.{{.Name}} is type CustomType. Assumes UnmarshalText has a pointer receiver (*CustomType).
			// The field itself must be addressable.
			err := (&options.{{.Name}}).UnmarshalText([]byte(val))
			if err != nil {
				slog.Warn("Could not parse environment variable for TextUnmarshaler option; using default or previously set value.", "envVar", "{{.EnvVar}}", "option", "{{.CliName}}", "value", val, "error", err)
			}
			{{end}}
		{{else if eq .TypeName "string"}}
		options.{{.Name}} = val
		{{else if eq .TypeName "int"}}
		if v, err := strconv.Atoi(val); err == nil {
			options.{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as int for option", "envVar", "{{.EnvVar}}", "option", "{{.Name}}", "value", val, "error", err)
		}
		{{else if eq .TypeName "bool"}}
		if v, err := strconv.ParseBool(val); err == nil {
			options.{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as bool for option", "envVar", "{{.EnvVar}}", "option", "{{.Name}}", "value", val, "error", err)
		}
		{{else if eq .TypeName "*string"}}
		if options.{{.Name}} == nil { options.{{.Name}} = new(string) }
		*options.{{.Name}} = val
		{{else if eq .TypeName "*int"}}
		if options.{{.Name}} == nil { options.{{.Name}} = new(int) }
		if v, err := strconv.Atoi(val); err == nil {
			*options.{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as *int for option", "envVar", "{{.EnvVar}}", "option", "{{.Name}}", "value", val, "error", err)
		}
		{{else if eq .TypeName "*bool"}}
		if options.{{.Name}} == nil { options.{{.Name}} = new(bool) }
		if v, err := strconv.ParseBool(val); err == nil {
			*options.{{.Name}} = v
		} else {
			slog.Warn("Could not parse environment variable as *bool for option", "envVar", "{{.EnvVar}}", "option", "{{.Name}}", "value", val, "error", err)
		}
		{{end}}
	}
	{{end}}
	{{end}}

	// 3. Set flags.
	{{range .Options}}
	{{if eq .TypeName "string"}}
	flag.StringVar(&options.{{.Name}}, "{{ KebabCase .Name }}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if eq .TypeName "int"}}
	flag.IntVar(&options.{{.Name}}, "{{ KebabCase .Name }}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else if eq .TypeName "bool"}}
	{{if and .IsRequired (eq (.DefaultValue | printf "%v") "true") }}
	// For required bools defaulting to true, we need a 'no-<flag>' to set it to false.
	// The options.{{.Name}} is already true (either by default or env var).
	var {{.Name}}_NoFlagIsPresent bool
	flag.BoolVar(&{{.Name}}_NoFlagIsPresent, "no-{{ KebabCase .Name }}", false, "Set {{ KebabCase .Name }} to false")
	{{else}}
	flag.BoolVar(&options.{{.Name}}, "{{ KebabCase .Name }}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if ne .DefaultValue nil -}}/* Original Default: {{.DefaultValue}}, Env: {{.EnvVar}} */{{- else if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{end}}
	{{else if eq .TypeName "*string"}}
	// For optional pointers, the initial value is nil. If an env var was set, it's non-nil.
	// If still nil, make it non-nil so flag.StringVar can write to it if the flag is provided.
	// The default value for the flag itself will be the current *options.{{.Name}} (if set by env) or ""
	var default{{.Name}}ValForFlag string
	if options.{{.Name}} != nil { default{{.Name}}ValForFlag = *options.{{.Name}} }
	if options.{{.Name}} == nil { options.{{.Name}} = new(string) } // Ensure flag has a place to write
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
	// options.{{.Name}} is type *CustomType. CustomType implements TextUnmarshaler and TextMarshaler (likely via *CustomType receiver)
	// Ensure options.{{.Name}} is initialized before use if it's nil.
	if options.{{.Name}} == nil {
		options.{{.Name}} = new({{.TypeName | TrimStar}})
		// Note: Assumes new({{.TypeName | TrimStar}}) results in a state
		// that is valid for MarshalText (e.g., gives empty string or a sensible default).
	}
	// First argument to TextVar is the TextUnmarshaler (options.{{.Name}} itself).
	// Third argument (value) is the TextMarshaler for the default (options.{{.Name}} itself).
	flag.TextVar(options.{{.Name}}, "{{.CliName}}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{else}}
	// options.{{.Name}} is type CustomType.
	// &options.{{.Name}} will be the TextUnmarshaler.
	// options.{{.Name}} will be the TextMarshaler for the default value.
	// This assumes CustomType implements TextMarshaler, and *CustomType implements TextUnmarshaler.
	flag.TextVar(&options.{{.Name}}, "{{.CliName}}", options.{{.Name}}, {{FormatHelpText .HelpText}} {{- if .EnvVar}}/* Env: {{.EnvVar}} */{{- end -}})
	{{end}}
	{{end}}
	{{end}}

	// 4. Parse.
	flag.Parse()
	flag.Visit(func(f *flag.Flag) { isFlagExplicitlySet[f.Name] = true })

	// Handle special case for required bools defaulting to true with 'no-<flag>'
	{{range .Options}}
	{{if eq .TypeName "bool"}}
	{{if and .IsRequired (eq (.DefaultValue | printf "%v") "true") }}
	if {{.Name}}_NoFlagIsPresent { // This var is from step 3
		options.{{.Name}} = false
	}
	{{end}}
	{{end}}
	{{end}}

	// 5. Perform required checks (excluding booleans).
	{{range .Options}}
	{{if .IsRequired}}
	{{if eq .TypeName "string"}}
	// A string is required. It must not be its original default if the flag wasn't set and env var wasn't set.
	// If default was empty: must not be empty.
	// If default was non-empty: must not be that specific non-empty value.
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
	// A *string is required. It must have been set by flag or env var.
	// If it was set by env var, options.{{.Name}} is not nil.
	// If it was set by flag, options.{{.Name}} is not nil.
	// The only way it's nil is if it had no default, no env var, and no flag.
	// Or, if it was set to "" by flag or env var.
	env{{.Name}}WasSet := false
	{{if .EnvVar}}
	if _, ok := os.LookupEnv("{{.EnvVar}}"); ok { env{{.Name}}WasSet = true }
	{{end}}
	if !isFlagExplicitlySet["{{KebabCase .Name}}"] && !env{{.Name}}WasSet { // if neither flag nor env was set
		{{if .DefaultValue}} // if there was an original default, it's fine, it's already set
		{{else}} // if no original default, and neither flag nor env, then it's an error if it's still nil (it would be) or empty string
		if options.{{.Name}} == nil || *options.{{.Name}} == "" {
			slog.Error("Missing required flag or environment variable, and no default provided", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
			os.Exit(1)
		}
		{{end}}
	} else if options.{{.Name}} == nil || *options.{{.Name}} == "" { // if flag or env was set, but resulted in empty/nil
		slog.Error("Required flag was set to an empty value", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
		os.Exit(1)
	}
	{{else if eq .TypeName "*int"}}
	// A *int is required. It must have been set by flag or env var if no original default.
	env{{.Name}}WasSet := false
	{{if .EnvVar}}
	if _, ok := os.LookupEnv("{{.EnvVar}}"); ok { env{{.Name}}WasSet = true }
	{{end}}
	if !isFlagExplicitlySet["{{KebabCase .Name}}"] && !env{{.Name}}WasSet { // if neither flag nor env was set
		{{if .DefaultValue}} // if there was an original default, it's fine
		{{else}} // if no original default, and neither flag nor env, then it's an error if it's still nil (it would be)
		if options.{{.Name}} == nil {
			slog.Error("Missing required flag or environment variable, and no default provided", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
			os.Exit(1)
		}
		{{end}}
	} else if options.{{.Name}} == nil { // if flag or env was set, but resulted in nil (e.g. *int flag not provided, but was optional so it's nil)
		slog.Error("Required flag was not provided or set to nil", "flag", "{{KebabCase .Name}}"{{if .EnvVar}}, "envVar", "{{.EnvVar}}"{{end}}, "option", "{{.Name}}")
		os.Exit(1)
	}
	// Not checking *bool here as per requirements
	{{end}}
	{{end}}

	{{if .EnumValues}}
	isValidChoice_{{.Name}} := false
	allowedChoices_{{.Name}} := []string{ {{range $i, $e := .EnumValues}}{{if $i}}, {{end}}{{printf "%q" $e}}{{end}} }

	{{if or (eq .TypeName "*string") (eq .TypeName "*int") (eq .TypeName "*bool")}} // Handle pointer types for enum
		if options.{{.Name}} != nil {
			currentValue_{{.Name}}Str := fmt.Sprintf("%v", *options.{{.Name}})
			isValidChoice_{{.Name}} = slices.Contains(allowedChoices_{{.Name}}, currentValue_{{.Name}}Str)
		} else { // options.{{.Name}} is nil
			{{if .IsRequired}}
			slog.Error("Required enum flag is nil", "flag", "{{ KebabCase .Name }}", "option", "{{.Name}}")
			os.Exit(1)
			{{else}}
			isValidChoice_{{.Name}} = true // Optional pointer enum that is nil is valid.
			{{end}}
		}
	{{else}} // Handle non-pointer types for enum
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
	{{end}} // End of if .HasOptions

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
		for _, name := range []string{
			"flag",
			"fmt",
			"log/slog",
			"os",
			"slices", // Added slices
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
