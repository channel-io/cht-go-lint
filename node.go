package lint

import "strings"

// NodeConfig is the YAML shape of a node: its policy plus its children.
//
// `may_import` and `shared` are declared on a node by its parent (the parent's
// `children` map keys this config), so they live with the sibling wiring.
type NodeConfig struct {
	// Shared marks the node importable by any node within its parent's subtree.
	Shared bool `yaml:"shared,omitempty"`
	// MayImport lists the sibling names this node may import.
	MayImport []string `yaml:"may_import,omitempty"`
	// Children declares the direct subdirectories that are wired as child nodes.
	Children map[string]*NodeConfig `yaml:"children,omitempty"`
	// Template names a reusable child-set (from the root `templates:`) to apply as
	// this node's children — e.g. a global layer wiring shared by every domain.
	// Explicitly listed children win over template entries of the same name.
	Template string `yaml:"template,omitempty"`
}

// expandTemplates resolves every `template:` reference in a children tree by
// merging the named template's children into the referencing node. Templates are
// shared, read-only NodeConfig values, so domains can reuse one layer wiring
// instead of repeating it.
func expandTemplates(children map[string]*NodeConfig, templates map[string]map[string]*NodeConfig) {
	for _, c := range children {
		if c == nil {
			continue
		}
		if c.Template != "" {
			if tmpl, ok := templates[c.Template]; ok {
				if c.Children == nil {
					c.Children = map[string]*NodeConfig{}
				}
				for name, tc := range tmpl {
					if _, exists := c.Children[name]; !exists {
						c.Children[name] = tc
					}
				}
			}
		}
		expandTemplates(c.Children, templates)
	}
}

// Node is one directory in the architecture tree.
//
// A node is "walling" when it has children (it walls them apart). A node is
// "walled" — isolated from its siblings — by virtue of its parent having
// declared it. Both `Shared` and `MayImport` are the node's own policy, supplied
// by the parent that declared it.
type Node struct {
	Name      string           // directory segment, e.g. "consumer"
	Path      string           // slash path from the top feature level, e.g. "kafka/consumer"
	Parent    *Node            // nil for the root
	Children  map[string]*Node // every directory on a config path, incl. auto-created intermediates
	Shared    bool
	MayImport []string

	// Walling is true only when a config explicitly declared this node's
	// children. Intermediate nodes auto-created to host a deeper config are NOT
	// walling — their children carry no wall. The import rule walls a sibling
	// pair only when their common parent is walling, so a deep config never
	// promotes a leaf ancestor into a wall.
	Walling bool
}

// IsWalling reports whether the node walls its children (declared them explicitly).
func (n *Node) IsWalling() bool { return n.Walling }

// NodeTree is the assembled architecture tree.
type NodeTree struct {
	Root  *Node    // the synthetic root node (Path "")
	Roots []string // path prefixes under which top-level nodes live, e.g. ["pkg"]
}

// BuildNodeTree assembles a tree from the root node's children.
func BuildNodeTree(roots []string, children map[string]*NodeConfig) *NodeTree {
	root := &Node{Children: map[string]*Node{}}
	buildChildren(root, children)
	return &NodeTree{Root: root, Roots: roots}
}

func buildChildren(parent *Node, cfgs map[string]*NodeConfig) {
	if len(cfgs) > 0 {
		parent.Walling = true // it explicitly declares children → walls them apart
	}
	for name, c := range cfgs {
		n := &Node{
			Name:     name,
			Path:     joinNodePath(parent.Path, name),
			Parent:   parent,
			Children: map[string]*Node{},
		}
		if c != nil {
			n.Shared = c.Shared
			n.MayImport = c.MayImport
			buildChildren(n, c.Children)
		}
		parent.Children[name] = n
	}
}

func joinNodePath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

// Chain returns the node chain that owns a directory path relative to the module
// root. Callers pass a directory (the package), never a file name. The walk
// descends declared nodes; when it reaches a segment that is not a declared child
// of a **walling** node, that directory is still a node — an unnamed, deny-default
// sibling — so it is materialised once and the walk stops (its own subdirectories
// are its code). Under a non-walling node, an undeclared segment is just that
// node's code and the walk stops with no extra node.
func (t *NodeTree) Chain(relPath string) []*Node {
	rel, ok := t.stripRoot(relPath)
	if !ok || rel == "" {
		return nil
	}
	var chain []*Node
	cur := t.Root
	for _, seg := range strings.Split(rel, "/") {
		next, ok := cur.Children[seg]
		if !ok {
			if cur.Walling {
				chain = append(chain, &Node{
					Name:   seg,
					Path:   joinNodePath(cur.Path, seg),
					Parent: cur,
				})
			}
			break
		}
		chain = append(chain, next)
		cur = next
	}
	return chain
}

// attachNodeAt finds (or creates) the node for a directory relative to the
// module root and applies a co-located config to it — its children wiring, plus
// any policy it carries. Intermediate nodes are created as needed.
func (t *NodeTree) attachNodeAt(relDir string, c *NodeConfig) {
	rel, ok := t.stripRoot(relDir)
	if !ok || rel == "" {
		return
	}
	cur := t.Root
	for _, seg := range strings.Split(rel, "/") {
		next, ok := cur.Children[seg]
		if !ok {
			next = &Node{
				Name:     seg,
				Path:     joinNodePath(cur.Path, seg),
				Parent:   cur,
				Children: map[string]*Node{},
			}
			cur.Children[seg] = next
		}
		cur = next
	}
	if c == nil {
		return
	}
	if c.Shared {
		cur.Shared = true
	}
	if len(c.MayImport) > 0 {
		cur.MayImport = c.MayImport
	}
	buildChildren(cur, c.Children)
}

// stripRoot removes a configured root prefix (e.g. "pkg/"). It returns ok=false
// when the path lies outside every configured root. With no roots configured the
// path is used as-is.
func (t *NodeTree) stripRoot(relPath string) (string, bool) {
	relPath = strings.TrimPrefix(relPath, "./")
	if len(t.Roots) == 0 {
		return relPath, true
	}
	for _, r := range t.Roots {
		if relPath == r {
			return "", true
		}
		if strings.HasPrefix(relPath, r+"/") {
			return strings.TrimPrefix(relPath, r+"/"), true
		}
	}
	return "", false
}
