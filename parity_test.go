package lint_test

import (
	"fmt"
	"sort"
	"testing"

	lint "github.com/channel-io/cht-go-lint"
	_ "github.com/channel-io/cht-go-lint/rules"
)

// Parity tests prove the node-tree engine (dependency/import) reproduces the
// legacy import-edge rules exactly. The legacy rule is the live oracle: both run
// on the same fixture and must flag the same violation locations. See
// docs/superpowers/specs/2026-07-09-dependency-rule-parity-design.md.

// parityKey identifies a flagged import edge. Rule/Message/Severity/Found
// legitimately differ between the legacy rule and dependency/import (e.g.
// layer-direction sets Found to the layer name, the path-based rules to the
// import path), so parity is judged on location alone. Go allows one import per
// line, so (File, Line) uniquely identifies the offending edge.
type parityKey struct {
	File string
	Line int
}

func violationKeys(r *lint.Report) map[parityKey]bool {
	keys := map[parityKey]bool{}
	for _, v := range r.Violations() {
		keys[parityKey{File: v.File, Line: v.Line}] = true
	}
	return keys
}

func sortedKeys(m map[parityKey]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, fmt.Sprintf("%s:%d", k.File, k.Line))
	}
	sort.Strings(out)
	return out
}

// assertParity runs both configs against the same fixture (Root already set on
// each) and asserts they flag the identical set of (File, Line) locations.
// minViolations guards against a silent 0 == 0 passing as parity: both sides
// must produce at least that many violations.
func assertParity(t *testing.T, legacy, nodeTree *lint.Config, minViolations int) {
	t.Helper()
	legacyKeys := violationKeys(lint.Check(legacy))
	nodeKeys := violationKeys(lint.Check(nodeTree))

	if len(legacyKeys) < minViolations || len(nodeKeys) < minViolations {
		t.Fatalf("false-parity guard: want >=%d violations each, got legacy=%d node-tree=%d",
			minViolations, len(legacyKeys), len(nodeKeys))
	}

	onlyLegacy, onlyNode := diffKeys(legacyKeys, nodeKeys)
	if len(onlyLegacy) > 0 || len(onlyNode) > 0 {
		t.Errorf("parity mismatch:\n  only legacy caught:    %v\n  only node-tree caught: %v",
			onlyLegacy, onlyNode)
	}
}

func diffKeys(a, b map[parityKey]bool) (onlyA, onlyB []string) {
	am, bm := map[parityKey]bool{}, map[parityKey]bool{}
	for k := range a {
		if !b[k] {
			am[k] = true
		}
	}
	for k := range b {
		if !a[k] {
			bm[k] = true
		}
	}
	return sortedKeys(am), sortedKeys(bm)
}

// --- dependency/layer-direction ---------------------------------------------

// A layer may only import layers below it: model < repo < service. The reverse
// edge (model importing repo) is the violation both engines must catch.
func layerDirectionFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "internal/model/user.go", "package model\n\ntype User struct{}\n")
	writeGoFile(t, dir, "internal/repo/user_repo.go",
		"package repo\n\nimport \"example.com/test/internal/model\"\n\ntype UserRepo struct{ _ model.User }\n")
	writeGoFile(t, dir, "internal/service/user_svc.go",
		"package service\n\nimport \"example.com/test/internal/repo\"\n\ntype UserSvc struct{ _ repo.UserRepo }\n")
	// reverse-direction violation: model imports repo.
	writeGoFile(t, dir, "internal/model/bad.go",
		"package model\n\nimport \"example.com/test/internal/repo\"\n\nvar _ = repo.UserRepo{}\n")
	return dir
}

func TestParityLayerDirection(t *testing.T) {
	dir := layerDirectionFixture(t)

	legacy := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Layers: []lint.LayerConfig{
			{Name: "model", MayImport: []string{}},
			{Name: "repo", MayImport: []string{"model"}},
			{Name: "service", MayImport: []string{"model", "repo"}},
		},
		Location: &lint.LocationConfig{
			Strategy: "flat-pkg",
			Options:  map[string]any{"roots": []any{"internal"}},
		},
		Rules: map[string]lint.RuleConfig{"dependency/layer-direction": {Severity: lint.Error}},
	}

	nodeTree := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"internal"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"},
		Children: map[string]*lint.NodeConfig{
			"model":   {},
			"repo":    {MayImport: []string{"model"}},
			"service": {MayImport: []string{"model", "repo"}},
		},
		Rules: map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
	}

	assertParity(t, legacy, nodeTree, 1)
}

// --- dependency/module-isolation (KNOWN GAP) --------------------------------

