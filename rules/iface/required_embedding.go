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

	// Original logic: global base_interface + target_suffix
	if baseIface != "" {
		if err := r.checkGlobal(ctx, baseIface, targetSuffix); err != nil {
			return err
		}
	}

	// Enhanced: patterns-based conditional embedding
	patterns := ctx.Options.MapSlice("patterns")
	if len(patterns) > 0 {
		return r.checkPatterns(ctx, patterns)
	}

	return nil
}

func (r *RequiredEmbedding) checkGlobal(ctx *lint.Context, baseIface, targetSuffix string) error {
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

			if !hasEmbedding(td, baseIface) {
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

func (r *RequiredEmbedding) checkPatterns(ctx *lint.Context, patterns []map[string]any) error {
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, p := range patterns {
			tagKey, _ := p["tag"].(string)
			tagValue, _ := p["tag_value"].(string)
			layer, _ := p["layer"].(string)
			baseIface, _ := p["base_interface"].(string)
			targetSuffix, _ := p["target_suffix"].(string)

			if baseIface == "" {
				continue
			}

			// Filter by tag
			if tagKey != "" && file.Location.Tag(tagKey) != tagValue {
				continue
			}

			// Filter by layer
			if layer != "" && file.Location.Layer != layer {
				continue
			}

			for _, td := range file.Types {
				if !td.IsInterface || !td.Exported {
					continue
				}

				// If target_suffix is specified, only check matching interfaces
				if targetSuffix != "" && !strings.HasSuffix(td.Name, targetSuffix) {
					continue
				}

				// Don't require the base interface to embed itself.
				if td.Name == baseIface {
					continue
				}

				if !hasEmbedding(td, baseIface) {
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
		}

		return nil
	})
}

func hasEmbedding(td lint.TypeDecl, baseIface string) bool {
	for _, emb := range td.Embedded {
		if emb == baseIface || strings.HasSuffix(emb, "."+baseIface) {
			return true
		}
	}
	return false
}
