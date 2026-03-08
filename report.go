package lint

import (
	"fmt"
	"strings"
	"sync"
)

// Report collects violations from rule checks.
type Report struct {
	mu         sync.Mutex
	violations []Violation
}

// NewReport creates an empty report.
func NewReport() *Report {
	return &Report{}
}

// Add appends a violation to the report. Safe for concurrent use.
func (r *Report) Add(v Violation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.violations = append(r.violations, v)
}

// Violations returns all collected violations.
func (r *Report) Violations() []Violation {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Violation, len(r.violations))
	copy(result, r.violations)
	return result
}

// Errors returns only error-severity violations.
func (r *Report) Errors() []Violation {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []Violation
	for _, v := range r.violations {
		if v.Severity == Error {
			result = append(result, v)
		}
	}
	return result
}

// Warnings returns only warning-severity violations.
func (r *Report) Warnings() []Violation {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []Violation
	for _, v := range r.violations {
		if v.Severity == Warn {
			result = append(result, v)
		}
	}
	return result
}

// HasErrors returns true if any error-severity violations exist.
func (r *Report) HasErrors() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.violations {
		if v.Severity == Error {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-severity violations.
func (r *Report) ErrorCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, v := range r.violations {
		if v.Severity == Error {
			n++
		}
	}
	return n
}

// WarningCount returns the number of warning-severity violations.
func (r *Report) WarningCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, v := range r.violations {
		if v.Severity == Warn {
			n++
		}
	}
	return n
}

// Total returns the total number of violations.
func (r *Report) Total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.violations)
}

// String returns a human-readable summary of all violations.
func (r *Report) String() string {
	violations := r.Violations()
	if len(violations) == 0 {
		return "no violations found"
	}
	var sb strings.Builder
	for _, v := range violations {
		fmt.Fprintln(&sb, v.String())
	}
	fmt.Fprintf(&sb, "\n%d error(s), %d warning(s)\n", r.ErrorCount(), r.WarningCount())
	return sb.String()
}

// ByRule groups violations by rule name.
func (r *Report) ByRule() map[string][]Violation {
	violations := r.Violations()
	result := make(map[string][]Violation)
	for _, v := range violations {
		result[v.Rule] = append(result[v.Rule], v)
	}
	return result
}
