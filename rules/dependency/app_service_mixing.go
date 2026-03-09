package dependency

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&AppServiceMixing{})
}

// AppServiceMixing enforces that a single App Service file cannot import both repo/ and
// infra/client/event layers. This enforces separation of pure business logic from orchestration.
type AppServiceMixing struct{}

func (r *AppServiceMixing) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/app-service-mixing",
		Description: "App Service files must not mix repo and infra/client/event imports",
		Category:    "dependency",
		Tier:        lint.TierComponentAware,
	}
}

func (r *AppServiceMixing) Check(ctx *lint.Context) error {
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		loc := file.Location

		// Filter: must be domain-level (component set, no sub-component)
		if loc.Component == "" || loc.SubComponent != "" {
			return nil
		}

		// Filter: must be a service/svc/appsvc layer
		if loc.Layer != "service" && loc.Layer != "svc" && loc.Layer != "appsvc" {
			return nil
		}

		// Filter: must NOT be a public service file
		if loc.Tag("isPublicSvc") == "true" {
			return nil
		}

		sourceComp := loc.Component

		var hasRepo bool
		var hasInfraLayer bool
		var repoImport string
		var infraImport string

		for _, imp := range file.Imports {
			if !ctx.Analyzer.IsInternalImport(imp.Path) {
				continue
			}

			iloc := ctx.Analyzer.ImportLocation(imp.Path)
			if iloc.Component != sourceComp {
				continue
			}

			if iloc.Layer == "repo" {
				hasRepo = true
				repoImport = strings.TrimPrefix(imp.Path, ctx.Analyzer.ModulePath()+"/")
			}

			if iloc.Layer == "infra" || iloc.Layer == "client" || iloc.Layer == "event" {
				hasInfraLayer = true
				infraImport = strings.TrimPrefix(imp.Path, ctx.Analyzer.ModulePath()+"/")
			}
		}

		if hasRepo && hasInfraLayer {
			ctx.Report.Add(lint.Violation{
				Rule:     "dependency/app-service-mixing",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  fmt.Sprintf("app service file mixes repo (%s) and infra/client/event (%s) imports; separate business logic from orchestration", repoImport, infraImport),
				Found:    "both repo and infra/client/event imports",
				Expected: "either repo OR infra/client/event, not both",
			})
		}
		return nil
	})
}
