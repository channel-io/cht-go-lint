package naming

import (
	"fmt"
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

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
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
