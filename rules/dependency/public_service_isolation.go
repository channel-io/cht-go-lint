package dependency

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&PublicServiceIsolation{})
}

// PublicServiceIsolation enforces that Public Service files only delegate to svc (service) interfaces.
// Public Service files (Location.Tag("isPublicSvc") == "true") must not directly import
// repo, infra, client, event, or handler layers from their own domain.
type PublicServiceIsolation struct{}

func (r *PublicServiceIsolation) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/public-service-isolation",
		Description: "Public Service files must not import repo, infra, client, event, or handler layers from their own domain",
		Category:    "dependency",
		Tier:        lint.TierComponentAware,
	}
}

func (r *PublicServiceIsolation) Check(ctx *lint.Context) error {
	forbiddenLayers := map[string]bool{
		"repo":    true,
		"infra":   true,
		"client":  true,
		"event":   true,
		"handler": true,
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		if file.Location.Tag("isPublicSvc") != "true" {
			return nil
		}

		sourceComp := file.Location.Component
		if sourceComp == "" {
			return nil
		}

		for _, imp := range file.Imports {
			if !ctx.Analyzer.IsInternalImport(imp.Path) {
				continue
			}

			iloc := ctx.Analyzer.ImportLocation(imp.Path)
			if iloc.Component != sourceComp {
				continue
			}

			if forbiddenLayers[iloc.Layer] {
				ctx.Report.Add(lint.Violation{
					Rule:     "dependency/public-service-isolation",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     imp.Pos.Line,
					Message:  fmt.Sprintf("public service file must not import %q layer from its own domain %q; delegate via service interfaces", iloc.Layer, sourceComp),
					Found:    strings.TrimPrefix(imp.Path, ctx.Analyzer.ModulePath()+"/"),
					Expected: "import via svc/service interfaces only",
				})
			}
		}
		return nil
	})
}
