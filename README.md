# cht-go-lint

[![CI](https://github.com/channel-io/cht-go-lint/actions/workflows/ci.yml/badge.svg)](https://github.com/channel-io/cht-go-lint/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/channel-io/cht-go-lint.svg)](https://pkg.go.dev/github.com/channel-io/cht-go-lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/channel-io/cht-go-lint)](https://goreportcard.com/report/github.com/channel-io/cht-go-lint)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)

Architecture linter for Go projects. Enforce layer dependencies, naming conventions, DDD patterns, and structural rules through static analysis.

## Features

- **41 built-in rules** across 5 categories (dependency, naming, interface, structure, DDD)
- **Tier system** — rules declare their config requirements; only applicable rules run
- **Presets** — start with `clean-arch` or build your own configuration
- **Location strategies** — map your project structure (`nested-domain`, `flat-pkg`) to architectural layers
- **Test integration** — run as `go test` for CI-friendly checks
- **Multiple output formats** — text, JSON, GitHub Actions annotations

## Installation

### Go install

```bash
go install github.com/channel-io/cht-go-lint/cmd/cht-go-lint@latest
```

### From releases

Download binaries from [GitHub Releases](https://github.com/channel-io/cht-go-lint/releases).

## Quick Start

```bash
# Create a default config file
cht-go-lint init

# Edit .cht-go-lint.yaml for your project, then:
cht-go-lint check

# Or use a preset
```

Minimal `.cht-go-lint.yaml`:

```yaml
module: github.com/your-org/your-project

extends:
  - clean-arch

# Override or add rules
rules:
  naming/file-naming: error
```

## Rules

### Dependency (11 rules)

| Rule | Tier | Description |
|------|------|-------------|
| `dependency/layer-direction` | layer-aware | Enforce allowed import direction between layers |
| `dependency/module-isolation` | component-aware | Enforce that components don't import each other's internals |
| `dependency/forbidden-imports` | universal | Ban specific import path patterns |
| `dependency/di-isolation` | domain-specific | DI framework code should be isolated in companion files |
| `dependency/infra-in-core` | layer-aware | Core layers must not import infrastructure packages |
| `dependency/handler-placement` | layer-aware | Handler layer files should only import allowed layers |
| `dependency/public-service-isolation` | layer-aware | Public Service files must not import repo/infra/client/event/handler layers |
| `dependency/app-service-mixing` | layer-aware | App Service files must not mix repo and infra/client/event imports |
| `dependency/cross-boundary` | component-aware | Cross-component imports must use public interfaces only |
| `dependency/subdomain-isolation` | component-aware | Sub-components within a component must not import each other |
| `dependency/handler-infra-isolation` | layer-aware | Handler layer must not import infrastructure layers directly |

### Naming (8 rules)

| Rule | Tier | Description |
|------|------|-------------|
| `naming/file-naming` | universal | Source file names must follow a naming convention |
| `naming/no-stutter` | universal | Type or function name should not repeat the package name |
| `naming/constructor-naming` | universal | Constructor functions (New*) should return the type they construct |
| `naming/impl-naming` | universal | Implementation structs should follow naming convention relative to their interface |
| `naming/forbidden-names` | universal | Types with certain prefixes or suffixes are forbidden |
| `naming/filename-matches-type` | universal | The primary type in a file should match the filename |
| `naming/layer-type-pattern` | layer-aware | Enforce type naming conventions per layer/tag |
| `naming/no-domain-prefix` | component-aware | Exported types should not be prefixed with the component name |

### Interface (5 rules)

| Rule | Tier | Description |
|------|------|-------------|
| `interface/constructor-return` | universal | Constructor functions should return an interface type, not a concrete struct |
| `interface/impl-pattern` | universal | Files with an exported interface should also have a private implementation struct |
| `interface/colocation` | universal | Interface and its implementation should be co-located |
| `interface/one-per-file` | universal | Each file should have at most one primary exported interface |
| `interface/required-embedding` | domain-specific | Certain interfaces must embed a base interface |

### Structure (9 rules)

| Rule | Tier | Description |
|------|------|-------------|
| `structure/forbidden-dirs` | universal | Forbid certain directory names anywhere in the project |
| `structure/required-dirs` | component-aware | Validate that required directories exist in each component |
| `structure/required-files` | domain-specific | Validate that required files exist in directories matching patterns |
| `structure/required-declarations` | domain-specific | Validate that specific files contain required declarations |
| `structure/file-content` | domain-specific | Restrict what declarations a specific file may contain |
| `structure/file-placement` | domain-specific | Enforce that certain files can only exist in specific directories |
| `structure/declaration-order` | universal | Enforce ordering of declarations within a file |
| `structure/import-alias` | universal | Import aliases should follow a naming convention |
| `structure/delegation-only` | layer-aware | Methods in target layers should only delegate to another type's method |

### DDD (8 rules)

| Rule | Tier | Description |
|------|------|-------------|
| `ddd/aggregate-boundary` | domain-specific | Aggregate roots must not directly reference other aggregates |
| `ddd/repository-per-aggregate` | domain-specific | Each aggregate root should have exactly one repository interface |
| `ddd/entity-identity` | domain-specific | Entity types should have an ID field |
| `ddd/value-object-immutable` | domain-specific | Value objects should not have setter methods |
| `ddd/domain-event-naming` | domain-specific | Domain events should follow naming conventions |
| `ddd/bounded-context-isolation` | domain-specific | Bounded contexts should not directly import each other |
| `ddd/no-domain-to-infra` | layer-aware | Domain layer must not import infrastructure packages |
| `ddd/service-layer` | layer-aware | Enforce separation between domain services and application services |

## Rule Tiers

Rules declare the minimum configuration they need:

| Tier | Requires | Examples |
|------|----------|---------|
| **universal** | Nothing | `naming/file-naming`, `structure/forbidden-dirs` |
| **layer-aware** | `layers` defined | `dependency/layer-direction` |
| **component-aware** | `components` defined | `dependency/module-isolation` |
| **domain-specific** | Rule-specific `options` | `ddd/aggregate-boundary` |

Rules are silently skipped if the config doesn't satisfy their tier.

## Configuration Reference

```yaml
# Go module path (required)
module: github.com/your-org/your-project

# Inherit from presets
extends:
  - clean-arch

# Location strategy maps file paths to architectural layers
location:
  strategy: flat-pkg  # or "nested-domain"
  options:
    # flat-pkg options:
    roots: ["internal", "pkg"]
    # nested-domain options:
    # domain_root: "internal/domain"
    # subdomain_dir: "subdomain"
    # saga_root: "internal/saga"

# Architectural layers and allowed imports
layers:
  - name: model
    may_import: []
  - name: repo
    may_import: [model]
  - name: service
    aliases: [svc]
    may_import: [model, repo]
  - name: handler
    may_import: [model, service]

# Component isolation
components:
  - name: user
    path: internal/domain/user
  - name: order
    path: internal/domain/order

# Rules: string shorthand or object form
rules:
  naming/file-naming: warn            # shorthand
  dependency/layer-direction: error
  ddd/aggregate-boundary:             # object form
    severity: error
    options:
      root_marker: "Aggregate"
```

## Presets

### `clean-arch`

A minimal Clean Architecture preset with 4 layers and 8 rules:

- Layers: `model` → `repo` → `service` (alias: `svc`) → `handler`
- Strategy: `flat-pkg`
- Rules: `dependency/layer-direction` (error), plus 7 naming/structure rules (warn)

```yaml
extends:
  - clean-arch
```

Presets can be extended and overridden — any rule or layer you define takes precedence.

## Location Strategies

Location strategies map file paths to architectural positions (component, layer, sub-component).

### `flat-pkg`

Simple structure where layers are directories under a root:

```
internal/
  user/
    model/
    repo/
    service/
    handler/
```

### `nested-domain`

For larger projects with subdomains:

```
internal/domain/
  user/
    subdomain/
      membership/
        model/
        repo/
        svc/
    model/
    repo/
    svc/
```

Options: `domain_root`, `subdomain_dir`, `saga_root`.

## Test Integration

Add architecture checks to your Go tests:

```go
package arch_test

import (
    "testing"

    lint "github.com/channel-io/cht-go-lint"
    _ "github.com/channel-io/cht-go-lint/preset"
    _ "github.com/channel-io/cht-go-lint/rules"
)

func TestArchitecture(t *testing.T) {
    // Quick: load .cht-go-lint.yaml from project root
    lint.QuickCheck(t, "../..")

    // Or with explicit config:
    // cfg, _ := lint.LoadConfig("../..")
    // lint.Run(t, cfg)
}
```

## CLI Usage

```
Usage: cht-go-lint <command> [options]

Commands:
  check       Run architecture lint checks
  list-rules  List all available rules
  init        Create a default configuration file

Options for 'check':
  --config <path>    Config file path (default: auto-detect .cht-go-lint.yaml)
  --format <fmt>     Output format: text, json, github (default: text)
  --rule <names>     Run specific rules (comma-separated)
```

### GitHub Actions

```yaml
- uses: actions/setup-go@v5
  with:
    go-version: '1.22'
- run: go install github.com/channel-io/cht-go-lint/cmd/cht-go-lint@v0.1.0
- run: cht-go-lint check --format github
```

The `github` format emits `::error` / `::warning` annotations that show inline on PRs.

## Writing Custom Rules

Implement the `Rule` interface and register in `init()`:

```go
package myrules

import lint "github.com/channel-io/cht-go-lint"

func init() {
    lint.Register(&MyRule{})
}

type MyRule struct{}

func (r *MyRule) Meta() lint.Meta {
    return lint.Meta{
        Name:        "custom/my-rule",
        Description: "My custom architecture rule",
        Category:    "custom",
        Tier:        lint.TierUniversal,
    }
}

func (r *MyRule) Check(ctx *lint.Context) error {
    return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
        // your logic here
        return nil
    })
}
```

Then import your package for side-effect registration:

```go
import _ "your-module/myrules"
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
