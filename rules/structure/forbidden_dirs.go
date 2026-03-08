package structure

import (
	"fmt"
	"io/fs"
	"path/filepath"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ForbiddenDirs{})
}

// ForbiddenDirs forbids certain directory names anywhere in the project.
type ForbiddenDirs struct{}

func (r *ForbiddenDirs) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/forbidden-dirs",
		Description: "Forbid certain directory names anywhere in the project",
		Category:    "structure",
		Tier:        lint.TierUniversal,
	}
}

var defaultForbiddenDirs = []string{"util", "utils", "common", "misc", "helper", "helpers", "shared"}

func (r *ForbiddenDirs) Check(ctx *lint.Context) error {
	names := ctx.Options.StringSlice("names")
	if len(names) == 0 {
		names = defaultForbiddenDirs
	}
	forbidden := make(map[string]bool, len(names))
	for _, n := range names {
		forbidden[n] = true
	}

	root := ctx.Analyzer.Root()
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		// Skip hidden and vendor dirs
		if name == ".git" || name == "vendor" || name == "node_modules" || name == "testdata" {
			return filepath.SkipDir
		}
		if forbidden[name] {
			relPath, _ := filepath.Rel(root, path)
			relPath = filepath.ToSlash(relPath)
			ctx.Report.Add(lint.Violation{
				Rule:     "structure/forbidden-dirs",
				Severity: ctx.Severity,
				File:     relPath,
				Message:  fmt.Sprintf("directory %q is forbidden", name),
				Found:    name,
			})
		}
		return nil
	})
}
