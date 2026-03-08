package lint

import (
	"fmt"
	"sort"
	"sync"
)

// Tier represents the level of configuration required for a rule to operate.
type Tier int

const (
	TierUniversal      Tier = iota // works without any config (naming, forbidden patterns)
	TierLayerAware                 // requires layers to be defined
	TierComponentAware             // requires components to be defined
	TierDomainSpecific             // requires domain-specific options (DDD markers, DI framework, etc.)
)

// Meta describes a rule's identity and characteristics.
type Meta struct {
	Name        string // fully qualified name, e.g., "dependency/layer-direction"
	Description string
	Category    string // e.g., "dependency", "naming", "ddd"
	Tier        Tier
	URL         string // documentation URL
}

// Rule is the interface all architecture lint rules must implement.
type Rule interface {
	Meta() Meta
	Check(ctx *Context) error
}

// Context provides everything a rule needs to perform its check.
type Context struct {
	Config   *Config
	Analyzer *CodebaseAnalyzer
	Report   *Report
	Severity Severity
	Options  Options
}

// AddViolation is a convenience method to add a violation with the current rule severity.
func (c *Context) AddViolation(file string, line int, message string) {
	c.Report.Add(Violation{
		Severity: c.Severity,
		File:     file,
		Line:     line,
		Message:  message,
	})
}

// AddViolationWithDetails adds a violation with found/expected context.
func (c *Context) AddViolationWithDetails(file string, line int, message, found, expected string) {
	c.Report.Add(Violation{
		Severity: c.Severity,
		File:     file,
		Line:     line,
		Message:  message,
		Found:    found,
		Expected: expected,
	})
}

// Options provides typed access to rule configuration options.
type Options struct {
	raw map[string]any
}

// NewOptions creates an Options from a raw map.
func NewOptions(m map[string]any) Options {
	if m == nil {
		m = make(map[string]any)
	}
	return Options{raw: m}
}

// Has returns true if the key exists.
func (o Options) Has(key string) bool {
	_, ok := o.raw[key]
	return ok
}

// String returns a string option with a default value.
func (o Options) String(key, defaultVal string) string {
	v, ok := o.raw[key]
	if !ok {
		return defaultVal
	}
	if s, ok := v.(string); ok {
		return s
	}
	return defaultVal
}

// Bool returns a boolean option with a default value.
func (o Options) Bool(key string, defaultVal bool) bool {
	v, ok := o.raw[key]
	if !ok {
		return defaultVal
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return defaultVal
}

// Int returns an integer option with a default value.
func (o Options) Int(key string, defaultVal int) int {
	v, ok := o.raw[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case int64:
		return int(n)
	}
	return defaultVal
}

// StringSlice returns a string slice option.
func (o Options) StringSlice(key string) []string {
	v, ok := o.raw[key]
	if !ok {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		result := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

// Map returns a nested map option.
func (o Options) Map(key string) map[string]any {
	v, ok := o.raw[key]
	if !ok {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// MapSlice returns a slice of maps option (for structured lists like DDD contexts).
func (o Options) MapSlice(key string) []map[string]any {
	v, ok := o.raw[key]
	if !ok {
		return nil
	}
	switch s := v.(type) {
	case []map[string]any:
		return s
	case []any:
		result := make([]map[string]any, 0, len(s))
		for _, item := range s {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
		return result
	}
	return nil
}

// --- Rule Registry ---

var (
	registryMu sync.RWMutex
	registry   = make(map[string]Rule)
)

// Register adds a rule to the global registry. Typically called in init().
func Register(r Rule) {
	registryMu.Lock()
	defer registryMu.Unlock()
	name := r.Meta().Name
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("cht-go-lint: rule %q already registered", name))
	}
	registry[name] = r
}

// Get returns a rule by name, or nil if not found.
func Get(name string) Rule {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[name]
}

// All returns all registered rules sorted by name.
func All() []Rule {
	registryMu.RLock()
	defer registryMu.RUnlock()
	rules := make([]Rule, 0, len(registry))
	for _, r := range registry {
		rules = append(rules, r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Meta().Name < rules[j].Meta().Name
	})
	return rules
}

// AllNames returns all registered rule names sorted.
func AllNames() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
