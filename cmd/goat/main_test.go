package main

import (
	"go/token"
	"os"
	"testing"

	"github.com/podhmo/goat/internal/config"
	"github.com/podhmo/goat/internal/help"
)

func TestHelpGenerateHelpOutput(t *testing.T) {
	const content = `
package main

import "github.com/podhmo/goat/goat"

// Options for testapp.
// This is a test application.
type Options struct {
	// Name of the user.
	Name string
	// Port number.
	Port int
}

func NewOptions() *Options {
	return &Options{
		Name: goat.Default("anonymous"),
		Port: goat.Default(8080),
	}
}

// Run the test application.
// It does something.
func Run(opts Options) error {
	return nil
}

func main() { /* Will be replaced */ }
`
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/testapp.go"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	cfg := &config.Config{
		RunFuncName:            "Run",
		OptionsInitializerName: "NewOptions",
		TargetFile:             tmpFile,
	}

	fset := token.NewFileSet()
	cmdMetadata, _, err := scanMain(fset, cfg)
	if err != nil {
		t.Fatalf("scanMain() error = %v", err)
	}

	got := help.GenerateHelp(cmdMetadata)
	want := `main - Run the test application.
         It does something.

Usage:
  main [flags] 

Flags:
  --name      string Name of the user. (required) (default: "anonymous")
  --port      int Port number. (required) (default: 8080)

  -h, --help Show this help message and exit
`
	if got != want {
		t.Logf("want:\n%s", want)
		t.Logf("got:\n%s", got)
		t.Errorf("help.GenerateHelp() = %q, want %q", got, want)
	}
}
