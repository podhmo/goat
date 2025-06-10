package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Package represents a single Go package.
// Its AST and resolved imports are loaded lazily.
type Package struct {
	Name       string          // Package name
	ImportPath string          // Import path
	Dir        string          // Directory containing package sources
	GoFiles    []string        // Go source files (non-test) relative to Dir
	RawMeta    PackageMetaInfo // Raw metadata from locator

	loader *Loader        // The loader that loaded this package
	fset   *token.FileSet // Reference to the loader's FileSet, primarily for parsing.

	parseOnce   sync.Once
	parsedFiles map[string]*ast.File // filename -> AST, parsed lazily
	parseErr    error

	resolveImportsOnce sync.Once
	// resolvedImports maps import path to resolved Package.
	// This is populated on demand.
	resolvedImports   map[string]*Package
	resolveImportsErr error

	// fileImports maps Go filename (relative to Dir) to its ast.ImportSpec.
	// This is populated when ASTs are parsed.
	fileImports map[string][]*ast.ImportSpec
}

// NewPackage creates a new Package instance from metadata.
// The loader is used to resolve dependencies later.
func NewPackage(meta PackageMetaInfo, loader *Loader) *Package {
	return &Package{
		Name:            meta.Name,
		ImportPath:      meta.ImportPath,
		Dir:             meta.Dir,
		GoFiles:         meta.GoFiles,
		RawMeta:         meta, // Keep original meta
		loader:          loader,
		fset:            loader.fset, // Store fset from loader
		parsedFiles:     make(map[string]*ast.File),
		resolvedImports: make(map[string]*Package),
		fileImports:     make(map[string][]*ast.ImportSpec),
	}
}

// ensureParsed parses all GoFiles in the package if not already done.
// It populates p.parsedFiles and p.fileImports.
func (p *Package) ensureParsed() error {
	p.parseOnce.Do(func() {
		// p.fset should be used here, taken from loader at construction
		if p.fset == nil {
			p.parseErr = fmt.Errorf("FileSet is nil in package %s, cannot parse", p.ImportPath)
			return
		}
		for _, goFile := range p.GoFiles {
			path := filepath.Join(p.Dir, goFile)
			fileAST, err := parser.ParseFile(p.fset, path, nil, parser.ParseComments)
			if err != nil {
				p.parseErr = fmt.Errorf("failed to parse %s: %w", path, err)
				return
			}
			p.parsedFiles[goFile] = fileAST

			// Collect import specs
			var imports []*ast.ImportSpec
			for _, importSpec := range fileAST.Imports {
				imports = append(imports, importSpec)
			}
			p.fileImports[goFile] = imports
		}
		// If Name wasn't available from locator, try to get it from AST
		if p.Name == "" && len(p.parsedFiles) > 0 {
			for _, fAST := range p.parsedFiles {
				p.Name = fAST.Name.Name
				break
			}
		}
	})
	return p.parseErr
}

// Files returns the parsed ASTs for all Go files in the package.
// It triggers parsing if not already done.
func (p *Package) Files() (map[string]*ast.File, error) {
	if err := p.ensureParsed(); err != nil {
		return nil, err
	}
	return p.parsedFiles, nil
}

// ResolveImport resolves an import path declared within this package
// to its corresponding Package object.
// The importPath must be the canonical, unquoted import path string.
func (p *Package) ResolveImport(importPath string) (*Package, error) {
	// First, ensure this package's ASTs are parsed to know its declared imports.
	if err := p.ensureParsed(); err != nil {
		return nil, fmt.Errorf("cannot resolve import %q from %q: failed to parse source package: %w", importPath, p.ImportPath, err)
	}

	// Check if this importPath is actually declared in this package's files.
	// This logic needs refinement: rawMeta.Imports might be better if available and reliable.
	// For now, assume the caller knows the importPath is valid for this package.
	var foundInFileImports bool
	for _, specs := range p.fileImports {
		for _, spec := range specs {
			unquoted, err := strconv.Unquote(spec.Path.Value)
			if err == nil && unquoted == importPath {
				foundInFileImports = true
				break
			}
		}
		if foundInFileImports {
			break
		}
	}
	// Or check rawMeta if it contains direct imports
	if !foundInFileImports && p.RawMeta.DirectImports != nil {
		for _, rawImp := range p.RawMeta.DirectImports {
			if rawImp == importPath {
				foundInFileImports = true
				break
			}
		}
	}

	if !foundInFileImports {
		return nil, fmt.Errorf("import path %q not declared in package %q", importPath, p.ImportPath)
	}

	// Check cache within the package instance first
	if resolved, ok := p.resolvedImports[importPath]; ok {
		return resolved, nil
	}

	// Ask the loader to resolve it
	resolvedPkg, err := p.loader.resolveImport(p.ImportPath, importPath)
	if err != nil {
		return nil, err // Error already includes context
	}

	p.resolvedImports[importPath] = resolvedPkg
	return resolvedPkg, nil
}

