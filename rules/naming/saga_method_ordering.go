package naming

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&SagaMethodOrdering{})
}

// SagaMethodOrdering checks that in saga operation files (non-definition files),
// saga methods appear before step structs.
type SagaMethodOrdering struct{}

func (r *SagaMethodOrdering) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/saga-method-ordering",
		Description: "Saga methods must appear before step structs in operation files",
		Category:    "naming",
		Tier:        lint.TierLayerAware,
	}
}

func (r *SagaMethodOrdering) Check(ctx *lint.Context) error {
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		if file.Location.Tag("isSaga") != "true" {
			return nil
		}
		if file.Location.Tag("isFxCompanion") == "true" {
			return nil
		}

		// Skip saga definition files (files with "saga" in filename)
		base := filepath.Base(file.RelPath)
		if strings.Contains(base, "saga") {
			return nil
		}

		// Find step struct receiver types: structs that are receivers for Execute or Rollback methods
		stepReceiverTypes := make(map[string]bool)
		for _, fd := range file.Funcs {
			if fd.ReceiverType != "" && (fd.Name == "Execute" || fd.Name == "Rollback") {
				rt := strings.TrimPrefix(fd.ReceiverType, "*")
				stepReceiverTypes[rt] = true
			}
		}

		if len(stepReceiverTypes) == 0 {
			return nil
		}

		// Find earliest step struct position from type declarations
		firstStepPos := 0
		for _, td := range file.Types {
			if td.IsStruct && stepReceiverTypes[td.Name] {
				if firstStepPos == 0 || td.Pos.Line < firstStepPos {
					firstStepPos = td.Pos.Line
				}
			}
		}

		if firstStepPos == 0 {
			return nil
		}

		// Find saga methods: methods whose receiver type contains "saga" (case-insensitive),
		// excluding Execute, Rollback, and methods starting with "handle"
		for _, fd := range file.Funcs {
			if fd.ReceiverType == "" {
				continue
			}
			rt := strings.TrimPrefix(fd.ReceiverType, "*")
			if !strings.Contains(strings.ToLower(rt), "saga") {
				continue
			}
			if fd.Name == "Execute" || fd.Name == "Rollback" {
				continue
			}
			if strings.HasPrefix(fd.Name, "handle") {
				continue
			}

			if fd.Pos.Line > firstStepPos {
				ctx.Report.Add(lint.Violation{
					Rule:     "naming/saga-method-ordering",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     fd.Pos.Line,
					Message:  fmt.Sprintf("saga method %q (line %d) must appear before step structs (first at line %d)", fd.Name, fd.Pos.Line, firstStepPos),
				})
			}
		}

		return nil
	})
}
