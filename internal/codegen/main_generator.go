package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"text/template"

	"github.com/podhmo/goat/internal/metadata"
)

// GenerateMain creates the Go code string for the new main() function
// based on the extracted command metadata.
func GenerateMain(cmdMeta *metadata.CommandMetadata) (string, error) {
	// Helper function for the template to join option names for the function call
	templateFuncs := template.FuncMap{
		"Title": strings.Title,
		"JoinFlagVars": func(options []metadata.Option) string {
			var names []string
			for _, opt := range options {
				names = append(names, strings.Title(opt.Name)+"Flag")
			}
			return strings.Join(names, ", ")
		},
		// Raw string output for default values to avoid auto-escaping in template
		"RawString": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	tmpl := template.Must(template.New("main").Funcs(templateFuncs).Parse(`
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	{{if .NeedsStrconv}}
	"strconv"
	{{end}}
	{{range .Imports}}
	"{{.}}"
	{{end}}
)

func main() {
	{{if .HasOptions}}
	// Declare individual variables for each flag
	{{range .Options}}
	var {{Title .Name}}Flag {{.Type}}
	{{end}}

	// Setup flag parsing for each option
	{{range .Options}}
	{{if eq .Type "string"}}
	flag.StringVar(&{{Title .Name}}Flag, "{{.Name}}", {{if .Default}}{{printf "%q" .Default}}{{else}}""{{end}}, "{{.Description}}")
	{{else if eq .Type "int"}}
	flag.IntVar(&{{Title .Name}}Flag, "{{.Name}}", {{if .Default}}{{.Default}}{{else}}0{{end}}, "{{.Description}}")
	{{else if eq .Type "bool"}}
	flag.BoolVar(&{{Title .Name}}Flag, "{{.Name}}", {{if .Default}}{{.Default}}{{else}}false{{end}}, "{{.Description}}")
	{{end}}
	{{end}}

	flag.Parse()

	// Handle environment variables and required flags
	{{range .Options}}
	{{if .Envvar}}
	if val, ok := os.LookupEnv("{{.Envvar}}"); ok {
		// If flag was set, it takes precedence. Only use env if flag is still its zero value.
		// This check is tricky for bools where false is a valid value AND the default.
		// And for numbers where 0 is a valid value AND the default.
		// A more robust way might involve checking if the flag was explicitly set.
		// For now, if default is zero-value, env var will override if set.
		// If default is non-zero, flag value (even if it's the default) takes precedence.
		{{if eq .Type "string"}}
		if {{Title .Name}}Flag == {{if .Default}}{{printf "%q" .Default}}{{else}}""{{end}} { // only override if flag is still at default
			{{Title .Name}}Flag = val
		}
		{{else if eq .Type "int"}}
		if {{Title .Name}}Flag == {{if .Default}}{{.Default}}{{else}}0{{end}} {
			if v, err := strconv.Atoi(val); err == nil {
				{{Title .Name}}Flag = v
			} else {
				log.Printf("Warning: could not parse environment variable {{.Envvar}} as int: %v", err)
			}
		}
		{{else if eq .Type "bool"}}
		if {{Title .Name}}Flag == {{if .Default}}{{.Default}}{{else}}false{{end}} {
			if v, err := strconv.ParseBool(val); err == nil {
				{{Title .Name}}Flag = v
			} else {
				log.Printf("Warning: could not parse environment variable {{.Envvar}} as bool: %v", err)
			}
		}
		{{end}}
	}
	{{end}}

	{{if .Required}}
	{{if eq .Type "string"}}
	if {{Title .Name}}Flag == "" {
		log.Fatalf("Missing required flag: -{{.Name}} {{if .Envvar}}or environment variable {{.Envvar}}{{end}}")
	}
	{{else if eq .Type "int"}}
	// This check for required int is tricky if 0 is a valid value AND the default.
	// If the default is non-zero, then if the flag is still that default, it's effectively "unset" by user.
	// If default is zero, we check if it was explicitly set or came from env.
	if {{Title .Name}}Flag == {{if .Default}}{{.Default}}{{else}}0{{end}} {
		isSet_{{Title .Name}} := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "{{.Name}}" {
				isSet_{{Title .Name}} = true
			}
		})
		envIsSource_{{Title .Name}} := false
		{{if .Envvar}}
		if val, ok := os.LookupEnv("{{.Envvar}}"); ok {
			if parsedVal, err := strconv.Atoi(val); err == nil && parsedVal == {{Title .Name}}Flag {
				envIsSource_{{Title .Name}} = true
			}
		}
		{{end}}
		if !isSet_{{Title .Name}} && !envIsSource_{{Title .Name}} {
			log.Fatalf("Missing required flag: -{{.Name}} {{if .Envvar}}or environment variable {{.Envvar}}{{end}}")
		}
	}
	{{else if eq .Type "bool"}}
	// For bools, "required" usually implies it must be explicitly set, or must be true.
	// If it must be true: if !{{Title .Name}}Flag { log.Fatalf("Flag -{{.Name}} must be true") }
	// If it must be set (and default is false), this is hard to check without knowing if it was user-set.
	// The current logic for env var precedence tries to handle this: if it's still default false, env can make it true.
	// If truly "required to be explicitly set", the logic would need flag.Visit.
	{{end}}
	{{end}}

	{{if .Enum}}
	isValidChoice_{{Title .Name}} := false
	allowedChoices_{{Title .Name}} := []string{ {{range $i, $e := .Enum}}{{if $i}}, {{end}}{{printf "%q" $e}}{{end}} }
	for _, choice := range allowedChoices_{{Title .Name}} {
		if {{Title .Name}}Flag == choice {
			isValidChoice_{{Title .Name}} = true
			break
		}
	}
	if !isValidChoice_{{Title .Name}} {
		log.Fatalf("Invalid value for -{{.Name}}: %s. Allowed choices are: %s", {{Title .Name}}Flag, strings.Join(allowedChoices_{{Title .Name}}, ", "))
	}
	{{end}}
	{{end}}
	{{end}}

	// Call the original run function
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

	needsStrconv := false
	for _, opt := range cmdMeta.Options {
		if opt.Envvar != "" && (opt.Type == "int" || opt.Type == "bool") {
			needsStrconv = true
			break
		}
	}
	// Ensure strconv is not duplicated in user's imports
	finalImports := []string{}
	userImportsStrconv := false
	for _, imp := range cmdMeta.RunFunc.Imports {
		if imp == "strconv" {
			userImportsStrconv = true
		}
		finalImports = append(finalImports, imp)
	}
	if needsStrconv && userImportsStrconv {
		needsStrconv = false // User already imports it, template will handle it via {{range .Imports}}
	}


	data := struct {
		RunFuncName    string
		RunFuncPackage string
		Options        []metadata.Option
		HasOptions     bool
		Imports        []string
		NeedsStrconv   bool
	}{
		RunFuncName:    cmdMeta.RunFunc.Name,
		RunFuncPackage: cmdMeta.RunFunc.PackageName,
		Options:        cmdMeta.Options,
		HasOptions:     len(cmdMeta.Options) > 0,
		Imports:        finalImports,
		NeedsStrconv:   needsStrconv,
	}

	var generatedCode bytes.Buffer
	if err := tmpl.Execute(&generatedCode, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	formattedCode, err := format.Source(generatedCode.Bytes())
	if err != nil {
		// For debugging, return the unformatted code to see the issue
		return "", fmt.Errorf("formatting generated code: %w\nRaw generated code:\n%s", err, generatedCode.String())
	}

	return string(formattedCode), nil
}
