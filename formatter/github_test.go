package formatter

import (
	"strings"
	"testing"

	lint "github.com/channel-io/cht-go-lint"
)

// A crafted file/message (e.g. from a config-derived name) must not inject a
// second workflow command — control chars are percent-encoded, so the output
// stays a single command line.
func TestGitHubEscaping(t *testing.T) {
	out := GitHub{}.Format([]lint.Violation{{
		Severity: lint.Error,
		Rule:     "node-tree/config",
		File:     "pkg/a\n::warning::x",
		Message:  "bad\nvalue",
	}})

	if c := strings.Count(out, "\n"); c != 1 {
		t.Errorf("injected newlines must be encoded — want 1 (trailing) newline, got %d:\n%q", c, out)
	}
	if !strings.Contains(out, "%0A") {
		t.Errorf("newline should be percent-encoded as %%0A:\n%q", out)
	}
	if strings.HasPrefix(out, "::error ") == false {
		t.Errorf("expected a single ::error command line:\n%q", out)
	}
}
