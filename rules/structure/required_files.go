package structure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&RequiredFiles{})
}

// RequiredFiles validates that certain files exist in directories matching
// configurable patterns. This generalizes domain-specific checks like
// "domains with subdomains must have alias.go".
//
// Options:
//
//	rules: list of rule objects, each with:
//	  scope: string - glob pattern for directories to check (e.g., "internal/domain/*")
//	  required: []string - filenames that must exist
//	  skip_suffix: string - skip directories ending with this suffix
//	  when_has_subdirs: bool - only require files when dir has subdirectories (non-layer dirs)
//	  layer_dirs: []string - directory names considered layer dirs (not subdomains)
type RequiredFiles struct{}

func (r *RequiredFiles) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/required-files",
		Description: "Validate that required files exist in directories matching patterns",
		Category:    "structure",
		Tier:        lint.TierComponentAware,
	}
}

func (r *RequiredFiles) Check(ctx *lint.Context) error {
	rules := ctx.Options.MapSlice("rules")
	if len(rules) == 0 {
		return nil
	}

	root := ctx.Analyzer.Root()

	for _, rule := range rules {
		scope, _ := rule["scope"].(string)
		if scope == "" {
			continue
		}

		required := toStringSlice(rule["required"])
		if len(required) == 0 {
			continue
		}

		skipSuffix, _ := rule["skip_suffix"].(string)
		whenHasSubdirs, _ := rule["when_has_subdirs"].(bool)
		layerDirNames := toStringSlice(rule["layer_dirs"])
		layerDirSet := make(map[string]bool, len(layerDirNames))
		for _, d := range layerDirNames {
			layerDirSet[d] = true
		}

		// Expand scope glob to find matching directories
		matches, err := filepath.Glob(filepath.Join(root, scope))
		if err != nil {
			continue
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || !info.IsDir() {
				continue
			}

			dirName := filepath.Base(match)

			// Skip directories ending with skipSuffix
			if skipSuffix != "" && strings.HasSuffix(dirName, skipSuffix) {
				continue
			}

			relDir, _ := filepath.Rel(root, match)
			relDir = filepath.ToSlash(relDir)

			// Check when_has_subdirs condition
			if whenHasSubdirs {
				hasSubdomain := false
				children, err := ctx.Analyzer.ListDirs(relDir)
				if err != nil {
					continue
				}
				for _, child := range children {
					if skipSuffix != "" && strings.HasSuffix(child, skipSuffix) {
						continue
					}
					if layerDirSet[child] {
						continue
					}
					hasSubdomain = true
					break
				}
				if !hasSubdomain {
					continue
				}
			}

			// Check each required file
			for _, reqFile := range required {
				filePath := filepath.Join(match, reqFile)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					ctx.Report.Add(lint.Violation{
						Rule:     "structure/required-files",
						Severity: ctx.Severity,
						File:     relDir,
						Message:  fmt.Sprintf("directory %q is missing required file %q", dirName, reqFile),
						Expected: reqFile,
					})
				}
			}
		}
	}

	return nil
}

func toStringSlice(v any) []string {
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
