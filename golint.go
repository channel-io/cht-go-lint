package lint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// golangciLintOutput represents the JSON output from golangci-lint.
type golangciLintOutput struct {
	Issues []golangciLintIssue `json:"Issues"`
}

type golangciLintIssue struct {
	FromLinter string          `json:"FromLinter"`
	Text       string          `json:"Text"`
	Pos        golangciLintPos `json:"Pos"`
}

type golangciLintPos struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
}

// RunGoLint executes golangci-lint as a subprocess and adds violations to the report.
// When fix is true, golangci-lint is invoked with --fix so that auto-fixable issues
// (e.g. goimports, gofmt) are corrected in-place; remaining violations are still reported.
func RunGoLint(cfg *Config, rpt *Report, fix bool) error {
	if cfg.GoLint == nil || !cfg.GoLint.Enabled {
		return nil
	}

	bin, err := exec.LookPath("golangci-lint")
	if err != nil {
		return fmt.Errorf("golangci-lint not found in PATH: %w", err)
	}

	// Write JSON to a temp file to avoid mixing with text output on stdout
	tmpFile, err := os.CreateTemp("", "cht-go-lint-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	args := []string{
		"run",
		"--output.json.path", tmpPath,
		"--issues-exit-code", "0",
		"--max-issues-per-linter", "0",
		"--max-same-issues", "0",
	}

	if fix {
		args = append(args, "--fix")
	}

	if cfg.GoLint.Config != "" {
		args = append(args, "-c", cfg.GoLint.Config)
	}

	args = append(args, cfg.GoLint.Args...)
	args = append(args, "./...")

	cmd := exec.Command(bin, args...)
	cmd.Dir = cfg.Root

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return fmt.Errorf("golangci-lint execution failed: %w\n%s", err, stderr.String())
		}
		// ExitError with non-zero exit — check if JSON was written
		if info, statErr := os.Stat(tmpPath); statErr != nil || info.Size() == 0 {
			return fmt.Errorf("golangci-lint failed: %s", stderr.String())
		}
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil || len(data) == 0 {
		return nil
	}

	var output golangciLintOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return fmt.Errorf("failed to parse golangci-lint JSON output: %w", err)
	}

	for _, issue := range output.Issues {
		relPath, err := filepath.Rel(cfg.Root, filepath.Join(cfg.Root, issue.Pos.Filename))
		if err != nil {
			relPath = issue.Pos.Filename
		}
		relPath = filepath.ToSlash(relPath)

		rpt.Add(Violation{
			Rule:     "go/" + issue.FromLinter,
			Severity: Error,
			File:     relPath,
			Line:     issue.Pos.Line,
			Message:  issue.Text,
		})
	}

	return nil
}
