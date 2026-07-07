# dependency/import fixtures

Example projects for the `dependency/import` rule, which enforces the recursive
node-tree location model (RFC `docs/rfcs/2026-06-29-recursive-location-model.md`).
The node-tree itself is the location substrate — its own unit tests live in
`node_tree_test.go`; these fixtures exercise the *rule* on top of it. Each is a
self-contained repo with its own `go.mod` and real `.cht-go-lint.yaml` files, so
the tests run actual config parsing, co-located discovery, tree assembly, and
import checking — not in-memory stubs.

| Fixture | Shows |
|---|---|
| `library/` | A heterogeneous library: `pkg/` features, a root-level `shared` foundation, deep recursion (`kafka/consumer` walls its own internals), and a hoisted `kafka/internal/codec` node granted only to `consumer` via `may_import`. Clean — no violations. |
| `msa/`     | A service: the channeltalk msa-v2 layers defined once via `default_template` and applied to every domain, plus a cross-domain call through a public layer. Clean — no violations. |
| `violations/` | A deliberately-broken library (reverse-direction, cross-feature, undeclared-sibling, and `producer → internal/codec` where only `consumer` is granted the hoisted node). |
| `msa-violations/` | A deliberately-broken service proving the template **combination**: one `layers` template enforces both layer direction (`model → svc` blocked) and domain isolation (`order → app/model` blocked) in every domain, while same-domain layer imports stay clean. |

Both `*violations*` fixtures: every import marked `// WANT-VIOLATION` must be reported and nothing else; `TestViolationsFixture` asserts the exact set for each.

These are also the canonical worked examples — start here to see how a real
`.cht-go-lint.yaml` is laid out for a library vs a service.
