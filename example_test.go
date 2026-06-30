package lint_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lint "github.com/channel-io/cht-go-lint"
)

// TestExampleProjects loads the bundled clean testdata projects from their real
// .cht-go-lint.yaml files (root + co-located) and checks each. Both are clean
// node-tree repos, so neither may report a violation — exercising real YAML
// parsing, co-located discovery, tree assembly, and the import rule on realistic
// layouts:
//
//   - library: a heterogeneous library (pkg/ features, per-feature internals)
//   - msa:     a service with the channeltalk/msa-v2 layers and directions
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

// TestViolationsFixture loads testdata/violations — a deliberately-broken project
// — and asserts the import rule reports a violation at exactly the lines marked
// with a `// WANT-VIOLATION` comment, and nowhere else. This guards against a
// "false clean" rule: a no-op rule would pass the clean projects but fail here.
func TestViolationsFixture(t *testing.T) {
	const root = "testdata/violations"

	cfg, err := lint.LoadConfig(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	report := lint.Check(cfg)

	got := map[string]bool{}
	for _, v := range report.Violations() {
		if v.Rule == "dependency/import" {
			got[fmt.Sprintf("%s:%d", v.File, v.Line)] = true
		}
	}

	want := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, _ := os.ReadFile(path)
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "WANT-VIOLATION") {
				want[fmt.Sprintf("%s:%d", rel, i+1)] = true
			}
		}
		return nil
	})

	if len(want) == 0 {
		t.Fatal("no WANT-VIOLATION markers found in fixture")
	}
	for k := range want {
		if !got[k] {
			t.Errorf("expected a violation at %s but none was reported", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("unexpected violation at %s", k)
		}
	}
}
