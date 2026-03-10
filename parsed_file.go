package lint

import (
	"go/ast"
	"go/token"
	"strings"
)

// ParsedFile holds parsed information about a single Go source file.
type ParsedFile struct {
	Path     string     // absolute path
	RelPath  string     // relative to project root
	Package  string     // package name
	Imports  []Import   // import declarations
	Types    []TypeDecl // type declarations
	Funcs    []FuncDecl // function declarations
	AST      *ast.File
	FileSet  *token.FileSet
	Location Location // architectural location (set by strategy)
}

// Import represents a single import declaration.
type Import struct {
	Path  string         // import path (e.g., "fmt")
	Alias string         // import alias (empty for default, "." for dot import)
	Pos   token.Position // source position
}

// TypeDecl represents a type declaration.
type TypeDecl struct {
	Name        string
	Exported    bool
	IsInterface bool
	IsStruct    bool
	IsAlias     bool     // type X = Y (type alias)
	Embedded    []string // embedded type names
	Methods     []string // method names (for interfaces)
	Pos         token.Position
}

// FuncDecl represents a function or method declaration.
type FuncDecl struct {
	Name          string
	Exported      bool
	ReceiverType  string // empty if not a method
	ReturnTypes   []string
	IsConstructor bool // starts with "New"
	Pos           token.Position
	Body          *ast.BlockStmt // for advanced body analysis
}

// --- Extraction functions ---

func extractImports(f *ast.File, fset *token.FileSet) []Import {
	var imports []Import
	for _, imp := range f.Imports {
		path := imp.Path.Value
		// Remove quotes
		if len(path) >= 2 {
			path = path[1 : len(path)-1]
		}
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		imports = append(imports, Import{
			Path:  path,
			Alias: alias,
			Pos:   fset.Position(imp.Pos()),
		})
	}
	return imports
}

func extractTypes(f *ast.File, fset *token.FileSet) []TypeDecl {
	var types []TypeDecl
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			td := TypeDecl{
				Name:     ts.Name.Name,
				Exported: ts.Name.IsExported(),
				IsAlias:  ts.Assign.IsValid(),
				Pos:      fset.Position(ts.Pos()),
			}
			switch t := ts.Type.(type) {
			case *ast.InterfaceType:
				td.IsInterface = true
				if t.Methods != nil {
					for _, m := range t.Methods.List {
						if len(m.Names) > 0 {
							td.Methods = append(td.Methods, m.Names[0].Name)
						} else if ident, ok := m.Type.(*ast.Ident); ok {
							td.Embedded = append(td.Embedded, ident.Name)
						} else if sel, ok := m.Type.(*ast.SelectorExpr); ok {
							if x, ok := sel.X.(*ast.Ident); ok {
								td.Embedded = append(td.Embedded, x.Name+"."+sel.Sel.Name)
							}
						}
					}
				}
			case *ast.StructType:
				td.IsStruct = true
				if t.Fields != nil {
					for _, field := range t.Fields.List {
						if len(field.Names) == 0 { // embedded field
							switch ft := field.Type.(type) {
							case *ast.Ident:
								td.Embedded = append(td.Embedded, ft.Name)
							case *ast.SelectorExpr:
								if x, ok := ft.X.(*ast.Ident); ok {
									td.Embedded = append(td.Embedded, x.Name+"."+ft.Sel.Name)
								}
							case *ast.StarExpr:
								if ident, ok := ft.X.(*ast.Ident); ok {
									td.Embedded = append(td.Embedded, ident.Name)
								}
							}
						}
					}
				}
			}
			types = append(types, td)
		}
	}
	return types
}

func extractFuncs(f *ast.File, fset *token.FileSet) []FuncDecl {
	var funcs []FuncDecl
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		fd := FuncDecl{
			Name:     fn.Name.Name,
			Exported: fn.Name.IsExported(),
			Pos:      fset.Position(fn.Pos()),
			Body:     fn.Body,
		}
		// Receiver type
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			fd.ReceiverType = typeToString(fn.Recv.List[0].Type)
		}
		// Return types
		if fn.Type.Results != nil {
			for _, result := range fn.Type.Results.List {
				fd.ReturnTypes = append(fd.ReturnTypes, typeToString(result.Type))
			}
		}
		// Constructor detection
		if len(fd.Name) > 3 && fd.Name[:3] == "New" && fd.Exported {
			fd.IsConstructor = true
		}
		funcs = append(funcs, fd)
	}
	return funcs
}

func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.ArrayType:
		return "[]" + typeToString(t.Elt)
	case *ast.MapType:
		return "map[" + typeToString(t.Key) + "]" + typeToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + typeToString(t.Value)
	case *ast.IndexExpr:
		// Generic type with single type parameter: T[X]
		return typeToString(t.X) + "[" + typeToString(t.Index) + "]"
	case *ast.IndexListExpr:
		// Generic type with multiple type parameters: T[X, Y]
		params := make([]string, len(t.Indices))
		for i, idx := range t.Indices {
			params[i] = typeToString(idx)
		}
		return typeToString(t.X) + "[" + strings.Join(params, ", ") + "]"
	}
	return ""
}
