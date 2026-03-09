// Package testutil provides helpers for testing custom lint rules.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	lint "github.com/channel-io/cht-go-lint"
)

// RuleTest defines a test case for a rule.
type RuleTest struct {
	Name       string
	Rule       lint.Rule
	Config     *lint.Config
	WantErrors int
	WantWarns  int
}

// RunRuleTests runs a set of rule test cases.
func RunRuleTests(t *testing.T, tests []RuleTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Helper()
			report := RunRule(t, tt.Rule, tt.Config)

			if got := report.ErrorCount(); got != tt.WantErrors {
				t.Errorf("errors: got %d, want %d\n%s", got, tt.WantErrors, report.String())
			}
			if got := report.WarningCount(); got != tt.WantWarns {
				t.Errorf("warnings: got %d, want %d\n%s", got, tt.WantWarns, report.String())
			}
		})
	}
}

// RunRule runs a single rule against a config and returns the report.
func RunRule(t *testing.T, rule lint.Rule, cfg *lint.Config) *lint.Report {
	t.Helper()

	strategy := resolveTestStrategy(cfg)
	a := lint.NewAnalyzer(cfg.Root, cfg.ModulePath, strategy, cfg.ExcludePaths)
	rpt := lint.NewReport()

	sev := lint.Error
	if rc, ok := cfg.Rules[rule.Meta().Name]; ok {
		sev = rc.Severity
	}

	opts := lint.NewOptions(cfg.RuleOptions(rule.Meta().Name))
	ctx := &lint.Context{
		Config:   cfg,
		Analyzer: a,
		Report:   rpt,
		Severity: sev,
		Options:  opts,
	}

	if err := rule.Check(ctx); err != nil {
		t.Fatalf("rule %s failed: %v", rule.Meta().Name, err)
	}
	return rpt
}

func resolveTestStrategy(cfg *lint.Config) lint.LocationStrategy {
	if cfg.Location == nil {
		return nil
	}
	switch cfg.Location.Strategy {
	case "nested-domain":
		return lint.NewNestedDomainStrategy(cfg)
	case "flat-pkg":
		return lint.NewFlatPkgStrategy(cfg)
	default:
		return nil
	}
}

// CreateFixture creates a temporary directory with Go source files for testing.
// files is a map of relative path to file content.
func CreateFixture(t *testing.T, modulePath string, files map[string]string) *lint.Config {
	t.Helper()
	dir := t.TempDir()

	// Write go.mod
	goMod := "module " + modulePath + "\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Write source files
	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return &lint.Config{
		Root:       dir,
		ModulePath: modulePath,
		Rules:      make(map[string]lint.RuleConfig),
	}
}
