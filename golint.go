package lint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
)

// golangciLintOutput represents the JSON output from golangci-lint.
type golangciLintOutput struct {
	Issues []golangciLintIssue `json:"Issues"`
}

type golangciLintIssue struct {
	FromLinter string             `json:"FromLinter"`
	Text       string             `json:"Text"`
	Pos        golangciLintPos    `json:"Pos"`
}

type golangciLintPos struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
}

// RunGoLint executes golangci-lint as a subprocess and adds violations to the report.
func RunGoLint(cfg *Config, rpt *Report) error {
	if cfg.GoLint == nil || !cfg.GoLint.Enabled {
		return nil
	}

	bin, err := exec.LookPath("golangci-lint")
	if err != nil {
		return fmt.Errorf("golangci-lint not found in PATH: %w", err)
	}

	args := []string{
		"run",
		"--output.json.path", "stdout",
		"--issues-exit-code", "0",
		"--max-issues-per-linter", "0",
		"--max-same-issues", "0",
	}

	if cfg.GoLint.Config != "" {
		args = append(args, "-c", cfg.GoLint.Config)
	}

	args = append(args, cfg.GoLint.Args...)
	args = append(args, "./...")

	cmd := exec.Command(bin, args...)
	cmd.Dir = cfg.Root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return fmt.Errorf("golangci-lint execution failed: %w\n%s", err, stderr.String())
		}
		// Exit code != 0 but not a crash — might be config issue
		// If stdout is empty, treat as error
		if stdout.Len() == 0 {
			return fmt.Errorf("golangci-lint failed: %s", stderr.String())
		}
	}

	if stdout.Len() == 0 {
		return nil
	}

	var output golangciLintOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
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
