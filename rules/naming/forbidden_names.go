package naming

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ForbiddenNames{})
}

// ForbiddenNames flags types with forbidden prefixes or suffixes.
type ForbiddenNames struct{}

func (r *ForbiddenNames) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/forbidden-names",
		Description: "Types with certain prefixes or suffixes are forbidden",
		Category:    "naming",
		Tier:        lint.TierUniversal,
	}
}

func (r *ForbiddenNames) Check(ctx *lint.Context) error {
	prefixes := ctx.Options.StringSlice("forbidden_prefixes")
	suffixes := ctx.Options.StringSlice("forbidden_suffixes")

	if len(prefixes) == 0 && len(suffixes) == 0 {
		return nil
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, td := range file.Types {
			for _, prefix := range prefixes {
				if strings.HasPrefix(td.Name, prefix) {
					ctx.Report.Add(lint.Violation{
						Rule:     "naming/forbidden-names",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     td.Pos.Line,
						Message:  fmt.Sprintf("type %q has forbidden prefix %q", td.Name, prefix),
						Found:    td.Name,
					})
				}
			}
			for _, suffix := range suffixes {
				if strings.HasSuffix(td.Name, suffix) {
					ctx.Report.Add(lint.Violation{
						Rule:     "naming/forbidden-names",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     td.Pos.Line,
						Message:  fmt.Sprintf("type %q has forbidden suffix %q", td.Name, suffix),
						Found:    td.Name,
					})
				}
			}
		}
		return nil
	})
}
