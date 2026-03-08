// Package rules imports all built-in rule packages for side-effect registration.
//
// Usage:
//
//	import _ "github.com/channel-io/cht-go-lint/rules"
package rules

import (
	_ "github.com/channel-io/cht-go-lint/rules/ddd"
	_ "github.com/channel-io/cht-go-lint/rules/dependency"
	_ "github.com/channel-io/cht-go-lint/rules/iface"
	_ "github.com/channel-io/cht-go-lint/rules/naming"
	_ "github.com/channel-io/cht-go-lint/rules/structure"
)
