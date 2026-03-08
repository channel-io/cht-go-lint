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
		// ::error file={name},line={line}::{message}
		fmt.Fprintf(&sb, "::%s file=%s,line=%d::[%s] %s\n",
			level, v.File, v.Line, v.Rule, v.Message)
	}
	return sb.String()
}
