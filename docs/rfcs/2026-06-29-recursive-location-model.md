# RFC: Recursive Location Model

> A temporary Korean translation lives at [`.ko.md`](./2026-06-29-recursive-location-model.ko.md).

- **Status:** Draft
- **Created:** 2026-06-29
- **Target:** cht-go-lint
- **Affects:** location strategies, the `dependency/*` rules, config schema

## Summary

Replace the fixed three-axis location model (`Component` / `SubComponent` /
`Layer`) with a **recursive node tree**. A file's architectural position becomes
a *chain of nodes* resolved by path-prefix matching, declared in
`.cht-go-lint.yaml` files that cascade by directory.

A node has **two fields** — `may_import` (which nodes it may import) and
`shared` (whether it may be imported by nodes in its parent's subtree). The
default is **deny**: sibling nodes are isolated unless an edge is declared.
Symbol/package visibility is left to Go (`internal/`, exported/unexported); cht
governs only the import graph that Go leaves open. The four structural
dependency rules (`module-isolation`, `layer-direction`, `cross-boundary`,
`subdomain-isolation`) collapse into a single check over the tree.

The change is additive and ships as a new major version. Existing consumers,
pinned to the current version, are unaffected; `flat-pkg` and `nested-domain`
remain available as presets.

## Motivation

These limits surfaced while applying cht-go-lint to `go-lib` — the first
**library** consumer and the first with **heterogeneous** internals. (All
current consumers are services: three use the `channeltalk/msa-v2` preset over
`nested-domain`, one uses `flat-pkg` with a uniform service-layer set, one uses
a minimal golangci-only config.)

1. **Not recursive.** `Location` has three fixed slots, so depth caps at three.
   A feature that nests deeper — e.g. `kafka` → `consumer` / `producer` → their
   own internals — cannot be expressed. Even `ch-app-store` stops at
   `domain / subdomain / layer` and never nests `subdomain` twice.

2. **SubComponent is marker-coupled.** It is assigned only when a literal
   `subdomain/` directory appears in the path. Forcing a marker directory into
   package paths is unnatural for a library.

3. **Layers are global.** A layer name carries one rule across every component.
   `go-lib`'s features are heterogeneous (`kafka` ≠ `sqlrepo` ≠ `errors`), so a
   single global layer vocabulary does not fit — the same name (`pool`) means
   different things in different features.

4. **Three rules, one concept.** `module-isolation`, `layer-direction`,
   `cross-boundary` all answer *"may file X import package Y?"* from the
   architectural labels of X and Y. They can be one rule.

## Scope of change

| Class | Items |
|---|---|
| **New** | node-tree location strategy; `Location` as a node chain; node config schema (`children` / `may_import` / `shared`); the unified `dependency/import` rule. |
| **Consolidated** | `module-isolation` + `layer-direction` + `cross-boundary` + `subdomain-isolation` → one `dependency/import` rule (4 → 1). |
| **Dropped** | per-node `public` surface, separate `foundations` list, `isolate` flags — superseded by Go `internal/` for visibility and `shared` for broadcast. |
| **Re-mapped** | `naming/*`, `structure/*`, `iface/*`, `ddd/*` read the node chain instead of `Component` / `Layer`. Not new rules. |
| **Kept** | engine (walk / parse / report / golangci), fix phase, parse cache; the tier concept (simplified). |

A model overhaul shipped additively, not a single new rule. The dependency
rules shrink (4 → 1); structural change is the substance.

## Background — current model

- `Location{ Component, SubComponent, Layer }` — three single-valued string
  slots assigned by a `LocationStrategy` (`flat-pkg` or `nested-domain`).
- Each dependency rule walks Go files, assigns a `Location` to each file and to
  each internal import, and compares the label pairs. Once layers/components are
  declared and a rule is enabled, it is **deny-by-default** (only declared
  `may_import` direction is allowed; cross-component internals are denied).
- A **tier gate** skips a rule whose prerequisite config is absent.
- Parsing is AST-only and cached per file across all rules.

## Terminology

**Tree**

- **node** — a directory that participates as a unit: either walled by its parent
  (a *walling* parent) or itself a *walling node*.
- **root** — the top node; always a node.
- **parent / child / sibling** — tree nesting; siblings share a parent.
- **ancestor / descendant / subtree** — up / down a chain; a subtree is a node
  plus all its descendants.
- **walling node** — a directory with a config; it walls its direct
  subdirectories into child nodes. Local — independent of whether it is itself
  walled.
- **leaf** — a node with no children; its subdirectories are part of its own
  code, not nodes. (A leaf is still a node.)
- **chain** — a file's `Location`: the nodes from root down to the node owning
  the file, e.g. `[root, kafka, consumer]`.

**Graph**

- **import** — an actual Go `import` in source.
- **edge** — a *declared, allowed* import relationship between two nodes. Default
  is no edge (deny); the linter checks each import against the edges.
- **`may_import`** — a directed edge declared on the importer (`A may_import B`
  is the edge `A → B`).
- **`shared`** — a broadcast edge: `B shared` opens edges from all of `B`'s
  siblings (and their subtrees) into `B`.
- **wall** — the default deny between sibling nodes; an edge is a gap in the wall.

**Policy**

- **deny-default** — sibling nodes have no edges unless one is declared.
- **divergence point** — the level where two chains split; the sibling level at
  which an import is checked.
- **visibility** — Go's `internal/` + exported/unexported identifiers. Separate
  from cht edges; cht defers surface control to Go.

## Proposed design

### Node tree

Architecture is a tree of **nodes**. A node maps to a directory and has two
fields:

| Field | Side | Meaning |
|---|---|---|
| `may_import` | pull (importer) | Nodes this node may import. Explicit, directed edges. |
| `shared` | push (importee) | If `true`, this node may be imported by any node within its **parent's subtree**. A common dependency, declared once instead of in every sibling's `may_import`. |

**What makes a directory a node.** Two separate things, often conflated:

- A directory **walls its children** — is a *walling node* — when it has a config
  (a `children` section, inline or co-located). This is **local**: its direct
  subdirectories become child nodes, isolated from each other by default. Adding a
  config to `kafka/consumer/pool` makes `pool` a walling node over its own
  subtree; it does *not* promote `consumer`. The repo root is always a walling
  node.
- A directory is **walled from its siblings** only when its *parent* is a walling
  node. So whether `consumer` is isolated from its siblings is `kafka`'s call, not
  `consumer`'s — node-hood at your level comes from your **parent's** config.

A directory with no config whose parent is a leaf is just code. Naming a child in
a config only attaches policy (`may_import` / `shared`); unnamed direct
subdirectories are nodes too (deny-default, no edges). A node's `path` is simply
its directory (e.g. `pkg/kafka/consumer`); parent/child follows from path prefix.

`shared` scope follows **position** — there is no separate "global vs sibling"
setting:

- a `shared` node directly under the repo root is importable everywhere (the
  old `foundations`, e.g. `errors`);
- a `shared` node inside `kafka` is importable within `kafka` (the old per-node
  `shared`, e.g. `kafka/core`).

**Edges are declared in the common parent.** `may_import` and `shared` for a set
of siblings live in the config of their shared parent — the level where they sit
together — not in a child's own file. `consumer → producer` is declared in
`kafka`'s config (under the `consumer` key); a split-out
`kafka/consumer/.cht-go-lint.yaml` declares only `consumer`'s *own* children and
never references a sibling. This keeps each level's wiring readable in one place
and follows how architecture linters centralize policy (Nx module boundaries,
ArchUnit, depguard) rather than how dependency managers scatter deps onto each
unit (Go imports, Bazel, Maven). The rule semantics are unchanged — `may_import`
is still node `S`'s outgoing set; only its *declaration site* is the parent.

### Default policy

The default is **deny**: sibling nodes are isolated. Consider an import from a
file in node `S` to a package in node `T`.

- **Vertical is always open.** If one of `S`, `T` is an ancestor of the other
  (they lie on the same root-to-leaf line), the import is allowed — a node sees
  its own subtree and may reach up to its enclosing features. `kafka` using its
  `consumer` child, or `consumer` using `kafka`'s shared types, is always fine.
- **Horizontal is checked at the divergence.** Otherwise `S` and `T` split at
  some level: let `Sc` and `Tc` be the sibling nodes that are the children of
  their lowest common ancestor. The import is allowed iff
  - `Tc ∈ Sc.may_import` — the sibling edge `Sc → Tc`, declared in the common
    parent. Entries name **siblings only**; how much of `Tc` is then reachable is
    Go's `internal/` decision, not a deeper `may_import` path. Or
  - `Tc` is `shared` and `Sc` lies in its parent's subtree.

  Otherwise it is a violation.

No `isolate` flag is needed — deny is the baseline, and `may_import` / `shared`
are how you open edges. Because the check always lands on the **divergence-level
siblings**, the same rule covers every depth: a deep `kafka/consumer/x`
importing `sqlrepo` resolves to the `kafka`-vs-`sqlrepo` check (so
`kafka.may_import` governs it, not `consumer`'s), and `consumer ⊥ producer`
resolves at `kafka`. A parent importing a child never bridges the child's
siblings: `kafka` using both `consumer` and `producer` does not let `consumer`
import `producer`.

**Visibility is Go's job; cht does not duplicate it.** Go already enforces
`internal/` (a package under `.../internal/...` is unreachable outside its
subtree), exported/unexported identifiers, acyclic imports, and module
boundaries. cht adds only the directed import graph Go leaves open (which node
may import which). A node hides its privates with `internal/`; cht's check runs
on top of — never instead of — Go's visibility.

**Cross-feature exposure** therefore needs no `public` field. To let `sqlrepo`
use `kafka/core`, the root config (their common parent) declares
`sqlrepo: { may_import: [kafka/core] }`; Go's `internal/` keeps `kafka`'s
privates unreachable regardless of what is declared.

### Location assignment

A file's `Location` is the **chain of nodes on its path**, the deepest node
owning the file. A directory that is not a node (it sits under a leaf) belongs to
the nearest node above it. No marker; depth unbounded.

```text
file:  pkg/kafka/consumer/pool/x.go
chain: [kafka, kafka/consumer]          # kafka and consumer are nodes
                                        # consumer is a leaf → pool is consumer's code
```

### Config placement

One filename everywhere: **`.cht-go-lint.yaml`**, cascading by directory (like
`.eslintrc` / `.editorconfig`). The tool already recognises this name.

- **Root** `.cht-go-lint.yaml` — global fields (`module`, `rules`, golangci
  settings) plus the root node's body, including inline node declarations for
  simple cases.
- **Co-located** `.cht-go-lint.yaml` inside a feature directory — wires that
  directory's *children* (its path is implied by location). A node's edge to a
  sibling still lives in the common parent's config, never here.
- **One site per node:** a node is declared in exactly one place — inline in an
  ancestor's config *or* its own co-located file, never both. Declaring the same
  node twice is an error, so there is no merge or precedence to reason about.
  (Root vs node is distinguished by content: the root carries `module:`.)
- **Global rules:** rule on/off and severity (naming, `forbidden-dirs`, …) live
  in the root `rules:` section, repo-wide. A global *import reach* is just a
  `shared` node at the root (e.g. `errors`) — no separate "global" concept is
  needed beyond these.
- **Top-level roots:** code usually lives under a prefix like `pkg/`. A
  `roots: [pkg]` option (carried from `flat-pkg`) tells the root node where its
  top-level feature children begin, so `pkg/kafka` and `pkg/sqlrepo` are the
  root's children rather than `pkg` being one node.
- **Severity override:** a node's config may set a rule's severity for its own
  subtree (cascade) — e.g. a legacy node kept at `warn` while the global default
  is `error`. This is the per-component severity override the current tool
  already has.

**Recommendation:** small, centrally-owned repos (`go-lib`) keep everything in a
single root file; large multi-team repos (`ch-app-store`) co-locate per feature.
Splitting a feature into its own file is the escape valve for scale, not the
default.

### Discovery & assembly

A startup phase walks the tree for `.cht-go-lint.yaml` files, merges them with
root inline nodes, and assembles the node tree by path. Cheap (YAML only, no
AST), once per run; the tree becomes the location strategy used by every rule.
The walk reuses the analyzer's existing exclusions (the built-in skip set —
`vendor`, `testdata`, `.git`, `generated`, `node_modules` — and the configured
`exclude_paths`) so vendored or generated config files never enter the tree.
This mirrors Bazel loading `BUILD` files before operating — a pattern also seen
in Rust `mod`/`pub`, Java JPMS, and Nx module boundaries.

```text
1. Load root config
2. Discover .cht-go-lint.yaml files (+ root inline nodes)   # new
3. Assemble node tree by path                               # new
4. Build analyzer with the tree as location strategy
5. Rule loop: walk .go files, assign Location via the tree, check
6. golangci integration
```

Step 5's `.go` walk applies the same exclusions and additionally skips
`_test.go` files, as the current engine does.

### Unified dependency rule

For an internal import, the engine takes the source and target node chains,
applies the Default-policy check (vertical → open; otherwise the divergence-level
sibling edge / `shared`), and reports a violation if it fails. This single check
subsumes `module-isolation` (sibling isolation), `layer-direction` (`may_import`
direction), `cross-boundary` (Go `internal/` for surface), and
`subdomain-isolation` (sibling isolation at any depth).

### Module extraction

Because the filename and node grammar are uniform, a node's
`.cht-go-lint.yaml` is already nearly a root config. Extracting a feature into
its own Go module is a **promotion**, not a rewrite — add `module:`:

```yaml
# before — a node inside go-lib (pkg/kafka/.cht-go-lint.yaml)
children: { core: { shared: true }, producer: {...}, consumer: { may_import: [producer] } }

# after — its own module root (kafka/.cht-go-lint.yaml)
module: github.com/channel-io/go-kafka    # the only line added
children: { core: { shared: true }, producer: {...}, consumer: { may_import: [producer] } }
```

Each module is analysed on its own — its own `.cht-go-lint.yaml`, its own tree,
its own run (`go-lib` already has a main module plus an `auth` module).
Cross-module imports are external and governed by Go's module system; **cht does
not span module boundaries.** (A "span mode" that kept an extracted node in the
tree to enforce `may_import` across the boundary would need multi-module analysis;
out of scope for v1.)

## Example

Directory:

```text
pkg/
├── errors/                 # shared at root → importable everywhere
├── kafka/
│   ├── .cht-go-lint.yaml    # this dir = the kafka node
│   ├── kafka.go             # kafka's public API (Go: exported)
│   ├── core/                # shared within kafka
│   ├── producer/
│   ├── consumer/
│   └── internal/            # Go hides this outside kafka
└── sqlrepo/
```

Config:

```yaml
# pkg/kafka/.cht-go-lint.yaml  (path implied = kafka)
children:
  core:     { shared: true }                  # consumer/producer may import it
  producer: {}
  consumer: { may_import: [producer] }         # consumer → producer (one direction)
```

- `kafka/consumer` may import `kafka/producer` and `kafka/core`; `kafka/producer`
  may not import `kafka/consumer`.
- Both may import `pkg/errors` (shared at root).
- `kafka ⊥ sqlrepo` by default; if `sqlrepo` needs `kafka`, the **root** config
  (their common parent) declares `sqlrepo: { may_import: [kafka] }` — `may_import`
  names siblings, so it grants `kafka` as a whole, and Go's `internal/` decides
  how much is actually reachable (here only the non-internal `core`).

## Rule re-mapping (the 41 rules)

- **`dependency/*`:** `module-isolation` + `layer-direction` + `cross-boundary`
  + `subdomain-isolation` → the single import rule. `forbidden-imports`,
  `infra-in-core`, `handler-*`, `*-service-*` express as `may_import`
  constraints or remain as path/option rules.
- **`naming/*`, `structure/*`, `iface/*`, `ddd/*`:** the principle — `dependency/*`
  becomes the one import rule, `naming/*` is delegated to golangci (revive), and
  the service-shaped `structure/*` / `iface/*` / `ddd/*` rules live only in the
  service presets (off for a library like `go-lib`). The exact per-rule mapping
  onto the node chain (deepest node ≈ today's component) is mechanical and settled
  during implementation, not a design question.
- **Tier gate:** components and layers unify into nodes, so the
  layer-aware / component-aware distinction collapses into a single "has nodes"
  gate. To revisit during implementation.

## Backward compatibility

- `flat-pkg` and `nested-domain` are kept as **presets** that synthesize a node
  tree from their path conventions, so existing configs work unchanged.
- **Version pinning is the safety net.** Consumers install a pinned version
  (`go install …@vX`). Shipping the node model as a new major (`v1`) leaves
  every current consumer on their pinned version untouched; they opt in by
  bumping.
- **Adoption stays gradual** the same way it does today: arch severities start
  at `warn` and tighten to `error` as a jungle is cleaned up.

## Alternatives considered

- **Open-by-default + `isolate` flag.** Considered: siblings open like Go,
  opt into strictness per node. Rejected in favour of **deny-by-default**
  (the current tool's direction): `may_import` only has teeth against a deny
  baseline, and deny-default keeps enforcement on by default rather than
  opt-in. Gradual adoption is handled by `warn` severity instead.
- **Per-node `public` surface.** Dropped — Go's `internal/` + exported
  identifiers already define a node's surface; a `public` list would duplicate
  Go. Cross-feature use is expressed by the importer's `may_import`.
- **`foundations` list / `visible_to: all` scope.** Dropped — a `shared` node at
  the repo root already reaches everywhere; position is the scope.
- **Two primitives (`module` / `layer`).** Optional labels/sugar on top of the
  node model; not the primitive.
- **Edge graph over path globs.** Rejected — loses the readable, semantic
  architecture and reduces cht-go-lint to an enriched `depguard`.
- **Incremental within the three-axis model.** Rejected — does not fix recursion
  or the marker. (An initial component-scoped-layers PR was closed for this RFC.)

## Deferred

- **Migration tooling.** Existing repos keep working through the `flat-pkg` /
  `nested-domain` presets, so no converter is needed to adopt. Whether to ship one
  that rewrites an old config into an explicit node tree is left open — decide
  after going through a manual migration once.

## Migration plan

1. RFC review.
2. Implement the node-tree strategy and the unified import rule behind the new
   config (additive; current strategies untouched).
3. Ship `flat-pkg` / `nested-domain` as presets over the new model.
4. Release as `v1` (major). `go-lib` adopts first with a single root file;
   services migrate when ready by bumping their pinned version.
