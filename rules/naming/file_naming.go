package naming

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&FileNaming{})
}

// FileNaming checks that source file names follow a naming convention.
type FileNaming struct{}

func (r *FileNaming) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/file-naming",
		Description: "Source file names must follow a naming convention",
		Category:    "naming",
		Tier:        lint.TierUniversal,
	}
}

var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

func (r *FileNaming) Check(ctx *lint.Context) error {
	convention := ctx.Options.String("convention", "snake_case")
	noPackageStutter := ctx.Options.Bool("no_package_stutter", false)

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		base := filepath.Base(file.RelPath)
		name := strings.TrimSuffix(base, ".go")

		if !matchesConvention(name, convention) {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/file-naming",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  fmt.Sprintf("file name %q does not follow %s convention", base, convention),
				Found:    base,
				Expected: convention,
			})
		}

		// Check if filename repeats the package name (e.g., install/install.go)
		// Normalize both: lowercase + remove underscores for robust comparison
		if noPackageStutter && file.Location.Tag("isAlias") != "true" &&
			strings.ReplaceAll(strings.ToLower(name), "_", "") == strings.ReplaceAll(strings.ToLower(file.Package), "_", "") {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/file-naming",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  fmt.Sprintf("file name %q repeats package name %q", base, file.Package),
				Found:    base,
			})
		}

		return nil
	})
}

func matchesConvention(name, convention string) bool {
	switch convention {
	case "snake_case":
		return snakeCaseRe.MatchString(name)
	default:
		return snakeCaseRe.MatchString(name)
	}
}