// module-isolation isolates components but exempts public_layers (default:
// model) — any component may import another component's model, but not its other
// layers. cross-boundary shares this shape (its boundary_layer / model / fx
// exemptions). node-tree cannot reproduce it today: importAllowed resolves the
// decision at the divergence sibling pair (users|orders) and never descends to
// the deepest target (orders/model). `shared` only reaches within the parent
// subtree; `may_import` opens the whole sibling. There is no per-layer cross-wall
// grant, so node-tree over-flags the users→orders/model edge legacy permits.
//
// This test documents that gap with evidence rather than proving parity. It is
// deliberately NOT an assertParity call. When the cross-wall-public primitive
// (`public: true`) lands, node-tree will stop flagging the model edge and this
// test will fail — that failure is the signal to convert it to a real parity
// test.
//
// Closing the gap (next slice, design first) needs two opt-in primitives:
//   - `public: true` on a node — importable across walls, decided at the deepest
//     target rather than the divergence sibling. Reproduces public_layers /
//     boundary_layer / allow_model_import as an explicit per-node opt-in.
//   - `companion_suffix: fx` — auto-grant N <-> N+suffix for the fx-companion
//     convention (cross-boundary's Xfx pairing). A template {self} variable does
//     not solve this: stamping creates children, not per-node siblings.
func TestParityModuleIsolation_knownGap(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "internal/orders/model/order.go", "package model\n\ntype Order struct{}\n")
	writeGoFile(t, dir, "internal/orders/repo/order_repo.go", "package repo\n\ntype OrderRepo struct{}\n")
	// cross-component to a non-public layer — violation for BOTH engines.
	writeGoFile(t, dir, "internal/users/svc/bad_repo.go",
		"package svc\n\nimport \"example.com/test/internal/orders/repo\"\n\nvar _ = repo.OrderRepo{}\n")
	// cross-component to the public layer (model) — legacy ALLOWS, node-tree blocks.
	writeGoFile(t, dir, "internal/users/svc/ok_model.go",
		"package svc\n\nimport \"example.com/test/internal/orders/model\"\n\nvar _ = model.Order{}\n")

	legacy := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Layers: []lint.LayerConfig{
			{Name: "model"}, {Name: "repo"}, {Name: "svc"},
		},
		Components: []lint.ComponentConfig{{Name: "orders"}, {Name: "users"}},
		Location: &lint.LocationConfig{
			Strategy: "flat-pkg",
			Options:  map[string]any{"roots": []any{"internal"}},
		},
		Rules: map[string]lint.RuleConfig{"dependency/module-isolation": {Severity: lint.Error}},
	}
	nodeTree := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"internal"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"},
		Children: map[string]*lint.NodeConfig{
			"users":  {Children: map[string]*lint.NodeConfig{"svc": {}}},
			"orders": {Children: map[string]*lint.NodeConfig{"repo": {}, "model": {}}},
		},
		Rules: map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
	}

	legacyKeys := violationKeys(lint.Check(legacy))
	nodeKeys := violationKeys(lint.Check(nodeTree))

	badRepo := parityKey{File: "internal/users/svc/bad_repo.go", Line: 3}
	okModel := parityKey{File: "internal/users/svc/ok_model.go", Line: 3}

	// The core isolation edge (non-public layer) is reproduced by both engines.
	if !legacyKeys[badRepo] || !nodeKeys[badRepo] {
		t.Fatalf("both engines must flag the cross-component non-public edge; legacy=%v node-tree=%v",
			legacyKeys[badRepo], nodeKeys[badRepo])
	}
	// The gap: legacy permits the public-layer (model) edge; node-tree over-flags it.
	if legacyKeys[okModel] {
		t.Fatalf("legacy module-isolation should permit the public-layer (model) edge")
	}
	if !nodeKeys[okModel] {
		t.Fatalf("expected node-tree to over-flag the public-layer edge (known gap). " +
			"If this now passes, the cross-wall `public:` primitive likely landed — " +
			"convert this to a real parity test and update the mapping doc.")
	}
}

// --- dependency/subdomain-isolation -----------------------------------------

// Sibling subdomains under one component must not import each other. With the
// default allow_model_import=false there is no cross-wall exception, so this
// maps cleanly onto nested walling: component `order` walls `subdomain`, which
// walls siblings `cart` and `payment`. cart→payment is the violation.
func subdomainIsolationFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "internal/domain/order/subdomain/payment/payment.go",
		"package payment\n\ntype Charge struct{}\n")
	// cart importing a sibling subdomain (payment) is the violation.
	writeGoFile(t, dir, "internal/domain/order/subdomain/cart/cart.go",
		"package cart\n\nimport \"example.com/test/internal/domain/order/subdomain/payment\"\n\nvar _ = payment.Charge{}\n")
	return dir
}

func TestParitySubdomainIsolation(t *testing.T) {
	dir := subdomainIsolationFixture(t)

	legacy := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Location:   &lint.LocationConfig{Strategy: "nested-domain"},
		// subdomain-isolation is TierComponentAware — it only runs when
		// components are declared. The declaration just unlocks the tier; the
		// rule derives components from the strategy, not this list.
		Components: []lint.ComponentConfig{{Name: "order"}},
		Rules:      map[string]lint.RuleConfig{"dependency/subdomain-isolation": {Severity: lint.Error}},
	}

	nodeTree := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"internal/domain"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"},
		Children: map[string]*lint.NodeConfig{
			"order": {Children: map[string]*lint.NodeConfig{
				"subdomain": {Children: map[string]*lint.NodeConfig{
					"cart":    {},
					"payment": {},
				}},
			}},
		},
		Rules: map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
	}

	assertParity(t, legacy, nodeTree, 1)
}
