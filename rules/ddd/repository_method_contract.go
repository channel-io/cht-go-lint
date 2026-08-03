package ddd

import (
	"fmt"
	"go/ast"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

const repositoryMethodContractRule = "ddd/repository-method-contract"

func init() {
	lint.Register(&RepositoryMethodContract{})
}

// RepositoryMethodContract enforces repository method vocabulary and Find/Fetch semantics.
type RepositoryMethodContract struct{}

func (r *RepositoryMethodContract) Meta() lint.Meta {
	return lint.Meta{
		Name:        repositoryMethodContractRule,
		Description: "Repository reads use Find/Fetch/FindAll semantics and domain mutations stay out of repositories",
		Category:    "ddd",
		Tier:        lint.TierLayerAware,
	}
}

func (r *RepositoryMethodContract) Check(ctx *lint.Context) error {
	targetLayers := repositoryTargetLayers(ctx.Options)
	forbiddenPrefixes := ctx.Options.StringSlice("forbidden_prefixes")
	if len(forbiddenPrefixes) == 0 {
		forbiddenPrefixes = []string{"Get", "List"}
	}
	discouragedMutationPrefixes := ctx.Options.StringSlice("discouraged_mutation_prefixes")
	allowedMutationMethods := ctx.Options.StringSlice("allowed_mutation_methods")
	if len(allowedMutationMethods) == 0 {
		allowedMutationMethods = []string{"Create", "Update", "Delete"}
	}
	allowedMutationMethodSet := stringSet(allowedMutationMethods)
	exclusions := newRepositoryMethodExclusions(ctx.Options)

	err := ctx.Analyzer.WalkGoFiles(func(_ string, file *lint.ParsedFile) error {
		if !targetLayers[file.Location.Layer] || ctx.Options.ShouldSkipFile(file.RelPath) {
			return nil
		}

		for _, decl := range file.AST.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				iface, ok := typeSpec.Type.(*ast.InterfaceType)
				if !ok || iface.Methods == nil {
					continue
				}
				for _, field := range iface.Methods.List {
					for _, name := range field.Names {
						method := name.Name
						forbiddenRead := hasAnyPrefix(method, forbiddenPrefixes)
						discouragedMutation := hasAnyPrefix(method, discouragedMutationPrefixes) && !allowedMutationMethodSet[method]
						if !forbiddenRead && !discouragedMutation {
							continue
						}
						if exclusions.match(file.RelPath, typeSpec.Name.Name, method) {
							continue
						}
						ctx.Report.Add(lint.Violation{
							Rule:     repositoryMethodContractRule,
							Severity: ctx.Severity,
							File:     file.RelPath,
							Line:     file.FileSet.Position(name.Pos()).Line,
							Message:  repositoryMethodMessage(method, forbiddenPrefixes),
							Found:    typeSpec.Name.Name + "." + method,
							Expected: "Find/Fetch/FindAll reads or Create/Update/Delete persistence operations",
						})
					}
				}
			}
		}

		for _, fn := range file.Funcs {
			if fn.ReceiverType == "" || !strings.HasPrefix(fn.Name, "Find") || fn.Body == nil {
				continue
			}
			delegated, ok := directDelegatedMethod(fn.Body)
			if !ok || (delegated != "Fetch" && delegated != "FetchBy") {
				continue
			}
			if exclusions.match(file.RelPath, fn.ReceiverType, fn.Name) {
				continue
			}
			ctx.Report.Add(lint.Violation{
				Rule:     repositoryMethodContractRule,
				Severity: ctx.Severity,
				File:     file.RelPath,
				Line:     fn.Pos.Line,
				Message:  fmt.Sprintf("Find method %q directly delegates to %s and therefore returns not-found errors", fn.Name, delegated),
				Found:    normalizeReceiver(fn.ReceiverType) + "." + fn.Name,
				Expected: "delegate to Find/FindBy, or rename the method to Fetch/FetchBy",
			})
		}
		return nil
	})
	exclusions.reportUnused(ctx, repositoryMethodContractRule)
	return err
}

func repositoryMethodMessage(method string, forbiddenPrefixes []string) string {
	if hasAnyPrefix(method, forbiddenPrefixes) {
		return fmt.Sprintf("repository method %q uses a forbidden read prefix", method)
	}
	return fmt.Sprintf("repository method %q encodes a specialized mutation; keep the decision in the service and use Update", method)
}

func directDelegatedMethod(body *ast.BlockStmt) (string, bool) {
	if len(body.List) != 1 {
		return "", false
	}
	ret, ok := body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return "", false
	}
	call, ok := ret.Results[0].(*ast.CallExpr)
	if !ok {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	return selector.Sel.Name, true
}
