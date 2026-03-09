package structure

import (
	"fmt"
	"go/ast"
	"go/token"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&RequiredDeclarations{})
}

// RequiredDeclarations validates that specific files contain required declarations.
// This generalizes checks like "alias.go must export a Public Service type alias".
//
// Options:
//
//	files: map of filename -> requirements object:
//	  tag: string - location tag to filter on (e.g., "isAlias"); if set, only checks matching files
//	  tag_value: string - expected tag value (default: "true")
//	  required_aliases: []string - at least one of these type alias names must exist
//	  required_types: []string - all of these type names must exist
type RequiredDeclarations struct{}

func (r *RequiredDeclarations) Meta() lint.Meta {
	return lint.Meta{
		Name:        "structure/required-declarations",
		Description: "Validate that specific files contain required declarations",
		Category:    "structure",
		Tier:        lint.TierComponentAware,
	}
}

func (r *RequiredDeclarations) Check(ctx *lint.Context) error {
	filesRaw := ctx.Options.Map("files")
	if len(filesRaw) == 0 {
		return nil
	}

	type fileReq struct {
		tag             string
		tagValue        string
		requiredAliases []string
		requiredTypes   []string
	}

	requirements := make(map[string]*fileReq, len(filesRaw))
	for filename, reqRaw := range filesRaw {
		reqMap, ok := reqRaw.(map[string]any)
		if !ok {
			continue
		}
		req := &fileReq{}
		if t, ok := reqMap["tag"].(string); ok {
			req.tag = t
		}
		if tv, ok := reqMap["tag_value"].(string); ok {
			req.tagValue = tv
		}
		if req.tagValue == "" {
			req.tagValue = "true"
		}

		switch v := reqMap["required_aliases"].(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					req.requiredAliases = append(req.requiredAliases, s)
				}
			}
		case []string:
			req.requiredAliases = v
		}

		switch v := reqMap["required_types"].(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					req.requiredTypes = append(req.requiredTypes, s)
				}
			}
		case []string:
			req.requiredTypes = v
		}

		requirements[filename] = req
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for filename, req := range requirements {
			// Match by tag or by filename
			if req.tag != "" {
				if file.Location.Tag(req.tag) != req.tagValue {
					continue
				}
			} else {
				// Match by filename
				if file.RelPath != filename && !matchesFilename(file.RelPath, filename) {
					continue
				}
			}

			// Check required_aliases: at least one must exist
			if len(req.requiredAliases) > 0 {
				if !hasAnyTypeAlias(file.AST, req.requiredAliases) {
					ctx.Report.Add(lint.Violation{
						Rule:     "structure/required-declarations",
						Severity: ctx.Severity,
						File:     file.RelPath,
						Line:     1,
						Message:  fmt.Sprintf("file must export at least one of these type aliases: %v", req.requiredAliases),
						Expected: fmt.Sprintf("type alias: one of %v", req.requiredAliases),
					})
				}
			}

			// Check required_types: all must exist
			if len(req.requiredTypes) > 0 {
				typeNames := collectTypeNames(file.AST)
				for _, rt := range req.requiredTypes {
					if !typeNames[rt] {
						ctx.Report.Add(lint.Violation{
							Rule:     "structure/required-declarations",
							Severity: ctx.Severity,
							File:     file.RelPath,
							Line:     1,
							Message:  fmt.Sprintf("file must declare type %q", rt),
							Expected: rt,
						})
					}
				}
			}
		}
		return nil
	})
}

// matchesFilename checks if a relative path ends with the given filename.
func matchesFilename(relPath, filename string) bool {
	if len(relPath) < len(filename) {
		return false
	}
	// Check if relPath ends with /filename or equals filename
	if relPath == filename {
		return true
	}
	suffix := "/" + filename
	return len(relPath) >= len(suffix) && relPath[len(relPath)-len(suffix):] == suffix
}

// hasAnyTypeAlias checks if the AST contains a type alias with any of the given names.
func hasAnyTypeAlias(f *ast.File, names []string) bool {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if nameSet[ts.Name.Name] && ts.Assign.IsValid() {
				return true
			}
		}
	}
	return false
}

// collectTypeNames returns a set of all type names declared in the file.
func collectTypeNames(f *ast.File) map[string]bool {
	names := make(map[string]bool)
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			names[ts.Name.Name] = true
		}
	}
	return names
}
