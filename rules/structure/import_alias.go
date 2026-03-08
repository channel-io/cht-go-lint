package structure

import (
	"fmt"
	"regexp"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ImportAlias{})
}

// ImportAlias validates that import aliases follow a naming convention.
type ImportAlias struct{}

func (r *ImportAlias) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/import-alias",
		Description: "Import aliases should follow a naming convention",
		Category:    "structure",
		Tier:        lint.TierUniversal,
	}
}

var conventionPatterns = map[string]*regexp.Regexp{
	"snake_case":  regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`),
	"camelCase":   regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`),
	"PascalCase":  regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`),
	"lower":       regexp.MustCompile(`^[a-z][a-z0-9]*$`),
}

func (r *ImportAlias) Check(ctx *lint.Context) error {
	convention := ctx.Options.String("convention", "snake_case")
	forbiddenAliases := ctx.Options.StringSlice("forbidden_aliases")

	forbiddenSet := make(map[string]bool, len(forbiddenAliases))
	for _, a := range forbiddenAliases {
		forbiddenSet[a] = true
	}

	pattern := conventionPatterns[convention]

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, imp := range file.Imports {
			if imp.Alias == "" {
				continue
			}

			// Check forbidden aliases
			if forbiddenSet[imp.Alias] {
				ctx.Report.Add(lint.Violation{
					Rule:     "structure/import-alias",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     imp.Pos.Line,
					Message:  fmt.Sprintf("import alias %q is forbidden", imp.Alias),
					Found:    imp.Alias,
				})
				continue
			}

			// Underscore alias (_) is used for side-effect imports, skip convention check
			if imp.Alias == "_" {
				continue
			}

			// Check convention
			if pattern != nil && !pattern.MatchString(imp.Alias) {
				ctx.Report.Add(lint.Violation{
					Rule:     "structure/import-alias",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     imp.Pos.Line,
					Message:  fmt.Sprintf("import alias %q does not follow %s convention", imp.Alias, convention),
					Found:    imp.Alias,
					Expected: convention,
				})
			}
		}
		return nil
	})
}
