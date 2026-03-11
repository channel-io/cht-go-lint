package lint_test

import (
	"testing"

	lint "github.com/channel-io/cht-go-lint"
)

func TestRunGoLintDisabled(t *testing.T) {
	rpt := lint.NewReport()

	// nil GoLint config — should be a no-op
	cfg := &lint.Config{Root: t.TempDir()}
	if err := lint.RunGoLint(cfg, rpt, false); err != nil {
		t.Errorf("expected nil error for nil GoLint, got: %v", err)
	}
	if rpt.Total() != 0 {
		t.Errorf("expected 0 violations, got %d", rpt.Total())
	}

	// Disabled GoLint config — should be a no-op
	cfg.GoLint = &lint.GoLintConfig{Enabled: false}
	if err := lint.RunGoLint(cfg, rpt, false); err != nil {
		t.Errorf("expected nil error for disabled GoLint, got: %v", err)
	}
	if rpt.Total() != 0 {
		t.Errorf("expected 0 violations, got %d", rpt.Total())
	}
}

func TestRunGoLintDisabledWithFix(t *testing.T) {
	rpt := lint.NewReport()

	// fix=true should still be a no-op when GoLint is disabled
	cfg := &lint.Config{Root: t.TempDir()}
	if err := lint.RunGoLint(cfg, rpt, true); err != nil {
		t.Errorf("expected nil error for nil GoLint with fix, got: %v", err)
	}
	if rpt.Total() != 0 {
		t.Errorf("expected 0 violations, got %d", rpt.Total())
	}
}

func TestRunGoLintNotInstalled(t *testing.T) {
	rpt := lint.NewReport()

	cfg := &lint.Config{
		Root:   t.TempDir(),
		GoLint: &lint.GoLintConfig{Enabled: true},
	}

	// Set PATH to empty to ensure golangci-lint is not found
	t.Setenv("PATH", "")

	err := lint.RunGoLint(cfg, rpt, false)
	if err == nil {
		t.Error("expected error when golangci-lint is not in PATH")
	}
}
