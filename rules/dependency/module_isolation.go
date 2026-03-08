package dependency

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ModuleIsolation{})
}

// ModuleIsolation enforces that components don't import each other's internal layers.
type ModuleIsolation struct{}

func (r *ModuleIsolation) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/module-isolation",
		Description: "Enforce that components (modules) don't import each other's internals",
		Category:    "dependency",
		Tier:        lint.TierComponentAware,
	}
}

func (r *ModuleIsolation) Check(ctx *lint.Context) error {
	allowedCross := ctx.Options.StringSlice("allowed_cross_imports")
	publicLayers := ctx.Options.StringSlice("public_layers")
	if len(publicLayers) == 0 {
		publicLayers = []string{"model"}
	}

	allowedPairs := make(map[string]bool)
	for _, pair := range allowedCross {
		allowedPairs[pair] = true
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		sourceComp := file.Location.Component
		if sourceComp == "" {
			return nil
		}

		for _, imp := range file.Imports {
			if !ctx.Analyzer.IsInternalImport(imp.Path) {
				continue
			}

			iloc := ctx.Analyzer.ImportLocation(imp.Path)
			if iloc.Component == "" || iloc.Component == sourceComp {
				continue
			}

			// Check if this is a public layer
			if contains(publicLayers, iloc.Layer) {
				continue
			}

			// Check if this cross-import is explicitly allowed
			pairKey := fmt.Sprintf("%s->%s", sourceComp, iloc.Component)
			if allowedPairs[pairKey] {
				continue
			}

			msg := fmt.Sprintf("component %q imports internal layer %q of component %q", sourceComp, iloc.Layer, iloc.Component)
			if iloc.Layer == "" {
				msg = fmt.Sprintf("component %q imports internals of component %q", sourceComp, iloc.Component)
			}

			ctx.Report.Add(lint.Violation{
				Rule:     "dependency/module-isolation",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     imp.Pos.Line,
				Message:  msg,
				Found:    strings.TrimPrefix(imp.Path, ctx.Analyzer.ModulePath()+"/"),
			})
		}
		return nil
	})
}
