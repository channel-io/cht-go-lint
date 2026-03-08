package lint

import (
	"encoding/json"
	"fmt"
)

// Severity controls how a rule violation is treated.
type Severity int

const (
	Off  Severity = iota // rule disabled
	Warn                 // report but don't fail
	Error                // fail the check
)

func (s Severity) String() string {
	switch s {
	case Off:
		return "off"
	case Warn:
		return "warn"
	case Error:
		return "error"
	default:
		return "unknown"
	}
}

// ParseSeverity parses a string into a Severity value.
func ParseSeverity(s string) Severity {
	switch s {
	case "off", "0":
		return Off
	case "warn", "warning", "1":
		return Warn
	case "error", "err", "2":
		return Error
	default:
		return Off
	}
}

func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Severity) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = ParseSeverity(str)
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("invalid severity: %s", string(data))
	}
	*s = Severity(n)
	return nil
}
