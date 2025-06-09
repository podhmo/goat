package lazyload

import (
	"go/ast"
	"reflect"
	"strconv"
)

// StructInfo represents information about a struct type.
type StructInfo struct {
	PackagePath string // Import path of the package containing this struct
	Name        string
	Node        *ast.TypeSpec // The AST node for the type spec (contains comments, etc.)
	Fields      []FieldInfo
	pkg         *Package // Reference to the package for resolving field types
}

// FieldInfo represents information about a single field in a struct.
type FieldInfo struct {
	Name     string
	Tag      string   // Raw tag string (e.g., `json:"name,omitempty"`)
	TypeExpr ast.Expr // AST expression for the field's type
	Embedded bool
}

// GetTag parses the struct tag and returns the value associated with the given key.
func (fi *FieldInfo) GetTag(key string) string {
	if fi.Tag == "" {
		return ""
	}
	// Unquote the tag first if it's quoted (like from ast.BasicLit.Value)
	unquotedTag := fi.Tag
	if len(unquotedTag) >= 2 && unquotedTag[0] == '`' && unquotedTag[len(unquotedTag)-1] == '`' {
		var err error
		unquotedTag, err = strconv.Unquote(fi.Tag)
		if err != nil {
			// fallback to original if unquoting fails, though it shouldn't for valid tags
			unquotedTag = fi.Tag
		}
	}
	return reflect.StructTag(unquotedTag).Get(key)
}

// ResolveType (Conceptual): This method would be responsible for analyzing
// fi.TypeExpr and determining the actual type, potentially loading
// other packages if it's an external type.
// For example:
// func (fi *FieldInfo) ResolveType(currentPkg *Package) (ResolvedType, error) {
//    switch t := fi.TypeExpr.(type) {
//    case *ast.Ident:
//        // Could be a predeclared type, or a type in currentPkg
//    case *ast.SelectorExpr:
//        // External type: X.Sel
//        // pkgIdent, ok := t.X.(*ast.Ident)
//        // if !ok { ... error ... }
//        // Find import path for pkgIdent.Name in currentPkg
//        // importedPkg, err := currentPkg.ResolveImport(foundImportPath)
//        // return importedPkg.GetType(t.Sel.Name) // GetType would be a new method on Package
//    }
//    return nil, fmt.Errorf("type resolution not yet implemented for %T", fi.TypeExpr)
// }
