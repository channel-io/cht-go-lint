package ddd

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&BoundedContextIsolation{})
}

// BoundedContextIsolation ensures bounded contexts do not directly import each other.
type BoundedContextIsolation struct{}

func (r *BoundedContextIsolation) Meta() lint.Meta {
	return lint.Meta{
		Name:        "ddd/bounded-context-isolation",
		Description: "Bounded contexts should not directly import each other",
		Category:    "ddd",
		Tier:        lint.TierDomainSpecific,
	}
}

func (r *BoundedContextIsolation) Check(ctx *lint.Context) error {
	contexts := ctx.Options.MapSlice("contexts")
	if len(contexts) == 0 {
		return nil
	}

	allowedComm := ctx.Options.StringSlice("allowed_communication")
	if len(allowedComm) == 0 {
		allowedComm = []string{"event", "interface"}
	}
	allowedSet := make(map[string]bool, len(allowedComm))
	for _, a := range allowedComm {
		allowedSet[a] = true
	}

	// Parse context definitions
	type contextDef struct {
		name string
		path string
	}
	var ctxDefs []contextDef
	for _, c := range contexts {
		name, _ := c["name"].(string)
		path, _ := c["path"].(string)
		if name == "" || path == "" {
			continue
		}
		ctxDefs = append(ctxDefs, contextDef{name: name, path: path})
	}

	if len(ctxDefs) < 2 {
		return nil
	}

	modulePath := ctx.Analyzer.ModulePath()

	// For each context, walk its files and check imports
	for _, current := range ctxDefs {
		cur := current // capture for closure
		err := ctx.Analyzer.WalkDir(cur.path, func(path string, file *lint.ParsedFile) error {
			for _, imp := range file.Imports {
				for _, other := range ctxDefs {
					if other.name == cur.name {
						continue
					}
					otherImportPrefix := modulePath + "/" + other.path
					if !strings.HasPrefix(imp.Path, otherImportPrefix) {
						continue
					}

					// Check if import goes through an allowed communication layer
					afterPrefix := strings.TrimPrefix(imp.Path, otherImportPrefix)
					afterPrefix = strings.TrimPrefix(afterPrefix, "/")
					isAllowed := false
					for allowed := range allowedSet {
						if strings.HasPrefix(afterPrefix, allowed+"/") || afterPrefix == allowed {
							isAllowed = true
							break
						}
					}

					if !isAllowed {
						ctx.Report.Add(lint.Violation{
							Rule:     "ddd/bounded-context-isolation",
							Severity: ctx.Severity,
							File:     file.RelPath,
							Line:     imp.Pos.Line,
							Message:  fmt.Sprintf("bounded context %q must not import from context %q", cur.name, other.name),
							Found:    imp.Path,
						})
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
