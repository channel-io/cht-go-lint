package naming

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&FilenameMatchesType{})
}

// FilenameMatchesType checks that the primary type in a file matches the filename.
type FilenameMatchesType struct{}

func (r *FilenameMatchesType) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/filename-matches-type",
		Description: "The primary type in a file should match the filename",
		Category:    "naming",
		Tier:        lint.TierUniversal,
	}
}

func (r *FilenameMatchesType) Check(ctx *lint.Context) error {
	strict := ctx.Options.Bool("strict", false)
	skipFiles := make(map[string]bool)
	for _, f := range ctx.Options.StringSlice("skip_files") {
		skipFiles[f] = true
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		if skipFiles[filepath.Base(file.RelPath)] {
			return nil
		}

		// Find the first exported type (prefer interfaces, then structs, then any).
		var primaryType string
		for _, td := range file.Types {
			if td.Exported {
				primaryType = td.Name
				break
			}
		}
		if primaryType == "" {
			return nil
		}

		base := filepath.Base(file.RelPath)
		name := strings.TrimSuffix(base, ".go")

		// Convert type name to snake_case, then normalize both
		typeSnake := pascalToSnake(primaryType)
		normFile := strings.ToLower(strings.ReplaceAll(name, "_", ""))
		normType := strings.ToLower(strings.ReplaceAll(typeSnake, "_", ""))

		// Bidirectional prefix check: either one must be a prefix of the other
		isMatch := strings.HasPrefix(normFile, normType) || strings.HasPrefix(normType, normFile)

		if !isMatch {
			severity := ctx.Severity
			msg := fmt.Sprintf("file %q primary type %q does not match filename (expected bidirectional prefix match)", base, primaryType)
			if strict {
				msg = fmt.Sprintf("file %q primary type %q does not match filename (strict: expected exact match %q)", base, primaryType, snakeToPascal(name))
			}
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/filename-matches-type",
				Severity: severity,
				File:     file.RelPath,
				Line:     1,
				Message:  msg,
				Found:    primaryType,
				Expected: snakeToPascal(name),
			})
		}

		return nil
	})
}

// snakeToPascal converts a snake_case string to PascalCase.
func snakeToPascal(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

// pascalToSnake converts a PascalCase string to snake_case.
func pascalToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			prev := rune(s[i-1])
			if prev >= 'a' && prev <= 'z' {
				b.WriteRune('_')
			} else if i+1 < len(s) {
				next := rune(s[i+1])
				if next >= 'a' && next <= 'z' {
					b.WriteRune('_')
				}
			}
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}
