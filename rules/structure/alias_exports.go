package structure

import (
	"fmt"
	"go/ast"
	"go/token"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&AliasExports{})
}

// AliasExports enforces that alias.go files export a Public Service type alias
// (e.g., type Svc = svc.Public or type Public = svc.Public).
type AliasExports struct{}

func (r *AliasExports) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/alias-exports",
		Description: "alias.go must export a Public Service type alias (Svc or Public)",
		Category:    "structure",
		Tier:        lint.TierComponentAware,
	}
}

func (r *AliasExports) Check(ctx *lint.Context) error {
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		if file.Location.Tag("isAlias") != "true" {
			return nil
		}

		if hasPublicServiceAlias(file.AST) {
			return nil
		}

		ctx.Report.Add(lint.Violation{
			Rule:     "structure/alias-exports",
			Severity: ctx.Severity,
			File:     file.RelPath,
			Line:     1,
			Message:  fmt.Sprintf("alias.go must export Public Service: type Svc = svc.Public"),
			Expected: "type Svc = ... or type Public = ...",
		})
		return nil
	})
}

// hasPublicServiceAlias inspects the AST for a type alias named "Svc" or "Public"
// (i.e., a TypeSpec where Assign is valid, indicating `type X = Y`).
func hasPublicServiceAlias(f *ast.File) bool {
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
			if (ts.Name.Name == "Svc" || ts.Name.Name == "Public") && ts.Assign.IsValid() {
				return true
			}
		}
	}
	return false
}
