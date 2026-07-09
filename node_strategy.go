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

// Identify returns the location of a file path relative to the module root. A
// file's node is its directory's node, so the trailing file name is dropped
// before resolving the chain.
func (s *NodeTreeStrategy) Identify(relPath string) Location {
	dir := ""
	if i := strings.LastIndex(relPath, "/"); i >= 0 {
		dir = relPath[:i]
	}
	return Location{Tags: map[string]string{}, Nodes: s.tree.Chain(dir)}
}

// ParseImport returns the location of an internal import path.
func (s *NodeTreeStrategy) ParseImport(importPath, modulePath string) ImportLocation {
	iloc := ImportLocation{}
	if importPath != modulePath && !strings.HasPrefix(importPath, modulePath+"/") {
		return iloc
	}
	iloc.IsSameModule = true
	// importPath == modulePath means the module-root package itself; its rel is
	// empty. TrimPrefix would leave the full path intact (no trailing slash to
	// match), so special-case it to avoid feeding a bogus rel to Chain.
	rel := ""
	if importPath != modulePath {
		rel = strings.TrimPrefix(importPath, modulePath+"/")
	}
	iloc.Nodes = s.tree.Chain(rel)
	iloc.IsInternal = hasInternalSegment(rel)
	return iloc
}

func hasInternalSegment(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == internalSegment {
			return true
		}
	}
	return false
}
