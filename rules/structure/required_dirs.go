package structure

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&RequiredDirs{})
}

// RequiredDirs validates that required directories exist in each component.
type RequiredDirs struct{}

func (r *RequiredDirs) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/required-dirs",
		Description: "Validate that required directories exist in each component",
		Category:    "structure",
		Tier:        lint.TierComponentAware,
	}
}

func (r *RequiredDirs) Check(ctx *lint.Context) error {
	dirs := ctx.Options.StringSlice("dirs")
	if len(dirs) == 0 {
		return nil
	}

	for _, comp := range ctx.Config.Components {
		existing, err := ctx.Analyzer.ListDirs(comp.Path)
		if err != nil {
			continue
		}
		existingSet := make(map[string]bool, len(existing))
		for _, d := range existing {
			existingSet[d] = true
		}
		for _, required := range dirs {
			if !existingSet[required] {
				ctx.Report.Add(lint.Violation{
					Rule:     "structure/required-dirs",
					Severity: ctx.Severity,
					File:     comp.Path,
					Message:  fmt.Sprintf("component %q is missing required directory %q", comp.Name, required),
					Expected: required,
				})
			}
		}
	}
	return nil
}
