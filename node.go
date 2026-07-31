package lint

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	internalSegment = "internal" // non-walling path segment (a Go visibility marker, not a node)
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
// unknown or self-referential template, or a co-located config that failed to
// parse. Surfaced as a diagnostic so a broken config fails
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
	for _, key := range nodeKeys(rel) {
		next, ok := cur.Children[key]
		if !ok {
			if cur.Walling {
				chain = append(chain, &Node{
					Name:   key,
					Path:   joinNodePath(cur.Path, key),
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

// nodeKeys splits a module-relative directory into node keys.
//
// `internal/` is a Go visibility marker, not an architecture node: it does not
// wall its children, so they belong to the enclosing walling node as ordinary
// deny-default siblings. It stays in the key of the segment that follows, so a
// node's name and path match where it sits on disk — `internal/codec` rather
// than `codec`. Go already blocks any reach from outside the subtree; cht only
// governs the edges inside it.
func nodeKeys(rel string) []string {
	segments := strings.Split(rel, "/")
	keys := make([]string, 0, len(segments))
	prefix := ""
	for _, segment := range segments {
		if segment == internalSegment {
			prefix += internalSegment + "/"
			continue
		}
		keys = append(keys, prefix+segment)
		prefix = ""
	}
	return keys
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
	for _, key := range nodeKeys(rel) {
		next, ok := cur.Children[key]
		if !ok {
			next = &Node{
				Name:     key,
				Path:     joinNodePath(cur.Path, key),
				Parent:   cur,
				Children: map[string]*Node{},
			}
			cur.Children[key] = next
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
