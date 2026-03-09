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
	defaultOrder := ctx.Options.StringSlice("order")
	if len(defaultOrder) == 0 {
		defaultOrder = defaultDeclOrder
	}

	// Build per-layer overrides
	layerOverrides := make(map[string][]string)
	if overridesRaw := ctx.Options.Map("layer_overrides"); overridesRaw != nil {
		for layer, orderRaw := range overridesRaw {
			switch v := orderRaw.(type) {
			case []any:
				var o []string
				for _, item := range v {
					if s, ok := item.(string); ok {
						o = append(o, s)
					}
				}
				layerOverrides[layer] = o
			case []string:
				layerOverrides[layer] = v
			}
		}
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		// Determine which order to use based on file's layer
		order := defaultOrder
		if file.Location.Layer != "" {
			if override, ok := layerOverrides[file.Location.Layer]; ok {
				order = override
			}
		}

		// Build priority map: category -> index
		priority := make(map[string]int, len(order))
		for i, cat := range order {
			priority[cat] = i
		}

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
