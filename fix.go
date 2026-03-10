package lint

import (
	"fmt"
	"sort"
	"sync"
)

// FixableRule is implemented by rules that support auto-fix.
type FixableRule interface {
	FixMeta() FixMeta
	Fix(ctx *FixContext) error
}

// FixMeta describes a fixer's identity.
type FixMeta struct {
	RuleName    string
	Description string
}

// FixContext provides everything a fixer needs.
type FixContext struct {
	Config   *Config
	Analyzer *CodebaseAnalyzer
	Options  Options
	DryRun   bool

	mu      sync.Mutex
	results []FixResult
}

// FixResult records a single file fix.
type FixResult struct {
	File     string
	RuleName string
}

// RecordFix records that a file was fixed by a rule.
func (c *FixContext) RecordFix(file, rule string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, FixResult{File: file, RuleName: rule})
}

// Results returns all recorded fix results.
func (c *FixContext) Results() []FixResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]FixResult, len(c.results))
	copy(out, c.results)
	return out
}

// --- Fixer Registry ---

var (
	fixersMu sync.RWMutex
	fixers   = make(map[string]FixableRule)
)

// RegisterFixer adds a fixer to the global registry. Typically called in init().
func RegisterFixer(f FixableRule) {
	fixersMu.Lock()
	defer fixersMu.Unlock()
	name := f.FixMeta().RuleName
	if _, exists := fixers[name]; exists {
		panic(fmt.Sprintf("cht-go-lint: fixer %q already registered", name))
	}
	fixers[name] = f
}

// GetFixer returns a fixer by rule name, or nil if not found.
func GetFixer(name string) FixableRule {
	fixersMu.RLock()
	defer fixersMu.RUnlock()
	return fixers[name]
}

// AllFixers returns all registered fixers sorted by rule name.
func AllFixers() []FixableRule {
	fixersMu.RLock()
	defer fixersMu.RUnlock()
	result := make([]FixableRule, 0, len(fixers))
	for _, f := range fixers {
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].FixMeta().RuleName < result[j].FixMeta().RuleName
	})
	return result
}

// RunFixers executes all enabled fixers and returns fix results.
func RunFixers(cfg *Config, analyzer *CodebaseAnalyzer, dryRun bool) []FixResult {
	ctx := &FixContext{
		Config:   cfg,
		Analyzer: analyzer,
		DryRun:   dryRun,
	}
	for _, fixer := range AllFixers() {
		name := fixer.FixMeta().RuleName
		if cfg.EffectiveSeverity(name, "") == Off {
			continue
		}
		if rule := Get(name); rule != nil && !tierSatisfied(rule.Meta().Tier, cfg) {
			continue
		}
		ctx.Options = NewOptions(cfg.RuleOptions(name))
		_ = fixer.Fix(ctx)
	}
	return ctx.Results()
}
