package lint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	lint "github.com/channel-io/cht-go-lint"
	_ "github.com/channel-io/cht-go-lint/rules"
)

func TestRuleRegistration(t *testing.T) {
	rules := lint.All()
	if len(rules) == 0 {
		t.Fatal("no rules registered")
	}

	// Check that all expected categories are present
	categories := make(map[string]int)
	for _, r := range rules {
		categories[r.Meta().Category]++
	}

	expected := map[string]int{
		"dependency": 12,
		"naming":     8,
		"interface":  5,
		"structure":  9,
		"ddd":        8,
	}
	for cat, want := range expected {
		if got := categories[cat]; got != want {
			t.Errorf("category %q: got %d rules, want %d", cat, got, want)
		}
	}
	t.Logf("total rules registered: %d", len(rules))
}

func TestCheckWithNoConfig(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "main.go", `package main

func main() {}
`)
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules:      map[string]lint.RuleConfig{},
	}

	// With no rules enabled, should find no violations
	report := lint.Check(cfg)
	if report.Total() != 0 {
		t.Errorf("expected 0 violations with all rules off, got %d:\n%s", report.Total(), report.String())
	}
}

func TestCheckFileNaming(t *testing.T) {
	dir := t.TempDir()
	// snake_case is fine
	writeGoFile(t, dir, "good_name.go", "package test\n")
	// camelCase is a violation
	writeGoFile(t, dir, "badName.go", "package test\n")
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"naming/file-naming": {Severity: lint.Error},
		},
	}

	report := lint.Check(cfg)
	if report.ErrorCount() != 1 {
		t.Errorf("expected 1 error for camelCase filename, got %d:\n%s", report.ErrorCount(), report.String())
	}
}

func TestCheckNoStutter(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "user.go", `package user

type UserService struct{}
type Service struct{}
`)
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"naming/no-stutter": {Severity: lint.Error},
		},
	}

	report := lint.Check(cfg)
	if report.ErrorCount() < 1 {
		t.Errorf("expected at least 1 error for stuttering type UserService in package user, got %d:\n%s",
			report.ErrorCount(), report.String())
	}
}

func TestCheckLayerDirection(t *testing.T) {
	dir := t.TempDir()
	// Create a simple project structure
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "internal/model/user.go", "package model\n\ntype User struct{}\n")
	writeGoFile(t, dir, "internal/repo/user_repo.go", `package repo

import "example.com/test/internal/model"

type UserRepo struct{ _ model.User }
`)
	// Service importing repo is fine, but model importing repo is a violation
	writeGoFile(t, dir, "internal/service/user_svc.go", `package service

import "example.com/test/internal/repo"

type UserSvc struct{ _ repo.UserRepo }
`)

	cfg := &lint.Config{
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
		Rules: map[string]lint.RuleConfig{
			"dependency/layer-direction": {Severity: lint.Error},
		},
	}

	report := lint.Check(cfg)
	// service->repo is allowed, repo->model is allowed, so 0 violations
	if report.ErrorCount() != 0 {
		t.Errorf("expected 0 errors for valid layer direction, got %d:\n%s",
			report.ErrorCount(), report.String())
	}
}

func TestCheckForbiddenDirs(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "main.go", "package main\n")

	// Create a forbidden directory
	utilDir := filepath.Join(dir, "util")
	os.MkdirAll(utilDir, 0755)
	writeGoFile(t, dir, "util/helpers.go", "package util\n")

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"structure/forbidden-dirs": {Severity: lint.Error},
		},
	}

	report := lint.Check(cfg)
	if report.ErrorCount() < 1 {
		t.Errorf("expected at least 1 error for forbidden 'util' directory, got %d:\n%s",
			report.ErrorCount(), report.String())
	}
}

