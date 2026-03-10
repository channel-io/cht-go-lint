package channeltalk

import lint "github.com/channel-io/cht-go-lint"

func init() {
	lint.RegisterPreset(&lint.Preset{
		Name: "channeltalk/msa-v2",
		Layers: []lint.LayerConfig{
			{Name: "model", MayImport: []string{}},
			{Name: "repo", MayImport: []string{"model"}},
			{Name: "service", Aliases: []string{"svc"}, MayImport: []string{"model", "repo", "service"}},
			{Name: "appsvc", Aliases: []string{"app_svc"}, MayImport: []string{"model", "repo", "service", "infra"}},
			{Name: "publicsvc", Aliases: []string{"public_svc"}, MayImport: []string{"model", "appsvc"}},
			{Name: "handler", MayImport: []string{"model", "service", "appsvc", "publicsvc", "saga"}},
			{Name: "client", MayImport: []string{"model"}},
			{Name: "event", MayImport: []string{"model"}},
			{Name: "infra", MayImport: []string{"model"}},
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
			"dependency/module-isolation": {Severity: lint.Error},
			"dependency/cross-boundary":   {Severity: lint.Error},
			"dependency/forbidden-imports": {
				Severity: lint.Error,
				Options:  map[string]any{"patterns": []any{"*/internal/shared/*"}},
			},
			"dependency/di-isolation": {
				Severity: lint.Error,
				Options:  map[string]any{"framework": "fx", "companion_suffix": "fx"},
			},
			"dependency/infra-in-core":           {Severity: lint.Error},
			"dependency/handler-placement": {
				Severity: lint.Error,
				Options:  map[string]any{"allowed_imports": []any{"model", "service", "appsvc", "publicsvc", "saga"}},
			},
			"dependency/public-service-isolation": {Severity: lint.Error},
			"dependency/app-service-mixing":       {Severity: lint.Error},
			"dependency/subdomain-isolation": {
				Severity: lint.Error,
				Options:  map[string]any{"allow_model_import": true},
			},
			"dependency/handler-infra-isolation": {Severity: lint.Error},

			// Naming rules
			"naming/no-stutter": {
				Severity: lint.Error,
				Options:  map[string]any{"check_component_name": true, "skip_files": []any{"alias.go"}},
			},
			"naming/impl-naming":       {Severity: lint.Error},
			"naming/constructor-naming": {
				Severity: lint.Error,
				Options:  map[string]any{"skip_files": []any{"fx.go"}},
			},
			"naming/file-naming": {
				Severity: lint.Error,
				Options:  map[string]any{"no_package_stutter": true},
			},
			"naming/forbidden-names": {
				Severity: lint.Error,
				Options:  map[string]any{"forbidden_suffixes": []any{"Helper"}},
			},
			"naming/filename-matches-type": {
				Severity: lint.Error,
				Options:  map[string]any{"skip_files": []any{"alias.go", "dto.go", "types.go", "fx.go"}},
			},
			"naming/no-domain-prefix": {
				Severity: lint.Error,
				Options:  map[string]any{"skip_layers": []any{"model", "repo"}},
			},
			"naming/layer-type-pattern": {
				Severity: lint.Error,
				Options: map[string]any{
					"patterns": []any{
						map[string]any{
							"tag":                          "isPublicSvc",
							"required_interface":           "Public",
							"required_struct":              "public",
							"no_impl_suffix":               true,
							"constructor_returns_interface": true,
						},
						map[string]any{
							"tag":                          "isSaga",
							"filename_contains":            "saga",
							"skip_tags":                    map[string]any{"isFxCompanion": "true"},
							"required_interface_suffix":    "Saga",
							"required_struct_match":        "case_insensitive",
							"constructor_returns_interface": true,
						},
					},
				},
			},

			// Interface rules
			"interface/impl-pattern": {
				Severity: lint.Error,
				Options:  map[string]any{"skip_layers": []any{"repo", "service", "saga"}},
			},
			"interface/constructor-return": {Severity: lint.Error},
			"interface/colocation":         {Severity: lint.Error},
			"interface/one-per-file":       {Severity: lint.Error},
			"interface/required-embedding": {
				Severity: lint.Error,
				Options: map[string]any{
					"patterns": []any{
						map[string]any{"tag": "handler_type", "tag_value": "api/http", "layer": "handler", "base_interface": "RouteRegistrant"},
						map[string]any{"tag": "handler_type", "tag_value": "api/jsonrpc", "layer": "handler", "base_interface": "Registrant"},
					},
				},
			},

			// Structure rules
			"structure/required-dirs": {
				Severity: lint.Error,
				Options:  map[string]any{"dirs": []any{"model", "svc"}},
			},
			"structure/forbidden-dirs": {
				Severity: lint.Error,
				Options: map[string]any{
					"scoped": []any{
						map[string]any{
							"scope_paths": []any{"internal/domain/*/subdomain/*"},
							"names":       []any{"handler", "consumer", "infra", "client"},
						},
						map[string]any{
							"scope_paths": []any{"internal/domain/*"},
							"names":       []any{"domain"},
						},
					},
				},
			},
			"structure/file-content": {
				Severity: lint.Error,
				Options: map[string]any{
					"files": map[string]any{
						"alias.go": map[string]any{
							"allow": []any{"type_alias", "const", "var"},
						},
					},
				},
			},
			"structure/declaration-order": {
				Severity: lint.Error,
				Options: map[string]any{
					"layer_overrides": map[string]any{
						"saga": []any{"const", "var", "interface", "func", "struct"},
					},
				},
			},
			"structure/import-alias": {
				Severity: lint.Error,
				Options:  map[string]any{"no_same_component_alias": true},
			},
			"structure/delegation-only": {
				Severity: lint.Error,
				Options:  map[string]any{"target_layers": []any{"publicsvc"}},
			},
			"structure/required-declarations": {
				Severity: lint.Error,
				Options: map[string]any{
					"files": map[string]any{
						"alias.go": map[string]any{
							"tag":              "isAlias",
							"required_aliases": []any{"Svc", "Public"},
						},
					},
				},
			},
			"structure/required-files": {
				Severity: lint.Error,
				Options: map[string]any{
					"rules": []any{
						map[string]any{
							"scope":            "internal/domain/*",
							"skip_suffix":      "fx",
							"when_has_subdirs": true,
							"layer_dirs":       []any{"model", "repo", "svc", "service", "infra", "client", "event", "handler", "consumer"},
							"required":         []any{"alias.go"},
						},
					},
				},
			},
			"structure/file-placement": {
				Severity: lint.Error,
				Options: map[string]any{
					"rules": []any{
						map[string]any{
							"filename":   "fx.go",
							"dir_suffix": "fx",
							"skip_dirs":  []any{"test", "vendor"},
						},
					},
				},
			},
		},
	})
}
