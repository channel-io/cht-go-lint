# dependency/import fixtures

Behavior fixtures for the `dependency/import` rule (recursive node-tree model, RFC
`docs/rfcs/2026-06-29-recursive-location-model.md`). **One fixture per behavior.** Each is a
self-contained repo (`go.mod` + `.cht-go-lint.yaml` + `.go`) that mixes allowed and blocked
imports in the same tree:

- an **allowed** import has no marker — it must stay clean (flagging it fails the test);
- a **blocked** import carries a `// WANT-VIOLATION` comment — it must be reported.

`TestImportFixtures` runs the rule once per fixture and asserts the reported violations are
exactly the marked lines. So each fixture proves both the allowed and the blocked side of its
behavior in one place — no separate clean vs. violation projects.

| Fixture | Allowed (no marker) | Blocked (`// WANT-VIOLATION`) |
|---|---|---|
| `sibling-isolation` | consumer → producer (may_import) | kafka → sqlrepo (sibling feature); undeclared `loose` → producer |
| `layer-direction` | svc → repo → model (down) | model → svc (up) |
| `shared-scope` | → errs (root shared, everywhere); consumer → kafka/core (feature-local shared) | consumer → producer (not shared, no edge) |
| `internal-hoist` | consumer → internal/codec (granted) | producer → internal/codec (not granted) |
| `template-combination` | order/svc → order/model (same-domain layer) | order/svc → app/model (domain isolation); order/model → order/svc (layer direction) |
| `cross-domain-grant` | app → order/publicsvc (app may_import order) | order → app/model (reverse, not granted) |

Config-assembly diagnostics (`node-tree/config`: unknown/self-referential template, unparseable
co-located config, hoist name collision, unscannable directory) are **not** import-graph
behaviors — they are covered by unit tests (`lint_test.go`, `node_tree_test.go`), not fixtures.
