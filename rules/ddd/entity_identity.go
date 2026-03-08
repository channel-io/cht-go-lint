package ddd

import (
	"fmt"
	"go/ast"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&EntityIdentity{})
}

// EntityIdentity ensures entity types have an ID field.
type EntityIdentity struct{}

func (r *EntityIdentity) Meta() lint.Meta {
	return lint.Meta{
		Name:        "ddd/entity-identity",
		Description: "Entity types should have an ID field",
		Category:    "ddd",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *EntityIdentity) Check(ctx *lint.Context) error {
	entityMarkers := ctx.Options.StringSlice("entity_markers")
	if len(entityMarkers) == 0 {
		entityMarkers = []string{"Entity"}
	}
	idField := ctx.Options.String("id_field", "ID")

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, decl := range file.AST.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range genDecl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if !isEntity(ts.Name.Name, entityMarkers) {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				if !hasField(st, idField) {
					pos := file.FileSet.Position(ts.Pos())
					ctx.Report.Add(lint.Violation{
						Rule:     "ddd/entity-identity",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     pos.Line,
						Message:  fmt.Sprintf("entity %q must have an %q field", ts.Name.Name, idField),
						Expected: idField,
					})
				}
			}
		}
		return nil
	})
}

func isEntity(name string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func hasField(st *ast.StructType, fieldName string) bool {
	for _, field := range st.Fields.List {
		for _, name := range field.Names {
			if name.Name == fieldName {
				return true
			}
		}
	}
	return false
}
