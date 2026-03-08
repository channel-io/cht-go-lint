package ddd

import (
	"fmt"
	"go/ast"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&AggregateBoundary{})
}

// AggregateBoundary ensures aggregate roots do not directly reference other aggregates.
type AggregateBoundary struct{}

func (r *AggregateBoundary) Meta() lint.Meta {
	return lint.Meta{
		Name:        "ddd/aggregate-boundary",
		Description: "Aggregate roots must not directly reference other aggregates",
		Category:    "ddd",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *AggregateBoundary) Check(ctx *lint.Context) error {
	rootMarker := ctx.Options.String("root_marker", "Aggregate")
	noCrossRef := ctx.Options.Bool("no_cross_aggregate_reference", true)
	if !noCrossRef {
		return nil
	}

	// First pass: collect all aggregate root type names
	aggregateRoots := make(map[string]bool)
	ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, t := range file.Types {
			if t.IsStruct && strings.HasSuffix(t.Name, rootMarker) {
				aggregateRoots[t.Name] = true
			}
		}
		return nil
	})

	if len(aggregateRoots) == 0 {
		return nil
	}

	// Second pass: check struct fields of aggregate roots
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
				if !aggregateRoots[ts.Name.Name] {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok || st.Fields == nil {
					continue
				}
				for _, field := range st.Fields.List {
					refName := extractTypeName(field.Type)
					if refName != "" && refName != ts.Name.Name && aggregateRoots[refName] {
						pos := file.FileSet.Position(field.Pos())
						ctx.Report.Add(lint.Violation{
							Rule:     "ddd/aggregate-boundary",
							Severity: ctx.Severity,
							File:     file.RelPath,
							Line:     pos.Line,
							Message:  fmt.Sprintf("aggregate %q must not reference aggregate %q directly", ts.Name.Name, refName),
							Found:    refName,
						})
					}
				}
			}
		}
		return nil
	})
}

// extractTypeName extracts the base type name from a field type expression,
// stripping pointers, slices, and maps.
func extractTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return extractTypeName(t.X)
	case *ast.ArrayType:
		return extractTypeName(t.Elt)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}
