# Legacy dependency rule → node-tree config mapping

How the legacy import-edge rules translate onto the node-tree model
(`dependency/import`). Each mapping is proven by a golden parity test in
`parity_test.go`: the legacy rule and the node-tree config run on the same
fixture and must flag the same `(File, Line)` locations, with the legacy rule as
a live oracle.

This is the recipe for migrating the remaining rules. Nothing here removes a
legacy rule — the model reproduces their behavior; the rules stay.

## Reproduced (full parity)

### `dependency/layer-direction`

A layer may only import layers below it. Legacy declares an ordered
`layers:` list; node-tree declares the same layers as sibling children with an
explicit `may_import` chain.

```yaml
# legacy
layers: [ {name: model}, {name: repo, may_import: [model]},
          {name: service, may_import: [model, repo]} ]
location: { strategy: flat-pkg, options: { roots: [internal] } }

# node-tree
location: { strategy: node-tree }
roots: [internal]
children:
  model: {}
  repo:    { may_import: [model] }
  service: { may_import: [model, repo] }
```

Reverse edges (e.g. `model` importing `repo`) are denied by default under the
walling root; forward edges are allowed by `may_import`. Test:
`TestParityLayerDirection`.

### `dependency/subdomain-isolation`

Sibling subdomains under one component must not import each other. With the
default `allow_model_import=false` there is no cross-wall exception, so it maps
directly onto nested walling.

```yaml
# legacy
location: { strategy: nested-domain }
components: [ {name: order} ]   # unlocks TierComponentAware

# node-tree — layout internal/domain/<comp>/subdomain/<sub>/...
location: { strategy: node-tree }
roots: [internal/domain]
children:
  order:
    children:
      subdomain:
        children:
          cart:    {}
          payment: {}
```

`cart → payment` diverges at the `cart|payment` sibling pair under the walling
`subdomain` node and is denied. Test: `TestParitySubdomainIsolation`.

**Note — tier gating.** `subdomain-isolation` is `TierComponentAware`; it only
runs when `components:` are declared. Omit them and the legacy rule silently does
nothing (0 violations). The parity harness's false-parity guard caught exactly
this during development — a reminder that the legacy oracle must actually fire.

## Known gaps (not yet reproducible)

### `dependency/module-isolation` and `dependency/cross-boundary`

Both isolate components but exempt a **named layer across the component wall**:

- `module-isolation`: `public_layers` (default `[model]`) — any component may
  import another component's `model`, but not its other layers.
- `cross-boundary`: `boundary_layer` (default `publicsvc`) + `allow_model_import`
  (default true) + an **fx companion** convention (`Xfx` may pair with `X`).

node-tree cannot express these today. `importAllowed` (in
`rules/dependency/import.go`) resolves the decision at the **divergence sibling
pair** (e.g. `users|orders`) and never descends to the deepest target
(`orders/model`). `shared` reaches only within the parent subtree; `may_import`
opens the whole sibling. There is no per-layer cross-wall grant, so node-tree
over-flags the `users → orders/model` edge that legacy permits. Documented with
evidence by `TestParityModuleIsolation_knownGap`.

**Proposed primitives (next slice — design first):**

1. `public: true` on a node — importable across walls regardless of divergence
   (checked at the deepest target, not the sibling pair). Reproduces
   `public_layers` / `boundary_layer` / `allow_model_import`, but as an explicit
   per-node opt-in rather than a hardcoded layer name. This is arguably *cleaner*
   than the legacy blanket exemption: you choose which nodes are cross-wall
   public instead of "any layer literally named model, always".
2. `companion_suffix: fx` — for any node `N`, auto-grant `N ↔ N+suffix`.
   Reproduces the fx-companion convention without per-pair `may_import`.
   (A template `{self}` name variable does **not** solve this: template stamping
   creates children, not per-node siblings, and does not iterate existing nodes.)

Alternatively, if the companion layout can change, nesting the companion as a
child named `fx` (`orders/fx/` instead of a sibling `ordersfx`) needs no new
primitive at all — the ancestor↔descendant edge is already open, and a
constant-named `fx` child stamps cleanly from a `default_template`.
