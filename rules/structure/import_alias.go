package structure

import (
	"fmt"
	"regexp"
	"strings"

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
	noSameComponentAlias := ctx.Options.Bool("no_same_component_alias", false)

	forbiddenSet := make(map[string]bool, len(forbiddenAliases))
	for _, a := range forbiddenAliases {
		forbiddenSet[a] = true
	}

	pattern := conventionPatterns[convention]

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		// Collect base package names for disambiguation check
		var basePkgNames map[string]int
		if noSameComponentAlias {
			basePkgNames = make(map[string]int)
			for _, imp := range file.Imports {
				parts := splitImportPath(imp.Path)
				if len(parts) > 0 {
					basePkgNames[parts[len(parts)-1]]++
				}
			}
		}

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

			// Check for unnecessary alias on same-component imports
			if noSameComponentAlias && ctx.Analyzer.IsInternalImport(imp.Path) {
				iloc := ctx.Analyzer.ImportLocation(imp.Path)
				if file.Location.Component != "" &&
					iloc.Component == file.Location.Component &&
					iloc.SubComponent == file.Location.SubComponent {
					// Same component + subcomponent - alias is unnecessary
					// unless there's a name conflict
					parts := splitImportPath(imp.Path)
					basePkg := parts[len(parts)-1]
					if basePkgNames[basePkg] <= 1 {
						ctx.Report.Add(lint.Violation{
							Rule:     "structure/import-alias",
							Severity: ctx.Severity,
							File:     file.RelPath,
							Line:     imp.Pos.Line,
							Message:  fmt.Sprintf("unnecessary import alias %q for same-component import", imp.Alias),
							Found:    imp.Alias,
						})
						continue
					}
				}
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

func splitImportPath(path string) []string {
	return strings.Split(path, "/")
}
