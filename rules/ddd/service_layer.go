package ddd

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ServiceLayer{})
}

// ServiceLayer enforces separation between domain services and application services.
type ServiceLayer struct{}

func (r *ServiceLayer) Meta() lint.Meta {
	return lint.Meta{
		Name:        "ddd/service-layer",
		Description: "Enforce separation between domain services and application services",
		Category:    "ddd",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *ServiceLayer) Check(ctx *lint.Context) error {
	domainSvcPath := ctx.Options.String("domain_service_path", "")
	appSvcPath := ctx.Options.String("app_service_path", "")
	domainNoInfra := ctx.Options.Bool("domain_service_no_infra", true)

	if domainSvcPath == "" && appSvcPath == "" {
		return nil
	}

	infraPatterns := ctx.Options.StringSlice("infra_patterns")

	// Check domain services don't import infra
	if domainSvcPath != "" && domainNoInfra && len(infraPatterns) > 0 {
		err := ctx.Analyzer.WalkDir(domainSvcPath, func(path string, file *lint.ParsedFile) error {
			for _, imp := range file.Imports {
				if matchesAnyPattern(imp.Path, infraPatterns) {
					ctx.Report.Add(lint.Violation{
						Rule:     "ddd/service-layer",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     imp.Pos.Line,
						Message:  fmt.Sprintf("domain service must not import infrastructure package %q", imp.Path),
						Found:    imp.Path,
					})
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Check app services don't directly import domain internals
	// App services should only import through the domain service interface layer
	if appSvcPath != "" && domainSvcPath != "" {
		modulePath := ctx.Analyzer.ModulePath()
		domainInternalPrefix := modulePath + "/" + domainSvcPath

		err := ctx.Analyzer.WalkDir(appSvcPath, func(path string, file *lint.ParsedFile) error {
			for _, imp := range file.Imports {
				if !ctx.Analyzer.IsInternalImport(imp.Path) {
					continue
				}
				// App services can import the domain service path itself (for the interface),
				// but should not import sub-packages of domain services (internal impl)
				if imp.Path != domainInternalPrefix && hasSubPath(imp.Path, domainInternalPrefix) {
					ctx.Report.Add(lint.Violation{
						Rule:     "ddd/service-layer",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     imp.Pos.Line,
						Message:  fmt.Sprintf("application service should not import domain service internals %q", imp.Path),
						Found:    imp.Path,
					})
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func hasSubPath(importPath, prefix string) bool {
	return len(importPath) > len(prefix) && importPath[:len(prefix)] == prefix && importPath[len(prefix)] == '/'
}
