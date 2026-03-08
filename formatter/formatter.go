// Package formatter provides output formatters for lint reports.
package formatter

import lint "github.com/channel-io/cht-go-lint"

// Formatter formats a lint report into a specific output format.
type Formatter interface {
	Format(violations []lint.Violation) string
}
