package naming

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&NoStutter{})
}

// NoStutter flags type or function names that repeat the package name.
type NoStutter struct{}

func (r *NoStutter) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/no-stutter",
		Description: "Type or function name should not repeat the package name",
		Category:    "naming",
		Tier:        lint.TierUniversal,
	}
}

func (r *NoStutter) Check(ctx *lint.Context) error {
	excludeConstructors := ctx.Options.Bool("exclude_constructors", true)
	checkComponentName := ctx.Options.Bool("check_component_name", false)
	skipFiles := make(map[string]bool)
	for _, f := range ctx.Options.StringSlice("skip_files") {
		skipFiles[f] = true
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		if skipFiles[filepath.Base(file.RelPath)] {
			return nil
		}

		pkgLower := strings.ToLower(file.Package)

		for _, td := range file.Types {
			if strings.HasPrefix(strings.ToLower(td.Name), pkgLower) {
				ctx.Report.Add(lint.Violation{
					Rule:     "naming/no-stutter",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     td.Pos.Line,
					Message:  fmt.Sprintf("type %q stutters with package name %q", td.Name, file.Package),
				})
			}
			// Also check against component name from LocationStrategy
			if checkComponentName && td.Exported && file.Location.Component != "" {
				compLower := strings.ToLower(file.Location.Component)
				if compLower != pkgLower && strings.HasPrefix(strings.ToLower(td.Name), compLower) {
					ctx.Report.Add(lint.Violation{
						Rule:     "naming/no-stutter",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     td.Pos.Line,
						Message:  fmt.Sprintf("type %q stutters with component name %q", td.Name, file.Location.Component),
					})
				}
			}
		}

		for _, fd := range file.Funcs {
			if fd.ReceiverType != "" {
				continue
			}
			if excludeConstructors && fd.IsConstructor {
				continue
			}
			if strings.HasPrefix(strings.ToLower(fd.Name), pkgLower) {
				ctx.Report.Add(lint.Violation{
					Rule:     "naming/no-stutter",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     fd.Pos.Line,
					Message:  fmt.Sprintf("function %q stutters with package name %q", fd.Name, file.Package),
				})
			}
		}

		return nil
	})
}
