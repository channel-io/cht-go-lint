# Contributing to cht-go-lint

Thank you for your interest in contributing! This guide will help you get started.

## Development Setup

```bash
git clone https://github.com/channel-io/cht-go-lint.git
cd cht-go-lint
go test ./...
```

Requirements:
- Go 1.22+
- No CGO dependencies

## Project Structure

```
cmd/cht-go-lint/   CLI entry point
formatter/         Output formatters (text, json, github)
preset/            Built-in presets (clean-arch, etc.)
rules/
  dependency/      Import direction and isolation rules
  naming/          File and type naming rules
  iface/           Interface pattern rules
  structure/       Directory and file structure rules
  ddd/             Domain-Driven Design rules
testutil/          Test helpers
```

Core files in the root package:

- `rule.go` — `Rule` interface, `Context`, `Options`, rule registry
- `config.go` — YAML config loading and access
- `lint.go` — `Check()`, `Run()`, `QuickCheck()` APIs
- `location.go` — `LocationStrategy` interface and built-in strategies
- `analyzer.go` — Go AST parsing and file walking
- `report.go` — Thread-safe violation collection

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Rule files: one rule per file, named after the rule (e.g., `file_naming.go`)
- Register rules in `init()` functions
- Use `ctx.Options` for configurable behavior with sensible defaults

## Writing a New Rule

Here is a complete example based on `rules/naming/file_naming.go`:

```go
package naming

import (
    "fmt"
    "path/filepath"
    "strings"

    lint "github.com/channel-io/cht-go-lint"
)

func init() {
    lint.Register(&FileNaming{})
}

type FileNaming struct{}

func (r *FileNaming) Meta() lint.Meta {
    return lint.Meta{
        Name:        "naming/file-naming",
        Description: "Source file names must follow a naming convention",
        Category:    "naming",
        Tier:        lint.TierUniversal, // no config needed
    }
}

func (r *FileNaming) Check(ctx *lint.Context) error {
    convention := ctx.Options.String("convention", "snake_case")

    return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
        base := filepath.Base(file.RelPath)
        name := strings.TrimSuffix(base, ".go")

        if !isValid(name, convention) {
            ctx.Report.Add(lint.Violation{
                Rule:     "naming/file-naming",
                Severity: ctx.Severity,
                File:     file.RelPath,
                Line:     1,
                Message:  fmt.Sprintf("file name %q does not follow %s", base, convention),
            })
        }
        return nil
    })
}
```

### Steps to add a rule

1. Create a file in the appropriate `rules/<category>/` directory
2. Implement the `Rule` interface (`Meta()` and `Check()`)
3. Call `lint.Register(&YourRule{})` in `init()`
4. Choose the correct `Tier`:
   - `TierUniversal` — works without any config
   - `TierLayerAware` — requires `layers` in config
   - `TierComponentAware` — requires `components` in config
   - `TierDomainSpecific` — requires rule-specific options
5. Add tests using `testutil.RunRuleTests` or `testutil.RunRule`
6. Add the rule to the README rules table

### Testing rules

```go
func TestFileNaming(t *testing.T) {
    testutil.RunRuleTests(t, []testutil.RuleTest{
        {
            Name: "valid snake_case",
            Files: map[string]string{
                "internal/user/user_service.go": "package user",
            },
            WantViolations: 0,
        },
        {
            Name: "invalid camelCase",
            Files: map[string]string{
                "internal/user/userService.go": "package user",
            },
            WantViolations: 1,
        },
    })
}
```

## Adding a Preset

Create a new file under `preset/` and register it in `init()`:

```go
package preset

import lint "github.com/channel-io/cht-go-lint"

func init() {
    lint.RegisterPreset(&lint.Preset{
        Name:   "my-preset",
        Layers: []lint.LayerConfig{...},
        Rules:  map[string]lint.RuleConfig{...},
    })
}
```

## Pull Request Process

1. Fork the repo and create a feature branch
2. Write tests for new rules or behaviors
3. Run `go test -race ./...` locally
4. Ensure `go vet ./...` passes
5. Submit a PR with a clear description

## Reporting Issues

- Use the [bug report template](https://github.com/channel-io/cht-go-lint/issues/new?template=bug_report.md) for bugs
- Use the [feature request template](https://github.com/channel-io/cht-go-lint/issues/new?template=feature_request.md) for new rules
