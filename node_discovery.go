package lint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// mergeColocatedNodes walks the repo for nested .cht-go-lint.yaml files and
// merges each into the tree at the node matching its directory. The root config
// itself is skipped (already loaded). The walk reuses the analyzer's skip set and
// the configured exclude_paths so vendored or generated config files never enter
// the tree. A co-located file that cannot be read or parsed is recorded as a tree
// issue (surfaced as a diagnostic) rather than silently dropped — a broken policy
// file must not quietly disable enforcement for its subtree.
func mergeColocatedNodes(tree *NodeTree, root string, excludePaths []string) {
	if root == "" {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			if rel, e := filepath.Rel(root, path); e == nil && rel != "." && pathExcluded(rel, excludePaths) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != ".cht-go-lint.yaml" && d.Name() != ".cht-go-lint.yml" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, relErr := filepath.Rel(root, dir)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil // the root config — already loaded
		}
		configPath := filepath.ToSlash(filepath.Join(rel, d.Name()))
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			tree.Issues = append(tree.Issues, NodeIssue{Path: rel, Message: fmt.Sprintf("failed to read %s: %v", configPath, readErr)})
			return nil
		}
		var nc NodeConfig
		if err := yaml.Unmarshal(data, &nc); err != nil {
			tree.Issues = append(tree.Issues, NodeIssue{Path: rel, Message: fmt.Sprintf("failed to parse %s: %v", configPath, err)})
			return nil
		}
		tree.attachNodeAt(rel, &nc)
		return nil
	})
}
