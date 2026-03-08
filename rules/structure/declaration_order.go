package structure

import (
	"fmt"
	"go/ast"
	"go/token"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&DeclarationOrder{})
}

// DeclarationOrder enforces ordering of declarations within a file.
type DeclarationOrder struct{}

func (r *DeclarationOrder) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/declaration-order",
		Description: "Enforce ordering of declarations within a file",
		Category:    "structure",
		Tier:        lint.TierUniversal,
	}
}

var defaultDeclOrder = []string{"const", "var", "interface", "struct", "func"}

func (r *DeclarationOrder) Check(ctx *lint.Context) error {
	order := ctx.Options.StringSlice("order")
	if len(order) == 0 {
		order = defaultDeclOrder
	}

	// Build priority map: category -> index
	priority := make(map[string]int, len(order))
	for i, cat := range order {
		priority[cat] = i
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		lastPriority := -1
		lastCategory := ""

		for _, decl := range file.AST.Decls {
			cat, pos := classifyDecl(decl, file.FileSet)
			if cat == "" {
				continue
			}
			p, known := priority[cat]
			if !known {
				continue
			}
			if p < lastPriority {
				ctx.Report.Add(lint.Violation{
					Rule:     "structure/declaration-order",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     pos.Line,
					Message:  fmt.Sprintf("%s declaration should appear before %s declarations", cat, lastCategory),
					Found:    cat,
					Expected: lastCategory,
				})
			}
			if p >= lastPriority {
				lastPriority = p
				lastCategory = cat
			}
		}
		return nil
	})
}

func classifyDecl(decl ast.Decl, fset *token.FileSet) (string, token.Position) {
	switch d := decl.(type) {
	case *ast.GenDecl:
		switch d.Tok {
		case token.CONST:
			return "const", fset.Position(d.Pos())
		case token.VAR:
			return "var", fset.Position(d.Pos())
		case token.TYPE:
			// Determine if interface or struct based on first spec
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch ts.Type.(type) {
				case *ast.InterfaceType:
					return "interface", fset.Position(d.Pos())
				case *ast.StructType:
					return "struct", fset.Position(d.Pos())
				}
			}
			return "struct", fset.Position(d.Pos())
		}
	case *ast.FuncDecl:
		return "func", fset.Position(d.Pos())
	}
	return "", token.Position{}
}
