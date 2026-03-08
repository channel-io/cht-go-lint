package iface

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&OnePerFile{})
}

// OnePerFile checks that each file has at most one primary exported interface.
type OnePerFile struct{}

func (r *OnePerFile) Meta() lint.Meta {
	return lint.Meta{
		Name:        "interface/one-per-file",
		Description: "Each file should have at most one primary exported interface",
		Category:    "interface",
		Tier:        lint.TierUniversal,
	}
}

func (r *OnePerFile) Check(ctx *lint.Context) error {
	max := ctx.Options.Int("max", 1)

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		var exported []lint.TypeDecl
		for _, td := range file.Types {
			if td.IsInterface && td.Exported {
				exported = append(exported, td)
			}
		}

		if len(exported) > max {
			ctx.Report.Add(lint.Violation{
				Rule:     "interface/one-per-file",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     exported[max].Pos.Line,
				Message:  fmt.Sprintf("file has %d exported interfaces (max %d)", len(exported), max),
				Found:    fmt.Sprintf("%d", len(exported)),
				Expected: fmt.Sprintf("<= %d", max),
			})
		}

		return nil
	})
}
