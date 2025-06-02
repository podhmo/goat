package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
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
			// Determine start and end of the main function in the original source
			// Start of the function (including doc comments if any)
			// We take the position of the 'func' keyword as the definitive start of what we replace.
			// If there are doc comments, ast.Node.Pos() for FuncDecl usually includes them.
			// Let's use the provided mainFuncPos as the start of replacement.
			startOffset := mainFuncPos.Offset

			// End of the function (the closing '}')
			// mainFuncNode.End() gives the position immediately after the closing brace.
			endTokenPos := mainFuncNode.Body.Rbrace
			endOffset := fileSet.Position(endTokenPos).Offset + 1 // +1 to include the brace itself

			if startOffset < 0 || endOffset < startOffset || endOffset > len(originalContent) {
				return fmt.Errorf("invalid offsets for main function replacement: start=%d, end=%d, file_len=%d", startOffset, endOffset, len(originalContent))
			}

			var buf bytes.Buffer
			buf.Write(originalContent[:startOffset])
			buf.WriteString(newMainContent)
			// Add a newline after the new main content if it doesn't end with one,
			// and if there's content following it, or if it's at the EOF.
			if !strings.HasSuffix(newMainContent, "\n") {
				buf.WriteString("\n")
			}
			buf.Write(originalContent[endOffset:])
			newContent = buf.Bytes()
		}
	}

	// Format the generated code
	formattedContent, err := format.Source(newContent)
	if err != nil {
		// Write the unformatted content for debugging purposes if formatting fails
		// os.WriteFile(filePath+".unformatted", newContent, 0644)
		return fmt.Errorf("formatting generated code for %s: %w\nOriginal newContent was:\n%s", filePath, err, string(newContent))
	}

	// Write the modified content back to filePath
	err = os.WriteFile(filePath, formattedContent, 0644) // Default permissions
	if err != nil {
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
