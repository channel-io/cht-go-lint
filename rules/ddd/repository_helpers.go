package ddd

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

type repositoryMethodExclusion struct {
	path     string
	receiver string
	method   string
	reason   string
	used     bool
}

type repositoryMethodExclusions struct {
	items        []*repositoryMethodExclusion
	rejectUnused bool
}

func newRepositoryMethodExclusions(options lint.Options) *repositoryMethodExclusions {
	exclusions := &repositoryMethodExclusions{
		rejectUnused: options.Bool("reject_unused_excludes", false),
	}
	for _, raw := range options.MapSlice("exclude_methods") {
		path := stringOption(raw, "path")
		reason := stringOption(raw, "reason")
		symbols := stringSliceOption(raw, "symbols")
		if len(symbols) == 0 {
			exclusions.items = append(exclusions.items, &repositoryMethodExclusion{
				path:     path,
				receiver: normalizeReceiver(stringOption(raw, "receiver")),
				method:   stringOption(raw, "method"),
				reason:   reason,
			})
			continue
		}
		for _, symbol := range symbols {
			receiver, method := splitRepositorySymbol(symbol)
			exclusions.items = append(exclusions.items, &repositoryMethodExclusion{
				path:     path,
				receiver: receiver,
				method:   method,
				reason:   reason,
			})
		}
	}
	return exclusions
}

func (e *repositoryMethodExclusions) match(path, receiver, method string) bool {
	receiver = normalizeReceiver(receiver)
	for _, item := range e.items {
		if item.path == "" || item.method == "" {
			continue
		}
		if !repositoryPathMatches(path, item.path) {
			continue
		}
		if item.receiver != "" && item.receiver != receiver {
			continue
		}
		if item.method != "" && item.method != method {
			continue
		}
		item.used = true
		return true
	}
	return false
}

type repositoryImportExclusion struct {
	path       string
	importPath string
	reason     string
	used       bool
}

type repositoryImportExclusions struct {
	items        []*repositoryImportExclusion
	rejectUnused bool
}

func newRepositoryImportExclusions(options lint.Options) *repositoryImportExclusions {
	exclusions := &repositoryImportExclusions{
		rejectUnused: options.Bool("reject_unused_excludes", false),
	}
	for _, raw := range options.MapSlice("exclude_imports") {
		path := stringOption(raw, "path")
		reason := stringOption(raw, "reason")
		imports := stringSliceOption(raw, "imports")
		if len(imports) == 0 {
			imports = []string{stringOption(raw, "import")}
		}
		for _, importPath := range imports {
			exclusions.items = append(exclusions.items, &repositoryImportExclusion{
				path:       path,
				importPath: importPath,
				reason:     reason,
			})
		}
	}
	return exclusions
}

func (e *repositoryImportExclusions) match(path, importPath string) bool {
	for _, item := range e.items {
		if item.path == "" || item.importPath == "" {
			continue
		}
		if !repositoryPathMatches(path, item.path) || item.importPath != importPath {
			continue
		}
		item.used = true
		return true
	}
	return false
}

func (e *repositoryImportExclusions) reportUnused(ctx *lint.Context, rule string) {
	if !e.rejectUnused {
		return
	}
	for _, item := range e.items {
		if item.used {
			continue
		}
		message := fmt.Sprintf("unused repository import exclusion for %q", item.importPath)
		if item.reason != "" {
			message += ": " + item.reason
		}
		ctx.Report.Add(lint.Violation{
			Rule:     rule,
			Severity: ctx.Severity,
			File:     item.path,
			Line:     1,
			Message:  message,
			Found:    item.importPath,
			Expected: "remove the stale exclude_imports entry",
		})
	}
}

func (e *repositoryMethodExclusions) reportUnused(ctx *lint.Context, rule string) {
	if !e.rejectUnused {
		return
	}
	for _, item := range e.items {
		if item.used {
			continue
		}
		symbol := item.method
		if item.receiver != "" {
			symbol = item.receiver + "." + item.method
		}
		message := fmt.Sprintf("unused repository lint exclusion for %q", symbol)
		if item.reason != "" {
			message += ": " + item.reason
		}
		ctx.Report.Add(lint.Violation{
			Rule:     rule,
			Severity: ctx.Severity,
			File:     item.path,
			Line:     1,
			Message:  message,
			Found:    symbol,
			Expected: "remove the stale exclude_methods entry",
		})
	}
}

func repositoryPathMatches(actual, configured string) bool {
	actual = filepath.ToSlash(actual)
	configured = filepath.ToSlash(strings.TrimPrefix(configured, "./"))
	if configured == "" {
		return true
	}
	if actual == configured || strings.HasSuffix(actual, "/"+configured) {
		return true
	}
	if strings.ContainsAny(configured, "*?[") {
		matched, _ := filepath.Match(configured, actual)
		return matched
	}
	return false
}

func normalizeReceiver(receiver string) string {
	return strings.TrimPrefix(receiver, "*")
}

func splitRepositorySymbol(symbol string) (string, string) {
	receiver, method, found := strings.Cut(symbol, ".")
	if !found {
		return "", symbol
	}
	return normalizeReceiver(receiver), method
}

func stringOption(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func stringSliceOption(values map[string]any, key string) []string {
	value, ok := values[key]
	if !ok {
		return nil
	}
	switch items := value.(type) {
	case []string:
		return items
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if stringValue, ok := item.(string); ok {
				result = append(result, stringValue)
			}
		}
		return result
	default:
		return nil
	}
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func repositoryTargetLayers(options lint.Options) map[string]bool {
	layers := options.StringSlice("target_layers")
	if len(layers) == 0 {
		layers = []string{"repo"}
	}
	return stringSet(layers)
}
