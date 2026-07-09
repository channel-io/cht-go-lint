package lint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// mergeColocatedNodes walks the repo for nested .cht-go-lint.yaml files and
// merges each into the tree at the node matching its directory. The root config
// itself is skipped (already loaded). The walk reuses the analyzer's skip set and
// the configured exclude_paths so vendored or generated config files never enter
// the tree. A co-located file that cannot be read or parsed is recorded as a tree
// issue (surfaced as a diagnostic) rather than silently dropped — a broken policy
// file must not quietly disable enforcement for its subtree.
//
// A `template:` reference in a co-located file is expanded against the same
// root-level templates as inline children, so co-located nodes stamp layer
// wiring exactly like root-declared ones. (default_template is only applied to
// root children; co-located nodes must reference their template explicitly.)
func mergeColocatedNodes(tree *NodeTree, root string, excludePaths []string, templates map[string]map[string]*NodeConfig) {
	if root == "" {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// A directory that cannot be scanned would silently drop any
			// co-located config beneath it — surface it as a diagnostic.
			rel, e := filepath.Rel(root, path)
			if e != nil {
				rel = path // fall back so the scan error is never dropped
			}
			rel = filepath.ToSlash(rel)
			tree.Issues = append(tree.Issues, NodeIssue{Path: rel, Message: fmt.Sprintf("failed to scan %s: %v", rel, err)})
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			if excludedDir(root, path, excludePaths) {
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
			// A found config that cannot be located relative to root must not be
			// dropped silently — that would disable enforcement for its subtree.
			tree.Issues = append(tree.Issues, NodeIssue{Path: filepath.ToSlash(dir), Message: fmt.Sprintf("failed to resolve config path %s: %v", filepath.ToSlash(path), relErr)})
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
		// Expand any `template:` reference before attaching. Key the node by its
		// own directory name so template-resolution issues report a useful path.
		name := rel
		if i := strings.LastIndex(rel, "/"); i >= 0 {
			name = rel[i+1:]
		}
		var issues []NodeIssue
		expandTemplates(map[string]*NodeConfig{name: &nc}, templates, map[string]bool{}, &issues)
		tree.Issues = append(tree.Issues, issues...)
		tree.attachNodeAt(rel, &nc)
		return nil
	})
}
