package structure

import (
	"go/ast"
	"go/token"
	"os"
	"sort"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.RegisterFixer(&DeclarationOrderFixer{})
}

// DeclarationOrderFixer reorders top-level declarations to match the configured order.
type DeclarationOrderFixer struct{}

func (f *DeclarationOrderFixer) FixMeta() lint.FixMeta {
	return lint.FixMeta{
		RuleName:    "structure/declaration-order",
		Description: "Reorder top-level declarations to match configured order",
	}
}

var defaultOrder = []string{"const", "var", "interface", "struct", "func"}

func (f *DeclarationOrderFixer) Fix(ctx *lint.FixContext) error {
	order := ctx.Options.StringSlice("order")
	if len(order) == 0 {
		order = defaultOrder
	}

	priority := buildPriority(order)

	// Build per-layer overrides
	layerOverrides := make(map[string]map[string]int)
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
				layerOverrides[layer] = buildPriority(o)
			case []string:
				layerOverrides[layer] = buildPriority(v)
			}
		}
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		filePriority := priority
		if file.Location.Layer != "" {
			if override, ok := layerOverrides[file.Location.Layer]; ok {
				filePriority = override
			}
		}

		if !needsReorder(file.AST.Decls, filePriority) {
			return nil
		}

		ctx.RecordFix(file.RelPath, "structure/declaration-order")
		if ctx.DryRun {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		dstFile, err := decorator.Parse(src)
		if err != nil {
			return nil
		}

		sort.SliceStable(dstFile.Decls, func(i, j int) bool {
			return declPriority(dstFile.Decls[i], filePriority) <
				declPriority(dstFile.Decls[j], filePriority)
		})

		out, err := os.Create(path)
		if err != nil {
			return nil
		}
		defer out.Close()
		return decorator.Fprint(out, dstFile)
	})
}

func buildPriority(order []string) map[string]int {
	p := make(map[string]int, len(order))
	for i, cat := range order {
		p[cat] = i
	}
	return p
}

// needsReorder checks if declarations are already in the correct order using go/ast (cached).
func needsReorder(decls []ast.Decl, priority map[string]int) bool {
	lastPriority := -1
	for _, decl := range decls {
		cat := classifyASTDecl(decl)
		if cat == "" {
			continue
		}
		p, known := priority[cat]
		if !known {
			continue
		}
		if p < lastPriority {
			return true
		}
		lastPriority = p
	}
	return false
}

// classifyASTDecl classifies a go/ast declaration into a category.
func classifyASTDecl(decl ast.Decl) string {
	switch d := decl.(type) {
	case *ast.GenDecl:
		switch d.Tok {
		case token.CONST:
			return "const"
		case token.VAR:
			return "var"
		case token.TYPE:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch ts.Type.(type) {
				case *ast.InterfaceType:
					return "interface"
				case *ast.StructType:
					return "struct"
				}
			}
			return "struct"
		case token.IMPORT:
			return ""
		}
	case *ast.FuncDecl:
		return "func"
	}
	return ""
}

// classifyDstDecl classifies a dst declaration into a category.
func classifyDstDecl(decl dst.Decl) string {
	switch d := decl.(type) {
	case *dst.GenDecl:
		switch d.Tok {
		case token.CONST:
			return "const"
		case token.VAR:
			return "var"
		case token.TYPE:
			for _, spec := range d.Specs {
				ts, ok := spec.(*dst.TypeSpec)
				if !ok {
					continue
				}
				switch ts.Type.(type) {
				case *dst.InterfaceType:
					return "interface"
				case *dst.StructType:
					return "struct"
				}
			}
			return "struct"
		case token.IMPORT:
			return ""
		}
	case *dst.FuncDecl:
		return "func"
	}
	return ""
}

// declPriority returns the sort priority for a dst declaration.
// Import declarations get -1 to stay at the top.
func declPriority(decl dst.Decl, priority map[string]int) int {
	cat := classifyDstDecl(decl)
	if cat == "" {
		return -1 // imports stay at top
	}
	if p, ok := priority[cat]; ok {
		return p
	}
	return len(priority) // unknown categories go to the end
}
