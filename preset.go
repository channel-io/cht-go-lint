package lint

import "sync"

// Preset represents a named collection of rule configurations.
type Preset struct {
	Name   string
	Layers []LayerConfig
	Rules  map[string]RuleConfig
	Location *LocationConfig
}

var (
	presetMu sync.RWMutex
	presets  = make(map[string]*Preset)
)

// RegisterPreset adds a preset to the global registry.
func RegisterPreset(p *Preset) {
	presetMu.Lock()
	defer presetMu.Unlock()
	presets[p.Name] = p
}

// GetPreset returns a preset by name.
func GetPreset(name string) *Preset {
	presetMu.RLock()
	defer presetMu.RUnlock()
	return presets[name]
}

// resolvePresets merges extended presets into the config.
// Later values (user config) override earlier values (presets).
func resolvePresets(cfg *Config) {
	if len(cfg.Extends) == 0 {
		return
	}

	for _, name := range cfg.Extends {
		p := GetPreset(name)
		if p == nil {
			continue
		}

		// Merge layers (preset provides defaults, user overrides)
		if len(cfg.Layers) == 0 && len(p.Layers) > 0 {
			cfg.Layers = p.Layers
		}

		// Merge location (preset provides default, user overrides)
		if cfg.Location == nil && p.Location != nil {
			cfg.Location = p.Location
		}

		// Merge rules (preset provides defaults, user overrides)
		if p.Rules != nil {
			if cfg.Rules == nil {
				cfg.Rules = make(map[string]RuleConfig)
			}
			for name, rc := range p.Rules {
				if _, exists := cfg.Rules[name]; !exists {
					cfg.Rules[name] = rc
				}
			}
		}
	}
}
