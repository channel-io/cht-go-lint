package ddd

import (
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"strings"
	"unicode"

	lint "github.com/channel-io/cht-go-lint"
)

const repositoryErrorContractRule = "ddd/repository-error-contract"

func init() {
	lint.Register(&RepositoryErrorContract{})
}

// RepositoryErrorContract enforces shared error classification and wrapping at repository boundaries.
type RepositoryErrorContract struct{}

func (r *RepositoryErrorContract) Meta() lint.Meta {
	return lint.Meta{
		Name:        repositoryErrorContractRule,
		Description: "Repository errors use sqlrepo/apierr classification and go-lib wrapping",
		Category:    "ddd",
		Tier:        lint.TierLayerAware,
	}
}

func (r *RepositoryErrorContract) Check(ctx *lint.Context) error {
	targetLayers := repositoryTargetLayers(ctx.Options)
	forbiddenImports := ctx.Options.StringSlice("forbidden_imports")
	if len(forbiddenImports) == 0 {
		forbiddenImports = []string{"github.com/pkg/errors", "github.com/friendsofgo/errors"}
	}
	forbiddenImportSet := stringSet(forbiddenImports)
	exclusions := newRepositoryMethodExclusions(ctx.Options)
	importExclusions := newRepositoryImportExclusions(ctx.Options)

	err := ctx.Analyzer.WalkGoFiles(func(_ string, file *lint.ParsedFile) error {
		if !targetLayers[file.Location.Layer] || ctx.Options.ShouldSkipFile(file.RelPath) {
			return nil
		}

		for _, imp := range file.Imports {
			if !forbiddenImportSet[imp.Path] {
				continue
			}
			if importExclusions.match(file.RelPath, imp.Path) {
				continue
			}
			ctx.Report.Add(lint.Violation{
				Rule:     repositoryErrorContractRule,
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     imp.Pos.Line,
				Message:  fmt.Sprintf("repository imports legacy error package %q", imp.Path),
				Found:    imp.Path,
				Expected: "github.com/channel-io/go-lib/pkg/errors",
			})
		}

		for _, decl := range file.AST.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					if !name.IsExported() || !isRepositorySentinelName(name.Name) {
						continue
					}
					if exclusions.match(file.RelPath, "", name.Name) {
						continue
					}
					ctx.Report.Add(lint.Violation{
						Rule:     repositoryErrorContractRule,
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     file.FileSet.Position(name.Pos()).Line,
						Message:  fmt.Sprintf("repository declares custom sentinel error %q", name.Name),
						Found:    name.Name,
						Expected: "use sqlrepo/apierr classification and apierr.Is helpers",
					})
				}
			}
		}

		errorQualifiers, apiErrorQualifiers := repositoryErrorQualifiers(file.Imports)
		for _, fn := range file.Funcs {
			if fn.Body == nil {
				continue
			}
			unwrapped := containsUnwrappedErrorReturn(fn.Body)
			unclassified := containsUnclassifiedErrorReturn(fn.Body, errorQualifiers, apiErrorQualifiers)
			if !unwrapped && !unclassified {
				continue
			}
			if exclusions.match(file.RelPath, fn.ReceiverType, fn.Name) {
				continue
			}
			symbol := fn.Name
			if fn.ReceiverType != "" {
				symbol = normalizeReceiver(fn.ReceiverType) + "." + fn.Name
			}
			message := fmt.Sprintf("repository function %q returns an error variable without wrapping", fn.Name)
			expected := "wrap the cause with go-lib errors.Wrap/Wrapf before apierr classification"
			if !unwrapped && unclassified {
				message = fmt.Sprintf("repository function %q returns an unclassified constructed error", fn.Name)
				expected = "classify repository boundary errors with sqlrepo/apierr"
			}
			ctx.Report.Add(lint.Violation{
				Rule:     repositoryErrorContractRule,
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     fn.Pos.Line,
				Message:  message,
				Found:    symbol,
				Expected: expected,
			})
		}
		return nil
	})
	exclusions.reportUnused(ctx, repositoryErrorContractRule)
	importExclusions.reportUnused(ctx, repositoryErrorContractRule)
	return err
}

func containsUnwrappedErrorReturn(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		ret, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, result := range ret.Results {
			if containsUnwrappedErrorIdentifier(result) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func containsUnwrappedErrorIdentifier(expr ast.Expr) bool {
	switch value := expr.(type) {
	case *ast.Ident:
		name := strings.ToLower(value.Name)
		return name == "err" || strings.HasSuffix(name, "err") || strings.HasSuffix(name, "error")
	case *ast.CallExpr:
		if selector, ok := value.Fun.(*ast.SelectorExpr); ok {
			switch selector.Sel.Name {
			case "Wrap", "Wrapf", "WithStack":
				return false
			}
		}
		for _, argument := range value.Args {
			if containsUnwrappedErrorIdentifier(argument) {
				return true
			}
		}
	case *ast.ParenExpr:
		return containsUnwrappedErrorIdentifier(value.X)
	}
	return false
}

func containsUnclassifiedErrorReturn(body *ast.BlockStmt, errorQualifiers, apiErrorQualifiers map[string]bool) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		if found {
			return false
		}
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		ret, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for _, result := range ret.Results {
			if isUnclassifiedConstructedError(result, errorQualifiers, apiErrorQualifiers) {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func isUnclassifiedConstructedError(expr ast.Expr, errorQualifiers, apiErrorQualifiers map[string]bool) bool {
	switch value := expr.(type) {
	case *ast.CallExpr:
		selector, ok := value.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		qualifier, ok := selector.X.(*ast.Ident)
		if !ok || apiErrorQualifiers[qualifier.Name] {
			return false
		}
		return errorQualifiers[qualifier.Name] && (selector.Sel.Name == "New" || selector.Sel.Name == "Errorf")
	case *ast.ParenExpr:
		return isUnclassifiedConstructedError(value.X, errorQualifiers, apiErrorQualifiers)
	}
	return false
}

func repositoryErrorQualifiers(imports []lint.Import) (map[string]bool, map[string]bool) {
	errorQualifiers := make(map[string]bool)
	apiErrorQualifiers := make(map[string]bool)
	for _, imp := range imports {
		qualifier := imp.Alias
		if qualifier == "" {
			qualifier = path.Base(imp.Path)
		}
		if qualifier == "." || qualifier == "_" {
			continue
		}
		if imp.Path == "errors" || imp.Path == "fmt" || strings.HasSuffix(imp.Path, "/errors") {
			errorQualifiers[qualifier] = true
		}
		if strings.HasSuffix(imp.Path, "/apierr") {
			apiErrorQualifiers[qualifier] = true
		}
	}
	return errorQualifiers, apiErrorQualifiers
}

func isRepositorySentinelName(name string) bool {
	if name == "Err" {
		return true
	}
	if !strings.HasPrefix(name, "Err") || len(name) == len("Err") {
		return false
	}
	for _, next := range name[len("Err"):] {
		return unicode.IsUpper(next)
	}
	return false
}
