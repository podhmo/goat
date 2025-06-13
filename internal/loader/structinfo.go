package loader

import (
	"fmt"
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
	// ParentStruct allows FieldInfo to access its containing struct's context, like its package.
	ParentStruct *StructInfo
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

// ImplementsInterface checks if the field's type implements the target interface.
// targetInterfacePackagePath is the full import path of the package defining the interface.
// targetInterfaceName is the name of the interface.
// This is a simplified AST-based check and has limitations (e.g., complex type matching in signatures).
func (fi *FieldInfo) ImplementsInterface(ctx context.Context, targetInterfacePackagePath string, targetInterfaceName string) (bool, error) {
	if fi.ParentStruct == nil || fi.ParentStruct.pkg == nil {
		return false, fmt.Errorf("FieldInfo has no ParentStruct or parent package context")
	}
	currentPackage := fi.ParentStruct.pkg

	// 1. Determine the field's base type name and its defining package.
	var actualTypeName string
	var actualPkg *Package
	var isPointer bool
	var typeForAnalysis ast.Expr = fi.TypeExpr

	if star, ok := typeForAnalysis.(*ast.StarExpr); ok {
		isPointer = true
		typeForAnalysis = star.X
	}
	_ = isPointer // Acknowledge isPointer, it might be used in more advanced signature checks later.

	switch t := typeForAnalysis.(type) {
	case *ast.Ident:
		actualTypeName = t.Name
		actualPkg = currentPackage // Type defined in the same package as the struct
	case *ast.SelectorExpr:
		pkgSelectorIdent, ok := t.X.(*ast.Ident)
		if !ok {
			return false, fmt.Errorf("unsupported selector expression for field type: %T", t.X)
		}
		// Find the AST file where the parent struct is defined to resolve imports.
		var structDefiningFile *ast.File
		parentStructNode := fi.ParentStruct.Node
		for _, fileAST := range currentPackage.parsedFiles { // Assumes parsedFiles is populated
			ast.Inspect(fileAST, func(n ast.Node) bool {
				if n == parentStructNode {
					structDefiningFile = fileAST
					return false
				}
				return true
			})
			if structDefiningFile != nil {
				break
			}
		}
		if structDefiningFile == nil {
			return false, fmt.Errorf("could not find defining AST file for struct %s", fi.ParentStruct.Name)
		}

		_, definingPkg, err := currentPackage.GetImportPathBySelector(ctx, pkgSelectorIdent.Name, structDefiningFile)
		if err != nil {
			return false, fmt.Errorf("failed to resolve package for selector '%s' in field '%s': %w", pkgSelectorIdent.Name, fi.Name, err)
		}
		actualPkg = definingPkg
		actualTypeName = t.Sel.Name
	default:
		return false, fmt.Errorf("unsupported field type expression %T for ImplementsInterface check", fi.TypeExpr)
	}

	if actualPkg == nil { // Should be caught by earlier errors, but defensive.
		return false, fmt.Errorf("could not determine defining package for type of field '%s'", fi.Name)
	}

	// 2. Get methods of the field's type.
	// GetMethodsForType should inherently handle methods on T and *T if typeName is base name.
	// If isPointer is true, we are checking if *T implements, so methods on *T are primary.
	// If !isPointer, we are checking if T implements, so methods on T are primary.
	// The current GetMethodsForType implementation checks both T and *T, which is good.
	methods, err := actualPkg.GetMethodsForType(actualTypeName)
	if err != nil {
		return false, fmt.Errorf("failed to get methods for type '%s' in package '%s': %w", actualTypeName, actualPkg.ImportPath, err)
	}

	// 3. Load the target interface definition.
	var interfaceDefPkg *Package
	if targetInterfacePackagePath == "" || targetInterfacePackagePath == currentPackage.ImportPath {
		interfaceDefPkg = currentPackage
	} else if targetInterfacePackagePath == actualPkg.ImportPath {
		// if interface is in the same package as the field's type definition itself
		interfaceDefPkg = actualPkg
	} else {
		// Resolve the interface's package using the loader from the struct's package context.
		// The loader needs a 'from' path for context, using currentPackage.ImportPath.
		resolvedInterfacePkg, err := currentPackage.loader.resolveImport(ctx, currentPackage.ImportPath, targetInterfacePackagePath)
		if err != nil {
			return false, fmt.Errorf("failed to resolve interface package '%s': %w", targetInterfacePackagePath, err)
		}
		interfaceDefPkg = resolvedInterfacePkg
	}

	interfaceTypeAst, _, err := interfaceDefPkg.FindInterface(targetInterfaceName)
	if err != nil {
		return false, fmt.Errorf("failed to find interface '%s' in package '%s': %w", targetInterfaceName, interfaceDefPkg.ImportPath, err)
	}
	if interfaceTypeAst == nil || interfaceTypeAst.Methods == nil {
		return false, fmt.Errorf("interface '%s' in package '%s' has no methods defined or is invalid", targetInterfaceName, interfaceDefPkg.ImportPath)
	}

	// 4. For each method in the interface, check if a corresponding method exists on the type.
	// This is a simplified check: names must match, and param/result counts must match.
	// Type matching is by string representation via astutils.ExprToTypeName, which has limitations.
	for _, ifaceMethodField := range interfaceTypeAst.Methods.List {
		if len(ifaceMethodField.Names) == 0 {
			continue // Should not happen for valid interface methods
		}
		ifaceMethodName := ifaceMethodField.Names[0].Name
		ifaceFuncType, ok := ifaceMethodField.Type.(*ast.FuncType)
		if !ok {
			return false, fmt.Errorf("interface method %s is not a func type", ifaceMethodName)
		}

		foundMatchingMethod := false
		for _, actualMethod := range methods {
			if actualMethod.Name.Name == ifaceMethodName {
				// Basic signature check (name already matches)
				actualFuncType := actualMethod.Type
				if len(ifaceFuncType.Params.List) == len(actualFuncType.Params.List) &&
					(ifaceFuncType.Results == nil && actualFuncType.Results == nil ||
						(ifaceFuncType.Results != nil && actualFuncType.Results != nil &&
							len(ifaceFuncType.Results.List) == len(actualFuncType.Results.List))) {
					// TODO: Enhance signature comparison. Current check is very basic (counts only).
					// A more robust check would use astutils.ExprToTypeName or (ideally) type checking,
					// but astutils.ExprToTypeName needs package context for selectors.
					// For this simplified version, matching counts is a starting point.
					foundMatchingMethod = true
					break
				}
			}
		}
		if !foundMatchingMethod {
			// fmt.Printf("Method %s not found or signature mismatch for type %s implementing %s.%s\n", ifaceMethodName, actualTypeName, targetInterfacePackagePath, targetInterfaceName)
			return false, nil // Method not found or signature mismatch (simplified)
		}
	}

	return true, nil
}
