package lint_test

import (
	"testing"

	lint "github.com/channel-io/cht-go-lint"
)

// TestExampleProjects loads the bundled testdata projects from their real
// .cht-go-lint.yaml files (root + co-located) and checks each. Both are clean
// node-tree repos, so neither may report a violation — exercising real YAML
// parsing, co-located discovery, tree assembly, and the import rule on realistic
// layouts:
//
//   - library: a heterogeneous library (pkg/ features, per-feature internals)
//   - msa:     a service with uniform directional layers per domain
func TestExampleProjects(t *testing.T) {
	for _, dir := range []string{"testdata/library", "testdata/msa"} {
		t.Run(dir, func(t *testing.T) {
			cfg, err := lint.LoadConfig(dir)
			if err != nil {
				t.Fatalf("load %s config: %v", dir, err)
			}
			report := lint.Check(cfg)
			if report.ErrorCount() != 0 || report.WarningCount() != 0 {
				t.Errorf("%s should be clean, got %d errors / %d warnings:\n%s",
					dir, report.ErrorCount(), report.WarningCount(), report.String())
			}
		})
	}
}
