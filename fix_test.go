package lint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lint "github.com/channel-io/cht-go-lint"
	_ "github.com/channel-io/cht-go-lint/fixers"
	_ "github.com/channel-io/cht-go-lint/rules"
)

func TestFixDeclarationOrder(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "bad.go", `package test

func Hello() {}

const Name = "test"

var X = 1

type MyStruct struct{}
`)

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"structure/declaration-order": {Severity: lint.Error},
		},
	}

	// First check without fix — should have violations
	report := lint.Check(cfg)
	if report.ErrorCount() == 0 {
		t.Fatal("expected violations before fix")
	}

	// Fix and re-check
	report = lint.CheckWithFix(cfg, true, false)
	if report.ErrorCount() != 0 {
		t.Errorf("expected 0 violations after fix, got %d:\n%s", report.ErrorCount(), report.String())
	}

	fixResults := report.FixResults()
	if len(fixResults) != 1 {
		t.Errorf("expected 1 fix result, got %d", len(fixResults))
	}

	// Verify file content is reordered
	content, _ := os.ReadFile(filepath.Join(dir, "bad.go"))
	src := string(content)
	constIdx := strings.Index(src, "const Name")
	varIdx := strings.Index(src, "var X")
	structIdx := strings.Index(src, "type MyStruct")
	funcIdx := strings.Index(src, "func Hello")

	if constIdx > varIdx || varIdx > structIdx || structIdx > funcIdx {
		t.Errorf("declarations not in expected order:\n%s", src)
	}
}

func TestFixDryRun(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "bad.go", `package test

func Hello() {}

const Name = "test"
`)

	original, _ := os.ReadFile(filepath.Join(dir, "bad.go"))

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"structure/declaration-order": {Severity: lint.Error},
		},
	}

	report := lint.CheckWithFix(cfg, true, true)

	// Should report fix results
	if len(report.FixResults()) == 0 {
		t.Error("expected fix results in dry-run")
	}

	// File should NOT be modified
	after, _ := os.ReadFile(filepath.Join(dir, "bad.go"))
	if string(after) != string(original) {
		t.Error("dry-run should not modify files")
	}

	// Should still report violations (file wasn't fixed)
	if report.ErrorCount() == 0 {
		t.Error("expected violations in dry-run (file not modified)")
	}
}

func TestFixPreservesComments(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "commented.go", `package test

// Hello says hello.
func Hello() {}

// Name is the name.
const Name = "test"
`)

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"structure/declaration-order": {Severity: lint.Error},
		},
	}

	report := lint.CheckWithFix(cfg, true, false)
	if report.ErrorCount() != 0 {
		t.Errorf("expected 0 violations after fix, got %d:\n%s", report.ErrorCount(), report.String())
	}

	content, _ := os.ReadFile(filepath.Join(dir, "commented.go"))
	src := string(content)

	// Comment should be associated with const, not orphaned
	if !strings.Contains(src, "// Name is the name.\nconst Name") {
		t.Errorf("comment not preserved with const declaration:\n%s", src)
	}
	if !strings.Contains(src, "// Hello says hello.\nfunc Hello") {
		t.Errorf("comment not preserved with func declaration:\n%s", src)
	}
}

func TestFixNoChangeOnCorrectOrder(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "good.go", `package test

const Name = "test"

var X = 1

type MyStruct struct{}

func Hello() {}
`)

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"structure/declaration-order": {Severity: lint.Error},
		},
	}

	report := lint.CheckWithFix(cfg, true, false)
	if len(report.FixResults()) != 0 {
		t.Errorf("expected 0 fix results for correct file, got %d", len(report.FixResults()))
	}
	if report.ErrorCount() != 0 {
		t.Errorf("expected 0 violations for correct file, got %d", report.ErrorCount())
	}
}

func TestFixImportsStayAtTop(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	writeGoFile(t, dir, "imports.go", `package test

import "fmt"

func Hello() { fmt.Println("hi") }

const Name = "test"
`)

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Rules: map[string]lint.RuleConfig{
			"structure/declaration-order": {Severity: lint.Error},
		},
	}

	report := lint.CheckWithFix(cfg, true, false)
	if report.ErrorCount() != 0 {
		t.Errorf("expected 0 violations after fix, got %d:\n%s", report.ErrorCount(), report.String())
	}

	content, _ := os.ReadFile(filepath.Join(dir, "imports.go"))
	src := string(content)

	importIdx := strings.Index(src, "import")
	constIdx := strings.Index(src, "const Name")
	funcIdx := strings.Index(src, "func Hello")

	if importIdx > constIdx || importIdx > funcIdx {
		t.Errorf("import should stay at top:\n%s", src)
	}
	if constIdx > funcIdx {
		t.Errorf("const should appear before func:\n%s", src)
	}
}

func TestFixWithLayerOverrides(t *testing.T) {
	dir := t.TempDir()
	writeGoFile(t, dir, "go.mod", "module example.com/test\n\ngo 1.22\n")
	// File where func comes before struct — violation in default order but valid in overridden order
	writeGoFile(t, dir, "internal/model/entity.go", `package model

func NewUser() *User { return &User{} }

type User struct{}
`)

	cfg := &lint.Config{
		Root:       dir,
		ModulePath: "example.com/test",
		Layers: []lint.LayerConfig{
			{Name: "model", MayImport: []string{}},
		},
		Location: &lint.LocationConfig{
			Strategy: "flat-pkg",
			Options:  map[string]any{"roots": []any{"internal"}},
		},
		Rules: map[string]lint.RuleConfig{
			"structure/declaration-order": {
				Severity: lint.Error,
				Options: map[string]any{
					"layer_overrides": map[string]any{
						"model": []any{"func", "struct"},
					},
				},
			},
		},
	}

	report := lint.CheckWithFix(cfg, true, false)

	// With layer override [func, struct], the file is already correct
	if len(report.FixResults()) != 0 {
		t.Errorf("expected 0 fix results (file matches layer override order), got %d", len(report.FixResults()))
	}
	if report.ErrorCount() != 0 {
		t.Errorf("expected 0 violations, got %d:\n%s", report.ErrorCount(), report.String())
	}
}
