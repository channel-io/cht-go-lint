package formatter

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

// Text outputs violations in a human-readable format.
type Text struct{}

func (Text) Format(violations []lint.Violation) string {
	if len(violations) == 0 {
		return "No violations found.\n"
	}

	var sb strings.Builder
	errors, warnings := 0, 0
	for _, v := range violations {
		loc := v.File
		if v.Line > 0 {
			loc = fmt.Sprintf("%s:%d", v.File, v.Line)
		}

		severity := "error"
		if v.Severity == lint.Warn {
			severity = "warning"
			warnings++
		} else {
			errors++
		}

		fmt.Fprintf(&sb, "  %s  %s  %s", loc, severity, v.Message)
		if v.Rule != "" {
			fmt.Fprintf(&sb, "  (%s)", v.Rule)
		}
		fmt.Fprintln(&sb)
	}

	fmt.Fprintf(&sb, "\n%d error(s), %d warning(s)\n", errors, warnings)
	return sb.String()
}
