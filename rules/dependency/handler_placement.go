package dependency

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&HandlerPlacement{})
}

// HandlerPlacement enforces that handler layer files only import allowed layers.
type HandlerPlacement struct{}

func (r *HandlerPlacement) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/handler-placement",
		Description: "Handler layer files should only import allowed layers",
		Category:    "dependency",
		Tier:        lint.TierLayerAware,
	}
}

func (r *HandlerPlacement) Check(ctx *lint.Context) error {
	handlerLayers := ctx.Options.StringSlice("handler_layers")
	if len(handlerLayers) == 0 {
		handlerLayers = []string{"handler"}
	}

	allowedImports := ctx.Options.StringSlice("allowed_imports")
	if len(allowedImports) == 0 {
		allowedImports = []string{"model", "service", "appsvc", "publicsvc"}
	}

	return ctx.Analyzer.WalkGoFiles(func(_ string, file *lint.ParsedFile) error {
		if !contains(handlerLayers, file.Location.Layer) {
			return nil
		}

		for _, imp := range file.Imports {
			if !ctx.Analyzer.IsInternalImport(imp.Path) {
				continue
			}

			iloc := ctx.Analyzer.ImportLocation(imp.Path)
			if iloc.Layer == "" {
				continue
			}

			// Allow importing from the same handler layer
			if contains(handlerLayers, iloc.Layer) {
				continue
			}

			if !contains(allowedImports, iloc.Layer) {
				ctx.Report.Add(lint.Violation{
					Rule:     "dependency/handler-placement",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     imp.Pos.Line,
					Message:  fmt.Sprintf("handler layer must not import layer %q", iloc.Layer),
					Found:    iloc.Layer,
					Expected: fmt.Sprintf("one of %v", allowedImports),
				})
			}
		}
		return nil
	})
}
