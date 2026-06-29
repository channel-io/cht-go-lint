# RFC: Recursive Location Model

- **Status:** Draft
- **Date:** 2026-06-29
- **Target:** cht-go-lint
- **Affects:** location strategies, the `dependency/*` rules, config schema

## Summary

Replace the fixed three-axis location model (`Component` / `SubComponent` /
`Layer`) with a **recursive node tree**. A file's architectural position becomes
a *chain of nodes* resolved by path-prefix matching, declared in a hybrid of a
root config and optional co-located per-feature config files. The three
dependency rules (`module-isolation`, `layer-direction`, `cross-boundary`)
collapse into a single `may_import` + visibility check over the tree.

The change is additive and ships as a new major version. Existing consumers,
pinned to the current version, are unaffected; `flat-pkg` and `nested-domain`
remain available as presets that synthesize a node tree.

## Motivation

These limits surfaced while applying cht-go-lint to `go-lib` — the first
**library** consumer and the first with **heterogeneous** internals. (All
current consumers are services: three use the `channeltalk/msa-v2` preset
over `nested-domain`, one uses `flat-pkg` with a uniform service-layer set,
one uses a minimal golangci-only config.)

1. **Not recursive.** `Location` has three fixed slots, so depth caps at three.
   A feature that nests deeper — e.g. `kafka` → `consumer` / `producer` → their
   own internals — cannot be expressed. Even the most structured consumer
   (`ch-app-store`) stops at `domain / subdomain / layer` and never nests
   `subdomain` twice.

2. **SubComponent is marker-coupled.** `SubComponent` is only assigned when a
   literal `subdomain/` directory appears in the path (the `nested-domain`
   strategy). Forcing a `subdomain` marker directory into package paths is
   unnatural for a library.

3. **Layers are global.** A layer name carries a single rule across every
   component. `go-lib`'s features are heterogeneous (`kafka` ≠ `sqlrepo` ≠
   `errors`), so a single global layer vocabulary does not fit — the same name
   (`pool`) means different things in different features.

4. **Three rules, one concept.** `module-isolation`, `layer-direction`, and
   `cross-boundary` all answer the same question — *"may file X import package
   Y?"* — from the architectural labels of X and Y. They can be one rule.

## Background — current model

- `Location{ Component, SubComponent, Layer }` — three single-valued string
  slots assigned by a `LocationStrategy` (`flat-pkg` or `nested-domain`).
- Each dependency rule walks Go files, assigns a `Location` to each file and to
  each internal import, and compares the label pairs:
  - `layer-direction` — `Layer.may_import` direction (layer-aware tier).
  - `module-isolation` — components must not import each other's internals;
    escape via `public_layers` / `allowed_cross_imports` (component-aware tier).
  - `cross-boundary` — cross-component imports must target a public boundary
    layer (component-aware tier).
- A **tier gate** skips a rule whose prerequisite config is absent
  (`HasLayers()` / `HasComponents()`), so registered-but-irrelevant rules do
  not fire.
- Parsing is AST-only (`go/parser`, no type check) and cached per file across
  all rules.

## Proposed design

### Node tree

Architecture is a tree of **nodes**. A node maps to a directory and declares:

| Field | Meaning |
|---|---|
| `path` | Where the node lives, e.g. `kafka/consumer`. Children are inferred by path prefix. |
| `public` | The surface (types / sub-paths) visible *across* the node boundary. Everything else is internal. |
| `may_import` | Other nodes this node may import — explicit, directed edges. |
| `shared` | The node is importable by its sibling nodes under the same parent (an intra-feature foundation). |
| `isolate_children` | Children are isolated from each other by default. |

A top-level `foundations` set lists nodes importable by anyone (e.g. `errors`,
`log`).

### Location assignment

A file's `Location` is the **chain of declared nodes whose paths are prefixes
of the file's path**, with the deepest match owning the file. Directories with
no declared node belong to their nearest declared ancestor. No marker
directory; depth is unbounded.

```
file:  pkg/kafka/consumer/pool/x.go
chain: [kafka, kafka/consumer]          # deepest declared prefix = kafka/consumer
                                        # `pool` is undeclared → part of consumer
```

### Config placement (hybrid)

Two placements, assembled into one tree:

- **Root** `.cht-go-lint.yaml` — `module`, `foundations`, presets, severities,
  golangci settings, and inline node declarations for simple cases.
- **Co-located** arch files (e.g. `arch.yaml`) inside a feature directory,
  describing that feature's subtree.

Cascade: the closer (co-located) declaration overrides the root for the same
node. The root config is, in effect, the arch file of the root node — one
uniform mechanism.

