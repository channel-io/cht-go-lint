package lint

import (
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// mergeColocatedNodes walks the repo for nested .cht-go-lint.yaml files and
// merges each into the tree at the node matching its directory. The root config
// itself is skipped (already loaded). The walk reuses the same skip set as the
// analyzer so vendored or generated config files never enter the tree.
func mergeColocatedNodes(tree *NodeTree, root string) {
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
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var nc NodeConfig
		if yaml.Unmarshal(data, &nc) != nil {
			return nil
		}
		tree.attachNodeAt(rel, &nc)
		return nil
	})
}
