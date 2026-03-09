package lint_test

import (
	"os"
	"path/filepath"
	"testing"

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
		"dependency": 11,
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

func writeGoFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	os.MkdirAll(filepath.Dir(full), 0755)
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
