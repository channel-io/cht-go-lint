package formatter

import (
	"strings"
	"testing"

	lint "github.com/channel-io/cht-go-lint"
)

// A crafted file/message (e.g. from a config-derived name) must not inject a
// second workflow command, and property delimiters must be encoded so the
// file=...,line=... parsing stays intact.
func TestGitHubEscaping(t *testing.T) {
	tests := []struct {
		name         string
		file, msg    string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:         "newline neutralized",
			file:         "pkg/a\n::warning::x",
			msg:          "bad\nvalue",
			wantContains: []string{"%0A"},
		},
		{
			name:         "carriage return encoded",
			file:         "pkg/a",
			msg:          "line\rmore",
			wantContains: []string{"%0D"},
		},
		{
			name:         "property delimiters encoded in file",
			file:         "pkg/a,b:c",
			msg:          "m",
			wantContains: []string{"%2C", "%3A"},
		},
		{
			name:         "percent escaped first, no double-encode",
			file:         "p",
			msg:          "100% off\n",
			wantContains: []string{"100%25 off%0A"},
			wantAbsent:   []string{"%250A"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := GitHub{}.Format([]lint.Violation{{
				Severity: lint.Error, Rule: lint.ConfigRuleName, File: tt.file, Message: tt.msg,
			}})
			if c := strings.Count(out, "\n"); c != 1 { // only the trailing newline
				t.Errorf("want a single command line, got %d newlines:\n%q", c, out)
			}
			for _, w := range tt.wantContains {
				if !strings.Contains(out, w) {
					t.Errorf("want %q in output:\n%q", w, out)
				}
			}
			for _, w := range tt.wantAbsent {
				if strings.Contains(out, w) {
					t.Errorf("did not want %q in output:\n%q", w, out)
				}
			}
		})
	}
}
