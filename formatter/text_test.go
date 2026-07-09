package formatter

import (
	"strings"
	"testing"

	lint "github.com/channel-io/cht-go-lint"
)

// The text formatter is the CLI default and does no escaping of its own; safety
// comes from Report.Add sanitizing control chars. Drive the real pipeline
// (Add -> Violations -> Format) and assert no injected line survives.
func TestTextNoInjectionThroughReport(t *testing.T) {
	r := lint.NewReport()
	r.Add(lint.Violation{
		Severity: lint.Error, Rule: lint.ConfigRuleName,
		File: "pkg/x\n::error::pwned", Message: "bad\r\nvalue",
	})
	out := Text{}.Format(r.Violations())
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "::") {
			t.Errorf("text output must not contain a line starting with ::, got:\n%q", out)
		}
	}
}

func TestTextFormat(t *testing.T) {
	out := Text{}.Format([]lint.Violation{{
		Severity: lint.Error, Rule: "r", File: "f.go", Line: 3, Message: "m",
	}})
	if !strings.Contains(out, "f.go:3") || !strings.Contains(out, "1 error") {
		t.Errorf("unexpected text output:\n%q", out)
	}
}
