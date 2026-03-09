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

	// Scoped rules: additional forbidden dir checks limited to specific paths
	type scopedRule struct {
		scopePaths []string
		names      map[string]bool
	}
	var scopedRules []scopedRule
	if rawScoped := ctx.Options.MapSlice("scoped"); len(rawScoped) > 0 {
		for _, s := range rawScoped {
			var paths []string
			switch v := s["scope_paths"].(type) {
			case []any:
				for _, item := range v {
					if str, ok := item.(string); ok {
						paths = append(paths, str)
					}
				}
			case []string:
				paths = v
			}
			nameSet := make(map[string]bool)
			switch v := s["names"].(type) {
			case []any:
				for _, item := range v {
					if str, ok := item.(string); ok {
						nameSet[str] = true
					}
				}
			case []string:
				for _, str := range v {
					nameSet[str] = true
				}
			}
			if len(paths) > 0 && len(nameSet) > 0 {
				scopedRules = append(scopedRules, scopedRule{scopePaths: paths, names: nameSet})
			}
		}
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

		relPath, _ := filepath.Rel(root, path)
		relPath = filepath.ToSlash(relPath)

		// Check global forbidden dirs
		if forbidden[name] {
			ctx.Report.Add(lint.Violation{
				Rule:     "structure/forbidden-dirs",
				Severity: ctx.Severity,
				File:     relPath,
				Message:  fmt.Sprintf("directory %q is forbidden", name),
				Found:    name,
			})
		}

		// Check scoped rules
		for _, sr := range scopedRules {
			if !sr.names[name] {
				continue
			}
			parentPath := filepath.ToSlash(filepath.Dir(relPath))
			for _, scopePath := range sr.scopePaths {
				if matched, _ := filepath.Match(scopePath, parentPath); matched {
					ctx.Report.Add(lint.Violation{
						Rule:     "structure/forbidden-dirs",
						Severity: ctx.Severity,
						File:     relPath,
						Message:  fmt.Sprintf("directory %q is forbidden in this location", name),
						Found:    name,
					})
					break
				}
			}
		}
		return nil
	})
}
