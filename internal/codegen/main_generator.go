package codegen

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/podhmo/goat/internal/metadata"
)

// GenerateMain creates the Go code string for the new main() function
// based on the extracted command metadata.
// If generateFullFile is true, it returns a complete Go file content including package and imports.
// Otherwise, it returns only the main function body.
func GenerateMain(cmdMeta *metadata.CommandMetadata, helpText string, generateFullFile bool) (string, error) {
	// Helper function for the template to join option names for the function call
	templateFuncs := template.FuncMap{
		"Title": strings.Title,
		"JoinFlagVars": func(options []*metadata.OptionMetadata) string {
			var names []string
			for _, opt := range options {
				names = append(names, strings.Title(opt.Name)+"Flag") // Name is the Go field name, correct for var name
			}
			return strings.Join(names, ", ")
		},
	}

	tmpl := template.Must(template.New("main").Funcs(templateFuncs).Parse(`
func main() {
	{{if .HasOptions}}
	{{range .Options}}
	var {{Title .Name}}Flag {{.TypeName}}
	{{end}}

	{{range .Options}}
	{{if eq .TypeName "string"}}
	flag.StringVar(&{{Title .Name}}Flag, "{{.Name}}", {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}}, "{{.HelpText}}")
	{{else if eq .TypeName "int"}}
	flag.IntVar(&{{Title .Name}}Flag, "{{.Name}}", {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}}, "{{.HelpText}}")
	{{else if eq .TypeName "bool"}}
	flag.BoolVar(&{{Title .Name}}Flag, "{{.Name}}", {{if .DefaultValue}}{{.DefaultValue}}{{else}}false{{end}}, "{{.HelpText}}")
	{{end}}
	{{end}}

	{{if .HelpText}}
	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			fmt.Fprintln(os.Stdout, {{printf "%q" .HelpText}})
			os.Exit(0)
		}
	}
	{{end}}

	flag.Parse()

	{{range .Options}}
	{{if .EnvVar}}
	if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
		// If flag was set, it takes precedence. Only use env if flag is still its zero value.
		// This check is tricky for bools where false is a valid value AND the default.
		// And for numbers where 0 is a valid value AND the default.
		// A more robust way might involve checking if the flag was explicitly set.
		// For now, if default is zero-value, env var will override if set.
		// If default is non-zero, flag value (even if it's the default) takes precedence.
		{{if eq .TypeName "string"}}
		if {{Title .Name}}Flag == {{if .DefaultValue}}{{printf "%q" .DefaultValue}}{{else}}""{{end}} { // only override if flag is still at default
			{{Title .Name}}Flag = val
		}
		{{else if eq .TypeName "int"}}
		if {{Title .Name}}Flag == {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}} {
			if v, err := strconv.Atoi(val); err == nil {
				{{Title .Name}}Flag = v
			} else {
				log.Printf("Warning: could not parse environment variable {{.EnvVar}} as int: %v", err)
			}
		}
		{{else if eq .TypeName "bool"}}
		if {{Title .Name}}Flag == {{if .DefaultValue}}{{.DefaultValue}}{{else}}false{{end}} {
			if v, err := strconv.ParseBool(val); err == nil {
				{{Title .Name}}Flag = v
			} else {
				log.Printf("Warning: could not parse environment variable {{.EnvVar}} as bool: %v", err)
			}
		}
		{{end}}
	}
	{{end}}

	{{if .IsRequired}}
	{{if eq .TypeName "string"}}
	if {{Title .Name}}Flag == "" {
		log.Fatalf("Missing required flag: -{{.Name}}{{if .EnvVar}} or environment variable {{.EnvVar}}{{end}}")
	}
	{{else if eq .TypeName "int"}}
	// This check for required int is tricky if 0 is a valid value AND the default.
	// If the default is non-zero, then if the flag is still that default, it's effectively "unset" by user.
	// If default is zero, we check if it was explicitly set or came from env.
	if {{Title .Name}}Flag == {{if .DefaultValue}}{{.DefaultValue}}{{else}}0{{end}} {
		isSet_{{Title .Name}} := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "{{.Name}}" {
				isSet_{{Title .Name}} = true
			}
		})
		envIsSource_{{Title .Name}} := false
		{{if .EnvVar}}
		if val, ok := os.LookupEnv("{{.EnvVar}}"); ok {
			if parsedVal, err := strconv.Atoi(val); err == nil && parsedVal == {{Title .Name}}Flag {
				envIsSource_{{Title .Name}} = true
			}
		}
		{{end}}
		if !isSet_{{Title .Name}} && !envIsSource_{{Title .Name}} {
			log.Fatalf("Missing required flag: -{{.Name}}{{if .EnvVar}} or environment variable {{.EnvVar}}{{end}}")
		}
	}
	{{else if eq .TypeName "bool"}}
	// For bools, "required" usually implies it must be explicitly set, or must be true.
	// If it must be set (and default is false), this is hard to check without knowing if it was user-set.
	// The current logic for env var precedence tries to handle this: if it's still default false, env can make it true.
	// If truly "required to be explicitly set", the logic would need flag.Visit.
	{{end}}
	{{end}}

	{{if .EnumValues}}
	isValidChoice_{{Title .Name}}Flag := false
	allowedChoices_{{Title .Name}}Flag := []string{ {{range $i, $e := .EnumValues}}{{if $i}}, {{end}}{{printf "%q" $e}}{{end}} }
	for _, choice := range allowedChoices_{{Title .Name}}Flag {
		if {{Title .Name}}Flag == choice {
			isValidChoice_{{Title .Name}}Flag = true
			break
		}
	}
	if !isValidChoice_{{Title .Name}}Flag {
		log.Fatalf("Invalid value for -{{.Name}}: %s. Allowed choices are: %s", {{Title .Name}}Flag, strings.Join(allowedChoices_{{Title .Name}}Flag, ", "))
	}
	{{end}}
	{{end}}
	{{end}}

	{{if .HasOptions}}
	err := {{.RunFuncPackage}}.{{.RunFuncName}}({{ JoinFlagVars .Options }})
	{{else}}
	err := {{.RunFuncPackage}}.{{.RunFuncName}}()
	{{end}}
	if err != nil {
		log.Fatal(err)
	}
}
`))

	// RunFuncInfo no longer provides Imports.
	// Necessary direct imports like "flag", "fmt", "log", "os", "strconv", "strings"
	// will be added explicitly to the generated code.
	// User-specific imports from the original run command's package must be handled
	// by the user ensuring the run command's package itself is importable and correct.

	data := struct {
		RunFuncName    string
		RunFuncPackage string
		Options        []*metadata.OptionMetadata
		HasOptions     bool
		// Imports field is removed as it was unused and imports are now static
		HelpText string
	}{
		RunFuncName:    cmdMeta.RunFunc.Name,
		RunFuncPackage: cmdMeta.RunFunc.PackageName,
		Options:        cmdMeta.Options,
		HasOptions:     len(cmdMeta.Options) > 0,
		HelpText:       helpText,
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
		for _, imp := range defaultImports {
			sb.WriteString("\t\"" + imp + "\"\n")
		}
		sb.WriteString(")\n\n")
		sb.WriteString(generatedCode.String())
		return sb.String(), nil
	}
	return generatedCode.String(), nil
}
