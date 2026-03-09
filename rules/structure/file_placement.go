package structure

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&FilePlacement{})
}

// FilePlacement enforces that certain files can only exist in specific directories.
// This generalizes rules like "fx.go must be in a directory ending with fx".
//
// Options:
//
//	rules: list of rule objects, each with:
//	  filename: string - the filename to constrain (e.g., "fx.go")
//	  dir_suffix: string - an ancestor directory must end with this suffix
//	  dir_pattern: string - an ancestor directory must match this glob pattern
//	  skip_dirs: []string - skip files under these top-level directories
type FilePlacement struct{}

func (r *FilePlacement) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/file-placement",
		Description: "Enforce that certain files can only exist in specific directories",
		Category:    "structure",
		Tier:        lint.TierUniversal,
	}
}

func (r *FilePlacement) Check(ctx *lint.Context) error {
	rules := ctx.Options.MapSlice("rules")
	if len(rules) == 0 {
		return nil
	}

	type placementRule struct {
		filename   string
		dirSuffix  string
		dirPattern string
		skipDirs   map[string]bool
	}

	var pRules []placementRule
	for _, rr := range rules {
		fn, _ := rr["filename"].(string)
		if fn == "" {
			continue
		}
		pr := placementRule{filename: fn}
		pr.dirSuffix, _ = rr["dir_suffix"].(string)
		pr.dirPattern, _ = rr["dir_pattern"].(string)

		skipList := toStringSliceFP(rr["skip_dirs"])
		if len(skipList) > 0 {
			pr.skipDirs = make(map[string]bool, len(skipList))
			for _, d := range skipList {
				pr.skipDirs[d] = true
			}
		}

		pRules = append(pRules, pr)
	}

	root := ctx.Analyzer.Root()

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		base := filepath.Base(path)

		for _, pr := range pRules {
			if base != pr.filename {
				continue
			}

			// Check skip_dirs
			rel := file.RelPath
			if len(pr.skipDirs) > 0 {
				topDir := strings.SplitN(rel, "/", 2)[0]
				if pr.skipDirs[topDir] {
					continue
				}
			}

			// Walk up the directory tree to find matching ancestor
			found := false
			dir := filepath.Dir(path)
			for dir != root && dir != "." && dir != "/" {
				dirName := filepath.Base(dir)
				if pr.dirSuffix != "" && strings.HasSuffix(dirName, pr.dirSuffix) {
					found = true
					break
				}
				if pr.dirPattern != "" {
					if matched, _ := filepath.Match(pr.dirPattern, dirName); matched {
						found = true
						break
					}
				}
				dir = filepath.Dir(dir)
			}

			if !found {
				expected := ""
				if pr.dirSuffix != "" {
					expected = fmt.Sprintf("directory ending with %q", pr.dirSuffix)
				}
				if pr.dirPattern != "" {
					expected = fmt.Sprintf("directory matching %q", pr.dirPattern)
				}
				ctx.Report.Add(lint.Violation{
					Rule:     "structure/file-placement",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     1,
					Message:  fmt.Sprintf("%s must be inside %s", pr.filename, expected),
					Found:    filepath.Base(filepath.Dir(path)),
					Expected: expected,
				})
			}
		}

		return nil
	})
}

func toStringSliceFP(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}
