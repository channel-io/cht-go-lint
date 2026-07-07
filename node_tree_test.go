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

	cols := tree.InternalCollisions(root)
	if len(cols) != 1 || cols[0].Name != "codec" || cols[0].Node != "kafka" {
		t.Fatalf("want one codec collision under kafka, got %+v", cols)
	}

	// No internal/ dir under a walling node → no collisions.
	clean := t.TempDir()
	if err := os.MkdirAll(filepath.Join(clean, "pkg/kafka/producer"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := tree.InternalCollisions(clean); len(got) != 0 {
		t.Fatalf("want no collisions, got %+v", got)
	}
}
