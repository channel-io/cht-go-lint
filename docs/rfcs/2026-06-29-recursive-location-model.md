# RFC: Recursive Location Model

- **Status:** Draft
- **Date:** 2026-06-29
- **Target:** cht-go-lint
- **Affects:** location strategies, the `dependency/*` rules, config schema

## Summary

Replace the fixed three-axis location model (`Component` / `SubComponent` /
`Layer`) with a **recursive node tree**. A file's architectural position becomes
a *chain of nodes* resolved by path-prefix matching, declared in
`.cht-go-lint.yaml` files that cascade by directory. The four structural
dependency rules (`module-isolation`, `layer-direction`, `cross-boundary`,
`subdomain-isolation`) collapse into a single `may_import` + visibility check
over the tree.

The change is additive and ships as a new major version. Existing consumers,
pinned to the current version, are unaffected; `flat-pkg` and `nested-domain`
remain available as presets that synthesize a node tree.

## Motivation

These limits surfaced while applying cht-go-lint to `go-lib` — the first
**library** consumer and the first with **heterogeneous** internals. (All
current consumers are services: three use the `channeltalk/msa-v2` preset over
`nested-domain`, one uses `flat-pkg` with a uniform service-layer set, one uses
a minimal golangci-only config.)

1. **Not recursive.** `Location` has three fixed slots, so depth caps at three.
   A feature that nests deeper — e.g. `kafka` → `consumer` / `producer` → their
   own internals — cannot be expressed. Even the most structured consumer
   (`ch-app-store`) stops at `domain / subdomain / layer` and never nests
   `subdomain` twice.

2. **SubComponent is marker-coupled.** `SubComponent` is only assigned when a
   literal `subdomain/` directory appears in the path. Forcing a `subdomain`
   marker directory into package paths is unnatural for a library.

3. **Layers are global.** A layer name carries a single rule across every
   component. `go-lib`'s features are heterogeneous (`kafka` ≠ `sqlrepo` ≠
   `errors`), so a single global layer vocabulary does not fit — the same name
   (`pool`) means different things in different features.

4. **Three rules, one concept.** `module-isolation`, `layer-direction`, and
   `cross-boundary` all answer the same question — *"may file X import package
   Y?"* — from the architectural labels of X and Y. They can be one rule.

## Scope of change

| Class | Items |
|---|---|
| **New** | node-tree location strategy; `Location` as a node chain; node config schema (`nodes` / `may_import` / `public` / `shared` / `foundations`); the unified `dependency/import` rule. |
| **Consolidated** | `module-isolation` + `layer-direction` + `cross-boundary` + `subdomain-isolation` → the single `dependency/import` rule (4 → 1). |
| **Re-mapped** | `naming/*`, `structure/*`, `iface/*`, `ddd/*` adjust to read the node chain instead of `Component` / `Layer`. Not new rules. |
| **Kept** | engine (walk / parse / report / golangci), fix phase, parse cache; the tier concept (simplified). |

This is a **model overhaul shipped additively**, not a single new rule. The
dependency rules shrink (4 → 1); structural change is the substance.

## Background — current model

- `Location{ Component, SubComponent, Layer }` — three single-valued string
  slots assigned by a `LocationStrategy` (`flat-pkg` or `nested-domain`).
- Each dependency rule walks Go files, assigns a `Location` to each file and to
  each internal import, and compares the label pairs.
- A **tier gate** skips a rule whose prerequisite config is absent
  (`HasLayers()` / `HasComponents()`).
- Parsing is AST-only (`go/parser`, no type check) and cached per file across
  all rules.

## Proposed design

### Node tree

Architecture is a tree of **nodes**. A node maps to a directory and has three
knobs, framed by direction:

| Field | Direction | Meaning |
|---|---|---|
| `may_import` | outgoing | Other nodes this node may import. Explicit, directed edges. |
| `public` | incoming — *what* | The surface (types / sub-paths) visible across the node boundary. Everything else is internal. |
| `shared` | incoming — *who* | The node is importable by its sibling nodes under the same parent (an intra-feature foundation). |

Plus one root-level field:

- `foundations` | incoming — *who (all)* | nodes importable by anyone (e.g. `errors`, `log`).

Children are inferred by path prefix; a node's `path` is its key (in a parent's
`children`) or implied by the file's location (see *Config placement*).

### Default policy

Most behaviour is **implicit** — declared config carries only *deviations*
(allowances), the same way Go treats lowercase as private by default and you
only mark `exported` names. The baseline, applied without any declaration:

