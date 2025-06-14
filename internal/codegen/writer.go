package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"strings"

	"golang.org/x/tools/imports" // Added for goimports functionality
)

// WriteMain takes the original file path, its AST, the new main function content (as string),
// and the position of the old main function, then writes the modified content.
func WriteMain(
	filePath string,
	fileSet *token.FileSet, // Added FileSet to correctly get offsets
	fileAst *ast.File,
	newMainContent string,
	mainFuncPos *token.Position, // This is the position of the 'func' keyword
) error {
	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading original file %s: %w", filePath, err)
	}

	var newContent []byte

	if mainFuncPos == nil {
		// Main function not found, append newMainContent to the end of the file.
		// Ensure there's a newline between existing content and new main, if needed.
		if len(originalContent) > 0 && originalContent[len(originalContent)-1] != '\n' {
			originalContent = append(originalContent, '\n')
		}
		originalContent = append(originalContent, '\n') // Add an extra newline for separation

		newContent = append(originalContent, []byte(newMainContent)...)
	} else {
		// Main function found, replace it.
		var mainFuncNode *ast.FuncDecl
		for _, decl := range fileAst.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
				// Check if this is the exact main function we are looking for
				// by comparing the position of 'func' keyword
				funcPos := fileSet.Position(fn.Pos())
				if funcPos.IsValid() && mainFuncPos.IsValid() && funcPos.Offset == mainFuncPos.Offset {
					mainFuncNode = fn
					break
				}
			}
		}

		if mainFuncNode == nil {
			// This case should ideally not happen if mainFuncPos is not nil
			// and points to a valid 'main' function declaration.
			// However, as a fallback, append.
			if len(originalContent) > 0 && originalContent[len(originalContent)-1] != '\n' {
				originalContent = append(originalContent, '\n')
			}
			originalContent = append(originalContent, '\n')
			newContent = append(originalContent, []byte(newMainContent)...)
		} else {
			// New line-by-line replacement logic
			originalLines := strings.Split(string(originalContent), "\n")

			// Determine the correct start line for replacement.
			// If the function has documentation comments, start replacement from the beginning of those comments.
			// Otherwise, start from the 'func' keyword.
			var mainDeclStartNode ast.Node = mainFuncNode
			if mainFuncNode.Doc != nil && len(mainFuncNode.Doc.List) > 0 {
				mainDeclStartNode = mainFuncNode.Doc
			}
			mainDeclStartLine := fileSet.Position(mainDeclStartNode.Pos()).Line // 1-based
			mainDeclEndLine := fileSet.Position(mainFuncNode.Body.Rbrace).Line  // 1-based

			var builder strings.Builder

			// Append lines before main function declaration
			// (mainDeclStartLine is 1-based, originalLines is 0-indexed)
			for i := 0; i < mainDeclStartLine-1; i++ {
				builder.WriteString(originalLines[i])
				builder.WriteString("\n")
			}

			builder.WriteString(newMainContent)
			if !strings.HasSuffix(newMainContent, "\n") {
				builder.WriteString("\n")
			}

			// Append lines after the main function's original closing brace.
			// mainDeclEndLine is the 1-based line number of the original main func's closing '}'.
			// originalLines is 0-indexed. Lines from index mainDeclEndLine onwards in originalLines
			// are the lines that came *after* the original main function's body.
			if mainDeclEndLine < len(originalLines) {
				for i := mainDeclEndLine; i < len(originalLines); i++ {
					builder.WriteString(originalLines[i])
					// Add a newline after each line, except if it's the last line from split()
					// and that last line is empty (which indicates the original file ended with a newline).
					// format.Source will handle the final trailing newline for the whole file.
					if i < len(originalLines)-1 || originalLines[i] != "" {
						builder.WriteString("\n")
					}
				}
			}

			// The call to format.Source later will ensure Go-specific formatting,
			// including a single trailing newline if appropriate for Go files.
			newContent = []byte(builder.String())
		}
	}

	// Use imports.Process to format and add/remove imports
	formattedContent, err := imports.Process(filePath, newContent, nil)
	if err != nil {
		return fmt.Errorf("processing (goimports) generated code for %s: %w\nOriginal newContent was:\n%s", filePath, err, string(newContent))
	}

	if err := os.WriteFile(filePath, formattedContent, 0644); err != nil { // Default permissions
		return fmt.Errorf("writing modified content to %s: %w", filePath, err)
	}

	return nil
}

// Helper function to ensure strings.HasSuffix works as expected with WriteString
// (it's fine, just for completeness of thought if there were complex scenarios)
func endsWithNewline(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[len(s)-1] == '\n'
}
