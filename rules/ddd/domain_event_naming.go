package ddd

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&DomainEventNaming{})
}

// DomainEventNaming ensures domain events follow naming conventions.
type DomainEventNaming struct{}

func (r *DomainEventNaming) Meta() lint.Meta {
	return lint.Meta{
		Name:        "ddd/domain-event-naming",
		Description: "Domain events should follow naming conventions",
		Category:    "ddd",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *DomainEventNaming) Check(ctx *lint.Context) error {
	eventSuffix := ctx.Options.String("event_suffix", "Event")
	requirePastTense := ctx.Options.Bool("require_past_tense", true)

	if !requirePastTense {
		return nil
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, t := range file.Types {
			if !strings.HasSuffix(t.Name, eventSuffix) {
				continue
			}

			// Extract the word before the Event suffix
			// e.g., OrderCreatedEvent -> OrderCreated -> split by uppercase -> "Created"
			baseName := strings.TrimSuffix(t.Name, eventSuffix)
			if baseName == "" {
				continue
			}

			lastWord := extractLastWord(baseName)
			if lastWord == "" {
				continue
			}

			if !isPastTense(lastWord) {
				ctx.Report.Add(lint.Violation{
					Rule:     "ddd/domain-event-naming",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     t.Pos.Line,
					Message:  fmt.Sprintf("event %q should use past tense (e.g., %sCreatedEvent)", t.Name, baseName),
					Found:    t.Name,
				})
			}
		}
		return nil
	})
}

// extractLastWord splits a PascalCase name and returns the last word.
func extractLastWord(name string) string {
	// Split PascalCase: find last uppercase start
	lastUpper := -1
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] >= 'A' && name[i] <= 'Z' {
			lastUpper = i
			break
		}
	}
	if lastUpper < 0 {
		return name
	}
	return name[lastUpper:]
}

// isPastTense is a best-effort heuristic check for English past tense.
func isPastTense(word string) bool {
	lower := strings.ToLower(word)
	pastTenseSuffixes := []string{"ed", "ted", "ied", "ged", "ked", "ned", "sed", "zed"}
	for _, suffix := range pastTenseSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	// Common irregular past tenses
	irregulars := map[string]bool{
		"sent": true, "built": true, "ran": true, "set": true,
		"done": true, "gone": true, "begun": true, "lost": true,
		"found": true, "made": true, "paid": true, "sold": true,
		"told": true, "brought": true, "bought": true, "caught": true,
		"taught": true, "thought": true, "left": true, "held": true,
		"kept": true, "felt": true, "met": true, "put": true,
		"read": true, "run": true, "shut": true, "split": true,
		"cut": true, "hit": true, "let": true, "spread": true,
	}
	return irregulars[lower]
}
