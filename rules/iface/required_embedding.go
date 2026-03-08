package iface

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&RequiredEmbedding{})
}

// RequiredEmbedding checks that certain interfaces embed a base interface.
type RequiredEmbedding struct{}

func (r *RequiredEmbedding) Meta() lint.Meta {
	return lint.Meta{
		Name:        "interface/required-embedding",
		Description: "Certain interfaces must embed a base interface",
		Category:    "interface",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *RequiredEmbedding) Check(ctx *lint.Context) error {
	baseIface := ctx.Options.String("base_interface", "")
	targetSuffix := ctx.Options.String("target_suffix", "Service")

	if baseIface == "" {
		return nil
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, td := range file.Types {
			if !td.IsInterface || !td.Exported {
				continue
			}
			if !strings.HasSuffix(td.Name, targetSuffix) {
				continue
			}
			// Don't require the base interface to embed itself.
			if td.Name == baseIface {
				continue
			}

			found := false
			for _, emb := range td.Embedded {
				if emb == baseIface || strings.HasSuffix(emb, "."+baseIface) {
					found = true
					break
				}
			}

			if !found {
				ctx.Report.Add(lint.Violation{
					Rule:     "interface/required-embedding",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     td.Pos.Line,
					Message:  fmt.Sprintf("interface %q must embed %q", td.Name, baseIface),
					Expected: baseIface,
				})
			}
		}

		return nil
	})
}
