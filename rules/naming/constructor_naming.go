package naming

import (
	"fmt"
	"path/filepath"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&ConstructorNaming{})
}

// ConstructorNaming checks that constructor functions return the type they construct.
type ConstructorNaming struct{}

func (r *ConstructorNaming) Meta() lint.Meta {
	return lint.Meta{
		Name:        "naming/constructor-naming",
		Description: "Constructor functions (New*) should return the type they construct",
		Category:    "naming",
		Tier:        lint.TierUniversal,
	}
}

func (r *ConstructorNaming) Check(ctx *lint.Context) error {
	requireIfaceReturn := ctx.Options.Bool("require_interface_return", false)
	forbiddenReturnNames := ctx.Options.StringSlice("forbidden_return_names")
	skipFiles := make(map[string]bool)
	for _, f := range ctx.Options.StringSlice("skip_files") {
		skipFiles[f] = true
	}

	return ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		if skipFiles[filepath.Base(file.RelPath)] {
			return nil
		}
		// Build a set of interface names in this file for require_interface_return check.
		ifaceNames := make(map[string]bool)
		if requireIfaceReturn {
			for _, td := range file.Types {
				if td.IsInterface {
					ifaceNames[td.Name] = true
				}
			}
		}

		for _, fd := range file.Funcs {
			if !fd.IsConstructor || fd.ReceiverType != "" {
				continue
			}

			typeName := fd.Name[3:] // strip "New"
			if typeName == "" {
				continue
			}

			if !returnsType(fd.ReturnTypes, typeName) {
				ctx.Report.Add(lint.Violation{
					Rule:     "naming/constructor-naming",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     fd.Pos.Line,
					Message:  fmt.Sprintf("constructor %q should return type %q", fd.Name, typeName),
					Found:    strings.Join(fd.ReturnTypes, ", "),
					Expected: typeName,
				})
				continue
			}

			// Check for forbidden generic return type names (e.g., Handler, Svc, Repo)
			if len(forbiddenReturnNames) > 0 {
				for _, rt := range fd.ReturnTypes {
					clean := strings.TrimPrefix(rt, "*")
					for _, forbidden := range forbiddenReturnNames {
						if clean == forbidden {
							ctx.Report.Add(lint.Violation{
								Rule:     "naming/constructor-naming",
								Severity: ctx.Severity,
								File:     file.RelPath,
								Line:     fd.Pos.Line,
								Message:  fmt.Sprintf("constructor %q returns generic type name %q; use a more specific name", fd.Name, clean),
								Found:    clean,
							})
						}
					}
				}
			}

			if requireIfaceReturn && !returnsInterface(fd.ReturnTypes, ifaceNames) {
				ctx.Report.Add(lint.Violation{
					Rule:     "naming/constructor-naming",
					Severity: ctx.Severity,
					File:     file.RelPath,
					Line:     fd.Pos.Line,
					Message:  fmt.Sprintf("constructor %q should return an interface type, not a concrete struct", fd.Name),
					Found:    strings.Join(fd.ReturnTypes, ", "),
				})
			}
		}
		return nil
	})
}

func returnsType(returnTypes []string, typeName string) bool {
	for _, rt := range returnTypes {
		clean := strings.TrimPrefix(rt, "*")
		if clean == typeName {
			return true
		}
	}
	return false
}

func returnsInterface(returnTypes []string, ifaceNames map[string]bool) bool {
	for _, rt := range returnTypes {
		clean := strings.TrimPrefix(rt, "*")
		if ifaceNames[clean] {
			return true
		}
	}
	return false
}
