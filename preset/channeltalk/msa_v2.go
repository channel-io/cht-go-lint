package channeltalk

import lint "github.com/channel-io/cht-go-lint"

func init() {
	lint.RegisterPreset(&lint.Preset{
		Name: "channeltalk/msa-v2",
		Layers: []lint.LayerConfig{
			{Name: "model", MayImport: []string{}},
			{Name: "repo", MayImport: []string{"model"}},
			{Name: "service", Aliases: []string{"svc"}, MayImport: []string{"model", "repo", "service"}},
			{Name: "appsvc", Aliases: []string{"app_svc"}, MayImport: []string{"model", "repo", "service"}},
			{Name: "publicsvc", Aliases: []string{"public_svc"}, MayImport: []string{"model", "appsvc"}},
			{Name: "handler", MayImport: []string{"model", "service", "appsvc", "publicsvc"}},
			{Name: "client", MayImport: []string{"model"}},
			{Name: "event", MayImport: []string{"model"}},
			{Name: "infra", MayImport: []string{"model", "repo"}},
			{Name: "saga", MayImport: []string{"model", "publicsvc"}},
		},
		Location: &lint.LocationConfig{
			Strategy: "nested-domain",
			Options: map[string]any{
				"domain_root":   "internal/domain",
				"subdomain_dir": "subdomain",
				"saga_root":     "internal/saga",
			},
		},
		Rules: map[string]lint.RuleConfig{
			// Dependency rules
			"dependency/layer-direction":  {Severity: lint.Error},
			"dependency/module-isolation":  {Severity: lint.Error},
			"dependency/cross-boundary":    {Severity: lint.Error},
			"dependency/forbidden-imports": {Severity: lint.Error},
			"dependency/di-isolation": {
				Severity: lint.Error,
				Options:  map[string]any{"framework": "fx", "companion_suffix": "fx"},
			},
			"dependency/infra-in-core": {Severity: lint.Error},
			"dependency/handler-placement": {Severity: lint.Error},

			// Naming rules
			"naming/no-stutter":            {Severity: lint.Error},
			"naming/impl-naming":           {Severity: lint.Error},
			"naming/constructor-naming":    {Severity: lint.Error},
			"naming/file-naming":           {Severity: lint.Error},
			"naming/forbidden-names":       {Severity: lint.Warn},
			"naming/filename-matches-type": {Severity: lint.Warn},

			// Interface rules
			"interface/impl-pattern":        {Severity: lint.Error},
			"interface/constructor-return":   {Severity: lint.Error},
			"interface/colocation":           {Severity: lint.Error},
			"interface/one-per-file":         {Severity: lint.Warn},
			"interface/required-embedding":   {Severity: lint.Off},

			// Structure rules
			"structure/required-dirs":      {Severity: lint.Warn},
			"structure/forbidden-dirs":     {Severity: lint.Error},
			"structure/file-content":       {Severity: lint.Error},
			"structure/declaration-order":  {Severity: lint.Warn},
			"structure/import-alias":       {Severity: lint.Warn},
			"structure/delegation-only":    {Severity: lint.Warn},
		},
	})
}
