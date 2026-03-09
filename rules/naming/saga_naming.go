package naming

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&SagaNaming{})
}

// SagaNaming checks that saga definition files define a Saga interface,
// a matching private struct, and that constructors return the interface.
type SagaNaming struct{}

func (r *SagaNaming) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/saga-naming",
		Description: "Saga files must define a Saga-suffixed interface with matching private struct",
		Category:    "naming",
		Tier:        lint.TierLayerAware,
	}
}

func (r *SagaNaming) Check(ctx *lint.Context) error {
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		if file.Location.Tag("isSaga") != "true" {
			return nil
		}
		if file.Location.Tag("isFxCompanion") == "true" {
			return nil
		}

		base := filepath.Base(file.RelPath)
		if !strings.Contains(base, "saga") {
			return nil
		}

		// Find interface ending with "Saga"
		var sagaIfaceName string
		for _, td := range file.Types {
			if td.IsInterface && td.Exported && strings.HasSuffix(td.Name, "Saga") {
				sagaIfaceName = td.Name
				break
			}
		}

		if sagaIfaceName == "" {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/saga-naming",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  fmt.Sprintf("saga file %q must define an interface ending with 'Saga'", base),
			})
			return nil
		}

		// Find matching private struct (case-insensitive match)
		hasMatchingStruct := false
		for _, td := range file.Types {
			if td.IsStruct && !td.Exported && strings.EqualFold(td.Name, sagaIfaceName) {
				hasMatchingStruct = true
				break
			}
		}

		if !hasMatchingStruct {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/saga-naming",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  fmt.Sprintf("saga file must define a private struct matching interface %q (case-insensitive)", sagaIfaceName),
				Expected: strings.ToLower(sagaIfaceName[:1]) + sagaIfaceName[1:],
			})
		}

		// Constructors must return the Saga interface, not a pointer to struct
		for _, fd := range file.Funcs {
			if !fd.IsConstructor || fd.ReceiverType != "" {
				continue
			}

			for _, rt := range fd.ReturnTypes {
				if strings.HasPrefix(rt, "*") {
					ctx.Report.Add(lint.Violation{
						Rule:     "naming/saga-naming",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     fd.Pos.Line,
						Message:  fmt.Sprintf("constructor %q must return the Saga interface, not a pointer to struct", fd.Name),
						Found:    rt,
						Expected: sagaIfaceName,
					})
				}
			}
		}

		return nil
	})
}
