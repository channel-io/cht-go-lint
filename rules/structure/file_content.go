package structure

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&FileContent{})
}

// FileContent restricts what declarations a specific file may contain.
type FileContent struct{}

func (r *FileContent) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/file-content",
		Description: "Restrict what declarations a specific file may contain",
		Category:    "structure",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *FileContent) Check(ctx *lint.Context) error {
	filesOpt := ctx.Options.Map("files")
	if len(filesOpt) == 0 {
		return nil
	}

	// Parse file configs: filename -> set of allowed declaration types
	type fileRule struct {
		allow map[string]bool
	}
	fileRules := make(map[string]fileRule)
	for filename, cfg := range filesOpt {
		cfgMap, ok := cfg.(map[string]any)
		if !ok {
			continue
		}
		fr := fileRule{allow: make(map[string]bool)}
		if allowRaw, ok := cfgMap["allow"]; ok {
			switch v := allowRaw.(type) {
			case []any:
				for _, item := range v {
					if s, ok := item.(string); ok {
						fr.allow[s] = true
					}
				}
			case []string:
				for _, s := range v {
					fr.allow[s] = true
				}
			}
		}
		fileRules[filename] = fr
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		basename := filepath.Base(file.RelPath)
		fr, ok := fileRules[basename]
		if !ok {
			return nil
		}

		for _, decl := range file.AST.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						declType := classifyTypeSpec(s)
						if !fr.allow[declType] {
							pos := file.FileSet.Position(s.Pos())
							ctx.Report.Add(lint.Violation{
								Rule:     "structure/file-content",
								Severity: ctx.Severity,
								File:     file.RelPath,
								Line:     pos.Line,
								Message:  fmt.Sprintf("file %q may not contain %s declaration %q", basename, declType, s.Name.Name),
								Found:    declType,
							})
						}
					case *ast.ValueSpec:
						declType := classifyValueSpec(d.Tok)
						if !fr.allow[declType] {
							pos := file.FileSet.Position(s.Pos())
							name := ""
							if len(s.Names) > 0 {
								name = s.Names[0].Name
							}
							ctx.Report.Add(lint.Violation{
								Rule:     "structure/file-content",
								Severity: ctx.Severity,
								File:     file.RelPath,
								Line:     pos.Line,
								Message:  fmt.Sprintf("file %q may not contain %s declaration %q", basename, declType, name),
								Found:    declType,
							})
						}
					}
				}
			case *ast.FuncDecl:
				if !fr.allow["func"] {
					pos := file.FileSet.Position(d.Pos())
					ctx.Report.Add(lint.Violation{
						Rule:     "structure/file-content",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     pos.Line,
						Message:  fmt.Sprintf("file %q may not contain func declaration %q", basename, d.Name.Name),
						Found:    "func",
					})
				}
			}
		}
		return nil
	})
}

func classifyTypeSpec(ts *ast.TypeSpec) string {
	// Check if it's a type alias (has Assign position set)
	if ts.Assign.IsValid() {
		return "type_alias"
	}
	switch ts.Type.(type) {
	case *ast.InterfaceType:
		return "interface"
	case *ast.StructType:
		return "struct"
	}
	return "type_alias" // other type definitions treated as aliases
}

func classifyValueSpec(tok token.Token) string {
	switch tok {
	case token.CONST:
		return "const"
	case token.VAR:
		return "var"
	}
	return "var"
}