1. A node may import its own descendants (its internals).
2. A node may import `foundations`.
3. Siblings are isolated (`⊥`) unless connected by `may_import` or `shared`.
4. Unrelated nodes (different subtrees) are isolated unless `may_import`.
5. Crossing a node boundary reaches only the target's `public`; everything else
   stays internal.
6. Upward import (a node importing an ancestor directly) — policy TBD
   (see Open Questions).

The root may tune the baseline:

```yaml
defaults:
  siblings: isolated        # or `open`
  upward_import: deny
  public_when_unset: root   # default public surface when `public` is omitted
```

**Leverage Go; do not duplicate it.** Go already enforces `internal/`
(subtree hiding), exported/unexported identifiers, acyclic imports, and module
boundaries. cht-go-lint adds only what Go leaves open: **which node may import
which** (isolation + direction) and a **node-level public surface** finer than
`internal/`. `public` should compose with `internal/`, not re-implement it.

### Location assignment

A file's `Location` is the **chain of declared nodes whose paths are prefixes
of the file's path**, deepest match owning the file. Directories with no
declared node belong to their nearest declared ancestor. No marker; depth is
unbounded.

```
file:  pkg/kafka/consumer/pool/x.go
chain: [kafka, kafka/consumer]          # deepest declared prefix = kafka/consumer
                                        # `pool` is undeclared → part of consumer
```

### Config placement

One filename everywhere: **`.cht-go-lint.yaml`**, cascading by directory (like
`.eslintrc` / `.editorconfig`). No new convention; the tool already recognises
this name.

- **Root** `.cht-go-lint.yaml` — global fields (`module`, `foundations`,
  `defaults`, `rules`, golangci settings) plus the root node's body, including
  inline node declarations for simple cases.
- **Co-located** `.cht-go-lint.yaml` inside a feature directory — that
  directory's node body. Its path is implied by location, so the node name is
  not repeated.
- **Cascade:** the closer (co-located) declaration overrides the root for the
  same node.
- Root vs node is distinguished by content: the root carries `module:`.

**Recommendation:** small, centrally-owned repos (`go-lib`) keep everything in a
single root file; large multi-team repos (`ch-app-store`) co-locate per feature.
Splitting a feature into its own file is the escape valve for scale, not the
default. (Multiple files all in the repo root is discouraged — files without the
co-location benefit.)

### Global rules

Rule enablement, severity, and global conventions live in the root `rules:`
section (unchanged in spirit from today). The node tree supplies *structural
data* (who may import whom); `rules:` says *which checks run and how strictly*.

```yaml
# root .cht-go-lint.yaml
rules:
  dependency/import: error              # the unified rule's severity
  structure/forbidden-dirs: error       # no util/common/helper anywhere
  dependency/forbidden-imports:
    severity: error
    options: { patterns: ["**/internal/legacy/**"] }
```

A node may override severity for its own subtree via cascade (e.g. a legacy
node at `warn` while the global default is `error`), mirroring today's
per-component severity override.

### Discovery & assembly

A startup phase walks the tree for `.cht-go-lint.yaml` files, merges them with
root inline nodes, and assembles the node tree by path. Cheap (YAML only, no
AST), once per run; the tree becomes the location strategy used by every rule.
This mirrors Bazel loading `BUILD` files before operating — a proven pattern
also seen in Rust `mod`/`pub`, Java JPMS, and Nx module boundaries.

```
1. Load root config
2. Discover .cht-go-lint.yaml files (+ root inline nodes)   # new
3. Assemble node tree by path                               # new
4. Build analyzer with the tree as location strategy
5. Rule loop: walk .go files, assign Location via the tree, check
6. golangci integration
```

### Unified dependency rule

For an internal import, let `S` be the source file's node chain and `T` the
imported package's node chain. The import is **allowed** iff any of:

- `T ∈ foundations`
- `T` is within `S`'s own subtree (self or ancestor)
- `T` is a `shared` sibling under a common ancestor
- `T ∈ S.may_import`

— **and** the imported symbol/sub-path ∈ `T.public`. Otherwise it is a
violation. This single check subsumes `module-isolation` (default sibling
isolation), `layer-direction` (`may_import` direction), `cross-boundary`
(public surface), and `subdomain-isolation` (sibling isolation at any depth).

### Module extraction

Because the filename and node grammar are uniform, a node's
`.cht-go-lint.yaml` is already nearly a root config. Extracting a feature into
its own Go module is a **promotion**, not a rewrite:

