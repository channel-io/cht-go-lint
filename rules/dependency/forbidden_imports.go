package dependency

import (
	"fmt"
	"path"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ForbiddenImports{})
}

// ForbiddenImports bans specific import path patterns.
type ForbiddenImports struct{}

func (r *ForbiddenImports) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/forbidden-imports",
		Description: "Ban specific import path patterns",
		Category:    "dependency",
		Tier:        lint.TierUniversal,
	}
}

func (r *ForbiddenImports) Check(ctx *lint.Context) error {
	patterns := ctx.Options.StringSlice("patterns")
	if len(patterns) == 0 {
		return nil
	}

	return ctx.Analyzer.WalkGoFiles(func(_ string, file *lint.ParsedFile) error {
		for _, imp := range file.Imports {
			for _, pattern := range patterns {
				if matchImportPattern(imp.Path, pattern) {
					ctx.Report.Add(lint.Violation{
						Rule:     "dependency/forbidden-imports",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     imp.Pos.Line,
						Message:  fmt.Sprintf("import %q is forbidden (matches pattern %q)", imp.Path, pattern),
						Found:    imp.Path,
					})
					break
				}
			}
		}
		return nil
	})
}

// matchImportPattern checks if an import path matches a pattern.
// Supports path.Match glob patterns, prefix matching (pattern ending with /*),
// and exact matching.
func matchImportPattern(importPath, pattern string) bool {
	// Try path.Match first (supports *, ?)
	if matched, err := path.Match(pattern, importPath); err == nil && matched {
		return true
	}

	// Support prefix matching with trailing /*
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if strings.HasPrefix(importPath, prefix+"/") || importPath == prefix {
			return true
		}
	}

	// Exact match
	return importPath == pattern
}
