package dependency

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&CrossBoundary{})
}

// CrossBoundary enforces that cross-component imports only use public interface layers.
type CrossBoundary struct{}

func (r *CrossBoundary) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/cross-boundary",
		Description: "Cross-component imports must use public interfaces only",
		Category:    "dependency",
		Tier:        lint.TierComponentAware,
	}
}

func (r *CrossBoundary) Check(ctx *lint.Context) error {
	boundaryLayer := ctx.Options.String("boundary_layer", "publicsvc")
	allowModel := ctx.Options.Bool("allow_model_import", true)

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

			// Allow the designated boundary layer
			if iloc.Layer == boundaryLayer {
				continue
			}

			// Optionally allow model imports
			if allowModel && iloc.Layer == "model" {
				continue
			}

			ctx.Report.Add(lint.Violation{
				Rule:     "dependency/cross-boundary",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     imp.Pos.Line,
				Message:  fmt.Sprintf("cross-component import from %q to %q must use boundary layer %q", sourceComp, iloc.Component, boundaryLayer),
				Found:    strings.TrimPrefix(imp.Path, ctx.Analyzer.ModulePath()+"/"),
				Expected: fmt.Sprintf("import via %q layer", boundaryLayer),
			})
		}
		return nil
	})
}