```yaml
# before — a node inside go-lib (pkg/kafka/.cht-go-lint.yaml)
public: [Consumer, Producer, Record]
children: { core: { shared: true }, producer: {...}, consumer: {...} }

# after — its own module root (kafka/.cht-go-lint.yaml)
module: github.com/channel-io/go-kafka    # the only line added
public: [Consumer, Producer, Record]
children: { core: { shared: true }, producer: {...}, consumer: {...} }
```

Each module then has its own root `.cht-go-lint.yaml`; cross-module imports
become external (Go's module system governs them). **Optional span mode:** a
node may carry its own `go.mod` (a distribution boundary) yet remain in the
tree, so `may_import` keeps being enforced across the boundary — useful for
`go-lib`'s `auth` (own module, still part of go-lib's architecture). Span mode
requires multi-module/workspace analysis (`IsInternalImport` over all repo
modules) and is an open design item.

## Examples

```yaml
# root .cht-go-lint.yaml
module: github.com/channel-io/go-lib
foundations: [errors, log]
rules:
  dependency/import: error
nodes:
  auth: { public: [Authenticator] }        # simple → inline in root
  # kafka declared in its own file below
```

```yaml
# pkg/kafka/.cht-go-lint.yaml  (co-located; path implied = kafka)
public: [Consumer, Producer, Record]
children:
  core:     { shared: true }
  producer: { public: [Publisher] }
  consumer: { public: [Subscriber], may_import: [producer] }
```

- `kafka/consumer` may import `kafka/producer` — but only `Publisher`.
- `kafka/producer` may not import `kafka/consumer` (not in `may_import`).
- Both may import `kafka/core` (`shared`) and `errors` / `log` (`foundations`).
- Outside `kafka`, only `Consumer` / `Producer` / `Record` are visible.

## Rule re-mapping (the 41 rules)

- **`dependency/*`:** `module-isolation` + `layer-direction` + `cross-boundary`
  + `subdomain-isolation` → the single import rule. `forbidden-imports`,
  `infra-in-core`, `handler-*`, `*-service-*` either express as `may_import`
  constraints or remain as path/option rules.
- **`naming/*`, `structure/*`, `iface/*`, `ddd/*`:** most reference the
  `Component` / `Layer` labels. They re-express against the node chain (deepest
  node ≈ today's component; node role ≈ today's layer). Largely mechanical; the
  exact mapping is an open item.
- **Tier gate:** with components and layers unified into nodes, the
  layer-aware / component-aware distinction collapses into a single "has nodes"
  gate. To revisit during implementation.

## Backward compatibility

- `flat-pkg` and `nested-domain` are kept as **presets** that synthesize a node
  tree from their path conventions, so existing configs work unchanged.
- **Version pinning is the safety net.** Consumers install a pinned version
  (`go install …@vX`). Shipping the node model as a new major (`v1`) leaves
  every current consumer on their pinned version untouched; they opt in by
  bumping. No forced migration.

## Alternatives considered

- **A — single recursive node (`may_import` + `public`).** Chosen as the core:
  it collapses the dependency rules and has prior art (Bazel visibility/deps,
  Rust `mod`/`pub`, ML modules, Nx boundaries).
- **B — two primitives (`module` / `layer`).** Kept as optional *labels/sugar*
  on top of A. More readable, but not the primitive.
- **C — edge graph over path globs.** Rejected: declaring allow/deny edges
  loses the readable, semantic architecture and reduces cht-go-lint to an
  enriched `depguard`.
- **D — incremental within the three-axis model** (per-component layers +
  path-based component). Rejected: does not fix recursion or the marker, and
  remains depth-capped. (An initial component-scoped-layers PR was closed in
  favour of this RFC.)
- **Config placement — centralized vs distributed.** Support both via the
  cascade; default by repo scale rather than mandating one.

## Open questions

1. Upward-import policy (default-deny vs allow) and other `defaults` values.
2. `may_import` reference syntax — relative (`../producer`) vs rooted
   (`kafka/producer`).
3. Exact re-mapping of `naming/*`, `structure/*`, `iface/*`, `ddd/*`.
4. `public` granularity — exported symbols vs sub-paths; how it composes with
   `internal/`.
5. Span mode — multi-module/workspace analysis for cross-`go.mod` enforcement.
6. Migration tooling — generate a node tree from an existing `flat-pkg` /
   `nested-domain` config.

## Migration plan

1. RFC review.
2. Implement the node-tree strategy and the unified import rule behind the new
   config (additive; current strategies untouched).
3. Ship `flat-pkg` / `nested-domain` as presets over the new model.
4. Release as `v1` (major). `go-lib` adopts first with a single root file;
   services migrate when ready by bumping their pinned version.
