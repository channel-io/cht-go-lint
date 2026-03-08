package structure

import (
	"fmt"
	"go/ast"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&DelegationOnly{})
}

// DelegationOnly enforces that methods in certain layers only delegate to another type's method.
type DelegationOnly struct{}

func (r *DelegationOnly) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/delegation-only",
		Description: "Methods in target layers should only delegate to another type's method",
		Category:    "structure",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *DelegationOnly) Check(ctx *lint.Context) error {
	targetLayers := ctx.Options.StringSlice("target_layers")
	if len(targetLayers) == 0 {
		return nil
	}
	targetSet := make(map[string]bool, len(targetLayers))
	for _, l := range targetLayers {
		targetSet[l] = true
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		layer := file.Location.Layer
		if !targetSet[layer] {
			return nil
		}

		for _, fn := range file.Funcs {
			// Only check methods (has receiver)
			if fn.ReceiverType == "" {
				continue
			}
			// Skip constructors
			if fn.IsConstructor {
				continue
			}
			if fn.Body == nil {
				continue
			}

			if !isDelegation(fn.Body) {
				ctx.Report.Add(lint.Violation{
					Rule:     "structure/delegation-only",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     fn.Pos.Line,
					Message:  fmt.Sprintf("method %q in layer %q must be a simple delegation (single return with method call)", fn.Name, layer),
				})
			}
		}
		return nil
	})
}

// isDelegation checks if a function body consists of exactly one statement
// that is a return with a single method call expression, or a single expression
// statement that is a method call (for void methods).
func isDelegation(body *ast.BlockStmt) bool {
	if len(body.List) != 1 {
		return false
	}

	stmt := body.List[0]
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		// return s.inner.Method(args...)
		if len(s.Results) != 1 {
			return false
		}
		return isMethodCall(s.Results[0])
	case *ast.ExprStmt:
		// s.inner.Method(args...) — void delegation
		return isMethodCall(s.X)
	}
	return false
}

func isMethodCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	// Must be a selector expression call: x.Method(...)
	_, ok = call.Fun.(*ast.SelectorExpr)
	return ok
}
