package lint

import "strings"

// NodeTreeStrategy is the recursive node-tree LocationStrategy. It resolves a
// file or import path to its node chain via the assembled tree.
type NodeTreeStrategy struct {
	tree *NodeTree
}

// NewNodeTreeStrategy wraps an assembled tree as a LocationStrategy.
func NewNodeTreeStrategy(tree *NodeTree) *NodeTreeStrategy {
	return &NodeTreeStrategy{tree: tree}
}

// Tree exposes the underlying node tree (used by the unified import rule).
func (s *NodeTreeStrategy) Tree() *NodeTree { return s.tree }

// Identify returns the location of a file path relative to the module root.
func (s *NodeTreeStrategy) Identify(relPath string) Location {
	return Location{Tags: map[string]string{}, Nodes: s.tree.Chain(relPath)}
}

// ParseImport returns the location of an internal import path.
func (s *NodeTreeStrategy) ParseImport(importPath, modulePath string) ImportLocation {
	iloc := ImportLocation{}
	if importPath != modulePath && !strings.HasPrefix(importPath, modulePath+"/") {
		return iloc
	}
	iloc.IsSameModule = true
	rel := strings.TrimPrefix(importPath, modulePath+"/")
	iloc.Nodes = s.tree.Chain(rel)
	iloc.IsInternal = hasInternalSegment(rel)
	return iloc
}

func hasInternalSegment(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == "internal" {
			return true
		}
	}
	return false
}
