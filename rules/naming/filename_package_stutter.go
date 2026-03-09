package naming

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&FilenamePackageStutter{})
}

// FilenamePackageStutter checks that filenames do not repeat the package name.
// For example, install/install.go is bad; install/handler.go is good.
type FilenamePackageStutter struct{}

func (r *FilenamePackageStutter) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/filename-package-stutter",
		Description: "Filenames must not repeat the package name",
		Category:    "naming",
		Tier:        lint.TierUniversal,
	}
}

func (r *FilenamePackageStutter) Check(ctx *lint.Context) error {
	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		// Skip alias files
		if file.Location.Tag("isAlias") == "true" {
			return nil
		}

		base := filepath.Base(file.RelPath)
		name := strings.TrimSuffix(base, ".go")

		pkg := file.Package

		// Normalize both: lowercase and remove underscores
		normalizedName := strings.ReplaceAll(strings.ToLower(name), "_", "")
		normalizedPkg := strings.ReplaceAll(strings.ToLower(pkg), "_", "")

		if normalizedName == normalizedPkg {
			ctx.Report.Add(lint.Violation{
				Rule:     "naming/filename-package-stutter",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     1,
				Message:  fmt.Sprintf("filename %q repeats package name %q", base, pkg),
				Found:    base,
			})
		}

		return nil
	})
}