**Recommendation:** small, centrally-owned repos (`go-lib`) keep everything in
a single root file; large multi-team repos (`ch-app-store`) co-locate per
feature. Splitting a feature into its own file is the escape valve for scale,
not the default. (A middle option — multiple files all in the repo root — is
discouraged: it adds files without the co-location benefit.)

### Discovery & assembly

A startup phase walks the tree for arch files, merges them with root inline
nodes, and assembles the node tree by path. This walk is cheap (YAML only, no
AST) and runs once; the tree becomes the location strategy used by every rule.
This mirrors how Bazel loads `BUILD` files before operating.

```
1. Load root config
2. Discover arch files (+ root inline nodes)        # new
3. Assemble node tree by path                       # new
4. Build analyzer with the tree as location strategy
5. Rule loop: walk .go files, assign Location via the tree, check
6. golangci integration
```

### Unified dependency rule

For an internal import, let `S` be the source file's node chain and `T` the
imported package's node chain. The import is **allowed** iff any of:

- `T ∈ foundations`
- `T` is within `S`'s own subtree (self or ancestor) — a node sees its own internals
- `T` is a `shared` sibling under a common ancestor
- `T ∈ S.may_import` **and** the imported symbol/sub-path ∈ `T.public`

Otherwise it is a violation. This single check subsumes:

- `module-isolation` → siblings are isolated by default (`T ∉ may_import`).
- `layer-direction` → direction is `T ∈ may_import`.
- `cross-boundary` → cross-node access is limited to `T.public`.

### Example

```yaml
# .cht-go-lint.yaml (root)
module: github.com/channel-io/go-lib
foundations: [errors, log]
nodes:
  kafka:
    public: [Consumer, Producer, Record]
    isolate_children: true
    children:
      core:     { shared: true }
      producer: { public: [Publisher] }
      consumer: { public: [Subscriber], may_import: [producer] }
```

- `kafka/consumer` may import `kafka/producer` — but only `Publisher` (its
  public surface), not its internals.
- `kafka/producer` may **not** import `kafka/consumer` (not in `may_import`).
- Both may import `kafka/core` (`shared`) and `errors` / `log` (`foundations`).
- Anything outside `kafka` sees only `Consumer` / `Producer` / `Record`.

## Rule re-mapping (the 41 rules)

- **`dependency/*`:** `module-isolation` + `layer-direction` + `cross-boundary`
  → the single import rule above. `subdomain-isolation` becomes emergent
  (sibling isolation at any depth). `forbidden-imports`, `infra-in-core`,
  `handler-*`, `*-service-*` either express as `may_import` constraints or
  remain as path/option rules.
- **`naming/*`, `structure/*`, `iface/*`, `ddd/*`:** most reference the
  `Component` / `Layer` labels. They re-express against the node chain (deepest
  node ≈ today's component; node role ≈ today's layer). Largely mechanical; the
  exact mapping is an open item.
- **Tier gate:** with components and layers unified into nodes, the
  layer-aware / component-aware distinction blurs into a single "has nodes"
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
  it collapses the three dependency rules and has prior art (Bazel
  visibility/deps, Rust `mod`/`pub`, ML modules).
- **B — two primitives (`module` / `layer`).** Kept as optional *labels/sugar*
  on top of A (a `module` defaults to isolation + required public surface; a
  `layer` defaults to ordered `may_import`). More readable, but not the
  primitive.
- **C — edge graph over path globs.** Rejected: declaring allow/deny edges
  loses the readable, semantic architecture and reduces cht-go-lint to an
  enriched `depguard`.
- **D — incremental within the three-axis model** (per-component layers +
  path-based component). Rejected: it does not fix recursion or the marker, and
  remains depth-capped. (An initial component-scoped-layers PR was closed in
  favor of this RFC.)
- **Config placement — centralized vs distributed.** Support both via the
  hybrid/cascade; default by repo scale rather than mandating one.

## Open questions

1. Default semantics — confirm "sibling-default-deny + `foundations` +
   `shared`" as the baseline.
2. arch file name/format; `may_import` as relative (`../producer`) vs absolute
   (`kafka/producer`) paths.
3. Exact re-mapping of `naming/*`, `structure/*`, `iface/*`, `ddd/*`.
4. `public` granularity — exported symbols vs sub-paths (e.g. `internal/`).
5. Migration tooling — generate a node tree from an existing `flat-pkg` /
   `nested-domain` config.

## Migration plan

1. RFC review.
2. Implement the node-tree strategy and the unified import rule behind the new
   config (additive; current strategies untouched).
3. Ship `flat-pkg` / `nested-domain` as presets over the new model.
4. Release as `v1` (major). `go-lib` adopts first with a single root file;
   services migrate when ready by bumping their pinned version.
