# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `dependency/subdomain-isolation` rule: sub-components within a component must not import each other, with `allow_model_import` option to downgrade model imports to warnings
- `dependency/handler-infra-isolation` rule: handler layer must not import infrastructure layers (client, infra, event) directly
- `naming/no-domain-prefix` rule: exported types should not be prefixed with the component name
- `interface/required-embedding` enhanced with `patterns` option for conditional embedding checks based on tag/layer filters

## [0.1.0] - 2026-03-09

### Added

- 38 built-in architecture lint rules across 5 categories:
  - **dependency** (9 rules): layer direction, module isolation, forbidden imports, DI isolation, infra-in-core, handler placement, public service isolation, app service mixing, cross-boundary
  - **naming** (7 rules): file naming, no stutter, constructor naming, impl naming, forbidden names, filename matches type, layer type pattern
  - **interface** (5 rules): constructor return, impl pattern, colocation, one per file, required embedding
  - **structure** (9 rules): forbidden dirs, required dirs, required files, required declarations, file content, file placement, declaration order, import alias, delegation only
  - **ddd** (8 rules): aggregate boundary, repository per aggregate, entity identity, value object immutable, domain event naming, bounded context isolation, no domain to infra, service layer
- 4-tier rule system (universal, layer-aware, component-aware, domain-specific)
- YAML-based configuration with `extends` for preset inheritance
- Built-in `clean-arch` preset
- Location strategies: `nested-domain` and `flat-pkg`
- CLI with `check`, `list-rules`, and `init` commands
- Output formats: text, JSON, GitHub Actions annotations
- Test integration via `Run()`, `Check()`, `QuickCheck()` APIs
- Test utilities (`testutil` package) for rule testing

[0.1.0]: https://github.com/channel-io/cht-go-lint/releases/tag/v0.1.0
