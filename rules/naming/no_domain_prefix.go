package naming

import (
	"fmt"
	"strings"
	"unicode"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&NoDomainPrefix{})
}

// NoDomainPrefix checks that exported type names do not use the component name
// as a prefix, which is redundant since the package already provides that context.
type NoDomainPrefix struct{}

func (r *NoDomainPrefix) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/no-domain-prefix",
		Description: "Exported types should not be prefixed with the component name",
		Category:    "naming",
		Tier:        lint.TierComponentAware,
	}
}

func (r *NoDomainPrefix) Check(ctx *lint.Context) error {
	skipLayers := ctx.Options.StringSlice("skip_layers")
	if len(skipLayers) == 0 {
		skipLayers = []string{"model", "repo"}
	}

	targetLayers := ctx.Options.StringSlice("target_layers")

	checkTypes := ctx.Options.StringSlice("check_types")
	if len(checkTypes) == 0 {
		checkTypes = []string{"interface"}
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		comp := file.Location.Component
		if comp == "" {
			return nil
		}

		// Skip FX companion components
		if file.Location.IsFxCompanion() {
			return nil
		}

		layer := file.Location.Layer

		// Filter by target/skip layers
		if len(targetLayers) > 0 {
			if !stringSliceContains(targetLayers, layer) {
				return nil
			}
		} else {
			if stringSliceContains(skipLayers, layer) {
				return nil
			}
		}

		prefix := ucFirst(comp)

		for _, td := range file.Types {
			if !td.Exported {
				continue
			}

			// Check if this type should be checked
			if !shouldCheckType(td, checkTypes) {
				continue
			}

			// Check if type name starts with component prefix and has more characters
			if strings.HasPrefix(td.Name, prefix) && len(td.Name) > len(prefix) {
				// Ensure the next character after prefix is uppercase (not part of a longer word)
				next := rune(td.Name[len(prefix)])
				if unicode.IsUpper(next) {
					ctx.Report.Add(lint.Violation{
						Rule:     "naming/no-domain-prefix",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     td.Pos.Line,
						Message:  fmt.Sprintf("type %q should not use component name %q as prefix", td.Name, comp),
						Found:    td.Name,
					})
				}
			}
		}
		return nil
	})
}

func ucFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func shouldCheckType(td lint.TypeDecl, checkTypes []string) bool {
	for _, ct := range checkTypes {
		switch ct {
		case "interface":
			if td.IsInterface {
				return true
			}
		case "struct":
			if td.IsStruct {
				return true
			}
		case "all":
			return true
		}
	}
	return false
}

func stringSliceContains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
