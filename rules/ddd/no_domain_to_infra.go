package ddd

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&NoDomainToInfra{})
}

// NoDomainToInfra ensures the domain layer does not import infrastructure packages.
type NoDomainToInfra struct{}

func (r *NoDomainToInfra) Meta() lint.Meta {
	return lint.Meta{
		Name:        "ddd/no-domain-to-infra",
		Description: "Domain layer must not import infrastructure packages",
		Category:    "ddd",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *NoDomainToInfra) Check(ctx *lint.Context) error {
	domainPaths := ctx.Options.StringSlice("domain_paths")
	infraPatterns := ctx.Options.StringSlice("infra_patterns")
	if len(domainPaths) == 0 || len(infraPatterns) == 0 {
		return nil
	}

	return ctx.Analyzer.WalkDirs(domainPaths, func(path string, file *lint.ParsedFile) error {
		for _, imp := range file.Imports {
			if matchesAnyPattern(imp.Path, infraPatterns) {
				ctx.Report.Add(lint.Violation{
					Rule:     "ddd/no-domain-to-infra",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     imp.Pos.Line,
					Message:  fmt.Sprintf("domain file must not import infrastructure package %q", imp.Path),
					Found:    imp.Path,
				})
			}
		}
		return nil
	})
}

func matchesAnyPattern(importPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(importPath, pattern) {
			return true
		}
	}
	return false
}

func matchPattern(importPath, pattern string) bool {
	// Support glob-style wildcards using filepath.Match on segments
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, importPath)
		if matched {
			return true
		}
		// Also try matching just the path prefix for patterns like "github.com/redis/*"
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(importPath, prefix+"/") {
				return true
			}
		}
		return false
	}
	// Exact match or prefix match
	return importPath == pattern || strings.HasPrefix(importPath, pattern+"/")
}
