package ddd

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&RepositoryPerAggregate{})
}

// RepositoryPerAggregate ensures each aggregate root has exactly one repository interface.
type RepositoryPerAggregate struct{}

func (r *RepositoryPerAggregate) Meta() lint.Meta {
	return lint.Meta{
		Name:        "ddd/repository-per-aggregate",
		Description: "Each aggregate root should have exactly one repository interface",
		Category:    "ddd",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *RepositoryPerAggregate) Check(ctx *lint.Context) error {
	repoSuffix := ctx.Options.String("repo_suffix", "Repository")
	rootMarker := ctx.Options.String("root_marker", "Aggregate")

	// Collect aggregate root names and repository names
	var aggregateRoots []aggregateInfo
	repos := make(map[string]bool)

	_ = ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, t := range file.Types {
			if t.IsStruct && strings.HasSuffix(t.Name, rootMarker) {
				aggregateRoots = append(aggregateRoots, aggregateInfo{
					name:    t.Name,
					file:    file.RelPath,
					line:    t.Pos.Line,
				})
			}
			if t.IsInterface && strings.HasSuffix(t.Name, repoSuffix) {
				repos[t.Name] = true
			}
		}
		return nil
	})

	// For each aggregate root, check if a matching repository exists
	// FooAggregate -> FooRepository
	for _, agg := range aggregateRoots {
		baseName := strings.TrimSuffix(agg.name, rootMarker)
		expectedRepo := baseName + repoSuffix
		if !repos[expectedRepo] {
			ctx.Report.Add(lint.Violation{
				Rule:     "ddd/repository-per-aggregate",
				Severity: ctx.Severity,
				File:     agg.file,
				Line:     agg.line,
				Message:  fmt.Sprintf("aggregate %q has no matching repository %q", agg.name, expectedRepo),
				Expected: expectedRepo,
			})
		}
	}
	return nil
}

type aggregateInfo struct {
	name string
	file string
	line int
}