// GetStruct attempts to find a struct type definition by name within the package.
// This is a simplified example; a real implementation would be more robust.
func (p *Package) GetStruct(name string) (*StructInfo, error) {
	if err := p.ensureParsed(); err != nil {
		return nil, err
	}

	for _, fileName := range p.GoFiles { // Iterate in defined order for consistency
		fileAST, ok := p.parsedFiles[fileName]
		if !ok {
			// This should not happen if ensureParsed worked
			return nil, fmt.Errorf("AST for file %s not found in package %s", fileName, p.Name)
		}

		var foundStruct *ast.StructType
		var foundSpec *ast.TypeSpec

		ast.Inspect(fileAST, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				if ts.Name.Name == name {
					if st, ok := ts.Type.(*ast.StructType); ok {
						foundSpec = ts
						foundStruct = st
						return false // Stop inspection
					}
				}
			}
			return true
		})

		if foundStruct != nil {
			structInfo := &StructInfo{
				PackagePath: p.ImportPath,
				Name:        name,
				Node:        foundSpec,            // Store TypeSpec for comments, etc.
				Fields:      make([]FieldInfo, 0), // Initialize to empty slice
				pkg:         p,                    // Store reference to current package
			}
			// Set ParentStruct for FieldInfos after structInfo is initialized.
			if foundStruct.Fields != nil {
				for _, field := range foundStruct.Fields.List {
					for _, fieldName := range field.Names {
						fi := FieldInfo{
							Name:         fieldName.Name,
							TypeExpr:     field.Type,
							ParentStruct: structInfo, // Set ParentStruct
						}
						if field.Tag != nil {
							fi.Tag = field.Tag.Value
						}
						structInfo.Fields = append(structInfo.Fields, fi)
					}
					// Handle embedded fields (Names is nil)
					if len(field.Names) == 0 && field.Type != nil {
						fi := FieldInfo{
							Name:         "", // Embedded field
							TypeExpr:     field.Type,
							Embedded:     true,
							ParentStruct: structInfo, // Set ParentStruct
						}
						if field.Tag != nil {
							fi.Tag = field.Tag.Value
						}
						structInfo.Fields = append(structInfo.Fields, fi)
					}
				}
			}
			return structInfo, nil
		}
	}
	return nil, fmt.Errorf("struct %q not found in package %q", name, p.Name)
}

// FindTypeSpec searches for an *ast.TypeSpec by name within the package.
// It returns the found TypeSpec, the package it belongs to (p), and an error if not found.
func (p *Package) FindTypeSpec(typeName string) (*ast.TypeSpec, *Package, error) {
	if err := p.ensureParsed(); err != nil {
		return nil, nil, fmt.Errorf("error ensuring package %s is parsed: %w", p.ImportPath, err)
	}

	for _, fileAST := range p.parsedFiles {
		var foundSpec *ast.TypeSpec
		ast.Inspect(fileAST, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				if ts.Name.Name == typeName {
					foundSpec = ts
					return false // Stop inspection for this file
				}
			}
			return true
		})
		if foundSpec != nil {
			return foundSpec, p, nil
		}
	}
	return nil, nil, fmt.Errorf("typespec %q not found in package %q", typeName, p.ImportPath)
}

// FindInterface searches for an interface type by name within the package.
// It returns the *ast.InterfaceType, its parent *ast.TypeSpec, and an error if not found.
func (p *Package) FindInterface(interfaceName string) (*ast.InterfaceType, *ast.TypeSpec, error) {
	typeSpec, _, err := p.FindTypeSpec(interfaceName)
	if err != nil {
		return nil, nil, err // Error already specifies typeName and package
	}
	if typeSpec == nil { // Should be covered by error from FindTypeSpec, but defensive
		return nil, nil, fmt.Errorf("typespec %q not found in package %q for interface search", interfaceName, p.ImportPath)
	}

	if interfaceType, ok := typeSpec.Type.(*ast.InterfaceType); ok {
		return interfaceType, typeSpec, nil
	}

	return nil, nil, fmt.Errorf("type %q in package %q is not an interface", interfaceName, p.ImportPath)
}

// GetMethodsForType collects all *ast.FuncDecl that have typeName or *typeName as their receiver.
func (p *Package) GetMethodsForType(typeName string) ([]*ast.FuncDecl, error) {
	if err := p.ensureParsed(); err != nil {
		return nil, fmt.Errorf("error ensuring package %s is parsed for GetMethodsForType: %w", p.ImportPath, err)
	}

	var methods []*ast.FuncDecl
	starTypeName := "*" + typeName

	for _, fileAST := range p.parsedFiles {
		for _, decl := range fileAST.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					recvField := fn.Recv.List[0]
					// Convert receiver type to string for comparison
					var recvTypeName string
					switch t := recvField.Type.(type) {
					case *ast.Ident:
						recvTypeName = t.Name
					case *ast.StarExpr:
						if ident, ok := t.X.(*ast.Ident); ok {
							recvTypeName = "*" + ident.Name
						}
					}

					if recvTypeName == typeName || recvTypeName == starTypeName {
						methods = append(methods, fn)
					}
				}
			}
		}
	}
	return methods, nil
}

// GetImportPathBySelector resolves a package selector (e.g., "json" from json.Marshal)
// used in a given astFile to its full import path and the resolved *Package.
// It uses the package's own resolved imports.
func (p *Package) GetImportPathBySelector(selectorName string, astFile *ast.File) (string, *Package, error) {
	if astFile == nil {
		return "", nil, fmt.Errorf("astFile cannot be nil for GetImportPathBySelector in package %s", p.ImportPath)
	}

	for _, importSpec := range astFile.Imports {
		importPath := strings.Trim(importSpec.Path.Value, "\"")
		// Resolve the import using the package's context to ensure it's loaded/cached.
		resolvedPkg, err := p.ResolveImport(importPath)
		if err != nil {
			// Could log this error or collect, but for now, if one import fails to resolve,
			// it might not be the one we're looking for anyway.
			continue
		}

		if importSpec.Name != nil { // Aliased import
			if importSpec.Name.Name == selectorName {
				return resolvedPkg.ImportPath, resolvedPkg, nil
			}
		} else { // Standard import (name derived from package's actual name)
			if resolvedPkg.Name == selectorName {
				return resolvedPkg.ImportPath, resolvedPkg, nil
			}
		}
	}
	return "", nil, fmt.Errorf("selector %q not found or resolved in imports of file belonging to package %q", selectorName, p.ImportPath)
}
