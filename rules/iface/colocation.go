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

	type ifaceInfo struct {
		relPath   string
		pkg       string
		line      int
		component string
	}
	// Use a slice per name to handle the same interface name in multiple packages.
	ifaceMap := make(map[string][]ifaceInfo)

	type structInfo struct {
		relPath   string
		pkg       string
		name      string
		component string
	}
	var structs []structInfo

	// Track which packages have any exported interface (including type aliases).
	// A struct that already has a same-package interface is likely correctly colocated.
	pkgHasIface := make(map[string]map[string]bool) // pkg → set of interface names

	// First pass: collect interfaces and structs.
	err := ctx.Analyzer.WalkGoFiles(func(path string, file *lint.ParsedFile) error {
		for _, td := range file.Types {
			if td.IsInterface && td.Exported {
				ifaceMap[td.Name] = append(ifaceMap[td.Name], ifaceInfo{
					relPath:   file.RelPath,
					pkg:       file.Package,
					line:      td.Pos.Line,
					component: file.Location.Component,
				})
				if pkgHasIface[file.Package] == nil {
					pkgHasIface[file.Package] = make(map[string]bool)
				}
				pkgHasIface[file.Package][td.Name] = true
			}
			// Track type aliases to exported types as potential interfaces.
			// type Client = otherPkg.Client → treated as interface in this package.
			if td.Exported && !td.IsInterface && !td.IsStruct && td.IsAlias {
				if pkgHasIface[file.Package] == nil {
					pkgHasIface[file.Package] = make(map[string]bool)
				}
				pkgHasIface[file.Package][td.Name] = true
			}
			if td.IsStruct && !td.Exported {
				structs = append(structs, structInfo{
					relPath:   file.RelPath,
					pkg:       file.Package,
					name:      td.Name,
					component: file.Location.Component,
				})
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Build a set of (package, interface-name) pairs for quick lookup.
	type pkgIface struct {
		pkg       string
		ifaceName string
	}
	samePkgIfaces := make(map[pkgIface]bool)
	for ifaceName, infos := range ifaceMap {
		for _, info := range infos {
			samePkgIfaces[pkgIface{pkg: info.pkg, ifaceName: ifaceName}] = true
		}
	}

	// Second pass: check colocation.
	for _, st := range structs {
		for ifaceName, infos := range ifaceMap {
			if !implMatchesIface(st.name, ifaceName) {
				continue
			}

			// If the struct name ends with "Impl" (e.g., "extensionImpl") and a more
			// specific interface exists in the same package (e.g., "ExtensionImpl"),
			// skip this less-specific match (e.g., "Extension").
			if strings.HasSuffix(st.name, "Impl") {
				specificIface := strings.ToUpper(st.name[:1]) + st.name[1:] // "ExtensionImpl"
				if specificIface != ifaceName && samePkgIfaces[pkgIface{pkg: st.pkg, ifaceName: specificIface}] {
					continue
				}
			}

			// If the struct's package already has an interface or type alias with the
			// matching name, the struct is already colocated — skip.
			upperName := strings.ToUpper(st.name[:1]) + st.name[1:]
			if pkgIfaces, ok := pkgHasIface[st.pkg]; ok && pkgIfaces[upperName] {
				continue
			}

			// Skip structs whose name matches the package name (e.g., `outbox.outbox`).
			// These typically implement a different interface in the same package.
			if st.name == st.pkg {
				continue
			}

			// Find the best matching interface: prefer same package, then same component.
			var matched *ifaceInfo
			for i := range infos {
				info := &infos[i]
				if info.pkg == st.pkg {
					// Already colocated — no violation.
					matched = nil
					break
				}
				if info.component == st.component && matched == nil {
					matched = info
				}
			}
			if matched == nil {
				continue
			}

			switch scope {
			case "file":
				if st.relPath != matched.relPath {
					ctx.Report.Add(lint.Violation{
						Rule:     "interface/colocation",
						Severity: ctx.Severity,
						File:     st.relPath,
						Line:     1,
						Message:  fmt.Sprintf("implementation %q should be in the same file as interface %q", st.name, ifaceName),
						Found:    st.relPath,
						Expected: matched.relPath,
					})
				}
			default: // "package"
				ctx.Report.Add(lint.Violation{
					Rule:     "interface/colocation",
					Severity: ctx.Severity,
					File:     st.relPath,
					Line:     1,
					Message:  fmt.Sprintf("implementation %q should be in the same package as interface %q", st.name, ifaceName),
					Found:    st.pkg,
					Expected: matched.pkg,
				})
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
