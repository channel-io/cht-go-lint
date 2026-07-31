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

// importFixtures are self-contained repos, one per dependency/import behavior. Each mixes
// allowed imports (no marker — must stay clean) with blocked imports (a `// WANT-VIOLATION`
// comment). The rule runs once per fixture and must report a violation at exactly the marked
// lines, so a single fixture verifies both the allowed and the blocked side of its behavior.
var importFixtures = []string{
	"sibling-isolation",    // deny-default between features + undeclared siblings
	"layer-direction",      // template layers: down allowed, up blocked
	"shared-scope",         // shared by position: root-global vs feature-local
	"internal-segment",     // internal/ names a node, granted per sibling
	"template-combination", // one template = layer direction AND domain isolation
	"cross-domain-grant",   // may_import opens one domain; reverse stays blocked
}

func TestImportFixtures(t *testing.T) {
	base := "testdata/rules/dependency/import"
	for _, name := range importFixtures {
		t.Run(name, func(t *testing.T) { assertExactViolations(t, filepath.Join(base, name)) })
	}
}

// assertExactViolations checks that dependency/import reports a violation at exactly the
// `// WANT-VIOLATION` lines in root, and nowhere else (allowed = not reported).
func assertExactViolations(t *testing.T, root string) {
	t.Helper()
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
		t.Fatalf("%s: no WANT-VIOLATION markers found in fixture", root)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("%s: expected a violation at %s but none reported", root, k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("%s: unexpected violation at %s (allowed import wrongly flagged)", root, k)
		}
	}
}
