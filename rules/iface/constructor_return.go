package iface

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ConstructorReturn{})
}

// ConstructorReturn checks that constructors return an interface type, not a concrete struct.
type ConstructorReturn struct{}

func (r *ConstructorReturn) Meta() lint.Meta {
	return lint.Meta{
		Name:        "interface/constructor-return",
		Description: "Constructor functions should return an interface type, not a concrete struct",
		Category:    "interface",
		Tier:        lint.TierUniversal,
	}
}

func (r *ConstructorReturn) Check(ctx *lint.Context) error {
	excludeInternal := ctx.Options.Bool("exclude_internal", false)

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		// Collect exported interface names in this file.
		ifaceNames := make(map[string]bool)
		for _, td := range file.Types {
			if td.IsInterface && td.Exported {
				ifaceNames[td.Name] = true
			}
		}

		if len(ifaceNames) == 0 {
			return nil
		}

		for _, fd := range file.Funcs {
			if !fd.IsConstructor || fd.ReceiverType != "" {
				continue
			}
			if excludeInternal && !fd.Exported {
				continue
			}

			returnsIface := false
			for _, rt := range fd.ReturnTypes {
				clean := strings.TrimPrefix(rt, "*")
				if ifaceNames[clean] {
					returnsIface = true
					break
				}
			}

			if !returnsIface {
				ctx.Report.Add(lint.Violation{
					Rule:     "interface/constructor-return",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     fd.Pos.Line,
					Message:  fmt.Sprintf("constructor %q should return an interface type, not a concrete struct", fd.Name),
					Found:    strings.Join(fd.ReturnTypes, ", "),
				})
			}
		}

		return nil
	})
}
