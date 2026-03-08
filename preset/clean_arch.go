package preset

import lint "github.com/channel-io/cht-go-lint"

func init() {
	lint.RegisterPreset(&lint.Preset{
		Name: "clean-arch",
		Layers: []lint.LayerConfig{
			{Name: "model", MayImport: []string{}},
			{Name: "repo", MayImport: []string{"model"}},
			{Name: "service", Aliases: []string{"svc"}, MayImport: []string{"model", "repo"}},
			{Name: "handler", MayImport: []string{"model", "service"}},
		},
		Location: &lint.LocationConfig{
			Strategy: "flat-pkg",
		},
		Rules: map[string]lint.RuleConfig{
			"dependency/layer-direction": {Severity: lint.Error},
			"dependency/module-isolation": {Severity: lint.Warn},
			"dependency/forbidden-imports": {Severity: lint.Warn},
			"naming/file-naming":           {Severity: lint.Warn},
			"naming/no-stutter":            {Severity: lint.Warn},
			"naming/constructor-naming":    {Severity: lint.Warn},
			"interface/constructor-return":  {Severity: lint.Warn},
			"structure/forbidden-dirs":      {Severity: lint.Warn},
		},
	})
}
