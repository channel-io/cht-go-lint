package dependency

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&LayerDirection{})
}

// LayerDirection enforces that imports follow the configured layer dependency graph.
type LayerDirection struct{}

func (r *LayerDirection) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/layer-direction",
		Description: "Enforce allowed import direction between layers",
		Category:    "dependency",
		Tier:        lint.TierLayerAware,
	}
}

func (r *LayerDirection) Check(ctx *lint.Context) error {
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		sourceLayer := file.Location.Layer
		if sourceLayer == "" {
			return nil
		}

		allowed, ok := ctx.Config.LayerMayImport(sourceLayer)
		if !ok {
			return nil
		}

		for _, imp := range file.Imports {
			if !ctx.Analyzer.IsInternalImport(imp.Path) {
				continue
			}

			iloc := ctx.Analyzer.ImportLocation(imp.Path)
			if iloc.Layer == "" || iloc.Layer == sourceLayer {
				continue
			}

			if !contains(allowed, iloc.Layer) {
				ctx.Report.Add(lint.Violation{
					Rule:     "dependency/layer-direction",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     imp.Pos.Line,
					Message:  fmt.Sprintf("layer %q must not import layer %q", sourceLayer, iloc.Layer),
					Found:    iloc.Layer,
					Expected: fmt.Sprintf("one of %v", allowed),
				})
			}
		}
		return nil
	})
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
