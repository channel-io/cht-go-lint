package lint_test

import (
	"testing"

	lint "github.com/channel-io/cht-go-lint"
)

// TestExampleProject loads the bundled testdata/example project from its real
// .cht-go-lint.yaml files (root + co-located) and checks it. The example is a
// clean node-tree repo, so it must report no violations — this exercises real
// YAML parsing, co-located discovery, tree assembly, and the import rule on a
// realistic directory layout.
func TestExampleProject(t *testing.T) {
	cfg, err := lint.LoadConfig("testdata/example")
	if err != nil {
		t.Fatalf("load example config: %v", err)
	}

	report := lint.Check(cfg)
	if report.ErrorCount() != 0 || report.WarningCount() != 0 {
		t.Errorf("example project should be clean, got %d errors / %d warnings:\n%s",
			report.ErrorCount(), report.WarningCount(), report.String())
	}
}
