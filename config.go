package lint

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the complete linter configuration.
type Config struct {
	Root       string                `yaml:"-"`
	ModulePath string                `yaml:"module"`
	Extends    []string              `yaml:"extends,omitempty"`
	Layers     []LayerConfig         `yaml:"layers,omitempty"`
	Components []ComponentConfig     `yaml:"components,omitempty"`
	Rules      map[string]RuleConfig `yaml:"rules,omitempty"`
	Location   *LocationConfig       `yaml:"location,omitempty"`
}

// LayerConfig defines a layer and its allowed imports.
type LayerConfig struct {
	Name      string   `yaml:"name"`
	Aliases   []string `yaml:"aliases,omitempty"`
	MayImport []string `yaml:"may_import,omitempty"`
}

// ComponentConfig defines a component with optional per-component rule overrides.
type ComponentConfig struct {
	Name  string                `yaml:"name"`
	Path  string                `yaml:"path"`
	Rules map[string]RuleConfig `yaml:"rules,omitempty"`
}

// LocationConfig configures the location strategy.
type LocationConfig struct {
	Strategy string         `yaml:"strategy"`
	Options  map[string]any `yaml:"options,omitempty"`
}

// RuleConfig holds the severity and options for a single rule.
// In YAML, it can be either a string ("error") or an object ({severity: error, options: {...}}).
type RuleConfig struct {
	Severity Severity       `yaml:"severity"`
	Options  map[string]any `yaml:"options,omitempty"`
}

// UnmarshalYAML handles both string ("error") and object forms.
func (rc *RuleConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		rc.Severity = ParseSeverity(value.Value)
		return nil
	}
	// For mapping nodes, decode severity from string
	type rawConfig struct {
		Severity string         `yaml:"severity"`
		Options  map[string]any `yaml:"options,omitempty"`
	}
	var raw rawConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}
	rc.Severity = ParseSeverity(raw.Severity)
	rc.Options = raw.Options
	return nil
}

var defaultConfigFiles = []string{".cht-go-lint.yaml", ".cht-go-lint.yml"}

// LoadConfig loads configuration from the default config file in root directory.
func LoadConfig(root string) (*Config, error) {
	for _, name := range defaultConfigFiles {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		cfg := &Config{Root: root}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
		if cfg.Rules == nil {
			cfg.Rules = make(map[string]RuleConfig)
		}
		return cfg, nil
	}
	return &Config{Root: root, Rules: make(map[string]RuleConfig)}, nil
}

// LoadConfigFrom loads configuration from a specific file path.
func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.Rules == nil {
		cfg.Rules = make(map[string]RuleConfig)
	}
	return cfg, nil
}

// EffectiveSeverity returns the severity for a rule, checking component overrides first.
func (c *Config) EffectiveSeverity(ruleName, component string) Severity {
	if component != "" {
		for _, comp := range c.Components {
			if comp.Name == component {
				if rc, ok := comp.Rules[ruleName]; ok {
					return rc.Severity
				}
			}
		}
	}
	if rc, ok := c.Rules[ruleName]; ok {
		return rc.Severity
	}
	return Off
}

// RuleOptions returns the options for a specific rule.
func (c *Config) RuleOptions(ruleName string) map[string]any {
	if rc, ok := c.Rules[ruleName]; ok {
		return rc.Options
	}
	return nil
}

// LayerMayImport returns the allowed import targets for a layer.
func (c *Config) LayerMayImport(layerName string) ([]string, bool) {
	for _, l := range c.Layers {
		if l.Name == layerName {
			return l.MayImport, true
		}
		for _, a := range l.Aliases {
			if a == layerName {
				return l.MayImport, true
			}
		}
	}
	return nil, false
}

// HasLayers returns true if layers are defined in the config.
func (c *Config) HasLayers() bool {
	return len(c.Layers) > 0
}

// HasComponents returns true if components are defined in the config.
func (c *Config) HasComponents() bool {
	return len(c.Components) > 0
}

// ResolveLayerName normalizes a layer name, resolving aliases to canonical names.
func (c *Config) ResolveLayerName(name string) string {
	for _, l := range c.Layers {
		if l.Name == name {
			return l.Name
		}
		for _, a := range l.Aliases {
			if a == name {
				return l.Name
			}
		}
	}
	return name
}
