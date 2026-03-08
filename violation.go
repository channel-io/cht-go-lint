package lint

import "fmt"

// Violation represents a single rule violation.
type Violation struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Message  string   `json:"message"`
	Found    string   `json:"found,omitempty"`
	Expected string   `json:"expected,omitempty"`
}

func (v Violation) String() string {
	loc := v.File
	if v.Line > 0 {
		loc = fmt.Sprintf("%s:%d", v.File, v.Line)
	}
	s := fmt.Sprintf("[%s] %s: %s", v.Severity, loc, v.Message)
	if v.Found != "" {
		s += fmt.Sprintf(" (found: %s", v.Found)
		if v.Expected != "" {
			s += fmt.Sprintf(", expected: %s", v.Expected)
		}
		s += ")"
	}
	return s
}
