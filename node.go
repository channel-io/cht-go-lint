package lint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	internalSegment = "internal" // transparent path segment (a Go visibility marker, not a node)
	templateNone    = "none"     // reserved template value: explicit opt-out
)

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

// applyDefaultTemplate gives every top-level node the default template unless it
// opts out. A node with its own `children` or `template` is left as-is; the
// reserved value `none` is an explicit opt-out (a foundation with no layers).
func applyDefaultTemplate(children map[string]*NodeConfig, name string) {
	if name == "" {
		return
	}
	for _, c := range children {
		if c == nil {
			continue
		}
		if c.Template == "" && len(c.Children) == 0 {
			c.Template = name
		}
	}
}

// NodeIssue is a configuration problem found while assembling the tree — an
// unknown or self-referential template, a co-located config that failed to parse,
// or a hoist name collision. Surfaced as a diagnostic so a broken config fails
// loudly instead of silently disabling enforcement.
type NodeIssue struct {
	Path    string // where to anchor the diagnostic (dir or config path); "" if none
	Message string
}

// expandTemplates resolves every `template:` reference in a children tree by
// merging a deep copy of the named template's children into the referencing node
// (a node's own children win). Templates let domains reuse one layer wiring
// instead of repeating it. The reserved name `none` expands to nothing. An unknown
// template name and a self-referential template are recorded as issues rather than
// being silently ignored or looping forever; `active` tracks the templates on the
// current expansion path to detect the cycle.
func expandTemplates(children map[string]*NodeConfig, templates map[string]map[string]*NodeConfig, active map[string]bool, issues *[]NodeIssue) {
	for name, c := range children {
		if c == nil {
			continue
		}
		if c.Template != "" && c.Template != templateNone {
			switch tmpl, ok := templates[c.Template]; {
			case !ok:
				*issues = append(*issues, NodeIssue{Path: name, Message: fmt.Sprintf("node %q references unknown template %q", name, c.Template)})
			case active[c.Template]:
				*issues = append(*issues, NodeIssue{Path: name, Message: fmt.Sprintf("node %q references self-referential template %q", name, c.Template)})
			default:
				if c.Children == nil {
					c.Children = map[string]*NodeConfig{}
				}
				for tn, tc := range tmpl {
					if _, exists := c.Children[tn]; !exists {
						c.Children[tn] = cloneNodeConfig(tc)
					}
				}
				active[c.Template] = true
				expandTemplates(c.Children, templates, active, issues)
				delete(active, c.Template)
				continue
			}
		}
		expandTemplates(c.Children, templates, active, issues)
	}
}

// cloneNodeConfig deep-copies a NodeConfig so a template's child configs are not
// aliased into every node that references the template (a later per-node edit
// would otherwise leak across domains).
func cloneNodeConfig(c *NodeConfig) *NodeConfig {
	if c == nil {
		return nil
	}
	n := &NodeConfig{Shared: c.Shared, Template: c.Template}
	if c.MayImport != nil {
		n.MayImport = append([]string(nil), c.MayImport...)
	}
	if len(c.Children) > 0 {
		n.Children = make(map[string]*NodeConfig, len(c.Children))
		for k, v := range c.Children {
			n.Children[k] = cloneNodeConfig(v)
		}
	}
	return n
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
	Root   *Node       // the synthetic root node (Path "")
	Roots  []string    // path prefixes under which top-level nodes live, e.g. ["pkg"]
	Issues []NodeIssue // configuration problems found while assembling (surfaced as diagnostics)
}

// BuildNodeTree assembles a tree from the root node's children.
func BuildNodeTree(roots []string, children map[string]*NodeConfig) *NodeTree {
	// The repo root is always a walling node (RFC invariant): its top-level
	// features are isolated by default even when every node is declared in a
	// co-located file and the root config lists no inline children.
	root := &Node{Children: map[string]*Node{}, Walling: true}
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
		if seg == internalSegment {
			// `internal/` is a Go visibility marker, not an architecture node.
			// Skip it transparently so its children hoist to cur's level as
			// deny-default siblings. Go already blocks any reach from outside
			// cur's subtree, so cht only governs the intra-subtree edges.
			continue
		}
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
		if seg == internalSegment {
			continue // transparent segment — mirrors Chain's hoisting
		}
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

// InternalCollision is a directory where a hoisted `internal/<name>` would claim
// the same node name as a real sibling `<name>` under the same walling node. Both
// map to one node, so the resolution is ambiguous — a configuration error.
type InternalCollision struct {
	Dir  string // the offending internal/<name> directory, relative to root (slash)
	Name string // the clashing node name
	Node string // logical path of the walling node the two would share
}

// InternalCollisions scans the filesystem for hoist name clashes: an `internal/`
// directory whose child names overlap the real sibling directories under the same
// walling node. Because `internal/` is transparent, both would resolve to a single
// child node of that walling node. Clashes only arise when the hoisted node is
// actually present, so most repos report none.
func (t *NodeTree) InternalCollisions(root string, excludePaths []string) []InternalCollision {
	var out []InternalCollision
	if root == "" {
		return out
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Scan failures are surfaced by mergeColocatedNodes, which walks the
			// same tree; keep this a pure query so re-invoking it is idempotent
			// and the diagnostic is not emitted twice.
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if excludedDir(root, path, excludePaths) {
			return filepath.SkipDir
		}
		if skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if d.Name() != internalSegment {
			return nil
		}
		rel, e := filepath.Rel(root, filepath.Dir(path))
		if e != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		node, ok := t.nodeForDir(rel)
		if !ok || !node.Walling {
			return nil // only a walling parent materialises hoisted child nodes
		}
		siblings := childDirSet(filepath.Dir(path))
		delete(siblings, internalSegment)
		for name := range childDirSet(path) {
			if siblings[name] {
				out = append(out, InternalCollision{
					Dir:  filepath.ToSlash(filepath.Join(rel, internalSegment, name)),
					Name: name,
					Node: node.Path,
				})
			}
		}
		return nil
	})
	return out
}

// pathExcluded reports whether a module-relative directory matches one of the
// configured exclude_paths (same prefix semantics as CodebaseAnalyzer.IsExcluded).
func pathExcluded(rel string, excludePaths []string) bool {
	rel = filepath.ToSlash(rel)
	for _, p := range excludePaths {
		p = filepath.ToSlash(strings.TrimSuffix(p, "/"))
		if p == "" {
			continue
		}
		if rel == p || strings.HasPrefix(rel, p+"/") {
			return true
		}
	}
	return false
}

// excludedDir reports whether the directory at path (under root) is covered by
// exclude_paths and its subtree should be skipped during a walk.
func excludedDir(root, path string, excludePaths []string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && pathExcluded(rel, excludePaths)
}

// nodeForDir returns the deepest node owning a directory relative to the module
// root (roots prefix included). ok is false when the directory lies outside every
// configured root.
func (t *NodeTree) nodeForDir(rel string) (*Node, bool) {
	if r, ok := t.stripRoot(rel); ok && r == "" {
		return t.Root, true
	}
	chain := t.Chain(rel)
	if len(chain) == 0 {
		return nil, false
	}
	return chain[len(chain)-1], true
}

// childDirSet returns the immediate subdirectory names of dir (skip set excluded).
func childDirSet(dir string) map[string]bool {
	set := map[string]bool{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return set
	}
	for _, e := range entries {
		if e.IsDir() && !skipDirs[e.Name()] {
			set[e.Name()] = true
		}
	}
	return set
}
