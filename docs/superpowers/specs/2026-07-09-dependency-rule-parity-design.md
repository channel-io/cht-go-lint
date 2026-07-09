# Dependency-rule parity: node-tree reproduces the legacy edge rules

Date: 2026-07-09
Status: approved, in implementation
Branch: `feat/dependency-rule-parity` (base `origin/main`)

## Goal

Prove that the node-tree engine (`dependency/import`) can reproduce the behavior
of the four legacy import-edge rules **exactly**, by writing an equivalent
node-tree config for each and asserting — against the legacy rule as a live
oracle — that both flag the same violations.

This is the first slice of migrating the legacy rules onto the recursive
location model (PR #6 follow-up). It is **additive**: no legacy rule, config
field, or test is removed. Existing projects on `flat-pkg` / `msa-v2` are
untouched.

### The four rules (Slice A)

| Rule | Intent |
|------|--------|
| `dependency/module-isolation` | top-level modules must not import each other |
| `dependency/cross-boundary` | imports must not cross a declared boundary |
| `dependency/layer-direction` | a layer may only import layers below it |
| `dependency/subdomain-isolation` | sibling subdomains must not import each other |

Only `layer-direction` has an existing test today; the other three need
fixtures authored from their rule semantics.

## Non-goals

- Deleting or altering any legacy rule (they remain the oracle).
- The remaining 38 rules (naming/structure/iface/ddd + the other dependency
  rules). Those need a projection layer (node chain → Layer/Component/Tags) and
  are separate slices.
- An automatic legacy-config → node-tree-config translator. Deferred until the
  manual mapping proves mechanical.

## Design

### Parity is conditional on an equivalent config

The engine is not magic. Parity means: **if the legacy rule's architecture is
re-expressed as an equivalent node-tree config, the engine flags the same
violations.** Authoring that translation is the work; the translation recipe is
a first-class deliverable (see Mapping doc).

Example — `layer-direction`:

```yaml
# legacy
layers: [ {name: model}, {name: repo, may_import: [model]},
          {name: service, may_import: [model, repo]} ]
location: { strategy: flat-pkg, options: { roots: [internal] } }

# node-tree equivalent
location: { strategy: node-tree }
roots: [internal]
children:
  model: {}
  repo:    { may_import: [model] }
  service: { may_import: [model, repo] }
```

### Parity harness

A single test helper:

```
assertParity(t, buildFixture, legacyCfg, nodeTreeCfg)
```

1. Build one `TempDir` fixture (clean packages + a deliberate violation).
2. Run `lint.Check` twice on it — once with `legacyCfg` (legacy strategy + legacy
   rule), once with `nodeTreeCfg` (node-tree strategy + `dependency/import`).
3. Reduce each report to a violation set keyed by **`(File, Line)`**.
4. Assert the two sets are equal; on mismatch, print the symmetric difference
   (locations flagged only-by-legacy vs only-by-node-tree).

**Key = `(File, Line)`, not the full violation.** `Rule`, `Message`, `Severity`,
and `Found` legitimately differ between the two rules — e.g. `layer-direction`
sets `Found` to the *layer name* (`iloc.Layer`) while the path-based rules set it
to the import path. `(File, Line)` is the behavioral signal: which import got
flagged. Go allows one import per line, so `(File, Line)` uniquely identifies the
offending edge.

**False-parity guard.** The violation fixture must produce ≥1 violation from each
side; `assertParity` fails if either side is empty. This prevents a silent
`0 == 0` from passing as parity (the exact failure mode this migration exists to
prevent).

### Per-rule deliverable

For each of the four rules:
- a fixture with both a clean case (0 violations) and a violation case;
- the legacy `Config` (Layers/strategy/etc.);
- the equivalent node-tree `Config`;
- an `assertParity` call for the clean case (both 0) and the violation case
  (both flag the same `(File, Line)` set).

### Production code changes are contingent

`dependency/import` already exists, so the default outcome is **tests + doc
only**. If a parity test reveals node-tree cannot express an edge the legacy rule
catches, that gap gets a minimal production fix (as with the co-located-template
gap in PR #6) — harness first, fixes only where parity breaks.

## Outcome

Two of the four reproduce with full parity; two hit a genuine node-tree
expressiveness gap.

| Rule | Result |
|------|--------|
| `layer-direction` | full parity — `may_import` chain |
| `subdomain-isolation` | full parity — nested walling |
| `module-isolation` | **gap** — `public_layers` (model) cross-wall exemption |
| `cross-boundary` | **gap** — same shape (`boundary_layer` / model / fx) |

The gap: `importAllowed` decides at the divergence sibling pair and never
descends to the deepest target, so node-tree cannot grant "one named layer is
importable across the component wall". It over-flags the `users → orders/model`
edge legacy permits. Documented with evidence, not silently skipped.

### This slice (merge now)

- `layer-direction`, `subdomain-isolation`: full parity tests, green.
- `module-isolation`: `TestParityModuleIsolation_knownGap` — asserts the current
  over-flag with evidence; fails (as a signal to update) once the gap is closed.
- `cross-boundary`: same root cause, documented in the mapping doc (no separate
  test — identical mechanism).
- `go build ./... && go vet ./... && go test ./...` green.
- Legacy rules and their tests untouched.

### Next slice (design first, separate work)

Close the gap with two opt-in primitives, designed properly rather than rushed:
- `public: true` — a node importable across walls (checked at the deepest
  target). Reproduces `public_layers` / `boundary_layer` / `allow_model_import`
  as an explicit per-node opt-in — cleaner than the legacy blanket exemption.
- `companion_suffix: fx` — auto-grant `N ↔ N+suffix` for the fx convention.

See `docs/dependency-rule-parity-mapping.md`.

## Files

- `parity_test.go` — harness + parity tests (package `lint_test`).
- `docs/dependency-rule-parity-mapping.md` — the translation recipe + gap record.
- No production files touched: node-tree already supports the reproduced edges;
  the gap rules are deferred to the primitives above.

## PR

Branch `feat/dependency-rule-parity` → PR to `origin`. Per-rule TDD: write the
parity test, make the node-tree config reproduce it, green.
