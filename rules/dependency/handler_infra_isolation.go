package dependency

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&HandlerInfraIsolation{})
}

// HandlerInfraIsolation enforces that handler layer files do not import
// infrastructure layers (client, infra, event) directly.
type HandlerInfraIsolation struct{}

func (r *HandlerInfraIsolation) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/handler-infra-isolation",
		Description: "Handler layer must not import infrastructure layers directly",
		Category:    "dependency",
		Tier:        lint.TierLayerAware,
	}
}

func (r *HandlerInfraIsolation) Check(ctx *lint.Context) error {
	handlerLayers := ctx.Options.StringSlice("handler_layers")
	if len(handlerLayers) == 0 {
		handlerLayers = []string{"handler"}
	}

	forbiddenLayers := ctx.Options.StringSlice("forbidden_layers")
	if len(forbiddenLayers) == 0 {
		forbiddenLayers = []string{"client", "infra", "event"}
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

			if contains(forbiddenLayers, iloc.Layer) {
				ctx.Report.Add(lint.Violation{
					Rule:     "dependency/handler-infra-isolation",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     imp.Pos.Line,
					Message:  fmt.Sprintf("handler layer must not import infrastructure layer %q", iloc.Layer),
					Found:    iloc.Layer,
					Expected: fmt.Sprintf("not one of %v", forbiddenLayers),
				})
			}
		}
		return nil
	})
}
