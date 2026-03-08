package iface

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ImplPattern{})
}

// ImplPattern checks that files with an exported interface also have a private implementation struct.
type ImplPattern struct{}

func (r *ImplPattern) Meta() lint.Meta {
	return lint.Meta{
		Name:        "interface/impl-pattern",
		Description: "Files with an exported interface should also have a private implementation struct",
		Category:    "interface",
		Tier:        lint.TierUniversal,
	}
}

func (r *ImplPattern) Check(ctx *lint.Context) error {
	require := ctx.Options.Bool("require", true)
	if !require {
		return nil
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		var exportedIfaces []lint.TypeDecl
		hasUnexportedStruct := false

		for _, td := range file.Types {
			if td.IsInterface && td.Exported {
				exportedIfaces = append(exportedIfaces, td)
			}
			if td.IsStruct && !td.Exported {
				hasUnexportedStruct = true
			}
		}

		if len(exportedIfaces) != 1 {
			return nil
		}

		if !hasUnexportedStruct {
			ctx.Report.Add(lint.Violation{
				Rule:     "interface/impl-pattern",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     exportedIfaces[0].Pos.Line,
				Message:  fmt.Sprintf("interface %q has no private implementation struct in the same file", exportedIfaces[0].Name),
			})
		}

		return nil
	})
}
