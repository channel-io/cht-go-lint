package ddd

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

const repositoryReadModifyWriteRule = "ddd/repository-read-modify-write"

func init() {
	lint.Register(&RepositoryReadModifyWrite{})
}

// RepositoryReadModifyWrite flags read-before-full-row-write flows without an explicit guard.
type RepositoryReadModifyWrite struct{}

func (r *RepositoryReadModifyWrite) Meta() lint.Meta {
	return lint.Meta{
		Name:        repositoryReadModifyWriteRule,
		Description: "Read-modify-write flows must use row/advisory locks or an explicit concurrency guard",
		Category:    "ddd",
		Tier:        lint.TierLayerAware,
	}
}

func (r *RepositoryReadModifyWrite) Check(ctx *lint.Context) error {
	targetLayers := repositoryTargetLayers(ctx.Options)
	readPrefixes := ctx.Options.StringSlice("read_prefixes")
	if len(readPrefixes) == 0 {
		readPrefixes = []string{"Find", "Fetch", "Get", "List"}
	}
	writePrefixes := ctx.Options.StringSlice("write_prefixes")
	if len(writePrefixes) == 0 {
		writePrefixes = []string{"Update", "Save", "Upsert"}
	}
	guardNames := ctx.Options.StringSlice("guard_calls")
	if len(guardNames) == 0 {
		guardNames = []string{"XLock", "SLock", "CompareAndSwap"}
	}
	exclusions := newRepositoryMethodExclusions(ctx.Options)

	err := ctx.Analyzer.WalkGoFiles(func(_ string, file *lint.ParsedFile) error {
		if !targetLayers[file.Location.Layer] || ctx.Options.ShouldSkipFile(file.RelPath) {
			return nil
		}
		for _, fn := range file.Funcs {
			if fn.ReceiverType == "" || fn.Body == nil {
				continue
			}
			analysis := analyzeReadModifyWrite(fn.Body, readPrefixes, writePrefixes, guardNames)
			if !analysis.readBeforeWrite {
				continue
			}
			if exclusions.match(file.RelPath, fn.ReceiverType, fn.Name) {
				continue
			}
			ctx.Report.Add(lint.Violation{
				Rule:     repositoryReadModifyWriteRule,
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     fn.Pos.Line,
				Message:  fmt.Sprintf("method %q reads before a full-row write without an explicit lock or concurrency guard", fn.Name),
				Found:    normalizeReceiver(fn.ReceiverType) + "." + fn.Name,
				Expected: "FetchForUpdate/FOR UPDATE, advisory lock, or compare-and-swap",
			})
		}
		return nil
	})
	exclusions.reportUnused(ctx, repositoryReadModifyWriteRule)
	return err
}

type readModifyWriteAnalysis struct {
	readBeforeWrite bool
}

func analyzeReadModifyWrite(body *ast.BlockStmt, readPrefixes, writePrefixes, guardNames []string) readModifyWriteAnalysis {
	firstReadByReceiver := make(map[string]token.Pos)
	firstWriteByReceiver := make(map[string]token.Pos)
	firstGuardByReceiver := make(map[string]token.Pos)
	var firstGlobalGuard token.Pos
	guardSet := stringSet(guardNames)

	ast.Inspect(body, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.CallExpr:
			selector, ok := value.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			name := selector.Sel.Name
			receiver := repositoryCallReceiver(selector.X)
			if receiver == "" {
				return true
			}
			if strings.Contains(name, "ForUpdate") && firstGuardByReceiver[receiver] == token.NoPos {
				firstGuardByReceiver[receiver] = value.Pos()
			}
			if guardSet[name] && firstGlobalGuard == token.NoPos {
				firstGlobalGuard = value.Pos()
			}
			if firstReadByReceiver[receiver] == token.NoPos && hasAnyPrefix(name, readPrefixes) {
				firstReadByReceiver[receiver] = value.Pos()
			}
			if firstWriteByReceiver[receiver] == token.NoPos && hasAnyPrefix(name, writePrefixes) {
				firstWriteByReceiver[receiver] = value.Pos()
			}
		case *ast.BasicLit:
			if value.Kind == token.STRING && strings.Contains(strings.ToUpper(value.Value), "FOR UPDATE") && firstGlobalGuard == token.NoPos {
				firstGlobalGuard = value.Pos()
			}
		}
		return true
	})

	for receiver, firstRead := range firstReadByReceiver {
		firstWrite := firstWriteByReceiver[receiver]
		if firstWrite == token.NoPos || firstRead >= firstWrite {
			continue
		}
		receiverGuard := firstGuardByReceiver[receiver]
		guarded := (receiverGuard != token.NoPos && receiverGuard < firstWrite) ||
			(firstGlobalGuard != token.NoPos && firstGlobalGuard < firstWrite)
		if !guarded {
			return readModifyWriteAnalysis{readBeforeWrite: true}
		}
	}
	return readModifyWriteAnalysis{}
}

func repositoryCallReceiver(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix := repositoryCallReceiver(value.X)
		if prefix == "" {
			return ""
		}
		return prefix + "." + value.Sel.Name
	case *ast.ParenExpr:
		return repositoryCallReceiver(value.X)
	}
	return ""
}
