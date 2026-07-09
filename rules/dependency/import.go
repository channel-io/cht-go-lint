package dependency

import (
	"fmt"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&Import{})
}

// Import is the unified dependency rule for the recursive node-tree model. It
// subsumes module-isolation, layer-direction, cross-boundary and
// subdomain-isolation: every internal import resolves to a check between the two
// nodes where the source and target chains diverge.
type Import struct{}

func (r *Import) Meta() lint.Meta {
	return lint.Meta{
		Name:        "dependency/import",
		Description: "Enforce the node-tree import graph (sibling isolation + may_import / shared)",
		Category:    "dependency",
		Tier:        lint.TierUniversal, // no-op unless the node-tree strategy populated chains
	}
}

func (r *Import) Check(ctx *lint.Context) error {
	return ctx.Analyzer.WalkGoFiles(func(_ string, file *lint.ParsedFile) error {
		src := file.Location.Nodes
		if len(src) == 0 {
			return nil // file owns no node (legacy strategy, or outside roots)
		}
		for _, imp := range file.Imports {
			if !ctx.Analyzer.IsInternalImport(imp.Path) {
				continue
			}
			iloc := ctx.Analyzer.ImportLocation(imp.Path)
			tgt := iloc.Nodes
			if len(tgt) == 0 {
				continue // imported package owns no node
			}
			ok, sc, tc := importAllowed(src, tgt)
			if ok {
				continue
			}
			ctx.Report.Add(lint.Violation{
				Rule:     "dependency/import",
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     imp.Pos.Line,
				Message:  fmt.Sprintf("node %q must not import sibling %q", sc.Path, tc.Path),
				Found:    tc.Path,
			})
		}
		return nil
	})
}

// importAllowed applies the node-tree default policy to two node chains. When the
// import is a violation it also returns the divergence-level sibling pair (Sc →
// Tc) for the diagnostic; on an allowed import both are nil.
func importAllowed(src, tgt []*lint.Node) (bool, *lint.Node, *lint.Node) {
	// Vertical: one chain is a prefix of the other (ancestor/descendant). Always
	// open — a node sees its own subtree and may reach up to its enclosing
	// features.
	if isPrefix(src, tgt) || isPrefix(tgt, src) {
		return true, nil, nil
	}
	// Horizontal: check the sibling pair at the divergence point.
	sc, tc := divergence(src, tgt)
	if sc == nil || tc == nil {
		return true, nil, nil
	}
	// A wall exists only when the common parent explicitly walls its children.
	// Intermediate nodes auto-created to host a deeper config are transparent, so
	// a deep config never walls a leaf ancestor's branches against each other.
	if sc.Parent == nil || !sc.Parent.Walling {
		return true, nil, nil
	}
	if tc.Shared {
		return true, nil, nil // sc lies in tc's parent subtree by construction
	}
	if contains(sc.MayImport, tc.Name) {
		return true, nil, nil
	}
	return false, sc, tc
}

// divergence returns the sibling nodes where the two chains first differ.
func divergence(a, b []*lint.Node) (*lint.Node, *lint.Node) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		// Compare by logical path: a node undeclared in config is synthesised
		// fresh per resolution, so two chains through the same directory hold
		// different *Node pointers for one logical node.
		if a[i].Path != b[i].Path {
			return a[i], b[i]
		}
	}
	return nil, nil
}

// isPrefix reports whether a is a prefix of b (a is an ancestor of, or equal to, b).
func isPrefix(a, b []*lint.Node) bool {
	if len(a) > len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path {
			return false
		}
	}
	return true
}
