package lint

import (
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
	tests := []struct {
		file string
		want string
	}{
		{"pkg/kafka/consumer/x.go", "kafka > kafka/consumer"},
		{"pkg/kafka/consumer/pool/x.go", "kafka > kafka/consumer"}, // pool is not a node
		{"pkg/kafka/kafka.go", "kafka"},                            // file directly under kafka
		{"pkg/errors/errors.go", "errors"},
		{"pkg/sqlrepo/x.go", "sqlrepo"},
		{"other/x.go", ""}, // outside roots
	}
	for _, tt := range tests {
		got := chainPaths(tree.Chain(tt.file))
		if got != tt.want {
			t.Errorf("Chain(%q) = %q, want %q", tt.file, got, tt.want)
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
