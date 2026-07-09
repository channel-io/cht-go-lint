package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleTree() *NodeTree {
	return BuildNodeTree([]string{"pkg"}, map[string]*NodeConfig{
		"errors":  {Shared: true},
		"sqlrepo": {},
		"kafka": {
			Children: map[string]*NodeConfig{
				"core":     {Shared: true},
				"producer": {},
				"consumer": {MayImport: []string{"producer"}},
			},
		},
	})
}

func chainPaths(nodes []*Node) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = n.Path
	}
	return strings.Join(parts, " > ")
}

func TestNodeTreeChain(t *testing.T) {
	tree := sampleTree()
	// Chain takes a directory (package) path; the caller drops any file name.
	tests := []struct {
		dir  string
		want string
	}{
		{"pkg/kafka/consumer", "kafka > kafka/consumer"},
		{"pkg/kafka/consumer/pool", "kafka > kafka/consumer"}, // consumer is a leaf → pool is its code
		{"pkg/kafka", "kafka"},
		{"pkg/errors", "errors"},
		{"pkg/sqlrepo", "sqlrepo"},
		{"pkg/kafka/newdir", "kafka > kafka/newdir"}, // undeclared under walling kafka → deny node
		{"pkg/newfeature", "newfeature"},             // undeclared top-level → deny node under root
		{"pkg/kafka/internal/codec", "kafka > kafka/codec"},     // internal/ skipped; codec hoists to a kafka child
		{"pkg/kafka/internal/codec/sub", "kafka > kafka/codec"}, // codec is a leaf → sub is its code
		{"pkg/kafka/internal", "kafka"},                         // lone internal/ is transparent → no node
		{"other", ""},                                // outside roots
	}
	for _, tt := range tests {
		got := chainPaths(tree.Chain(tt.dir))
		if got != tt.want {
			t.Errorf("Chain(%q) = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestNodeTreePolicy(t *testing.T) {
	tree := sampleTree()
	kafka := tree.Root.Children["kafka"]

	if !kafka.IsWalling() {
		t.Error("kafka should be a walling node (has children)")
	}
	if kafka.Children["consumer"].IsWalling() {
		t.Error("consumer should be a leaf (no children)")
	}

	consumer := kafka.Children["consumer"]
	if len(consumer.MayImport) != 1 || consumer.MayImport[0] != "producer" {
		t.Errorf("consumer.MayImport = %v, want [producer]", consumer.MayImport)
	}
	if !kafka.Children["core"].Shared {
		t.Error("core should be shared")
	}
	if kafka.Children["producer"].Shared {
		t.Error("producer should not be shared")
	}

	// Parent links and paths.
	if consumer.Parent != kafka {
		t.Error("consumer.Parent should be kafka")
	}
	if consumer.Path != "kafka/consumer" {
		t.Errorf("consumer.Path = %q, want kafka/consumer", consumer.Path)
	}
}

func TestInternalCollisions(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"pkg/kafka/codec", "pkg/kafka/internal/codec", "pkg/kafka/producer"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// kafka is walling (declares producer), so both pkg/kafka/codec and the
	// hoisted pkg/kafka/internal/codec would claim the child name "codec".
	tree := BuildNodeTree([]string{"pkg"}, map[string]*NodeConfig{
		"kafka": {Children: map[string]*NodeConfig{"producer": {}}},
	})

	cols := tree.InternalCollisions(root, nil)
	if len(cols) != 1 || cols[0].Name != "codec" || cols[0].Node != "kafka" {
		t.Fatalf("want one codec collision under kafka, got %+v", cols)
	}

	// No internal/ dir under a walling node → no collisions.
	clean := t.TempDir()
	if err := os.MkdirAll(filepath.Join(clean, "pkg/kafka/producer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := tree.InternalCollisions(clean, nil); len(got) != 0 {
		t.Fatalf("want no collisions, got %+v", got)
	}
}

func TestExpandTemplatesNoneOptOut(t *testing.T) {
	children := map[string]*NodeConfig{
		"errs":  {Shared: true, Template: templateNone},
		"order": {},
	}
	applyDefaultTemplate(children, "layers")
	var issues []NodeIssue
	expandTemplates(children, map[string]map[string]*NodeConfig{
		"layers": {"model": {}, "svc": {MayImport: []string{"model"}}},
	}, map[string]bool{}, &issues)

	if len(children["errs"].Children) != 0 {
		t.Errorf("template:none must not receive template children, got %v", children["errs"].Children)
	}
	if len(children["order"].Children) == 0 {
		t.Errorf("order should have received the default layers template")
	}
	if svc := children["order"].Children["svc"]; svc == nil || len(svc.MayImport) != 1 || svc.MayImport[0] != "model" {
		t.Errorf("order.svc should carry may_import [model] from the template, got %+v", svc)
	}
	if len(issues) != 0 {
		t.Errorf("no issues expected, got %v", issues)
	}
}

func TestExpandTemplatesIndirectCycleTerminates(t *testing.T) {
	children := map[string]*NodeConfig{"order": {Template: "a"}}
	var issues []NodeIssue
	// a -> b -> a: must terminate (cycle guard), not recurse forever.
	expandTemplates(children, map[string]map[string]*NodeConfig{
		"a": {"x": {Template: "b"}},
		"b": {"y": {Template: "a"}},
	}, map[string]bool{}, &issues)

	found := false
	for _, iss := range issues {
		if strings.Contains(iss.Message, "self-referential") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a self-referential cycle issue, got %v", issues)
	}
}

func TestPathExcludedBoundary(t *testing.T) {
	tests := []struct {
		rel      string
		excludes []string
		want     bool
	}{
		{"pkg/kafka", []string{"pkg/kafka"}, true},
		{"pkg/kafka/producer", []string{"pkg/kafka"}, true},
		{"pkg/kafka2", []string{"pkg/kafka"}, false},        // similar name, not a segment prefix
		{"pkg/kafka/producer", []string{"pkg/kafka/"}, true}, // trailing slash normalized
		{"pkg/kafka", nil, false},
	}
	for _, tt := range tests {
		if got := pathExcluded(tt.rel, tt.excludes); got != tt.want {
			t.Errorf("pathExcluded(%q, %v) = %v, want %v", tt.rel, tt.excludes, got, tt.want)
		}
	}
}

func TestCloneNodeConfigIsolation(t *testing.T) {
	children := map[string]*NodeConfig{
		"order":   {Template: "layers"},
		"payment": {Template: "layers"},
	}
	var issues []NodeIssue
	expandTemplates(children, map[string]map[string]*NodeConfig{
		"layers": {"svc": {MayImport: []string{"model"}}},
	}, map[string]bool{}, &issues)

	// Mutating order's expanded template child must not leak to payment's.
	children["order"].Children["svc"].MayImport[0] = "MUTATED"
	if got := children["payment"].Children["svc"].MayImport[0]; got != "model" {
		t.Errorf("template child-set must be deep-copied per node; payment leaked: %q", got)
	}
}

func TestNodeTreeStrategyParseImport(t *testing.T) {
	// No roots configured: the module root maps directly onto the node tree.
	// The bug only surfaces here — with roots, stripRoot filters the bogus rel
	// out anyway, masking it.
	tree := BuildNodeTree(nil, map[string]*NodeConfig{
		"kafka": {Children: map[string]*NodeConfig{"producer": {}}},
	})
	s := NewNodeTreeStrategy(tree)
	const mod = "example.com/test"

	tests := []struct {
		name       string
		importPath string
		wantSame   bool
		wantChain  string
	}{
		// importPath == modulePath is the module-root package itself; its rel is
		// empty, so it resolves to no node chain — not a bogus chain built from
		// the module path's own segments.
		{"module root package", mod, true, ""},
		{"in-module package", mod + "/kafka/producer", true, "kafka > kafka/producer"},
		{"out-of-module", "other.com/lib", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.ParseImport(tt.importPath, mod)
			if got.IsSameModule != tt.wantSame {
				t.Errorf("IsSameModule = %v, want %v", got.IsSameModule, tt.wantSame)
			}
			if chain := chainPaths(got.Nodes); chain != tt.wantChain {
				t.Errorf("Nodes = %q, want %q", chain, tt.wantChain)
			}
		})
	}
}
