package iface

import (
	"fmt"
	"strings"

	lint "github.com/channel-io/cht-go-lint"
)

func init() {
	lint.Register(&Colocation{})
}

// Colocation checks that an interface and its implementation are in the same file or package.
type Colocation struct{}

func (r *Colocation) Meta() lint.Meta {
	return lint.Meta{
		Name:        "interface/colocation",
		Description: "Interface and its implementation should be co-located",
		Category:    "interface",
		Tier:        lint.TierUniversal,
	}
}

func (r *Colocation) Check(ctx *lint.Context) error {
	scope := ctx.Options.String("scope", "package")

	// ifaceFile maps interface name to its file's RelPath.
	type ifaceInfo struct {
		relPath string
		pkg     string
		line    int
	}
	ifaceMap := make(map[string]ifaceInfo)

	// structFile maps potential impl struct names to their file info.
	type structInfo struct {
		relPath string
		pkg     string
		name    string
	}
	var structs []structInfo

	// First pass: collect interfaces and structs.
	err := ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, td := range file.Types {
			if td.IsInterface && td.Exported {
				ifaceMap[td.Name] = ifaceInfo{
					relPath: file.RelPath,
					pkg:     file.Package,
					line:    td.Pos.Line,
				}
			}
			if td.IsStruct && !td.Exported {
				structs = append(structs, structInfo{
					relPath: file.RelPath,
					pkg:     file.Package,
					name:    td.Name,
				})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Second pass: check colocation.
	for _, st := range structs {
		for ifaceName, info := range ifaceMap {
			if !implMatchesIface(st.name, ifaceName) {
				continue
			}

			switch scope {
			case "file":
				if st.relPath != info.relPath {
					ctx.Report.Add(lint.Violation{
						Rule:     "interface/colocation",
						Severity: ctx.Severity,
						File:     st.relPath,
						Line:     1,
						Message:  fmt.Sprintf("implementation %q should be in the same file as interface %q", st.name, ifaceName),
						Found:    st.relPath,
						Expected: info.relPath,
					})
				}
			default: // "package"
				if st.pkg != info.pkg {
					ctx.Report.Add(lint.Violation{
						Rule:     "interface/colocation",
						Severity: ctx.Severity,
						File:     st.relPath,
						Line:     1,
						Message:  fmt.Sprintf("implementation %q should be in the same package as interface %q", st.name, ifaceName),
						Found:    st.pkg,
						Expected: info.pkg,
					})
				}
			}
		}
	}

	return nil
}

// implMatchesIface returns true if structName could be an implementation of ifaceName.
// Matches: "handler" for "Handler", "handlerImpl" for "Handler".
func implMatchesIface(structName, ifaceName string) bool {
	lower := strings.ToLower(ifaceName[:1]) + ifaceName[1:]
	return structName == lower || structName == lower+"Impl"
}
