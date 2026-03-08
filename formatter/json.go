package formatter

import (
	"encoding/json"

	lint "github.com/channel-io/cht-go-lint"
)

// JSON outputs violations in JSON format.
type JSON struct {
	Pretty bool
}

func (f JSON) Format(violations []lint.Violation) string {
	var data []byte
	if f.Pretty {
		data, _ = json.MarshalIndent(violations, "", "  ")
	} else {
		data, _ = json.Marshal(violations)
	}
	return string(data) + "\n"
}
