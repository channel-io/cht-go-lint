package dependency

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&DIIsolation{})
}

// DIIsolation enforces that DI framework imports are isolated in companion files/directories.
type DIIsolation struct{}

func (r *DIIsolation) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/di-isolation",
		Description: "DI framework code should be isolated in companion files",
		Category:    "dependency",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *DIIsolation) Check(ctx *lint.Context) error {
	framework := ctx.Options.String("framework", "fx")
	companionSuffix := ctx.Options.String("companion_suffix", "fx")

	// Build the DI framework import prefix
	var frameworkImport string
	switch framework {
	case "fx":
		frameworkImport = "go.uber.org/fx"
	case "wire":
		frameworkImport = "github.com/google/wire"
	case "dig":
		frameworkImport = "go.uber.org/dig"
	default:
		frameworkImport = framework
	}

	return ctx.Analyzer.WalkGoFiles(func(_ string, file *lint.ParsedFile) error {
		for _, imp := range file.Imports {
			if !strings.HasPrefix(imp.Path, frameworkImport) {
				continue
			}

			// Check if file is in a companion directory or is a companion file
			if isCompanionFile(file.RelPath, companionSuffix) {
				continue
			}

			ctx.Report.Add(lint.Violation{
				Rule:     "dependency/di-isolation",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     imp.Pos.Line,
				Message:  fmt.Sprintf("DI framework import %q should only appear in companion files (directory ending with %q)", imp.Path, companionSuffix),
				Found:    file.RelPath,
			})
			break
		}
		return nil
	})
}

// isCompanionFile checks if a file is in a companion directory or is a companion file.
// A companion directory name ends with the companion suffix (e.g., "extensionfx").
// A companion file name ends with the suffix before .go (e.g., "module_fx.go").
func isCompanionFile(relPath string, suffix string) bool {
	parts := strings.Split(strings.ReplaceAll(relPath, "\\", "/"), "/")
	for _, part := range parts {
		if strings.HasSuffix(part, suffix) {
			return true
		}
	}
	// Also check if the filename itself ends with _suffix.go
	fileName := parts[len(parts)-1]
	return strings.HasSuffix(fileName, "_"+suffix+".go")
}