func TestConfigYAML(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
module: example.com/myproject
extends: []
layers:
  - name: model
    may_import: []
  - name: service
    aliases: [svc]
    may_import: [model]
rules:
  naming/file-naming: warn
  dependency/layer-direction:
    severity: error
    options:
      strict: true
`
	os.WriteFile(filepath.Join(dir, ".cht-go-lint.yaml"), []byte(yamlContent), 0644)

	cfg, err := lint.LoadConfig(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.ModulePath != "example.com/myproject" {
		t.Errorf("module: got %q, want %q", cfg.ModulePath, "example.com/myproject")
	}
	if len(cfg.Layers) != 2 {
		t.Errorf("layers: got %d, want 2", len(cfg.Layers))
	}
	if cfg.EffectiveSeverity("naming/file-naming", "") != lint.Warn {
		t.Error("naming/file-naming severity should be Warn")
	}
	if cfg.EffectiveSeverity("dependency/layer-direction", "") != lint.Error {
		t.Error("dependency/layer-direction severity should be Error")
	}
	if cfg.ResolveLayerName("svc") != "service" {
		t.Errorf("alias resolution: got %q, want %q", cfg.ResolveLayerName("svc"), "service")
	}
}

func TestExcludePaths(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")

	// Create files in multiple directories
	writeGoFile(t, dir, "internal/service.go", "package internal\n\ntype UserService struct{}\n")
	writeGoFile(t, dir, "lib/helper.go", "package lib\n\ntype LibHelper struct{}\n")
	writeGoFile(t, dir, "cmd/main.go", "package main\n")

	// Without exclude_paths, all files are scanned — expect violations from lib and cmd
	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"naming/file-naming": {Severity: lint.Error},
		},
	}
	report := lint.Check(cfg)
	totalWithout := report.Total()

	// Now create a file with bad naming in lib only
	writeGoFile(t, dir, "lib/badName.go", "package lib\n")

	cfg2 := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"naming/file-naming": {Severity: lint.Error},
		},
	}
	reportNoExclude := lint.Check(cfg2)
	errorsNoExclude := reportNoExclude.ErrorCount()

	// With exclude_paths, lib is excluded
	cfg3 := &lint.Config{
		Root:         dir,
		ModulePath:   "example.com/test",
		ExcludePaths: []string{"lib"},
		Rules: map[string]lint.RuleConfig{
			"naming/file-naming": {Severity: lint.Error},
		},
	}
	reportExclude := lint.Check(cfg3)
	errorsExclude := reportExclude.ErrorCount()

	// The excluded version should have fewer errors since lib/badName.go is skipped
	if errorsExclude >= errorsNoExclude {
		t.Errorf("exclude_paths should reduce violations: without=%d, with=%d",
			errorsNoExclude, errorsExclude)
	}

	_ = totalWithout
}

func TestExcludePathsYAML(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
module: example.com/test
exclude_paths:
  - lib
  - cmd
  - test
`
	os.WriteFile(filepath.Join(dir, ".cht-go-lint.yaml"), []byte(yamlContent), 0644)

	cfg, err := lint.LoadConfig(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if len(cfg.ExcludePaths) != 3 {
		t.Errorf("exclude_paths: got %d, want 3", len(cfg.ExcludePaths))
	}
	expected := []string{"lib", "cmd", "test"}
	for i, want := range expected {
		if i < len(cfg.ExcludePaths) && cfg.ExcludePaths[i] != want {
			t.Errorf("exclude_paths[%d]: got %q, want %q", i, cfg.ExcludePaths[i], want)
		}
	}
}

func TestGoLintConfigYAML(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
module: example.com/test
go_lint:
  enabled: true
  config: .golangci.yaml
  args:
    - --new-from-merge-base=origin/main
`
	os.WriteFile(filepath.Join(dir, ".cht-go-lint.yaml"), []byte(yamlContent), 0644)

	cfg, err := lint.LoadConfig(dir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.GoLint == nil {
		t.Fatal("go_lint should not be nil")
	}
	if !cfg.GoLint.Enabled {
		t.Error("go_lint.enabled should be true")
	}
	if cfg.GoLint.Config != ".golangci.yaml" {
		t.Errorf("go_lint.config: got %q, want %q", cfg.GoLint.Config, ".golangci.yaml")
	}
	if len(cfg.GoLint.Args) != 1 || cfg.GoLint.Args[0] != "--new-from-merge-base=origin/main" {
		t.Errorf("go_lint.args: got %v, want [--new-from-merge-base=origin/main]", cfg.GoLint.Args)
	}
}

func TestPresetMerge(t *testing.T) {
	// Register a test preset
	lint.RegisterPreset(&lint.Preset{
		Name: "test-preset",
		Layers: []lint.LayerConfig{
			{Name: "model", MayImport: []string{}},
			{Name: "handler", MayImport: []string{"model"}},
		},
		Rules: map[string]lint.RuleConfig{
			"naming/file-naming": {Severity: lint.Warn},
			"naming/no-stutter":  {Severity: lint.Error},
		},
	})

	cfg := &lint.Config{
		Root:       t.TempDir(),
		ModulePath: "example.com/test",
		Extends:    []string{"test-preset"},
		Rules: map[string]lint.RuleConfig{
			"naming/no-stutter": {Severity: lint.Warn}, // user override
		},
	}

	// Simulate what Check does
	report := lint.Check(cfg)
	_ = report

	// After preset merge, file-naming should come from preset (warn)
	if cfg.EffectiveSeverity("naming/file-naming", "") != lint.Warn {
		t.Error("naming/file-naming should be warn from preset")
	}
	// User override should win
	if cfg.EffectiveSeverity("naming/no-stutter", "") != lint.Warn {
		t.Error("naming/no-stutter should be warn (user override)")
	}
	// Layers should come from preset
	if len(cfg.Layers) != 2 {
		t.Errorf("layers: got %d, want 2 from preset", len(cfg.Layers))
	}
}

func TestNodeTreeImportRule(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")

	writeGoFile(t, dir, "pkg/kafka/core/core.go", "package core\n\ntype Record struct{}\n")
	writeGoFile(t, dir, "pkg/kafka/producer/producer.go", "package producer\n\ntype Publisher struct{}\n")
	// consumer → producer (allowed) + core (shared, allowed)
	writeGoFile(t, dir, "pkg/kafka/consumer/consumer.go", `package consumer

import (
	"example.com/test/pkg/kafka/producer"
	"example.com/test/pkg/kafka/core"
)

type Subscriber struct {
	_ producer.Publisher
	_ core.Record
}
`)
	// producer → consumer (reverse, violation)
	writeGoFile(t, dir, "pkg/kafka/producer/bad.go", `package producer

import "example.com/test/pkg/kafka/consumer"

var _ = consumer.Subscriber{}
`)
	// consumer → sqlrepo (cross-feature, violation)
	writeGoFile(t, dir, "pkg/kafka/consumer/cross.go", `package consumer

import "example.com/test/pkg/sqlrepo"

var _ = sqlrepo.DB{}
`)
	writeGoFile(t, dir, "pkg/sqlrepo/sqlrepo.go", "package sqlrepo\n\ntype DB struct{}\n")

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"pkg"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"},
		Children: map[string]*lint.NodeConfig{
			"sqlrepo": {},
			"kafka": {
				Children: map[string]*lint.NodeConfig{
					"core":     {Shared: true},
					"producer": {},
					"consumer": {MayImport: []string{"producer"}},
				},
			},
		},
		Rules: map[string]lint.RuleConfig{
			"dependency/import": {Severity: lint.Error},
		},
	}

	report := lint.Check(cfg)
	// Two violations: producer→consumer (reverse) and consumer→sqlrepo (cross-feature).
	// consumer→producer (may_import) and consumer→core (shared) are allowed.
	if report.ErrorCount() != 2 {
		t.Errorf("expected 2 errors, got %d:\n%s", report.ErrorCount(), report.String())
	}
}

func TestNodeTreeGlobalLayerTemplate(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")

	// One layer template, applied to two domains. A reverse-direction import
	// (model importing svc) must be caught in BOTH domains from the single
	// template definition — i.e. the layer rule is global, domain-agnostic.
	for _, d := range []string{"app", "order"} {
		writeGoFile(t, dir, "internal/"+d+"/svc/svc.go", "package svc\n\nfunc Do() {}\n")
		writeGoFile(t, dir, "internal/"+d+"/model/model.go", "package model\n\ntype T struct{}\n")
		writeGoFile(t, dir, "internal/"+d+"/model/bad.go",
			"package model\n\nimport \"example.com/test/internal/"+d+"/svc\"\n\nvar _ = svc.Do\n")
	}

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"internal"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"},
		Templates: map[string]map[string]*lint.NodeConfig{
			"layers": {
				"model": {Shared: true},
				"svc":   {MayImport: []string{"model"}},
			},
		},
		DefaultTemplate: "layers", // applied to every domain without per-domain repetition
		Children: map[string]*lint.NodeConfig{
			"app":   {},
			"order": {},
		},
		Rules: map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
	}

	report := lint.Check(cfg)
	// model -> svc reverse, once per domain = 2.
	if report.ErrorCount() != 2 {
		t.Errorf("global layer template should catch the reverse import in both domains (want 2), got %d:\n%s",
			report.ErrorCount(), report.String())
	}
}

func TestNodeTreeLeafAncestorTransparent(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")

	// A is declared but is a leaf (no children of its own). Two deep branches
	// each carry a co-located config, so x and y are auto-created intermediates.
	// Because A does not wall, x and y must NOT be walled against each other.
	writeGoFile(t, dir, "pkg/a/x/feat1/.cht-go-lint.yaml", "children:\n  thing: {}\n")
	writeGoFile(t, dir, "pkg/a/y/feat2/.cht-go-lint.yaml", "children:\n  thing: {}\n")
	writeGoFile(t, dir, "pkg/a/y/feat2/feat2.go", "package feat2\n\ntype T struct{}\n")
	writeGoFile(t, dir, "pkg/a/x/feat1/feat1.go", `package feat1

import "example.com/test/pkg/a/y/feat2"

var _ = feat2.T{}
`)

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"pkg"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"},
		Children:   map[string]*lint.NodeConfig{"a": {}}, // a is a leaf
		Rules:      map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
	}

	report := lint.Check(cfg)
	if report.ErrorCount() != 0 {
		t.Errorf("leaf ancestor must not wall its deep branches, got %d:\n%s",
			report.ErrorCount(), report.String())
	}
}

func TestNodeTreeColocated(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")

	// kafka's internal wiring lives in a co-located config, not the root.
	writeGoFile(t, dir, "pkg/kafka/.cht-go-lint.yaml", `children:
  producer: {}
  consumer:
    may_import: [producer]
`)
	writeGoFile(t, dir, "pkg/kafka/producer/producer.go", "package producer\n\ntype Publisher struct{}\n")
	// reverse — violation
	writeGoFile(t, dir, "pkg/kafka/producer/bad.go", `package producer

import "example.com/test/pkg/kafka/consumer"

func bad() any { return consumer.X }
`)
	writeGoFile(t, dir, "pkg/kafka/consumer/consumer.go", `package consumer

import "example.com/test/pkg/kafka/producer"

var X = 1
var _ = producer.Publisher{}
`)

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"pkg"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"},
		Children:   map[string]*lint.NodeConfig{"kafka": {}}, // children come from the co-located file
		Rules: map[string]lint.RuleConfig{
			"dependency/import": {Severity: lint.Error},
		},
	}

	report := lint.Check(cfg)
	// Only producer→consumer (reverse) violates; consumer→producer is allowed by
	// the co-located may_import.
	if report.ErrorCount() != 1 {
		t.Errorf("expected 1 error, got %d:\n%s", report.ErrorCount(), report.String())
	}
}

func writeGoFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	os.MkdirAll(filepath.Dir(full), 0755)
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// Control chars in a config-derived File/Message must be neutralized at the
// source so no formatter can emit an injected line (e.g. a forged workflow cmd).
func TestReportSanitizesDiagnostics(t *testing.T) {
	r := lint.NewReport()
	r.Add(lint.Violation{
		File:     "pkg/x\n::error::pwned",
		Message:  "bad\r\nvalue",
		Rule:     "ru\tle",
		Found:    "f\x1bound",
		Expected: "e\x7fxpected",
	})
	v := r.Violations()[0]
	for name, s := range map[string]string{
		"File": v.File, "Message": v.Message, "Rule": v.Rule, "Found": v.Found, "Expected": v.Expected,
	} {
		for _, c := range s {
			if unicode.IsControl(c) {
				t.Errorf("%s retains control char %q in %q", name, c, s)
			}
		}
	}
}

// An unreadable directory must surface a node-tree/config error, not be silently
// skipped (a scan failure is an enforcement gap).
func TestNodeTreeUnscannableDirReported(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("chmod-based unreadable dir does not apply to root")
	}
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "pkg/kafka/producer/producer.go", "package producer\n")
	locked := filepath.Join(dir, "pkg", "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755) // let TempDir cleanup remove it

	cfg := &lint.Config{
		Root: dir, ModulePath: "example.com/test", Roots: []string{"pkg"},
		Location: &lint.LocationConfig{Strategy: "node-tree"},
		Children: map[string]*lint.NodeConfig{"kafka": {Children: map[string]*lint.NodeConfig{"producer": {}}}},
		Rules:    map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
	}
	if !hasConfigError(lint.Check(cfg), "failed to scan") {
		t.Errorf("an unscannable directory must surface a node-tree/config error")
	}
}

// hasConfigError reports whether the report contains a node-tree/config
// diagnostic whose message contains substr.
func hasConfigError(report *lint.Report, substr string) bool {
	for _, v := range report.Violations() {
		if v.Rule == lint.ConfigRuleName && strings.Contains(v.Message, substr) {
			return true
		}
	}
	return false
}

// A broken node-tree config must fail loudly (a node-tree/config error), not
// silently drop the wall for its subtree.
func TestNodeTreeConfigErrors(t *testing.T) {
	base := func(dir string) *lint.Config {
		writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
		return &lint.Config{
			Root:       dir,
			ModulePath: "example.com/test",
			Roots:      []string{"pkg"},
			Location:   &lint.LocationConfig{Strategy: "node-tree"},
			Rules:      map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
		}
	}

	t.Run("unknown template (incl. default_template typo)", func(t *testing.T) {
		dir := t.TempDir()
		cfg := base(dir)
		cfg.Templates = map[string]map[string]*lint.NodeConfig{"layers": {"svc": {}}}
		cfg.DefaultTemplate = "layres" // typo — no such template
		cfg.Children = map[string]*lint.NodeConfig{"order": {}}
		if !hasConfigError(lint.Check(cfg), "unknown template") {
			t.Errorf("expected an unknown-template config error")
		}
	})

	t.Run("self-referential template terminates", func(t *testing.T) {
		dir := t.TempDir()
		cfg := base(dir)
		cfg.Templates = map[string]map[string]*lint.NodeConfig{"selfref": {"x": {Template: "selfref"}}}
		cfg.Children = map[string]*lint.NodeConfig{"order": {Template: "selfref"}}
		// Must return (cycle guard), not stack-overflow / hang.
		if !hasConfigError(lint.Check(cfg), "self-referential") {
			t.Errorf("expected a self-referential-template config error")
		}
	})

	t.Run("unparseable co-located config", func(t *testing.T) {
		dir := t.TempDir()
		cfg := base(dir)
		cfg.Children = map[string]*lint.NodeConfig{"kafka": {}}
		writeGoFile(t, dir, "pkg/kafka/.cht-go-lint.yaml", "children: [unterminated\n")
		writeGoFile(t, dir, "pkg/kafka/producer/producer.go", "package producer\n")
		if !hasConfigError(lint.Check(cfg), "failed to parse") {
			t.Errorf("expected a parse-failure config error for the co-located file")
		}
	})

	t.Run("reported even when dependency/import is off", func(t *testing.T) {
		dir := t.TempDir()
		cfg := base(dir)
		cfg.Rules = map[string]lint.RuleConfig{} // dependency/import not enabled
		cfg.Templates = map[string]map[string]*lint.NodeConfig{"layers": {"svc": {}}}
		cfg.DefaultTemplate = "layres" // typo — must still be reported
		cfg.Children = map[string]*lint.NodeConfig{"order": {}}
		if !hasConfigError(lint.Check(cfg), "unknown template") {
			t.Errorf("config errors must surface regardless of the dependency/import rule severity")
		}
	})
}

// The repo root is always walling: top-level features must be isolated even when
// the root config declares no inline children and everything is co-located.
func TestNodeTreeRootAlwaysWalls(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "pkg/a/.cht-go-lint.yaml", "children:\n  x: {}\n")
	writeGoFile(t, dir, "pkg/b/.cht-go-lint.yaml", "children:\n  y: {}\n")
	writeGoFile(t, dir, "pkg/b/y/y.go", "package y\n\ntype T struct{}\n")
	writeGoFile(t, dir, "pkg/a/x/x.go", `package x

import "example.com/test/pkg/b/y"

var _ = y.T{}
`)
	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"pkg"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"}, // no inline Children
		Rules:      map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
	}
	if lint.Check(cfg).ErrorCount() == 0 {
		t.Errorf("top-level a and b must be walled even with no inline root children")
	}
}

// A real sibling <x> colliding with a hoisted internal/<x> is reported end-to-end
// through Check, and honoring exclude_paths suppresses the scan.
func TestNodeTreeCollisionReported(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "pkg/kafka/codec/codec.go", "package codec\n\ntype A struct{}\n")
	writeGoFile(t, dir, "pkg/kafka/internal/codec/codec.go", "package codec\n\ntype B struct{}\n")
	writeGoFile(t, dir, "pkg/kafka/producer/producer.go", "package producer\n")
	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Roots:      []string{"pkg"},
		Location:   &lint.LocationConfig{Strategy: "node-tree"},
		Children: map[string]*lint.NodeConfig{
			"kafka": {Children: map[string]*lint.NodeConfig{"producer": {}}},
		},
		Rules: map[string]lint.RuleConfig{"dependency/import": {Severity: lint.Error}},
	}
	if !hasConfigError(lint.Check(cfg), "collides with real sibling") {
		t.Errorf("expected a hoist-collision config error")
	}

	cfg.ExcludePaths = []string{"pkg/kafka"}
	if hasConfigError(lint.Check(cfg), "collides") {
		t.Errorf("excluded path must not be scanned for collisions")
	}
}
