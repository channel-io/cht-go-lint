package lint

import (
	"path/filepath"
	"testing"
)

// Run executes architecture lint as a test, failing on errors and logging warnings.
func Run(t *testing.T, cfg *Config) {
	t.Helper()

	r := Check(cfg)

	errors := r.Errors()
	warnings := r.Warnings()

	if len(warnings) > 0 {
		for _, w := range warnings {
			t.Logf("WARN: %s", w)
		}
	}

	if len(errors) > 0 {
		for _, e := range errors {
			t.Errorf("%s", e)
		}
	}
}

// Check runs all enabled rules against the codebase and returns a report.
func Check(cfg *Config) *Report {
	// Resolve preset configurations
	resolvePresets(cfg)

	// Create location strategy
	strategy := resolveStrategy(cfg)

	// Create analyzer
	a := NewAnalyzer(cfg.Root, cfg.ModulePath, strategy)

	// Create report
	rpt := NewReport()

	// Run each enabled rule
	for _, rule := range All() {
		name := rule.Meta().Name
		sev := cfg.EffectiveSeverity(name, "")
		if sev == Off {
			continue
		}

		// Check tier requirements
		if !tierSatisfied(rule.Meta().Tier, cfg) {
			continue
		}

		opts := NewOptions(cfg.RuleOptions(name))
		ctx := &Context{
			Config:   cfg,
			Analyzer: a,
			Report:   rpt,
			Severity: sev,
			Options:  opts,
		}

		if err := rule.Check(ctx); err != nil {
			rpt.Add(Violation{
				Rule:     name,
				Severity: Error,
				Message:  "rule execution failed: " + err.Error(),
			})
		}
	}

	return rpt
}

// QuickCheck loads config from root directory and runs as a test.
func QuickCheck(t *testing.T, root string) {
	t.Helper()
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("cht-go-lint: failed to load config: %v", err)
	}
	Run(t, cfg)
}

// RunWithConfigFile loads config from a specific file and runs as a test.
func RunWithConfigFile(t *testing.T, root, configFile string) {
	t.Helper()
	cfg, err := LoadConfigFrom(filepath.Join(root, configFile))
	if err != nil {
		t.Fatalf("cht-go-lint: failed to load config: %v", err)
	}
	cfg.Root = root
	Run(t, cfg)
}

func resolveStrategy(cfg *Config) LocationStrategy {
	if cfg.Location == nil {
		return nil
	}
	switch cfg.Location.Strategy {
	case "nested-domain":
		return NewNestedDomainStrategy(cfg)
	case "flat-pkg":
		return NewFlatPkgStrategy(cfg)
	default:
		return nil
	}
}

func tierSatisfied(tier Tier, cfg *Config) bool {
	switch tier {
	case TierUniversal:
		return true
	case TierLayerAware:
		return cfg.HasLayers()
	case TierComponentAware:
		return cfg.HasComponents()
	case TierDomainSpecific:
		return true // always run if enabled; rule checks its own options
	default:
		return true
	}
}
