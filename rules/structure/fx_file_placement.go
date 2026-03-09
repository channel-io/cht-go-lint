package structure

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&FxFilePlacement{})
}

// FxFilePlacement enforces that files named fx.go must reside inside
// a directory (or ancestor directory) whose name ends with "fx".
type FxFilePlacement struct{}

func (r *FxFilePlacement) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/fx-file-placement",
		Description: "fx.go files must be inside a directory whose name ends with \"fx\"",
		Category:    "structure",
		Tier:        lint.TierUniversal,
	}
}

func (r *FxFilePlacement) Check(ctx *lint.Context) error {
	root := ctx.Analyzer.Root()

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		base := filepath.Base(path)
		if base != "fx.go" {
			return nil
		}

		// Skip files under test/ or vendor/ relative to root
		rel := file.RelPath
		if strings.HasPrefix(rel, "test/") || strings.HasPrefix(rel, "vendor/") {
			return nil
		}

		// Walk up the directory tree from the file's parent dir
		dir := filepath.Dir(path)
		for dir != root && dir != "." && dir != "/" {
			dirName := filepath.Base(dir)
			if strings.HasSuffix(dirName, "fx") {
				return nil
			}
			dir = filepath.Dir(dir)
		}

		ctx.Report.Add(lint.Violation{
			Rule:     "structure/fx-file-placement",
			Severity: ctx.Severity,
			File:     file.RelPath,
			Line:     1,
			Message:  fmt.Sprintf("fx.go must be inside a directory whose name ends with \"fx\""),
			Found:    filepath.Base(filepath.Dir(path)),
			Expected: "directory ending with \"fx\"",
		})
		return nil
	})
}
