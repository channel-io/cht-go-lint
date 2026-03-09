package dependency

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&SubdomainIsolation{})
}

// SubdomainIsolation enforces that sub-components within the same component
// do not import each other directly.
type SubdomainIsolation struct{}

func (r *SubdomainIsolation) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/subdomain-isolation",
		Description: "Sub-components within a component must not import each other",
		Category:    "dependency",
		Tier:        lint.TierComponentAware,
	}
}

func (r *SubdomainIsolation) Check(ctx *lint.Context) error {
	allowModel := ctx.Options.Bool("allow_model_import", false)

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		sourceComp := file.Location.Component
		sourceSub := file.Location.SubComponent
		if sourceComp == "" || sourceSub == "" {
			return nil
		}

		// Skip appsvc/publicsvc tagged files
		if file.Location.Tag("isPublicSvc") == "true" {
			return nil
		}

		for _, imp := range file.Imports {
			if !ctx.Analyzer.IsInternalImport(imp.Path) {
				continue
			}

			iloc := ctx.Analyzer.ImportLocation(imp.Path)

			// Only check same component, different sub-component
			if iloc.Component != sourceComp || iloc.SubComponent == "" || iloc.SubComponent == sourceSub {
				continue
			}

			// Skip FX companion imports
			if iloc.Component == sourceComp+"fx" || strings.HasSuffix(iloc.Component, "fx") {
				continue
			}

			// Optionally downgrade model imports to Warn
			if allowModel && iloc.Layer == "model" {
				ctx.Report.Add(lint.Violation{
					Rule:     "dependency/subdomain-isolation",
					Severity: lint.Warn,
					File:     file.RelPath,
					Line:     imp.Pos.Line,
					Message:  fmt.Sprintf("sub-component %q imports model from sibling %q (allowed as warning)", sourceSub, iloc.SubComponent),
					Found:    strings.TrimPrefix(imp.Path, ctx.Analyzer.ModulePath()+"/"),
				})
				continue
			}

			ctx.Report.Add(lint.Violation{
				Rule:     "dependency/subdomain-isolation",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     imp.Pos.Line,
				Message:  fmt.Sprintf("sub-component %q must not import sibling sub-component %q", sourceSub, iloc.SubComponent),
				Found:    strings.TrimPrefix(imp.Path, ctx.Analyzer.ModulePath()+"/"),
			})
		}
		return nil
	})
}
