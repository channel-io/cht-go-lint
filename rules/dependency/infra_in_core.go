package dependency

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&InfraInCore{})
}

// InfraInCore enforces that core layers do not import infrastructure packages.
type InfraInCore struct{}

func (r *InfraInCore) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/infra-in-core",
		Description: "Core layers must not import infrastructure packages",
		Category:    "dependency",
		Tier:        lint.TierLayerAware,
	}
}

func (r *InfraInCore) Check(ctx *lint.Context) error {
	coreLayers := ctx.Options.StringSlice("core_layers")
	if len(coreLayers) == 0 {
		coreLayers = []string{"model", "repo", "service"}
	}

	infraPatterns := ctx.Options.StringSlice("infra_patterns")
	if len(infraPatterns) == 0 {
		infraPatterns = []string{"database/sql", "github.com/redis/*", "github.com/aws/*"}
	}

	return ctx.Analyzer.WalkGoFiles(func(_ string, file *lint.ParsedFile) error {
		if !contains(coreLayers, file.Location.Layer) {
			return nil
		}

		for _, imp := range file.Imports {
			for _, pattern := range infraPatterns {
				if matchImportPattern(imp.Path, pattern) {
					ctx.Report.Add(lint.Violation{
						Rule:     "dependency/infra-in-core",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     imp.Pos.Line,
						Message:  fmt.Sprintf("core layer %q must not import infrastructure package %q", file.Location.Layer, imp.Path),
						Found:    imp.Path,
					})
					break
				}
			}
		}
		return nil
	})
}
