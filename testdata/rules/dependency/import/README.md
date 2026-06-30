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
| `library/` | A heterogeneous library: `pkg/` features, per-feature internals, a root-level `shared` foundation, and deep recursion (`kafka/consumer` walls its own internals). Clean — no violations. |
| `msa/`     | A service: the channeltalk msa-v2 layers defined once via `default_template` and applied to every domain, plus a cross-domain call through a public layer. Clean — no violations. |
| `violations/` | A deliberately-broken repo. Every import marked `// WANT-VIOLATION` must be reported and nothing else; `TestViolationsFixture` asserts the exact set. |

These are also the canonical worked examples — start here to see how a real
`.cht-go-lint.yaml` is laid out for a library vs a service.
