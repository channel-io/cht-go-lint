package formatter

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

// GitHub outputs violations as GitHub Actions workflow commands.
type GitHub struct{}

func (GitHub) Format(violations []lint.Violation) string {
	var sb strings.Builder
	for _, v := range violations {
		level := "error"
		if v.Severity == lint.Warn {
			level = "warning"
		}
		// Workflow commands are a line-based protocol, so any control char in a
		// rule/message/path (e.g. a config-derived file name or YAML key) must be
		// escaped or a crafted value could inject a second command.
		// https://docs.github.com/actions/reference/workflow-commands-for-github-actions
		fmt.Fprintf(&sb, "::%s file=%s,line=%d::%s\n",
			level, escapeProperty(v.File), v.Line, escapeData(fmt.Sprintf("[%s] %s", v.Rule, v.Message)))
	}
	return sb.String()
}

// escapeData percent-encodes the characters GitHub requires in a command's data.
func escapeData(s string) string {
	s = strings.ReplaceAll(s, "%", "%25") // must be first
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

// escapeProperty additionally encodes the property-value delimiters.
func escapeProperty(s string) string {
	s = escapeData(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
