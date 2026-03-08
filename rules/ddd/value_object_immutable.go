package ddd

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ValueObjectImmutable{})
}

// ValueObjectImmutable ensures value objects do not have pointer receiver methods.
type ValueObjectImmutable struct{}

func (r *ValueObjectImmutable) Meta() lint.Meta {
	return lint.Meta{
		Name:        "ddd/value-object-immutable",
		Description: "Value objects should not have setter methods (pointer receiver methods)",
		Category:    "ddd",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *ValueObjectImmutable) Check(ctx *lint.Context) error {
	voMarkers := ctx.Options.StringSlice("vo_markers")
	if len(voMarkers) == 0 {
		voMarkers = []string{"VO", "ValueObject"}
	}
	allowPointerReceivers := ctx.Options.Bool("allow_pointer_receivers", false)
	if allowPointerReceivers {
		return nil
	}

	// Collect value object type names
	voTypes := make(map[string]bool)
	ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, t := range file.Types {
			if t.IsStruct && isValueObject(t.Name, voMarkers) {
				voTypes[t.Name] = true
			}
		}
		return nil
	})

	if len(voTypes) == 0 {
		return nil
	}

	// Check methods on value object types
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, fn := range file.Funcs {
			if fn.ReceiverType == "" {
				continue
			}
			// Check if receiver is a pointer to a VO type
			receiverType := fn.ReceiverType
			if !strings.HasPrefix(receiverType, "*") {
				continue
			}
			baseType := strings.TrimPrefix(receiverType, "*")
			if voTypes[baseType] {
				ctx.Report.Add(lint.Violation{
					Rule:     "ddd/value-object-immutable",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     fn.Pos.Line,
					Message:  fmt.Sprintf("value object %q should not have pointer receiver method %q (implies mutation)", baseType, fn.Name),
					Found:    receiverType,
				})
			}
		}
		return nil
	})
}

func isValueObject(name string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}
